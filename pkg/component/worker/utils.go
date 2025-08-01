// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/config"

	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubernetes/pkg/kubelet/certificate/bootstrap"

	"github.com/sirupsen/logrus"
)

func BootstrapKubeletClientConfig(ctx context.Context, k0sVars *config.CfgVars, nodeName apitypes.NodeName, workerOpts *config.WorkerOptions, getBootstrapKubeconfig clientcmd.KubeconfigGetter) error {
	log := logrus.WithFields(logrus.Fields{"component": "bootstrap-kubelet", "node_name": nodeName})
	bootstrapKubeconfigPath := filepath.Join(k0sVars.DataDir, "kubelet-bootstrap.conf")

	// When using `k0s install` along with a join token, that join token
	// argument will be registered within the k0s service definition and be
	// passed to k0s each time it gets started as a service. Hence that token
	// needs to be ignored if it has already been used. This results in the
	// following order of precedence:

	switch {
	// 1: Regular kubelet kubeconfig file exists.
	// The kubelet kubeconfig has been bootstrapped already.
	case file.Exists(k0sVars.KubeletAuthConfigPath):
		return nil

	// 2: Kubelet bootstrap kubeconfig file exists.
	// The kubelet kubeconfig will be bootstrapped without a join token.
	case file.Exists(bootstrapKubeconfigPath):
		// Nothing to do here.

	// 3: A bootstrap kubeconfig can be created (usually via a join token).
	// Bootstrap the kubelet kubeconfig via a temporary bootstrap config file.
	case getBootstrapKubeconfig != nil:
		bootstrapKubeconfig, err := getBootstrapKubeconfig()
		if err != nil {
			return fmt.Errorf("failed to get bootstrap kubeconfig: %w", err)
		}

		// Write the kubelet bootstrap kubeconfig to a temporary file, as the
		// kubelet bootstrap API only accepts files.
		bootstrapKubeconfigPath, err = writeKubeletBootstrapKubeconfig(*bootstrapKubeconfig)
		if err != nil {
			return fmt.Errorf("failed to write bootstrap kubeconfig: %w", err)
		}

		// Ensure that the temporary kubelet bootstrap kubeconfig file will be
		// removed when done.
		defer func() {
			if err := os.Remove(bootstrapKubeconfigPath); err != nil && !os.IsNotExist(err) {
				log.WithError(err).Error("Failed to remove bootstrap kubeconfig file")
			}
		}()

		log.Debug("Wrote bootstrap kubeconfig file: ", bootstrapKubeconfigPath)

	// 4: None of the above, bail out.
	default:
		return errors.New("neither regular nor bootstrap kubeconfig files exist and no join token given; dunno how to make kubelet authenticate to API server")
	}

	log.Info("Bootstrapping client configuration")

	var attempt uint
	err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		attempt++
		err := bootstrap.LoadClientCert(
			ctx,
			k0sVars.KubeletAuthConfigPath,
			bootstrapKubeconfigPath,
			filepath.Join(k0sVars.KubeletRootDir, "pki"),
			nodeName,
		)
		if err != nil {
			log.WithError(err).WithField("attempt", attempt).Debug("Failed to bootstrap client configuration, retrying after backoff")
			return false, nil // retry
		}
		return true, nil // success
	})
	if err != nil {
		return fmt.Errorf("failed to bootstrap client configuration: %w", err)
	}

	log.Info("Successfully bootstrapped client configuration")
	return nil
}

func writeKubeletBootstrapKubeconfig(kubeconfig clientcmdapi.Config) (string, error) {
	if err := clientcmdapi.MinifyConfig(&kubeconfig); err != nil {
		return "", fmt.Errorf("failed to minify bootstrap kubeconfig: %w", err)
	}

	bytes, err := clientcmd.Write(kubeconfig)
	if err != nil {
		return "", err
	}

	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" && runtime.GOOS != "windows" {
		dir = "/run"
	}

	bootstrapFile, err := os.CreateTemp(dir, "k0s-*-kubelet-bootstrap-kubeconfig")
	if err != nil {
		return "", err
	}

	_, writeErr := bootstrapFile.Write(bytes)
	closeErr := bootstrapFile.Close()

	if writeErr != nil || closeErr != nil {
		rmErr := os.Remove(bootstrapFile.Name())
		// Don't propagate any fs.ErrNotExist errors. There is no point in doing
		// this, since the desired state is already reached: The bootstrap file
		// is no longer present on the file system.
		if errors.Is(rmErr, fs.ErrNotExist) {
			rmErr = nil
		}

		return "", errors.Join(writeErr, closeErr, rmErr)
	}

	return bootstrapFile.Name(), nil
}
