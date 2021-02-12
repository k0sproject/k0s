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
	"fmt"
	"os"

	"github.com/spf13/cobra"
	kubectl "k8s.io/kubectl/pkg/cmd"
)

var (
	kubectlCmd     = kubectl.NewKubectlCommand(os.Stdin, os.Stdout, os.Stderr)
	kubectlWrapCmd = &cobra.Command{
		Use:   "kc",
		Short: kubectlCmd.Short,
		Long:  kubectlCmd.Long,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(os.Args, args)
			if len(args) == 0 {
				kubectlCmd.Help()
				return
			}
			os.Args = os.Args[2:]
			os.Args[0] = "kubectl"
			fmt.Println(os.Args)
			kubectlCmd.SetArgs(os.Args[1:])
			kubectlCmd.Execute()
		},
	}
)

func init() {
	kubectlWrapCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		kubectlCmd.Help()
	})
}