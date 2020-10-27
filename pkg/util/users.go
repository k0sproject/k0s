/*
Copyright 2020 Mirantis, Inc.

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
package util

import (
	"os/user"
	"strconv"
)

// GetUID returns uid of given username and logs a warning if its missing
func GetUID(name string) (int, error) {
	entry, err := user.Lookup(name)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(entry.Uid)
}

// GetGID returns gid of given groupname and logs a warning if its missing
func GetGID(name string) (int, error) {
	entry, err := user.LookupGroup(name)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(entry.Gid)
}
