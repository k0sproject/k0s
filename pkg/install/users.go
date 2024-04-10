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
	"os/user"
	"slices"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
)

// CreateControllerUsers accepts a cluster config, and cfgVars and creates controller users accordingly
func CreateControllerUsers(clusterConfig *v1beta1.ClusterConfig, k0sVars *config.CfgVars) error {
	var errs []error
	for _, userName := range getControllerUserNames(clusterConfig.Spec.Install.SystemUsers) {
		if err := EnsureUser(userName, k0sVars.DataDir); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// CreateControllerUsers accepts a cluster config, and cfgVars and creates controller users accordingly
func DeleteControllerUsers(clusterConfig *v1beta1.ClusterConfig) error {
	var errs []error
	for _, userName := range getControllerUserNames(clusterConfig.Spec.Install.SystemUsers) {
		if _, err := users.GetUID(userName); err == nil {
			logrus.Debugf("Deleting user %q", userName)

			if err := deleteUser(userName); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}

// EnsureUser checks if a user exists, and creates it, if it doesn't
// TODO: we should also consider modifying the user, if the user exists, but with wrong settings
func EnsureUser(name string, homeDir string) error {
	_, err := users.GetUID(name)
	if errors.Is(err, user.UnknownUserError(name)) {
		logrus.Infof("Creating user %q", name)
		return createUser(name, homeDir)
	}
	return err
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
func createUser(userName string, homeDir string) error {
	shell, err := nologinShell()
	if err != nil {
		return err
	}

	_, err = exec.Command("useradd", `--home`, homeDir, `--shell`, shell, `--system`, `--no-create-home`, userName).Output()
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
