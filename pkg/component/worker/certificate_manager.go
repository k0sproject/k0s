// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"

	"github.com/bombsimon/logrusr/v4"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/certificate"
	k8skubeletcert "k8s.io/kubernetes/pkg/kubelet/certificate"
)

var _ certificate.Manager = (*CertificateManager)(nil)

type CertificateManager struct {
	config                  *rest.Config
	kubeletClientConfigPath string

	currentHash        string
	currentCertificate *tls.Certificate
	lock               sync.RWMutex
}

// loads the kubelet client certificate
// the k8s.io transport is implemented in a way
// that it always compares *tls.Cerificates by ==
// That means to avoid forced reloading certificates we need to return the same pointer
// from Current() each time.
// That's why instead of just returning the parsed certificate each time
// loadFromFilesystem checks based on the md5 hashsum of the certificate content
func (c *CertificateManager) loadFromFilesystem() error {

	raw, err := os.ReadFile(c.config.CertFile)
	if err != nil {
		return fmt.Errorf("can't hash certificate: %w", err)
	}
	newHash := fmt.Sprintf("%x", md5.Sum(raw))

	if newHash == c.currentHash {
		return nil
	}

	cert, err := tls.LoadX509KeyPair(c.config.CertFile, c.config.KeyFile)
	if err != nil {
		return fmt.Errorf("can't load key pair: %w", err)
	}

	// the code borrowed from kubelet assumes Leaf is loaded which does not happen via tls.LoadX509KeyPair...
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("can't parse certificate: %w", err)
	}
	c.currentHash = newHash
	c.currentCertificate = &cert

	return nil
}

func (c *CertificateManager) Current() *tls.Certificate {
	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.loadFromFilesystem(); err != nil {
		logrus.Warningf("can't update the certificate: %s", err)
		return c.currentCertificate
	}

	return c.currentCertificate
}

func (c *CertificateManager) GetRestConfig(ctx context.Context) (*rest.Config, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", c.kubeletClientConfigPath)
	if err != nil {
		return nil, err
	}
	c.config = restConfig
	if err := c.loadFromFilesystem(); err != nil {
		return nil, err
	}
	transportConfig := rest.AnonymousClientConfig(restConfig)
	if _, err := k8skubeletcert.UpdateTransport(logrusr.New(logrus.WithField("component", "worker-certificate-manager")), ctx.Done(), transportConfig, c, 0); err != nil {
		return nil, err
	}

	return transportConfig, nil
}

// TODO Do we need to implement these? In kubelet these are the bits that actually talk with API to get client certs
// So AFAIK we don't
func (c *CertificateManager) Start()              {}
func (c *CertificateManager) Stop()               {}
func (c *CertificateManager) ServerHealthy() bool { return true }

func NewCertificateManager(kubeletClientConfigPath string) *CertificateManager {
	return &CertificateManager{
		kubeletClientConfigPath: kubeletClientConfigPath,
	}
}
