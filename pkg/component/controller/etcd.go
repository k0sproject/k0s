// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/certificate"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/etcd"
	"github.com/k0sproject/k0s/pkg/supervisor"
	"github.com/k0sproject/k0s/pkg/token"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/client/pkg/v3/tlsutil"
	"golang.org/x/sync/errgroup"
)

const etcdGID = 0

// Etcd implement the component interface to run etcd
type Etcd struct {
	CertManager certificate.Manager
	Config      *v1beta1.EtcdConfig
	JoinClient  *token.JoinClient
	K0sVars     *config.CfgVars
	LogLevel    string
	LeaveOnStop func() bool

	supervisor     *supervisor.Supervisor
	executablePath string
	uid            int
}

var _ manager.Component = (*Etcd)(nil)
var _ manager.Ready = (*Etcd)(nil)

// Init extracts the needed binaries
func (e *Etcd) Init(_ context.Context) error {
	var err error

	if err = detectUnsupportedEtcdArch(); err != nil {
		return fmt.Errorf("missing environment variable: %w", err)
	}

	e.uid, err = users.LookupUID(constant.EtcdUser)
	if err != nil {
		err = fmt.Errorf("failed to lookup UID for %q: %w", constant.EtcdUser, err)
		e.uid = users.RootUID
		logrus.WithError(err).Warn("Running etcd as root, files with key material for etcd user will be owned by root")
	}

	err = dir.Init(e.K0sVars.EtcdDataDir, constant.EtcdDataDirMode) // https://docs.datadoghq.com/security_monitoring/default_rules/cis-kubernetes-1.5.1-1.1.11/
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", e.K0sVars.EtcdDataDir, err)
	}

	err = dir.Init(e.K0sVars.EtcdCertDir, constant.EtcdCertDirMode) // https://docs.datadoghq.com/security_monitoring/default_rules/cis-kubernetes-1.5.1-4.1.7/
	if err != nil {
		return fmt.Errorf("failed to create etcd cert dir: %w", err)
	}

	for _, f := range []string{e.K0sVars.EtcdDataDir, e.K0sVars.EtcdCertDir} {
		err = recursiveChown(f, e.uid, etcdGID)
		if err != nil && os.Geteuid() == 0 {
			return err
		}
	}
	e.executablePath, err = assets.StageExecutable(e.K0sVars.BinDir, "etcd")
	return err
}

func (e *Etcd) syncEtcdConfig(ctx context.Context, etcdRequest v1beta1.EtcdRequest, etcdCaCert, etcdCaCertKey string) ([]string, error) {
	logrus.Info("Synchronizing etcd config with existing cluster via ", e.JoinClient.Address())

	var etcdResponse v1beta1.EtcdResponse
	var err error
	retryErr := retry.Do(
		func() error {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			etcdResponse, err = e.JoinClient.JoinEtcd(ctx, etcdRequest)
			return err
		},
		// When joining multiple nodes in parallel, etcd can lose consensus and will return 500 responses
		// Allow for more time to recover (~ 4 minutes = 0+1+2+4+8+16+32+60+60+60)
		retry.Attempts(10),
		retry.Delay(1*time.Second),
		retry.MaxDelay(60*time.Second),
		retry.Context(ctx),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(attempt uint, err error) {
			logrus.WithError(err).Debug("Failed to synchronize etcd config in attempt #", attempt+1, ", retrying after backoff")
		}),
	)
	if retryErr != nil {
		if err != nil {
			retryErr = err
		}
		return nil, fmt.Errorf("failed to synchronize etcd config with existing cluster via %s: %w", e.JoinClient.Address(), retryErr)
	}

	logrus.Debugf("got cluster info: %v", etcdResponse.InitialCluster)
	// Write etcd ca cert&key
	if file.Exists(etcdCaCert) && file.Exists(etcdCaCertKey) {
		logrus.Warnf("etcd ca certs already exists, not gonna overwrite. If you wish to re-sync them, delete the existing ones.")
	} else {
		err = file.WriteContentAtomically(etcdCaCertKey, etcdResponse.CA.Key, constant.CertSecureMode)
		if err != nil {
			return nil, err
		}

		err = file.WriteContentAtomically(etcdCaCert, etcdResponse.CA.Cert, constant.CertSecureMode)
		if err != nil {
			return nil, err
		}
		for _, f := range []string{filepath.Dir(etcdCaCertKey), etcdCaCertKey, etcdCaCert} {
			if err := os.Chown(f, e.uid, etcdGID); err != nil && os.Geteuid() == 0 {
				return nil, err
			}
		}
	}
	return etcdResponse.InitialCluster, nil
}

// Run runs etcd if external cluster is not configured
func (e *Etcd) Start(ctx context.Context) error {
	etcdCaCert := filepath.Join(e.K0sVars.EtcdCertDir, "ca.crt")
	etcdCaCertKey := filepath.Join(e.K0sVars.EtcdCertDir, "ca.key")
	etcdServerCert := filepath.Join(e.K0sVars.EtcdCertDir, "server.crt")
	etcdServerKey := filepath.Join(e.K0sVars.EtcdCertDir, "server.key")
	etcdPeerCert := filepath.Join(e.K0sVars.EtcdCertDir, "peer.crt")
	etcdPeerKey := filepath.Join(e.K0sVars.EtcdCertDir, "peer.key")
	etcdSignKey := filepath.Join(e.K0sVars.EtcdCertDir, "jwt.key")
	etcdSignPub := filepath.Join(e.K0sVars.EtcdCertDir, "jwt.pub")

	logrus.Info("Starting etcd")

	var name string
	if etcdName, ok := e.Config.ExtraArgs["name"]; ok {
		name = etcdName
	} else if hostName, err := os.Hostname(); err != nil {
		return err
	} else {
		name = hostName
	}

	peerURL := e.Config.GetPeerURL()

	args := stringmap.StringMap{
		"--data-dir":                    e.K0sVars.EtcdDataDir,
		"--listen-client-urls":          "https://127.0.0.1:2379",
		"--advertise-client-urls":       "https://127.0.0.1:2379",
		"--client-cert-auth":            "true",
		"--listen-peer-urls":            peerURL,
		"--initial-advertise-peer-urls": peerURL,
		"--name":                        name,
		"--tls-min-version":             string(tlsutil.TLSVersion12),
		"--trusted-ca-file":             etcdCaCert,
		"--cert-file":                   etcdServerCert,
		"--key-file":                    etcdServerKey,
		"--peer-trusted-ca-file":        etcdCaCert,
		"--peer-key-file":               etcdPeerKey,
		"--peer-cert-file":              etcdPeerCert,
		"--log-level":                   e.LogLevel,
		"--peer-client-cert-auth":       "true",
		"--enable-pprof":                "false",
	}

	// Use the main etcd data directory as the source of truth to determine if this node has already joined
	// See https://etcd.io/docs/v3.5/learning/persistent-storage-files/#bbolt-btree-membersnapdb
	if file.Exists(filepath.Join(e.K0sVars.EtcdDataDir, "member", "snap", "db")) {
		logrus.Warnf("etcd db file(s) already exist, not gonna run join process")
	} else if e.JoinClient != nil {
		etcdRequest := v1beta1.EtcdRequest{
			Node:        name,
			PeerAddress: peerURL,
		}
		initialCluster, err := e.syncEtcdConfig(ctx, etcdRequest, etcdCaCert, etcdCaCertKey)
		if err != nil {
			return fmt.Errorf("failed to sync etcd config: %w", err)
		}
		args["--initial-cluster"] = strings.Join(initialCluster, ",")
		args["--initial-cluster-state"] = "existing"
	}

	if err := e.setupCerts(ctx); err != nil {
		return fmt.Errorf("failed to create etcd certs: %w", err)
	}

	// In case this is upgrade/restart, the sign key is not created
	if file.Exists(etcdSignKey) && file.Exists(etcdSignPub) {
		auth := fmt.Sprintf("jwt,pub-key=%s,priv-key=%s,sign-method=RS512,ttl=10m", etcdSignPub, etcdSignKey)
		args["--auth-token"] = auth
	}

	for name, value := range e.Config.ExtraArgs {
		argName := "--" + name
		if _, ok := args[argName]; ok {
			logrus.Warnf("overriding etcd flag with user provided value: %s", argName)
		}
		args[argName] = value
	}

	// Specifying a minimum version of TLS 1.3 _and_ a list of cipher suites
	// will be rejected.
	// https://github.com/etcd-io/etcd/pull/15156/files#diff-538c79cd00ec18cb43b5dddd5f36b979d9d050cf478a241304493284739d31bfR810-R813
	if args["--cipher-suites"] == "" && args["--tls-min-version"] != string(tlsutil.TLSVersion13) {
		args["--cipher-suites"] = constant.AllowedTLS12CipherSuiteNames()
	}

	logrus.Debugf("starting etcd with args: %v", args)

	e.supervisor = &supervisor.Supervisor{
		Name:          "etcd",
		BinPath:       e.executablePath,
		RunDir:        e.K0sVars.RunDir,
		DataDir:       e.K0sVars.DataDir,
		Args:          append(args.ToArgs(), e.Config.RawArgs...),
		UID:           e.uid,
		GID:           etcdGID,
		KeepEnvPrefix: true,
	}

	return e.supervisor.Supervise(ctx)
}

// Stop stops etcd
func (e *Etcd) Stop() error {
	s := e.supervisor
	if s == nil {
		return nil
	}

	if !e.LeaveOnStop() {
		return s.Stop()
	}

	var leaveErr, stopErr error
	deferTermination := make(chan struct{})
	ctx, cancel := context.WithCancelCause(context.TODO())

	go func() {
		defer cancel(errors.New("etcd process terminated"))
		stopErr = s.StopWith(supervisor.StopOpts{
			DeferGracefulTerminationUntil: deferTermination,
		})
	}()

	go func() {
		defer close(deferTermination)
		leaveErr = e.leave(ctx)
		if leaveErr == nil {
			// If the leave request was successful, we expect that etcd will
			// terminate itself. Wait 30 seconds before lifting the termination
			// request inhibition.
			select {
			case <-time.After(30 * time.Second):
			case <-ctx.Done():
			}
		}
	}()

	<-ctx.Done()
	<-deferTermination

	return errors.Join(leaveErr, stopErr)
}

func (e *Etcd) leave(ctx context.Context) error {
	log := logrus.WithField("component", "etcd")
	log.Info("Attempting to leave the cluster")

	if err := func() error {
		c, err := etcd.NewClient(e.K0sVars.CertRootDir, e.K0sVars.EtcdCertDir, e.Config)
		if err != nil {
			return fmt.Errorf("failed to initialize etcd client: %w", err)
		}
		defer c.Close()

		var memberID uint64
		return wait.ExponentialBackoffWithContext(ctx, wait.Backoff{
			Steps: 10, Duration: 3 * time.Second, Jitter: 1,
		}, func(context.Context) (bool, error) {
			// Decouple the request context from the outer context. The outer
			// context will only interrupt retries, not the inner call itself.
			// This is to ensure that the call is not terminated in-flight and
			// returns an otherwise inappropriate error.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if memberID == 0 {
				status, err := c.Status(ctx)
				if err != nil {
					log.WithError(err).Error("Failed to get etcd endpoint status")
					return false, nil
				}

				memberID = status.ID
				log = log.WithField("memberID", strconv.FormatUint(memberID, 16))
			}

			log.Info("Removing member")
			if err := c.DeleteMember(ctx, memberID); err != nil {
				if errors.Is(err, etcd.ErrMemberNotFound) {
					log.Info("Member ID changed")
					memberID = 0
					return false, nil
				}

				// After removing itself from the cluster, etcd starts shutting
				// itself down: Removal is committed through Raft, etcd applies
				// it. During apply, etcd notices "Oh, it's me!" and begins
				// shutting down. Meanwhile, the request handler is still
				// finishing the RPC. If the shutdown signal reaches the request
				// path first, it may return ErrServerStopped.
				//
				// Although we cannot completely rule out the possibility that
				// external factors caused etcd to shut down, we still consider
				// the self-removal to have succeeded.
				if errors.Is(err, etcd.ErrServerStopped) {
					log.Info("Server is stopping, assuming removal to have succeeded")
					return true, nil
				}

				log.WithError(err).Error("Failed to remove member")
				return false, nil
			}

			log.Info("Left the cluster")
			return true, nil
		})
	}(); err != nil {
		return fmt.Errorf("failed to leave the cluster: %w", cmp.Or(context.Cause(ctx), err))
	}

	return nil
}

func (e *Etcd) setupCerts(ctx context.Context) error {
	etcdCaCert := filepath.Join(e.K0sVars.EtcdCertDir, "ca.crt")
	etcdCaCertKey := filepath.Join(e.K0sVars.EtcdCertDir, "ca.key")

	if err := e.CertManager.EnsureCA("etcd/ca", "etcd-ca", e.Config.CA.ExpiresAfter.Duration); err != nil {
		return fmt.Errorf("failed to create etcd ca: %w", err)
	}

	eg, _ := errgroup.WithContext(ctx)

	eg.Go(func() error {
		// etcd client cert
		etcdCertReq := certificate.Request{
			Name:   "apiserver-etcd-client",
			CN:     "apiserver-etcd-client",
			O:      "apiserver-etcd-client",
			CACert: etcdCaCert,
			CAKey:  etcdCaCertKey,
			Hostnames: []string{
				"127.0.0.1",
				"localhost",
			},
		}

		uid, err := users.LookupUID(constant.ApiserverUser)
		if err != nil {
			err = fmt.Errorf("failed to lookup UID for %q: %w", constant.ApiserverUser, err)
			uid = users.RootUID
			logrus.WithError(err).Warn("Files with key material for kube-apiserver user will be owned by root")
		}

		_, err = e.CertManager.EnsureCertificate(etcdCertReq, uid, e.Config.CA.CertificatesExpireAfter.Duration)
		return err
	})

	eg.Go(func() error {
		// etcd server cert
		etcdCertReq := certificate.Request{
			Name:   filepath.Join("etcd", "server"),
			CN:     "etcd-server",
			O:      "etcd-server",
			CACert: etcdCaCert,
			CAKey:  etcdCaCertKey,
			Hostnames: []string{
				"127.0.0.1",
				"localhost",
			},
		}

		_, err := e.CertManager.EnsureCertificate(etcdCertReq, e.uid, e.Config.CA.CertificatesExpireAfter.Duration)
		return err
	})

	eg.Go(func() error {
		etcdPeerCertReq := certificate.Request{
			Name:   filepath.Join("etcd", "peer"),
			CN:     e.Config.PeerAddress,
			O:      "etcd-peer",
			CACert: etcdCaCert,
			CAKey:  etcdCaCertKey,
			Hostnames: []string{
				e.Config.PeerAddress,
			},
		}
		_, err := e.CertManager.EnsureCertificate(etcdPeerCertReq, e.uid, e.Config.CA.CertificatesExpireAfter.Duration)
		return err
	})

	eg.Go(func() error {
		return e.CertManager.CreateKeyPair("etcd/jwt", e.K0sVars, e.uid)
	})

	return eg.Wait()
}

// Health-check interface
func (e *Etcd) Ready() error {
	logrus.WithField("component", "etcd").Debug("checking etcd endpoint for health")
	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()
	err := etcd.CheckEtcdReady(ctx, e.K0sVars.CertRootDir, e.K0sVars.EtcdCertDir, e.Config)
	return err
}

func detectUnsupportedEtcdArch() error {
	// https://github.com/etcd-io/etcd/blob/v3.6.12/server/etcdmain/etcd.go#L258-L279
	switch runtime.GOARCH {
	case "amd64", "arm64", "ppc64le", "s390x", os.Getenv("ETCD_UNSUPPORTED_ARCH"):
		return nil
	}

	return fmt.Errorf("running etcd on %s requires ETCD_UNSUPPORTED_ARCH=%s", runtime.GOARCH, runtime.GOARCH)
}

func recursiveChown(name string, uid, gid int) error {
	if uid == 0 {
		return nil
	}
	if dir.IsDirectory(name) {
		if err := filepath.Walk(name, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			return os.Chown(path, uid, gid)
		}); err != nil {
			return fmt.Errorf("can't chmod file `%s`: %w", name, err)
		}
		return nil
	}
	return os.Chown(name, uid, gid)
}
