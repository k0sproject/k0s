/*
Copyright 2021 k0s Authors

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
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"reflect"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/internal/pkg/stringslice"
	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
)

func GetControllerUsers(clusterConfig *v1beta1.ClusterConfig) []string {
	return getUserList(*clusterConfig.Spec.Install.SystemUsers)
}

// CreateControllerUsers accepts a cluster config, and cfgVars and creates controller users accordingly
func CreateControllerUsers(clusterConfig *v1beta1.ClusterConfig, k0sVars constant.CfgVars) error {
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
		if exists, _ := users.CheckIfUserExists(v); exists {
			if err := DeleteUser(v); err != nil {
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
	shell, err := file.GetExecPath("nologin")
	if err != nil {
		return err
	}

	exists, err := users.CheckIfUserExists(name)
	// User doesn't exist
	if !exists && err == nil {
		// Create the User
		if err := CreateUser(name, homeDir, *shell); err != nil {
			return err
		}
		// User perhaps exists, but cannot be fetched
	} else if err != nil {
		return err
	}
	// verify that user can be fetched, and exists
	_, err = user.Lookup(name)
	if err != nil {
		return err
	}
	return nil
}

// CreateUser creates a system user with either `adduser` or `useradd` command
func CreateUser(userName string, homeDir string, shell string) error {
	var userCmd string
	var userCmdArgs []string

	logrus.Infof("creating user: %s", userName)
	_, err := file.GetExecPath("useradd")
	if err == nil {
		userCmd = "useradd"
		userCmdArgs = []string{`--home`, homeDir, `--shell`, shell, `--system`, `--no-create-home`, userName}
	} else {
		userCmd = "adduser"
		userCmdArgs = []string{`--disabled-password`, `--gecos`, `""`, `--home`, homeDir, `--shell`, shell, `--system`, `--no-create-home`, userName}
	}

	cmd := exec.Command(userCmd, userCmdArgs...)
	if err := execCmd(cmd); err != nil {
		return err
	}
	return nil
}

// DeleteUser deletes system users with either `deluser` or `userdel` command
func DeleteUser(userName string) error {
	var userCmd string
	var userCmdArgs []string

	logrus.Debugf("deleting user: %s", userName)
	_, err := file.GetExecPath("userdel")
	if err == nil {
		userCmd = "userdel"
		userCmdArgs = []string{userName}
	} else {
		userCmd = "deluser"
		userCmdArgs = []string{userName}
	}

	cmd := exec.Command(userCmd, userCmdArgs...)
	if err := execCmd(cmd); err != nil {
		return err
	}
	return nil
}

// cmd wrapper
func execCmd(cmd *exec.Cmd) error {
	logrus.Debugf("executing command: %v", quoteCmd(cmd))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run command %s: %v", quoteCmd(cmd), err)
	}
	return nil
}

// parse a cmd struct to string
func quoteCmd(cmd *exec.Cmd) string {
	if len(cmd.Args) == 0 {
		return fmt.Sprintf("%q", cmd.Path)
	}

	var q []string
	for _, s := range cmd.Args {
		q = append(q, fmt.Sprintf("%q", s))
	}
	return strings.Join(q, ` `)
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
