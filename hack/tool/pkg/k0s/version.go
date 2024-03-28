/*
Copyright 2022 k0s authors

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
package k0s

import (
	"os/exec"
	"strings"
)

// Version returns the version of the k0s binary at the provided path.
func Version(k0sBinaryPath string) (string, error) {
	out, err := exec.Command(k0sBinaryPath, "version").Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}
