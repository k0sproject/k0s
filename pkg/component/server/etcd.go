package server

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Mirantis/mke/pkg/apis/v1beta1"
	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/certificate"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	certDir    string
}

// Init extracts the needed binaries
func (e *Etcd) Init() error {
	var err error
	e.uid, err = util.GetUID(constant.EtcdUser)
	if err != nil {
		logrus.Warning(errors.Wrap(err, "Running etcd as root"))
	}

	err = util.InitDirectory(constant.EtcdDataDir, constant.EtcdDataDirMode) // https://docs.datadoghq.com/security_monitoring/default_rules/cis-kubernetes-1.5.1-1.1.11/
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", constant.EtcdDataDir)
	}

	e.gid, _ = util.GetGID(constant.Group)

	e.certDir = path.Join(constant.CertRoot, "etcd")
	for _, f := range []string{
		"ca.crt",
		"server.crt",
		"server.key",
	} {
		if err := os.Chown(path.Join(e.certDir, f), e.uid, e.gid); err != nil {
			// TODO: due to init race the only thing here is to log it and wait for retry
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
		fmt.Sprintf("--trusted-ca-file=%s", path.Join(e.certDir, "ca.crt")),
		fmt.Sprintf("--cert-file=%s", path.Join(e.certDir, "server.crt")),
		fmt.Sprintf("--key-file=%s", path.Join(e.certDir, "server.key")),
		fmt.Sprintf("--peer-trusted-ca-file=%s", path.Join(e.certDir, "ca.crt")),
		fmt.Sprintf("--peer-key-file=%s", path.Join(e.certDir, "peer.key")),
		fmt.Sprintf("--peer-cert-file=%s", path.Join(e.certDir, "peer.crt")),
		"--peer-client-cert-auth=true",
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
		etcdCaCertPath, etcdCaCertKey := filepath.Join(constant.CertRoot, "etcd", "ca.crt"), filepath.Join(constant.CertRoot, "etcd", "ca.key")
		if util.FileExists(etcdCaCertPath) && util.FileExists(etcdCaCertKey) {
			logrus.Warnf("etcd ca certs already exists, not gonna overwrite. If you wish to re-sync them, delete the existing ones.")
		} else {
			err := util.InitDirectory(filepath.Dir(etcdCaCertKey), constant.CertRootSecureMode) // https://docs.datadoghq.com/security_monitoring/default_rules/cis-kubernetes-1.5.1-4.1.7/
			if err != nil {
				return errors.Wrapf(err, "failed to create etcd cert dir")
			}
			err = ioutil.WriteFile(etcdCaCertKey, etcdResponse.CA.Key, constant.CertRootSecureMode)
			if err != nil {
				return err
			}

			err = ioutil.WriteFile(etcdCaCertPath, etcdResponse.CA.Cert, constant.CertRootSecureMode)
			if err != nil {
				return err
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
	etcdCaCertPath, etcdCaCertKey := filepath.Join(constant.CertRoot, "etcd", "ca.crt"), filepath.Join(constant.CertRoot, "etcd", "ca.key")
	// etcd client cert
	etcdCertReq := certificate.Request{
		Name:   "apiserver-etcd-client",
		CN:     "apiserver-etcd-client",
		O:      "apiserver-etcd-client",
		CACert: etcdCaCertPath,
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
		CACert: etcdCaCertPath,
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
		CACert: etcdCaCertPath,
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
