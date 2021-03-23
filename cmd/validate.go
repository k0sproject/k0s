/*
Copyright 2021 Mirantis, Inc.

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
	"strings"

	"github.com/spf13/cobra"

	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
)

var (
	validateCmd = &cobra.Command{
		Use:   "validate",
		Short: "Helper command for validating the config file",
	}
	validateConfigCmd = &cobra.Command{
		Use:   "config",
		Short: "Helper command for validating the config file",
		Long: `Example:
   k0s validate config --config path_to_config.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := validateConfig(cfgFile)
			if err != nil {
				fmt.Println(err)
			}
			return nil
		},
	}
)

func init() {
	validateCmd.AddCommand(validateConfigCmd)
	addPersistentFlags(validateConfigCmd)
}

// XXX: This is a duplication of the code under cmd/helpers.go ConfigFromYaml function.
// XXX: we should fix this and remove the duplication
func validateConfig(cfgPath string) (err error) {
	var clusterConfig *config.ClusterConfig

	if cfgPath == "" {
		// no config file exists, using defaults
		clusterConfig = config.DefaultClusterConfig(k0sVars)
	} else if isInputFromPipe() {
		clusterConfig, err = config.FromYamlPipe(os.Stdin, k0sVars)
	} else {
		clusterConfig, err = config.FromYamlFile(cfgPath, k0sVars)
	}
	if err != nil {
		return err
	}

	errors := clusterConfig.Validate()
	if len(errors) > 0 {
		messages := make([]string, len(errors))
		for _, e := range errors {
			messages = append(messages, e.Error())
		}
		return fmt.Errorf(strings.Join(messages, "\n"))
	}
	return nil
}
