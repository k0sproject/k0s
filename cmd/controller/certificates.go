// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/certificate"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// Certificates is the Component implementation to manage all k0s certs
type Certificates struct {
	CACert      string
	CertManager certificate.Manager
	ClusterSpec *v1beta1.ClusterSpec
	K0sVars     *config.CfgVars
}

// Init initializes the certificate component
func (c *Certificates) Init(ctx context.Context) error {
	eg, _ := errgroup.WithContext(ctx)
	// Common CA
	caCertPath := filepath.Join(c.K0sVars.CertRootDir, "ca.crt")
	caCertKey := filepath.Join(c.K0sVars.CertRootDir, "ca.key")

	if err := c.CertManager.EnsureCA("ca", "kubernetes-ca", c.ClusterSpec.API.CA.ExpiresAfter.Duration); err != nil {
		return err
	}

	// We need CA cert loaded to generate client configs
	logrus.Debugf("CA key and cert exists, loading")
	cert, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to read ca cert: %w", err)
	}
	c.CACert = string(cert)
	// Changing the URL here also requires changes in the "k0s kubeconfig admin" subcommand.
	kubeConfigAPIUrl := c.ClusterSpec.API.LocalURL()

	apiServerUID, err := users.LookupUID(constant.ApiserverUser)
	if err != nil {
		err = fmt.Errorf("failed to lookup UID for %q: %w", constant.ApiserverUser, err)
		apiServerUID = users.RootUID
		logrus.WithError(err).Warn("Files with key material for kube-apiserver user will be owned by root")
	}
	eg.Go(func() error {
		// Front proxy CA
		if err := c.CertManager.EnsureCA("front-proxy-ca", "kubernetes-front-proxy-ca", c.ClusterSpec.API.CA.ExpiresAfter.Duration); err != nil {
			return err
		}

		proxyCertPath, proxyCertKey := filepath.Join(c.K0sVars.CertRootDir, "front-proxy-ca.crt"), filepath.Join(c.K0sVars.CertRootDir, "front-proxy-ca.key")

		proxyClientReq := certificate.Request{
			Name:   "front-proxy-client",
			CN:     "front-proxy-client",
			O:      "front-proxy-client",
			CACert: proxyCertPath,
			CAKey:  proxyCertKey,
		}
		_, err := c.CertManager.EnsureCertificate(proxyClientReq, apiServerUID, c.ClusterSpec.API.CA.CertificatesExpireAfter.Duration)

		return err
	})

	eg.Go(func() error {
		// admin cert & kubeconfig
		adminReq := certificate.Request{
			Name:   "admin",
			CN:     "kubernetes-admin",
			O:      "system:masters",
			CACert: caCertPath,
			CAKey:  caCertKey,
		}
		adminCert, err := c.CertManager.EnsureCertificate(adminReq, users.RootUID, c.ClusterSpec.API.CA.CertificatesExpireAfter.Duration)
		if err != nil {
			return err
		}

		if err := kubeConfig(c.K0sVars.AdminKubeConfigPath, kubeConfigAPIUrl, c.CACert, adminCert.Cert, adminCert.Key, users.RootUID); err != nil {
			return err
		}

		return c.CertManager.CreateKeyPair("sa", c.K0sVars, apiServerUID)
	})

	eg.Go(func() error {
		// konnectivity kubeconfig
		konnectivityReq := certificate.Request{
			Name:   "konnectivity",
			CN:     "kubernetes-konnectivity",
			O:      "system:masters", // TODO: We need to figure out if konnectivity really needs superpowers
			CACert: caCertPath,
			CAKey:  caCertKey,
		}

		uid, err := users.LookupUID(constant.KonnectivityServerUser)
		if err != nil {
			err = fmt.Errorf("failed to lookup UID for %q: %w", constant.KonnectivityServerUser, err)
			uid = users.RootUID
			logrus.WithError(err).Warn("Files with key material for konnectivity-server user will be owned by root")
		}

		konnectivityCert, err := c.CertManager.EnsureCertificate(konnectivityReq, uid, c.ClusterSpec.API.CA.CertificatesExpireAfter.Duration)
		if err != nil {
			return err
		}

		return kubeConfig(c.K0sVars.KonnectivityKubeConfigPath, kubeConfigAPIUrl, c.CACert, konnectivityCert.Cert, konnectivityCert.Key, uid)
	})

	eg.Go(func() error {
		ccmReq := certificate.Request{
			Name:   "ccm",
			CN:     "system:kube-controller-manager",
			O:      "system:kube-controller-manager",
			CACert: caCertPath,
			CAKey:  caCertKey,
		}
		ccmCert, err := c.CertManager.EnsureCertificate(ccmReq, apiServerUID, c.ClusterSpec.API.CA.CertificatesExpireAfter.Duration)
		if err != nil {
			return err
		}

		return kubeConfig(filepath.Join(c.K0sVars.CertRootDir, "ccm.conf"), kubeConfigAPIUrl, c.CACert, ccmCert.Cert, ccmCert.Key, apiServerUID)
	})

	eg.Go(func() error {
		schedulerReq := certificate.Request{
			Name:   "scheduler",
			CN:     "system:kube-scheduler",
			O:      "system:kube-scheduler",
			CACert: caCertPath,
			CAKey:  caCertKey,
		}

		uid, err := users.LookupUID(constant.SchedulerUser)
		if err != nil {
			err = fmt.Errorf("failed to lookup UID for %q: %w", constant.SchedulerUser, err)
			uid = users.RootUID
			logrus.WithError(err).Warn("Files with key material for kube-scheduler user will be owned by root")
		}

		schedulerCert, err := c.CertManager.EnsureCertificate(schedulerReq, uid, c.ClusterSpec.API.CA.CertificatesExpireAfter.Duration)
		if err != nil {
			return err
		}

		return kubeConfig(filepath.Join(c.K0sVars.CertRootDir, "scheduler.conf"), kubeConfigAPIUrl, c.CACert, schedulerCert.Cert, schedulerCert.Key, uid)
	})

	eg.Go(func() error {
		kubeletClientReq := certificate.Request{
			Name:   "apiserver-kubelet-client",
			CN:     "apiserver-kubelet-client",
			O:      "system:masters",
			CACert: caCertPath,
			CAKey:  caCertKey,
		}
		_, err := c.CertManager.EnsureCertificate(kubeletClientReq, apiServerUID, c.ClusterSpec.API.CA.CertificatesExpireAfter.Duration)
		return err
	})

	hostnames := []string{
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc.cluster",
		"kubernetes.svc." + c.ClusterSpec.Network.ClusterDomain,
		"localhost",
		"127.0.0.1",
	}

	localIPs, err := detectLocalIPs(ctx)
	if err != nil {
		return fmt.Errorf("error detecting local IP: %w", err)
	}
	hostnames = append(hostnames, localIPs...)
	hostnames = append(hostnames, c.ClusterSpec.API.Sans()...)

	// Add to SANs the IPs from the control plane load balancer
	cplb := c.ClusterSpec.Network.ControlPlaneLoadBalancing
	if cplb != nil && cplb.Enabled && cplb.Keepalived != nil {
		for _, v := range cplb.Keepalived.VRRPInstances {
			for _, vip := range v.VirtualIPs {
				ip, _, err := net.ParseCIDR(vip)
				if err != nil {
					return fmt.Errorf("error parsing virtualIP %s: %w", vip, err)
				}
				hostnames = append(hostnames, ip.String())
			}
		}
	}

	internalAPIAddress, err := c.ClusterSpec.Network.InternalAPIAddresses()
	if err != nil {
		return err
	}
	hostnames = append(hostnames, internalAPIAddress...)

	eg.Go(func() error {
		serverReq := certificate.Request{
			Name:      "server",
			CN:        "kubernetes",
			O:         "kubernetes",
			CACert:    caCertPath,
			CAKey:     caCertKey,
			Hostnames: hostnames,
		}
		_, err = c.CertManager.EnsureCertificate(serverReq, apiServerUID, c.ClusterSpec.API.CA.CertificatesExpireAfter.Duration)
		return err
	})

	eg.Go(func() error {
		apiReq := certificate.Request{
			Name:      "k0s-api",
			CN:        "k0s-api",
			O:         "kubernetes",
			CACert:    caCertPath,
			CAKey:     caCertKey,
			Hostnames: hostnames,
		}
		// TODO Not sure about the user...
		_, err := c.CertManager.EnsureCertificate(apiReq, apiServerUID, c.ClusterSpec.API.CA.CertificatesExpireAfter.Duration)
		return err
	})

	return eg.Wait()
}

func detectLocalIPs(ctx context.Context) ([]string, error) {
	resolver := net.DefaultResolver

	addrs, err := resolver.LookupIPAddr(ctx, "localhost")
	if err != nil {
		return nil, err
	}

	if hostname, err := os.Hostname(); err == nil {
		hostnameAddrs, err := resolver.LookupIPAddr(ctx, hostname)
		if err == nil {
			addrs = append(addrs, hostnameAddrs...)
		} else if errors.Is(err, ctx.Err()) {
			return nil, err
		}
	}

	var localIPs []string
	for _, addr := range addrs {
		ip := addr.IP
		if ip.To4() != nil || ip.To16() != nil {
			localIPs = append(localIPs, ip.String())
		}
	}

	return localIPs, nil
}

func kubeConfig(dest string, url *url.URL, caCert, clientCert, clientKey string, ownerID int) error {
	// We always overwrite the kubeconfigs as the certs might be regenerated at startup
	const (
		clusterName = "local"
		contextName = "Default"
		userName    = "user"
	)

	kubeconfig, err := clientcmd.Write(clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{clusterName: {
			// The server URL is replaced in the "k0s kubeconfig admin" subcommand.
			Server:                   url.String(),
			CertificateAuthorityData: []byte(caCert),
		}},
		Contexts: map[string]*clientcmdapi.Context{contextName: {
			Cluster:  clusterName,
			AuthInfo: userName,
		}},
		CurrentContext: contextName,
		AuthInfos: map[string]*clientcmdapi.AuthInfo{userName: {
			ClientCertificateData: []byte(clientCert),
			ClientKeyData:         []byte(clientKey),
		}},
	})
	if err != nil {
		return err
	}

	err = file.WriteContentAtomically(dest, kubeconfig, constant.CertSecureMode)
	if err != nil {
		return err
	}

	return file.Chown(dest, ownerID, constant.CertSecureMode)
}
