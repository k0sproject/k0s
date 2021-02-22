/*
Copyright 2021 k0s authors

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
package cmd

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/install"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	installCmd.AddCommand(installControllerCmd)
	installCmd.AddCommand(installWorkerCmd)

	addPersistentFlags(installControllerCmd)
	addPersistentFlags(installWorkerCmd)
}

var (
	installCmd = &cobra.Command{
		Use:   "install",
		Short: "Helper command for setting up k0s on a brand-new system. Must be run as root (or with sudo)",
	}
)

// the setup functions:
// * Ensures that the proper users are created
// * sets up startup and logging for k0s
func setup(role string, args []string) error {
	if os.Geteuid() != 0 {
		logrus.Fatal("this command must be run as root!")
	}

	if role == "controller" {
		if err := createControllerUsers(); err != nil {
			logrus.Errorf("failed to create controller users: %v", err)
		}
	}

	err := install.EnsureService(args)
	if err != nil {
		logrus.Errorf("failed to install k0s service: %v", err)
	}
	return nil
}

func createControllerUsers() error {
	clusterConfig, err := ConfigFromYaml(cfgFile)
	if err != nil {
		return err
	}

	users := getUserList(*clusterConfig.Install.SystemUsers)

	var messages []string
	for _, v := range users {
		if err := install.EnsureUser(v, k0sVars.DataDir); err != nil {
			messages = append(messages, err.Error())
		}
	}

	if len(messages) > 0 {
		return fmt.Errorf(strings.Join(messages, "\n"))
	}
	return nil
}

func getUserList(sysUsers v1beta1.SystemUser) []string {
	v := reflect.ValueOf(sysUsers)
	values := make([]string, v.NumField())

	for i := 0; i < v.NumField(); i++ {
		values[i] = v.Field(i).String()
	}
	return values
}
