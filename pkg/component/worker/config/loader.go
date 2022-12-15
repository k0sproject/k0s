/*
Copyright 2022 k0s authors

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
	"context"
	"fmt"
	"time"

	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
)

// LoadProfile loads the worker profile with the given profile name from
// Kubernetes.
func LoadProfile(ctx context.Context, kubeconfig clientcmd.KubeconfigGetter, profileName string) (*Profile, error) {
	clientConfig, err := kubeutil.ClientConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client config: %w", err)
	}

	clientFactory := func() (kubernetes.Interface, error) {
		return kubernetes.NewForConfig(clientConfig)
	}

	return loadProfile(ctx, logrus.StandardLogger(), clientFactory, profileName)
}

func loadProfile(ctx context.Context, log logrus.FieldLogger, clientFactory func() (kubernetes.Interface, error), profileName string) (*Profile, error) {
	configMapName := fmt.Sprintf("%s-%s-%s", constant.WorkerConfigComponentName, profileName, constant.KubernetesMajorMinorVersion)

	var configMap *corev1.ConfigMap
	if err := retry.Do(
		func() error {
			client, err := clientFactory()
			if err != nil {
				return err
			}

			configMap, err = client.CoreV1().ConfigMaps("kube-system").Get(ctx, configMapName, metav1.GetOptions{})
			return err
		},
		retry.Context(ctx),
		retry.LastErrorOnly(true),
		retry.Delay(500*time.Millisecond),
		retry.OnRetry(func(attempt uint, err error) {
			log.WithError(err).Debugf("Failed to load configuration for worker profile in attempt #%d, retrying after backoff", attempt+1)
		}),
	); err != nil {
		if apierrors.IsUnauthorized(err) {
			err = fmt.Errorf("the k0s worker node credentials are invalid, the node needs to be rejoined into the cluster with a fresh bootstrap token: %w", err)
		}

		return nil, err
	}

	profile, err := FromConfigMapData(configMap.Data)
	if err != nil {
		return nil, err
	}

	return profile, nil
}
