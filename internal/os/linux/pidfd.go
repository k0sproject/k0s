//go:build linux

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

type PIDFD struct {
	fd int
}

// Opens the process with the given PID.
//
// Since Linux 5.3. See [man 2 pidfd_open].
//
// [man 2 pidfd_open]: https://www.man7.org/linux/man-pages/man2/pidfd_open.2.html
func OpenProcess(pid int) (*PIDFD, error) {
	// https://www.man7.org/linux/man-pages/man2/pidfd_open.2.html
	fd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		return nil, os.NewSyscallError("pidfd_open", err)
	}
	return &PIDFD{fd}, nil
}

func (p *PIDFD) Close() error {
	return os.NewSyscallError("close", unix.Close(p.fd))
}

// Sends a signal to the process.
//
// See [man 2 pidfd_send_signal].
//
// [man 2 pidfd_send_signal]: https://man7.org/linux/man-pages/man2/pidfd_send_signal.2.html
func (p *PIDFD) SendSignal(signal os.Signal) error {
	sig, ok := signal.(syscall.Signal)
	if !ok {
		return fmt.Errorf("%w: %s", errors.ErrUnsupported, signal)
	}

	return pidfdSendSignal(p.fd, sig)
}

// Waits for the process to terminate.
func (p *PIDFD) Wait(ctx context.Context) (err error) {
	// Setup an eventfd object to wake up the poll call from a goroutine when
	// the context gets canceled.
	// https://www.man7.org/linux/man-pages/man2/eventfd.2.html
	eventFD, err := unix.Eventfd(0, unix.EFD_CLOEXEC)
	if err != nil {
		return os.NewSyscallError("eventfd", err)
	}
	defer func() { err = errors.Join(err, os.NewSyscallError("close", unix.Close(eventFD))) }()

	exit, done := make(chan struct{}), make(chan error, 1)
	go func() {
		defer close(done)
		select {
		case <-ctx.Done():
			// eventfds accept an uint64 between 0 and 2^64-1.
			one := [8]byte{7: 1}
			_, err := unix.Write(eventFD, one[:])
			done <- os.NewSyscallError("write", err)
		case <-exit:
		}
	}()
	defer func() {
		close(exit)
		if doneErr := <-done; doneErr != nil {
			err = errors.Join(err, doneErr)
		}
	}()

	for {
		// https://www.man7.org/linux/man-pages/man2/poll.2.html
		fds := [2]unix.PollFd{
			{Fd: int32(p.fd), Events: unix.POLLIN},
			{Fd: int32(eventFD), Events: unix.POLLIN},
		}
		_, err := unix.Poll(fds[:], -1)

		switch {
		case errors.Is(err, syscall.EINTR):
			continue

		case err != nil:
			return os.NewSyscallError("poll", err)

		case fds[0].Revents&unix.POLLIN != 0:
			return nil // the process has terminated

		case fds[1].Revents&unix.POLLIN != 0:
			return context.Cause(ctx) // the context has been canceled

		default:
			return fmt.Errorf("woke up unexpectedly (0x%x / 0x%x)", fds[0].Revents, fds[1].Revents)
		}
	}
}

// Sends a signal to the process.
//
// Since Linux 5.1. See [man 2 pidfd_send_signal].
//
// [man 2 pidfd_send_signal]: https://man7.org/linux/man-pages/man2/pidfd_send_signal.2.html
func SendSignal(pidfd syscall.Conn, signal os.Signal) error {
	sig, ok := signal.(syscall.Signal)
	if !ok {
		return fmt.Errorf("%w: %s", errors.ErrUnsupported, signal)
	}

	conn, err := pidfd.SyscallConn()
	if err != nil {
		return err
	}

	outerErr := conn.Control(func(fd uintptr) {
		err = pidfdSendSignal(int(fd), sig)
	})

	return cmp.Or(err, outerErr)
}

// Send a signal to a process specified by a file descriptor.
//
// The calling process must either be in the same PID namespace as the process
// referred to by pidfd, or be in an ancestor of that namespace.
//
// https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git/commit/?h=3eb39f47934f9d5a3027fe00d906a45fe3a15fad
func pidfdSendSignal(pidfd int, sig syscall.Signal) error {
	// If the info argument is a NULL pointer, this is equivalent to specifying
	// a pointer to a siginfo_t buffer whose fields match the values that are
	// implicitly supplied when a signal is sent using kill(2):
	//
	//   * si_signo is set to the signal number;
	//   * si_errno is set to 0;
	//   * si_code is set to SI_USER;
	//   * si_pid is set to the caller's PID; and
	//   * si_uid is set to the caller's real user ID.
	info := (*unix.Siginfo)(nil)

	// The flags argument is reserved for future use; currently, this
	// argument must be specified as 0.
	flags := 0

	return os.NewSyscallError("pidfd_send_signal", unix.PidfdSendSignal(pidfd, sig, info, flags))
}
