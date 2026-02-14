//go:build unix

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
)

// Lookup looks up a user's UID by username. If the user cannot be found, the
// returned error is [ErrNotExist]. If an error is returned, the returned UID
// will be [UnknownUID].
func LookupUID(name string) (int, error) {
	var uid string

	if entry, err := user.Lookup(name); err != nil {
		if !errors.Is(err, user.UnknownUserError(name)) {
			return UnknownUID, err
		}

		err = ErrNotExist

		// fallback to call external `id` in case NSS is used
		out, idErr := exec.Command("id", "-u", name).Output()
		if idErr != nil {
			var exitErr *exec.ExitError
			if errors.As(idErr, &exitErr) {
				return UnknownUID, fmt.Errorf("%w (%w: %s)", err, idErr, bytes.TrimSpace(exitErr.Stderr))
			}
			return UnknownUID, fmt.Errorf("%w (%w)", err, idErr)
		}

		uid = string(bytes.TrimSpace(out))
	} else {
		uid = entry.Uid
	}

	parsedUID, err := strconv.Atoi(uid)
	if err != nil {
		return UnknownUID, fmt.Errorf("UID %q is not a decimal integer: %w", uid, err)
	}
	if parsedUID < 0 {
		return UnknownUID, fmt.Errorf("UID is negative: %d", parsedUID)
	}

	return parsedUID, nil
}
