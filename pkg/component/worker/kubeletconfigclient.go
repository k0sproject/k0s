package worker

import (
	"fmt"

	"github.com/Mirantis/mke/pkg/constant"
	k8sutil "github.com/Mirantis/mke/pkg/kubernetes"
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type KubeletConfigClient struct {
	kubeClient *kubernetes.Clientset
}

func NewKubeletConfigClient(kubeconfigPath string) (*KubeletConfigClient, error) {
	kubeClient, err := k8sutil.Client(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	return &KubeletConfigClient{
		kubeClient: kubeClient,
	}, nil
}

// Get reads the config from kube api
// NOTE: We probably need to refactor this to return something we can merge with user provided local config once we enable local worker configs
func (k *KubeletConfigClient) Get() (string, error) {
	cmName := fmt.Sprintf("kubelet-config-%s", constant.KubernetesMajorMinorVersion)
	cm, err := k.kubeClient.CoreV1().ConfigMaps("kube-system").Get(cmName, v1.GetOptions{})
	if err != nil {
		return "", errors.Wrap(err, "failed to get kubelet config from API")
	}
	config := cm.Data["kubelet"]
	if config == "" {
		return "", fmt.Errorf("no config found with key 'kubelet' in %s", cmName)
	}
	return config, nil
}
