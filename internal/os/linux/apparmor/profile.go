//go:build linux

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package apparmor

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unicode"

	pathrs "github.com/cyphar/filepath-securejoin/pathrs-lite"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/procfs"
	"golang.org/x/sys/unix"
)

// An AppArmor profile that is loaded in the kernel.
type Profile struct {
	name string
}

// Checks that the name can refer to a single loaded profile. It may not be
// empty, may not contain whitespace or control characters, and may not be a
// stack of profiles.
func ValidateProfileReference(name string) error {
	if name == "" {
		return errors.New("may not be empty")
	}
	// A stack ("a//&b") is a label composed of several profiles.
	// It's not a profile on its own, so it can't be looked up as one.
	if strings.Contains(name, "//&") {
		return errors.New("stacks of profiles are not supported")
	}
	// If not used in an unquoted context, things like path-based
	// names, child profiles ("parent//child") and policy namespaces
	// (":ns:name") are okay.
	for i, r := range name {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return fmt.Errorf("invalid character %q at offset %d", r, i)
		}
	}
	return nil
}

// Checks that the name is safe to use as an unquoted profile name in policy
// text and in labels. In addition to what [ValidateProfileReference] checks, it
// must consist of ASCII letters, digits, dots, underscores and dashes, and
// start with a letter or digit.
func ValidateProfileName(name string) error {
	if name == "" {
		return errors.New("may not be empty")
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case i > 0 && (r == '.' || r == '_' || r == '-'):
		default:
			return fmt.Errorf("invalid character %q at offset %d", r, i)
		}
	}
	return nil
}

// Compiles the given profile text and loads it into the kernel, replacing an
// already loaded profile of the same name. The name must match the one
// declared in the text and satisfy [ValidateProfileName]. The returned error
// matches [errors.ErrUnsupported] if the machine lacks any prerequisites.
func ReplaceProfile(ctx context.Context, name string, text io.Reader) (*Profile, error) {
	if err := ValidateProfileName(name); err != nil {
		return nil, fmt.Errorf("invalid profile name: %w", err)
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx,
		"/sbin/apparmor_parser", // https://github.com/containerd/containerd/blob/v2.3.6/pkg/apparmor/apparmor_linux.go#L38
		"--replace",
		"--quiet",      // don't emit warnings, stdout is discarded anyways
		"--skip-cache", // profiles read from stdin can't be cached
	)
	cmd.Stdin, cmd.Stderr = text, &stderr
	if err := cmd.Start(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &notFoundError{"parser", err}
		}
		return nil, err
	}
	if err := cmd.Wait(); err != nil {
		stderr := bytes.TrimSpace(stderr.Bytes())
		// The parser loads profiles through the kernel interface.
		if _, statErr := os.Stat("/sys/kernel/security/apparmor"); errors.Is(statErr, os.ErrNotExist) {
			if len(stderr) > 0 {
				err = errors.New(string(stderr))
			}
			return nil, &notFoundError{"kernel interface", fmt.Errorf("parser error: %w", err)}
		}
		if len(stderr) > 0 {
			err = fmt.Errorf("%w: %s", err, stderr)
		}
		return nil, err
	}

	// The parser loads whatever the text declares, so make sure that the
	// profile the caller asked for is among it.
	return LoadedProfile(name)
}

// Looks up a profile that is already loaded in the kernel. The name must
// satisfy [ValidateProfileReference]. The returned error matches
// [errors.ErrUnsupported] if the machine lacks any prerequisites.
func LoadedProfile(name string) (_ *Profile, err error) {
	if err := ValidateProfileReference(name); err != nil {
		return nil, fmt.Errorf("invalid profile name: %w", err)
	}

	// https://gitlab.com/apparmor/apparmor/-/wikis/AppArmorInterfaces#syskernelsecurityapparmorprofiles
	// Each line reads "<name> (<mode>)".
	profiles, err := os.Open("/sys/kernel/security/apparmor/profiles")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = &notFoundError{"kernel interface", err}
		}
		return nil, fmt.Errorf("failed to determine if profile is loaded: %w", err)
	}
	defer func() { err = errors.Join(err, profiles.Close()) }()
	lines := bufio.NewScanner(profiles)
	for lines.Scan() {
		if loaded, _, _ := bytes.Cut(lines.Bytes(), []byte(" (")); string(loaded) == name {
			return &Profile{name}, nil
		}
	}
	if err := lines.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate loaded profile names: %w", err)
	}

	return nil, errors.New("profile is not loaded")
}

// Indicates that a transition into an AppArmor profile couldn't be requested.
var ErrExecTransitionRequest = errors.New("failed to request AppArmor exec transition")

// Starts the command so that its process is confined by the profile. If the
// transition can't be requested, the command isn't started and the returned
// error wraps [ErrExecTransitionRequest].
func (p *Profile) Start(cmd *exec.Cmd) error {
	errCh := make(chan error, 1)
	go func() {
		// Exec transitions are requested per thread and consumed by the next
		// exec performed by that thread or by a process forked from it. Lock
		// the goroutine to its thread and never unlock it so that Go will
		// terminate the thread when the goroutine exits. This ensures that any
		// modified thread state dies along with it.
		runtime.LockOSThread()

		if closer, err := func() (closer procfs.ProcThreadSelfCloser, err error) {
			proc, err := procfs.OpenProcRoot()
			if err != nil {
				return nil, err
			}
			defer func() { err = errors.Join(err, proc.Close()) }()

			execHandle, execHandleDone, err := proc.OpenThreadSelf("attr/apparmor/exec")
			// Kernels before 4.17 don't have the LSM-specific attr subdirectory
			if errors.Is(err, os.ErrNotExist) {
				execHandle, execHandleDone, err = proc.OpenThreadSelf("attr/exec")
			}
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return nil, errors.New("the proc filesystem exposes no AppArmor exec attribute for the calling thread")
				}
				return nil, err
			}
			defer func() {
				if err != nil {
					execHandleDone()
					closer = nil
				}
			}()
			defer func() { err = errors.Join(err, execHandle.Close()) }()

			// OpenThreadSelf returns an O_PATH handle that can't be written to.
			execWriteHandle, err := pathrs.Reopen(execHandle, unix.O_WRONLY|unix.O_CLOEXEC)
			if err != nil {
				return nil, err
			}
			defer func() { err = errors.Join(err, execWriteHandle.Close()) }()

			if _, err = execWriteHandle.WriteString("exec " + p.name); err != nil {
				return nil, err
			}

			return execHandleDone, nil
		}(); err != nil {
			errCh <- fmt.Errorf("%w: %w", ErrExecTransitionRequest, err)
		} else {
			defer closer()
			errCh <- cmd.Start()
		}
	}()
	return <-errCh
}

type notFoundError struct {
	subject string
	inner   error
}

func (e *notFoundError) Error() string {
	return e.subject + " not found: " + e.inner.Error()
}

func (*notFoundError) Is(other error) bool {
	return other == errors.ErrUnsupported
}
