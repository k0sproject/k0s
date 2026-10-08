//go:build unix

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/certificate"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/etcd"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/apiserver/pkg/storage/storagebackend/factory"
)

func TestEtcdUnixSocket(t *testing.T) {
	runDir, err := os.MkdirTemp("", "etcd-") //nolint:usetesting // t.TempDir can exceed the unix socket path limit.
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(runDir)) })

	vars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	vars.RunDir = runDir
	vars.EtcdSocketPath = filepath.Join(runDir, constant.EtcdSocket)
	require.NoError(t, os.MkdirAll(vars.EtcdCertDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Dir(vars.EtcdSocketPath), 0750))

	e := &Etcd{K0sVars: vars, Config: v1beta1.DefaultEtcdConfig(), uid: os.Getuid()}
	e.CertManager = certificate.Manager{K0sVars: vars}
	// Certificate regeneration consults the cluster CA, which exists on a real controller.
	require.NoError(t, e.CertManager.EnsureCA("ca", "kubernetes-ca", e.Config.CA.ExpiresAfter.Duration))
	require.NoError(t, e.setupCerts(t.Context()))

	serverCert := filepath.Join(vars.EtcdCertDir, "server.crt")
	socketName := filepath.Base(vars.EtcdSocketPath)
	require.Contains(t, certificateNames(t, serverCert), socketName)

	// An upgraded node still has a server certificate issued before the socket name existed.
	_, err = e.CertManager.EnsureCertificate(certificate.Request{
		Name:      filepath.Join("etcd", "server"),
		CN:        "etcd-server",
		O:         "etcd-server",
		CACert:    filepath.Join(vars.EtcdCertDir, "ca.crt"),
		CAKey:     filepath.Join(vars.EtcdCertDir, "ca.key"),
		Hostnames: []string{"127.0.0.1", "localhost"},
	}, e.uid, e.Config.CA.CertificatesExpireAfter.Duration)
	require.NoError(t, err)
	require.NotContains(t, certificateNames(t, serverCert), socketName)
	require.NoError(t, e.setupCerts(t.Context()))
	require.Contains(t, certificateNames(t, serverCert), socketName)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go e.maintainSocketMode(ctx)

	// A stale socket is what an unclean etcd exit leaves behind.
	stale, err := net.ListenUnix("unix", &net.UnixAddr{Name: vars.EtcdSocketPath, Net: "unix"})
	require.NoError(t, err)
	stale.SetUnlinkOnClose(false)
	require.NoError(t, stale.Close())

	endpoint, err := url.Parse(e.Config.GetEndpointsAsString(vars.EtcdSocketPath))
	require.NoError(t, err)
	cfg := embed.NewConfig()
	cfg.Dir = vars.EtcdDataDir
	cfg.Logger = "zap"
	cfg.LogLevel = "error"
	cfg.ListenClientUrls = []url.URL{*endpoint}
	cfg.AdvertiseClientUrls = cfg.ListenClientUrls
	peer, err := url.Parse("https://127.0.0.1:0")
	require.NoError(t, err)
	cfg.ListenPeerUrls = []url.URL{*peer}
	cfg.AdvertisePeerUrls = cfg.ListenPeerUrls
	cfg.InitialCluster = cfg.InitialClusterFromName(cfg.Name)
	cfg.ClientTLSInfo = transport.TLSInfo{
		CertFile:       serverCert,
		KeyFile:        filepath.Join(vars.EtcdCertDir, "server.key"),
		TrustedCAFile:  filepath.Join(vars.EtcdCertDir, "ca.crt"),
		ClientCertAuth: true,
	}
	cfg.PeerTLSInfo = cfg.ClientTLSInfo

	server, err := embed.StartEtcd(cfg)
	require.NoError(t, err)
	closeServer := sync.OnceFunc(server.Close)
	t.Cleanup(closeServer)
	select {
	case <-server.Server.ReadyNotify():
	case <-time.After(20 * time.Second):
		t.Fatal("etcd did not become ready")
	}
	require.Eventually(t, func() bool {
		info, err := os.Stat(vars.EtcdSocketPath)
		return err == nil && info.Mode().Perm() == etcdSocketMode
	}, 5*time.Second, 10*time.Millisecond)

	dialCtx, dialCancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer dialCancel()
	client, err := etcd.NewClient(vars.CertRootDir, vars.EtcdCertDir, vars.EtcdSocketPath, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	require.NoError(t, client.Health(dialCtx))

	status, err := client.Status(dialCtx)
	require.NoError(t, err)
	require.Equal(t, etcd.MemberRoleLeader, status.Role)
	members, err := client.ListMembers(dialCtx)
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.Contains(t, members[0].PeerURL, "https://")

	prober, err := factory.CreateProber(storagebackend.Config{
		Type: storagebackend.StorageTypeETCD3,
		Transport: storagebackend.TransportConfig{
			ServerList:    e.Config.GetEndpoints(vars.EtcdSocketPath),
			CertFile:      e.Config.GetCertFilePath(vars.CertRootDir),
			KeyFile:       e.Config.GetKeyFilePath(vars.CertRootDir),
			TrustedCAFile: e.Config.GetCaFilePath(vars.EtcdCertDir),
		},
	})
	require.NoError(t, err)
	require.NoError(t, prober.Probe(dialCtx))
	require.NoError(t, prober.Close())

	metrics := &Metrics{K0sVars: vars, log: logrus.New()}
	metricsJob, err := metrics.newEtcdJob()
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(dialCtx, http.MethodGet, metricsJob.scrapeURL, nil)
	require.NoError(t, err)
	resp, err := metricsJob.scrapeClient.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(body), "etcd_server_has_leader")

	unauthConfig := *client.Config
	unauthConfig.TLS = unauthConfig.TLS.Clone()
	unauthConfig.TLS.Certificates = nil
	unauthConfig.TLS.GetClientCertificate = nil
	unauth, err := clientv3.New(unauthConfig)
	require.NoError(t, err)
	deniedCtx, deniedCancel := context.WithTimeout(dialCtx, time.Second)
	_, err = unauth.Get(deniedCtx, "/socket-test")
	deniedCancel()
	require.Error(t, err)
	require.NoError(t, unauth.Close())

	closeServer()
	_, err = os.Stat(vars.EtcdSocketPath)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func certificateNames(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	block, _ := pem.Decode(raw)
	require.NotNil(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	names := append([]string{}, cert.DNSNames...)
	for _, ip := range cert.IPAddresses {
		names = append(names, ip.String())
	}
	return names
}
