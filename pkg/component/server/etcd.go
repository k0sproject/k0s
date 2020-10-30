/*
Copyright 2020 Mirantis, Inc.

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
package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mirantis/mke/pkg/apis/v1beta1"
	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/certificate"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/etcd"
	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	etcdCaCert     = filepath.Join(constant.EtcdCertDir, "ca.crt")
	etcdCaCertKey  = filepath.Join(constant.EtcdCertDir, "ca.key")
	etcdServerCert = filepath.Join(constant.EtcdCertDir, "server.crt")
	etcdServerKey  = filepath.Join(constant.EtcdCertDir, "server.key")
	etcdPeerCert   = filepath.Join(constant.EtcdCertDir, "peer.crt")
	etcdPeerKey    = filepath.Join(constant.EtcdCertDir, "peer.key")
)

// Etcd implement the component interface to run etcd
type Etcd struct {
	Config      *config.EtcdConfig
	Join        bool
	JoinClient  *v1beta1.JoinClient
	CertManager certificate.Manager

	supervisor supervisor.Supervisor
	uid        int
	gid        int
}

// Init extracts the needed binaries
func (e *Etcd) Init() error {
	var err error
	e.uid, err = util.GetUID(constant.EtcdUser)
	if err != nil {
		logrus.Warning(errors.Wrap(err, "Running etcd as root"))
	}
	e.gid, _ = util.GetGID(constant.Group)

	err = util.InitDirectory(constant.EtcdDataDir, constant.EtcdDataDirMode) // https://docs.datadoghq.com/security_monitoring/default_rules/cis-kubernetes-1.5.1-1.1.11/
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", constant.EtcdDataDir)
	}

	err = util.InitDirectory(constant.EtcdCertDir, constant.EtcdCertDirMode) // https://docs.datadoghq.com/security_monitoring/default_rules/cis-kubernetes-1.5.1-4.1.7/
	if err != nil {
		return errors.Wrapf(err, "failed to create etcd cert dir")
	}

	for _, f := range []string{constant.EtcdDataDir, constant.EtcdCertDir} {
		err = os.Chown(f, e.uid, e.gid)
		if err != nil {
			return err
		}
	}

	for _, f := range []string{
		"ca.crt",
		"server.crt",
		"server.key",
	} {
		if err := os.Chown(path.Join(constant.EtcdCertDir, f), e.uid, e.gid); err != nil {
			// TODO: directory may not yet exist. log it and wait for retry for now
			logrus.Errorf("failed to chown %s: %s", f, err)
		}
	}

	return assets.Stage(constant.BinDir, "etcd", constant.BinDirMode, constant.Group)
}

// Run runs etcd
func (e *Etcd) Run() error {
	logrus.Info("Starting etcd")

	name, err := os.Hostname()
	if err != nil {
		return err
	}

	peerURL := fmt.Sprintf("https://%s:2380", e.Config.PeerAddress)
	args := []string{
		fmt.Sprintf("--data-dir=%s", constant.EtcdDataDir),
		"--listen-client-urls=https://127.0.0.1:2379",
		"--advertise-client-urls=https://127.0.0.1:2379",
		"--client-cert-auth=true",
		fmt.Sprintf("--listen-peer-urls=%s", peerURL),
		fmt.Sprintf("--initial-advertise-peer-urls=%s", peerURL),
		fmt.Sprintf("--name=%s", name),
		fmt.Sprintf("--trusted-ca-file=%s", etcdCaCert),
		fmt.Sprintf("--cert-file=%s", etcdServerCert),
		fmt.Sprintf("--key-file=%s", etcdServerKey),
		fmt.Sprintf("--peer-trusted-ca-file=%s", etcdCaCert),
		fmt.Sprintf("--peer-key-file=%s", etcdPeerKey),
		fmt.Sprintf("--peer-cert-file=%s", etcdPeerCert),
		"--peer-client-cert-auth=true",
		"--enable-pprof=false",
	}

	if util.FileExists(filepath.Join(constant.EtcdDataDir, "member", "snap", "db")) {
		logrus.Warnf("etcd db file(s) already exist, not gonna run join process")
		e.Join = false
	}

	if e.Join {
		logrus.Infof("starting to sync etcd config")
		etcdResponse, err := e.JoinClient.JoinEtcd(peerURL)
		if err != nil {
			return err
		}
		logrus.Infof("got cluster info: %v", etcdResponse.InitialCluster)
		// Write etcd ca cert&key
		if util.FileExists(etcdCaCert) && util.FileExists(etcdCaCertKey) {
			logrus.Warnf("etcd ca certs already exists, not gonna overwrite. If you wish to re-sync them, delete the existing ones.")
		} else {
			err = ioutil.WriteFile(etcdCaCertKey, etcdResponse.CA.Key, constant.CertSecureMode)
			if err != nil {
				return err
			}

			err = ioutil.WriteFile(etcdCaCert, etcdResponse.CA.Cert, constant.CertSecureMode)
			if err != nil {
				return err
			}
			for _, f := range []string{filepath.Dir(etcdCaCertKey), etcdCaCertKey, etcdCaCert} {
				if err := os.Chown(f, e.uid, e.gid); err != nil {
					return err
				}
			}
		}

		args = append(args, fmt.Sprintf("--initial-cluster=%s", strings.Join(etcdResponse.InitialCluster, ",")))
		args = append(args, "--initial-cluster-state=existing")
	}

	if err := e.setupCerts(); err != nil {
		return errors.Wrap(err, "failed to create etcd certs")
	}

	logrus.Infof("starting etcd with args: %v", args)

	e.supervisor = supervisor.Supervisor{
		Name:    "etcd",
		BinPath: assets.BinPath("etcd"),
		Dir:     constant.DataDir,
		Args:    args,
		UID:     e.uid,
		GID:     e.gid,
	}

	e.supervisor.Supervise()

	return nil
}

// Stop stops etcd
func (e *Etcd) Stop() error {
	return e.supervisor.Stop()
}

func (e *Etcd) setupCerts() error {
	if err := e.CertManager.EnsureCA("etcd/ca", "etcd-ca"); err != nil {
		return errors.Wrap(err, "failed to create etcd ca")
	}

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
	if _, err := e.CertManager.EnsureCertificate(etcdCertReq, constant.ApiserverUser); err != nil {
		return err
	}
	// etcd server cert
	etcdCertReq = certificate.Request{
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
	if _, err := e.CertManager.EnsureCertificate(etcdCertReq, constant.EtcdUser); err != nil {
		return err
	}

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
	if _, err := e.CertManager.EnsureCertificate(etcdPeerCertReq, constant.EtcdUser); err != nil {
		return err
	}

	return nil
}

// Health-check interface
func (e *Etcd) Healthy() error {
	if err := waitForHealthy(); err != nil {
		return err
	}
	return nil
}

// waitForHealthy waits until etcd is healthy and returns true upon success. If a timeout occurs, it returns false
func waitForHealthy() error {
	log := logrus.WithField("component", "etcd")
	ctx, cancelFunction := context.WithTimeout(context.Background(), 2*time.Minute)

	// clear up context after timeout
	defer cancelFunction()

	// loop forever, until the context is canceled or until etcd is healthy
	ticker := time.NewTicker(3 * time.Second)
	for {
		select {
		case <-ticker.C:
			log.Debug("checking etcd endpoint for health")
			err := etcd.CheckEtcdReady()
			if err != nil {
				log.Errorf("health-check: etcd might be down: %v", err)
			} else {
				log.Debug("etcd is healthy. closing check")
				return nil
			}
		case <-ctx.Done():
			return fmt.Errorf("etcd health-check timed out")
		}
	}
}
