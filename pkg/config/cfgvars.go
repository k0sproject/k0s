/*
Copyright 2023 k0s authors

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

package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/pflag"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
)

// CfgVars is a struct that holds all the config variables required for K0s
// Some of the variables are duplicates of the ones in the CLIOptions struct
// for historical and convenience reasons.
type CfgVars struct {
	InvocationID               string              // Unique ID for this invocation of k0s
	AdminKubeConfigPath        string              // The cluster admin kubeconfig location
	BinDir                     string              // location for all pki related binaries
	CertRootDir                string              // CertRootDir defines the root location for all pki related artifacts
	DataDir                    string              // Data directory containing k0s state
	KubeletRootDir             string              // Root directory for kubelet
	EtcdCertDir                string              // EtcdCertDir contains etcd certificates
	EtcdDataDir                string              // EtcdDataDir contains etcd state
	KineSocketPath             string              // The unix socket path for kine
	KonnectivitySocketDir      string              // location of konnectivity's socket path
	KubeletAuthConfigPath      string              // KubeletAuthConfigPath defines the default kubelet auth config path
	ManifestsDir               string              // location for all stack manifests
	RunDir                     string              // location of supervised pid files and sockets
	KonnectivityKubeConfigPath string              // location for konnectivity kubeconfig
	OCIBundleDir               string              // location for OCI bundles
	DefaultStorageType         v1beta1.StorageType // Default backend storage
	RuntimeConfigPath          string              // A static copy of the config loaded at startup
	StatusSocketPath           string              // The unix socket path for k0s status API
	StartupConfigPath          string              // The path to the config file used at startup

	// Helm config
	HelmHome             string
	HelmRepositoryCache  string
	HelmRepositoryConfig string

	stdin io.Reader
}

type CfgVarOption func(*CfgVars)

// Command represents cobra.Command
type command interface {
	InOrStdin() io.Reader
	Flags() *pflag.FlagSet
}

func WithCommand(cmd command) CfgVarOption {
	return func(c *CfgVars) {
		c.stdin = cmd.InOrStdin()

		flags := cmd.Flags()

		if f, err := flags.GetString("data-dir"); err == nil && f != "" {
			c.DataDir = f
		}

		if f, err := flags.GetString("kubelet-root-dir"); err == nil && f != "" {
			if f, err := filepath.Abs(f); err == nil {
				c.KubeletRootDir = f
			}
		}

		if f, err := flags.GetString("config"); err == nil && f != "" {
			c.StartupConfigPath = f
		}

		if f, err := flags.GetString("status-socket"); err == nil && f != "" {
			c.StatusSocketPath = f
		}

		if f, err := flags.GetBool("single"); err == nil && f {
			c.DefaultStorageType = v1beta1.KineStorageType
		} else {
			c.DefaultStorageType = v1beta1.EtcdStorageType
		}
	}
}

func DefaultCfgVars() *CfgVars {
	vars, _ := NewCfgVars(nil)
	return vars
}

// NewCfgVars returns a new CfgVars struct.
func NewCfgVars(cobraCmd command, dirs ...string) (*CfgVars, error) {
	var dataDir string
	var kubeletRootDir string

	if len(dirs) > 0 {
		dataDir = dirs[0]
	}

	if cobraCmd != nil {
		if val, err := cobraCmd.Flags().GetString("data-dir"); err == nil && val != "" {
			dataDir = val
		}
		if val, err := cobraCmd.Flags().GetString("kubelet-root-dir"); err == nil && val != "" {
			kubeletRootDir = val
		}
	}

	if dataDir == "" {
		dataDir = constant.DataDirDefault
	}

	// fetch absolute path for dataDir
	dataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, fmt.Errorf("invalid datadir: %w", err)
	}

	if kubeletRootDir == "" {
		kubeletRootDir = filepath.Join(dataDir, "kubelet")
	}
	kubeletRootDir, err = filepath.Abs(kubeletRootDir)
	if err != nil {
		return nil, err
	}

	var runDir string
	if os.Geteuid() == 0 {
		runDir = "/run/k0s"
	} else {
		runDir = filepath.Join(dataDir, "run")
	}

	certDir := filepath.Join(dataDir, "pki")
	helmHome := filepath.Join(dataDir, "helmhome")

	var invocationID [16]byte
	if _, err := rand.Read(invocationID[:]); err != nil {
		return nil, err
	}

	vars := &CfgVars{
		InvocationID:               hex.EncodeToString(invocationID[:]),
		AdminKubeConfigPath:        filepath.Join(certDir, "admin.conf"),
		BinDir:                     filepath.Join(dataDir, "bin"),
		OCIBundleDir:               filepath.Join(dataDir, "images"),
		CertRootDir:                certDir,
		DataDir:                    dataDir,
		KubeletRootDir:             kubeletRootDir,
		EtcdCertDir:                filepath.Join(certDir, "etcd"),
		EtcdDataDir:                filepath.Join(dataDir, "etcd"),
		KineSocketPath:             filepath.Join(runDir, constant.KineSocket),
		KonnectivitySocketDir:      filepath.Join(runDir, "konnectivity-server"),
		KubeletAuthConfigPath:      filepath.Join(dataDir, "kubelet.conf"),
		ManifestsDir:               filepath.Join(dataDir, "manifests"),
		RunDir:                     runDir,
		KonnectivityKubeConfigPath: filepath.Join(certDir, "konnectivity.conf"),
		RuntimeConfigPath:          filepath.Join(runDir, "k0s.yaml"),
		StatusSocketPath:           filepath.Join(runDir, "status.sock"),
		StartupConfigPath:          constant.K0sConfigPathDefault,

		// Helm Config
		HelmHome:             helmHome,
		HelmRepositoryCache:  filepath.Join(helmHome, "cache"),
		HelmRepositoryConfig: filepath.Join(helmHome, "repositories.yaml"),

		stdin: os.Stdin,
	}

	if cobraCmd != nil {
		WithCommand(cobraCmd)(vars)
	}

	return vars, nil
}

var defaultConfigPath = constant.K0sConfigPathDefault

func (c *CfgVars) NodeConfig() (*v1beta1.ClusterConfig, error) {
	if c.StartupConfigPath == "" {
		return nil, errors.New("config path is not set")
	}

	nodeConfig := v1beta1.DefaultClusterConfig()

	// Optionally override the default storage (used for single node clusters)
	if c.DefaultStorageType == v1beta1.KineStorageType {
		nodeConfig.Spec.Storage = &v1beta1.StorageSpec{
			Type: v1beta1.KineStorageType,
			Kine: v1beta1.DefaultKineConfig(c.DataDir),
		}
	}

	if c.StartupConfigPath == "-" {
		configReader := c.stdin
		c.stdin = nil
		if configReader == nil {
			return nil, errors.New("stdin already grabbed")
		}

		bytes, err := io.ReadAll(configReader)
		if err != nil {
			return nil, err
		}

		nodeConfig, err = nodeConfig.MergedWithYAML(bytes)
		if err != nil {
			return nil, err
		}
	} else {
		cfgContent, err := os.ReadFile(c.StartupConfigPath)
		if err != nil {
			if c.StartupConfigPath == defaultConfigPath && errors.Is(err, os.ErrNotExist) {
				// The default configuration file doesn't exist; continue with the defaults.
				return nodeConfig, nil
			}

			return nil, err
		}

		return nodeConfig.MergedWithYAML(cfgContent)
	}

	if nodeConfig.Spec.Storage.Type == v1beta1.KineStorageType && nodeConfig.Spec.Storage.Kine == nil {
		nodeConfig.Spec.Storage.Kine = v1beta1.DefaultKineConfig(c.DataDir)
	}

	return nodeConfig, nil
}
