// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"errors"
)

const (
	// An unknown (i.e. invalid) user ID. It's distinct from a UID's zero value,
	// which happens to be [RootUID]. Assuming root may or may not be a good
	// default, depending on the use case. Setting file ownership to root is a
	// restrictive and safe default, running programs as root is not. Therefore,
	// this is the preferred return value for UIDs in case of error; callers
	// must then explicitly decide on the fallback instead of accidentally
	// assuming root.
	UnknownUID = -1

	RootUID = 0 // User ID of the root user
)

var ErrNotExist = errors.New("user does not exist")
