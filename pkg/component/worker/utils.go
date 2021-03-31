package worker

import (
	"fmt"
	"io/ioutil"
	"path"

	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/token"
)

func HandleKubeletBootstrapToken(encodedToken string, k0sVars constant.CfgVars) error {
	kubeconfig, err := token.DecodeJoinToken(encodedToken)
	if err != nil {
		return errors.Wrap(err, "failed to decode token")
	}

	// Load the bootstrap kubeconfig to validate it
	clientCfg, err := clientcmd.Load(kubeconfig)
	if err != nil {
		return errors.Wrap(err, "failed to parse kubelet bootstrap auth from token")
	}
	kubeletCAPath := path.Join(k0sVars.CertRootDir, "ca.crt")
	if !util.FileExists(kubeletCAPath) {
		if err := util.InitDirectory(k0sVars.CertRootDir, constant.CertRootDirMode); err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to initialize dir: %v", k0sVars.CertRootDir))
		}
		err = ioutil.WriteFile(kubeletCAPath, clientCfg.Clusters["k0s"].CertificateAuthorityData, constant.CertMode)
		if err != nil {
			return errors.Wrap(err, "failed to write ca client cert")
		}
	}
	err = ioutil.WriteFile(k0sVars.KubeletBootstrapConfigPath, kubeconfig, constant.CertSecureMode)
	if err != nil {
		return errors.Wrap(err, "failed writing kubelet bootstrap auth config")
	}

	return nil
}

func LoadKubeletConfigClient(k0svars constant.CfgVars) (*KubeletConfigClient, error) {
	var kubeletConfigClient *KubeletConfigClient
	// Prefer to load client config from kubelet auth, fallback to bootstrap token auth
	clientConfigPath := k0svars.KubeletBootstrapConfigPath
	if util.FileExists(k0svars.KubeletAuthConfigPath) {
		clientConfigPath = k0svars.KubeletAuthConfigPath
	}

	kubeletConfigClient, err := NewKubeletConfigClient(clientConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to start kubelet config client: %v", err)
	}
	return kubeletConfigClient, nil
}
