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
package worker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/certificate"

	k8skubeletcert "k8s.io/kubernetes/pkg/kubelet/certificate"
)

var _ certificate.Manager = (*certManager)(nil)

type certManager struct {
	restConfig *rest.Config
}

func (c *certManager) Current() *tls.Certificate {
	cert, err := tls.LoadX509KeyPair(c.restConfig.CertFile, c.restConfig.KeyFile)
	if err != nil {
		return nil
	}

	// the code borrowed from kubelet assumes Leaf is loaded which does not happen via tls.LoadX509KeyPair...
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil
	}

	return &cert
}

// TODO Do we need to implement these? In kubelet these are the bits that actually talk with API to get client certs
// So AFAIK we don't
func (c *certManager) Start()              {}
func (c *certManager) Stop()               {}
func (c *certManager) ServerHealthy() bool { return true }

func GetRestConfig(ctx context.Context, kubeletClientConfigPath string) (*rest.Config, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeletClientConfigPath)
	if err != nil {
		return nil, err
	}

	certManager := &certManager{
		restConfig: restConfig,
	}

	transportConfig := rest.AnonymousClientConfig(restConfig)

	if _, err := k8skubeletcert.UpdateTransport(ctx.Done(), transportConfig, certManager, 5*time.Minute); err != nil {
		return nil, err
	}

	return transportConfig, nil
}
