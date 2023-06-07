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
	"fmt"
	"os/exec"
	"os/user"
	"reflect"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/stringslice"
	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
)

func GetControllerUsers(clusterConfig *v1beta1.ClusterConfig) []string {
	return getUserList(*clusterConfig.Spec.Install.SystemUsers)
}

// CreateControllerUsers accepts a cluster config, and cfgVars and creates controller users accordingly
func CreateControllerUsers(clusterConfig *v1beta1.ClusterConfig, k0sVars *config.CfgVars) error {
	users := getUserList(*clusterConfig.Spec.Install.SystemUsers)
	var messages []string
	for _, v := range users {
		if err := EnsureUser(v, k0sVars.DataDir); err != nil {
			messages = append(messages, err.Error())
		}
	}
	if len(messages) > 0 {
		return fmt.Errorf(strings.Join(messages, "\n"))
	}
	return nil
}

// CreateControllerUsers accepts a cluster config, and cfgVars and creates controller users accordingly
func DeleteControllerUsers(clusterConfig *v1beta1.ClusterConfig) error {
	cfgUsers := getUserList(*clusterConfig.Spec.Install.SystemUsers)
	var messages []string
	for _, v := range cfgUsers {
		if _, err := users.GetUID(v); err == nil {
			logrus.Debugf("deleting user: %s", v)

			if err := deleteUser(v); err != nil {
				messages = append(messages, err.Error())
			}
		}
	}
	if len(messages) > 0 {
		// don't fail the command, just notify on errors
		return fmt.Errorf(strings.Join(messages, "\n"))
	}
	return nil
}

// EnsureUser checks if a user exists, and creates it, if it doesn't
// TODO: we should also consider modifying the user, if the user exists, but with wrong settings
func EnsureUser(name string, homeDir string) error {
	_, err := users.GetUID(name)
	if errors.Is(err, user.UnknownUserError(name)) {
		logrus.Infof("creating user: %s", name)
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

// get user list
func getUserList(sysUsers v1beta1.SystemUser) []string {
	v := reflect.ValueOf(sysUsers)
	values := make([]string, v.NumField())

	for i := 0; i < v.NumField(); i++ {
		values[i] = v.Field(i).String()
	}
	return stringslice.Unique(values)
}
