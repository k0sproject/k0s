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
package cmd

import (
	"os"

	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/sirupsen/logrus"
	kubectl "k8s.io/kubectl/pkg/cmd"
)

var (
	kubectlCmd = kubectl.NewDefaultKubectlCommand()
)

func init() {
	//Get relevant Vars from constant package
	k0sVars = constant.GetConfig(dataDir)
	kubenv := os.Getenv("KUBECONFIG")
	if kubenv == "" {
		// Verify we can read the config before pushing it to env
		file, err := os.OpenFile(k0sVars.AdminKubeConfigPath, os.O_RDONLY, 0600)
		if err != nil {
			logrus.Errorf("cannot read admin kubeconfig at %s, is the server running?", k0sVars.AdminKubeConfigPath)
			return
		}
		file.Close()
		os.Setenv("KUBECONFIG", k0sVars.AdminKubeConfigPath)
	}
}
