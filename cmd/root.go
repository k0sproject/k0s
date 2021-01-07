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
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra/doc"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/constant"
)

var (
	cfgFile       string
	dataDir       string
	debugListenOn string
	debug         bool
	cmdLogLevels  map[string]string
	logging       map[string]string
	k0sVars       constant.CfgVars
)

var defaultLogLevels = map[string]string{
	"etcd":                    "info",
	"containerd":              "info",
	"konnectivity-server":     "1",
	"kube-apiserver":          "1",
	"kube-controller-manager": "1",
	"kube-scheduler":          "1",
	"kubelet":                 "1",
	"kube-proxy":              "1",
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: ./k0s.yaml)")
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", "Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!")
	rootCmd.PersistentFlags().StringVar(&debugListenOn, "debugListenOn", ":6060", "Http listenOn for debug pprof handler")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Debug logging (default: false)")
	rootCmd.PersistentFlags().StringToStringVarP(&cmdLogLevels, "logging", "l", defaultLogLevels, "Logging Levels for the different components")

	// initialize configuration
	err := initConfig()
	if err != nil {
		fmt.Printf("err: %v", err)
	}

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(tokenCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(workerCmd)
	rootCmd.AddCommand(APICmd)
	rootCmd.AddCommand(etcdCmd)
	rootCmd.AddCommand(docs)
	rootCmd.AddCommand(kubeconfigCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(completionCmd)

	rootCmd.DisableAutoGenTag = true
	longDesc = "k0s - The zero friction Kubernetes - https://k0sproject.io"
	if build.EulaNotice != "" {
		longDesc = longDesc + "\n" + build.EulaNotice
	}

	rootCmd.Long = longDesc
}

var (
	longDesc string

	rootCmd = &cobra.Command{
		Use:   "k0s",
		Short: "k0s - Zero Friction Kubernetes",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// set DEBUG from env, or from command flag
			if viper.GetString("debug") != "" || debug {
				logrus.SetLevel(logrus.DebugLevel)
				go func() {
					log.Println("starting debug server under", debugListenOn)
					log.Println(http.ListenAndServe(debugListenOn, nil))
				}()
			}

			// Set logging
			logging = setLogging(cmdLogLevels)

			// Get relevant Vars from constant package
			k0sVars = constant.GetConfig(dataDir)
		},
	}

	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the k0s version",

		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(build.Version)
		},
	}

	docs = &cobra.Command{
		Use:   "docs",
		Short: "Generate Markdown docs for the k0s binary",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := generateDocs()
			if err != nil {
				return err
			}
			return nil
		},
	}
)

func initConfig() error {
	// look for k0s.yaml in PWD
	if cfgFile == "" {
		execFolderPath, err := os.Getwd()
		if err != nil {
			return err
		}
		cfgFile = filepath.Join(execFolderPath, "k0s.yaml")
	}

	// check if config file exists
	if util.FileExists(cfgFile) {
		viper.SetConfigFile(cfgFile)
		logrus.Debugf("Using config file: %v", cfgFile)
	}

	// Add env vars to Config
	viper.AutomaticEnv()

	return nil
}

func generateDocs() error {
	if err := doc.GenMarkdownTree(rootCmd, "./docs/cli"); err != nil {
		return err
	}
	return nil
}

// setLogging merges the input from the command flag with the default log levels, so that a user can override just one single component
func setLogging(inputLogs map[string]string) map[string]string {
	for k := range inputLogs {
		defaultLogLevels[k] = inputLogs[k]
	}
	return defaultLogLevels
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		log.Fatal(err)
	}
}
