/*
Copyright 2020 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
