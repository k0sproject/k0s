//go:build linux

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// Sends a signal to the process.
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
// Since Linux 5.1.
// https://man7.org/linux/man-pages/man2/pidfd_send_signal.2.html
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
