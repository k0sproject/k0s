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

package install

import (
	"errors"
	"os/exec"
	"slices"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

// Ensures that all controller users exist and creates any missing users with
// the given home directory.
func EnsureControllerUsers(systemUsers *v1beta1.SystemUser, homeDir string) error {
	var shell string
	var errs []error
	for _, userName := range getControllerUserNames(systemUsers) {
		_, err := users.LookupUID(userName)
		if errors.Is(err, users.ErrNotExist) {
			if shell == "" {
				shell, err = nologinShell()
				if err != nil {
					// error out early, k0s won't be able to create any users anyways
					errs = append(errs, err)
					break
				}
			}

			logrus.Infof("Creating user %q", userName)
			err = createUser(userName, homeDir, shell)
		}
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// Deletes existing controller users.
func DeleteControllerUsers(systemUsers *v1beta1.SystemUser) error {
	var errs []error
	for _, userName := range getControllerUserNames(systemUsers) {
		if _, err := users.LookupUID(userName); err == nil {
			logrus.Debugf("Deleting user %q", userName)

			if err := deleteUser(userName); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}

// nologinShell returns the path to /sbin/nologin, /bin/false or equivalent or an error if neither is available
func nologinShell() (string, error) {
	for _, p := range []string{"nologin", "false"} {
		if shell, err := exec.LookPath(p); err == nil {
			return shell, nil
		}
	}
	return "", errors.New("failed to locate a nologin shell for creating users")
}

// CreateUser creates a system user with either `adduser` or `useradd` command
func createUser(userName, homeDir, shell string) error {
	_, err := exec.Command("useradd", `--home`, homeDir, `--shell`, shell, `--system`, `--no-create-home`, userName).Output()
	if errors.Is(err, exec.ErrNotFound) {
		_, err = exec.Command("adduser", `--disabled-password`, `--gecos`, `""`, `--home`, homeDir, `--shell`, shell, `--system`, `--no-create-home`, userName).Output()
	}
	return err
}

// DeleteUser deletes system users with either `deluser` or `userdel` command
func deleteUser(userName string) error {
	_, err := exec.Command("userdel", userName).Output()
	if errors.Is(err, exec.ErrNotFound) {
		_, err = exec.Command("deluser", userName).Output()
	}
	return err
}

// Returns the controller user names.
func getControllerUserNames(users *v1beta1.SystemUser) []string {
	userNames := []string{
		users.Etcd,
		users.Kine,
		users.Konnectivity,
		users.KubeAPIServer,
		users.KubeScheduler,
	}

	slices.Sort(userNames)
	return slices.Compact(userNames)
}
