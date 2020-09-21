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

	supervisor  supervisor.Supervisor
	uid         int
	gid         int
	etcdDataDir string
	certDir     string
}

// Init extracts the needed binaries
func (e *Etcd) Init() error {
	var err error
	e.uid, err = util.GetUID(constant.EtcdUser)
	if err != nil {
		logrus.Warning(errors.Wrap(err, "Running etcd as root"))
	}

	e.etcdDataDir = path.Join(constant.DataDir, "etcd")
	err = os.MkdirAll(e.etcdDataDir, 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", e.etcdDataDir)
	}

	e.gid, _ = util.GetGID(constant.Group)

	err = os.Chown(e.etcdDataDir, e.uid, e.gid)
	if err != nil {
		return errors.Wrapf(err, "failed to chown %s", e.etcdDataDir)
	}

	e.certDir = path.Join(constant.CertRoot, "etcd")
	os.Chown(path.Join(e.certDir, "ca.crt"), e.uid, e.gid)
	os.Chown(path.Join(e.certDir, "server.crt"), e.uid, e.gid)
	os.Chown(path.Join(e.certDir, "server.key"), e.uid, e.gid)

	return assets.Stage(constant.DataDir, path.Join("bin", "etcd"), constant.Group)
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
		fmt.Sprintf("--data-dir=%s", e.etcdDataDir),
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

	if util.FileExists(filepath.Join(e.etcdDataDir, "member", "snap", "db")) {
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
			err := os.MkdirAll(filepath.Dir(etcdCaCertKey), 0750)
			if err != nil {
				return errors.Wrapf(err, "failed to create etcd cert dir")
			}
			err = ioutil.WriteFile(etcdCaCertKey, etcdResponse.CA.Key, 0600)
			if err != nil {
				return err
			}

			err = ioutil.WriteFile(etcdCaCertPath, etcdResponse.CA.Cert, 0640)
			if err != nil {
				return err
			}
		}

		args = append(args, fmt.Sprintf("--initial-cluster=%s", strings.Join(etcdResponse.InitialCluster, ",")))
		args = append(args, "--initial-cluster-state=existing")
	}

	if err := e.setupCerts(); err != nil {
		return err
	}

	logrus.Infof("starting etcd with args: %v", args)

	e.supervisor = supervisor.Supervisor{
		Name:    "etcd",
		BinPath: assets.StagedBinPath(constant.DataDir, "etcd"),
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
		return err
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
