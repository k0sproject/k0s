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
	"os/exec"
	"os/user"
	"strconv"
	"strings"
)

// GetUID returns uid of given username and logs a warning if its missing
func GetUID(name string) (int, error) {
	entry, err := user.Lookup(name)
	if err == nil {
		return strconv.Atoi(entry.Uid)
	}
	if errors.Is(err, user.UnknownUserError(name)) {
		// fallback to call external `id` in case NSS is used
		out, err := exec.Command("/usr/bin/id", "-u", name).CombinedOutput()
		if err == nil {
			return strconv.Atoi(strings.TrimSuffix(string(out), "\n"))
		}
	}
	return 0, err
}
