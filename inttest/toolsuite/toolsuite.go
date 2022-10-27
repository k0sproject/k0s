// Copyright 2022 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package toolsuite

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"time"

	apclient "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2/clientset"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultToolImage = "tool:latest"
)

type ClusterOperation func(ctx context.Context, data ClusterData) error

type ClusterData struct {
	DataDir        string
	KubeConfigFile string
	PrivateKeyFile string
}

type Config struct {
	Image            string
	TestName         string
	DataDir          string
	Provider         string
	Command          string
	OperationTimeout time.Duration
}

var config Config

// init is used to populate all of the arguments that have been passed to the test.
func init() {
	flag.StringVar(&config.Image, "image", defaultToolImage, "The name of the image to use")
	flag.StringVar(&config.TestName, "testname", "", "The name of the test")
	flag.StringVar(&config.DataDir, "data-dir", "", "The tool data directory")
	flag.StringVar(&config.Provider, "provider", "", "The name of the tool provider")
	flag.StringVar(&config.Command, "command", "", "The provider command to invoke")
	flag.DurationVar(&config.OperationTimeout, "operation-timeout", 10*time.Minute, "The timeout for operations")
}

type ToolSuite struct {
	suite.Suite
	Config    Config
	Operation ClusterOperation
}

func (s *ToolSuite) Context() context.Context {
	return context.TODO()
}

func (s *ToolSuite) KubeConfigFile() string {
	return path.Join(s.Config.DataDir, "k0s.kubeconfig")
}

// generateDockerRunArgs builds up the arguments for a 'docker run', taking into consideration
// the provider, and all of the extra passthrough arguments provided.
func generateDockerRunArgs(c Config, command string, action string) ([]string, []string, error) {
	var runArgs, containerArgs []string
	var err error

	switch c.Provider {
	case "aws":
		runArgs, containerArgs, err = generateDockerRunArgsAWS(c, command, action)
	}

	if err != nil {
		return []string{}, []string{}, fmt.Errorf("provider argument is invalid/missing")
	}

	// Ensure that the volume mount to the data directory is provided to the run arguments.

	return append(runArgs, "-v", fmt.Sprintf("%s:/tool/data", c.DataDir)), containerArgs, nil
}

// generateDockerRunArgsAWS is the AWS-specific implementation of `generateDockerRunArgs()`, but
// also seeding the `docker` environment with the AWS credentials.
func generateDockerRunArgsAWS(c Config, command string, action string) ([]string, []string, error) {
	runArgs := []string{
		"-e", "AWS_ACCESS_KEY_ID",
		"-e", "AWS_SECRET_ACCESS_KEY",
		"-e", "AWS_SESSION_TOKEN",
	}

	return runArgs, append([]string{c.Provider, command, action}, flag.Args()...), nil
}

// SetupSuite will generate the appropriate arguments to 'create' the infrastructure as defined on
// the command line, and invoke it.
func (s *ToolSuite) SetupSuite() {
	if config.Image == "" {
		s.T().FailNow()
	}

	s.Config = config

	runArgs, containerArgs, err := generateDockerRunArgs(config, s.Config.Command, "create")
	if err != nil {
		s.T().FailNow()
	}

	if err := dockerRun(s.Config.Image, runArgs, containerArgs); err != nil {
		s.Failf("unable to run container", "%v\n", err)
		return
	}
}

// KubeClient returns a kubernetes API client
func (s *ToolSuite) KubeClient() (*kubernetes.Clientset, error) {
	return clientLoader(s.KubeConfigFile(), func(config *rest.Config) (*kubernetes.Clientset, error) {
		return kubernetes.NewForConfig(config)
	})
}

// AutopilotClient returns an autopilot API client
func (s *ToolSuite) AutopilotClient() (*apclient.Clientset, error) {
	return clientLoader(s.KubeConfigFile(), func(config *rest.Config) (*apclient.Clientset, error) {
		return apclient.NewForConfig(config)
	})
}

// clientLoader loads a kubernetes REST configuration, and returns an appropriate API instance via types.
func clientLoader[CT any](kubeConfigFile string, loader func(config *rest.Config) (*CT, error)) (*CT, error) {
	kubeConfigBytes, err := ioutil.ReadFile(kubeConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read kubeconfig '%s': %w", kubeConfigFile, err)
	}

	kubeConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	client, err := loader(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create kubernetes client: %w", err)
	}

	return client, nil
}

// TestEntrypoint is the main testing entrypoint for a single toolsuite test. This parses and loads
// kubernetes cluster configuration from `k0s.kubeconfig`, and passes it to a user-specified delegate
// operation handler.
func (s *ToolSuite) TestEntrypoint() {
	kubeConfigFile := path.Join(s.Config.DataDir, "k0s.kubeconfig")

	clusterData := ClusterData{
		DataDir:        s.Config.DataDir,
		KubeConfigFile: kubeConfigFile,
		PrivateKeyFile: path.Join(s.Config.DataDir, "private.pem"),
	}

	err := s.Operation(s.Context(), clusterData)
	assert.NoError(s.T(), err, "Failed in toolsuite handler: %v", err)
}

// TearDownSuite will generate the appropriate arguments to 'destroy' the infrastructure as defined on
// the command line, and invoke it.
func (s *ToolSuite) TearDownSuite() {
	runArgs, containerArgs, err := generateDockerRunArgs(config, s.Config.Command, "destroy")
	if err != nil {
		s.T().FailNow()
	}

	if err := dockerRun(s.Config.Image, runArgs, containerArgs); err != nil {
		s.Failf("unable to run container", "%v\n", err)
		return
	}
}

// dockerRun runs docker using the provided run and container arguments.
func dockerRun(image string, runArgs []string, containerArgs []string) error {
	args := []string{"run"}
	args = append(args, runArgs...)
	args = append(args, image)
	args = append(args, containerArgs...)

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
