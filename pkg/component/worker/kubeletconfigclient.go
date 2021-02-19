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
package worker

import (
	"context"
	"fmt"

	"github.com/k0sproject/k0s/pkg/constant"
	k8sutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// KubeletConfigClient is the client used to fetch kubelet config from a common config map
type KubeletConfigClient struct {
	kubeClient kubernetes.Interface
}

// NewKubeletConfigClient creates new KubeletConfigClient using the specified kubeconfig
func NewKubeletConfigClient(kubeconfigPath string) (*KubeletConfigClient, error) {
	kubeClient, err := k8sutil.NewClient(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	return &KubeletConfigClient{
		kubeClient: kubeClient,
	}, nil
}

// Get reads the config from kube api
func (k *KubeletConfigClient) Get(profile string) (string, error) {
	cmName := fmt.Sprintf("kubelet-config-%s-%s", profile, constant.KubernetesMajorMinorVersion)
	cm, err := k.kubeClient.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), cmName, v1.GetOptions{})
	if err != nil {
		return "", errors.Wrap(err, "failed to get kubelet config from API")
	}
	config := cm.Data["kubelet"]
	if config == "" {
		return "", fmt.Errorf("no config found with key 'kubelet' in %s", cmName)
	}
	return config, nil
}
