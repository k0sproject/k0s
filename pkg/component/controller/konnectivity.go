// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/k0scontext"
	"github.com/k0sproject/k0s/pkg/supervisor"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sirupsen/logrus"
)

// Konnectivity implements the component interface for konnectivity server
type Konnectivity struct {
	K0sVars     *config.CfgVars
	LogLevel    string
	ServerCount func() (uint, <-chan struct{})

	supervisor     *supervisor.Supervisor
	executablePath string
	uid            int

	clusterConfig *v1beta1.ClusterConfig
	log           *logrus.Entry

	*prober.EventEmitter

	stop func()
}

var _ manager.Component = (*Konnectivity)(nil)
var _ manager.Ready = (*Konnectivity)(nil)
var _ prober.Healthz = (*Konnectivity)(nil)

// Init ...
func (k *Konnectivity) Init(ctx context.Context) error {
	var err error
	k.uid, err = users.LookupUID(constant.KonnectivityServerUser)
	if err != nil {
		err = fmt.Errorf("failed to lookup UID for %q: %w", constant.KonnectivityServerUser, err)
		k.uid = users.RootUID
		k.EmitWithPayload("error getting UID for", err)
		logrus.WithError(err).Warn("Running konnectivity as root")
	}
	err = dir.Init(k.K0sVars.KonnectivitySocketDir, 0755)
	if err != nil {
		k.EmitWithPayload("failed to initialize socket directory", err)
		return fmt.Errorf("failed to initialize directory %s: %w", k.K0sVars.KonnectivitySocketDir, err)
	}

	err = os.Chown(k.K0sVars.KonnectivitySocketDir, k.uid, -1)
	if err != nil && os.Geteuid() == 0 {
		k.EmitWithPayload("failed to chown socket directory", err)
		return fmt.Errorf("failed to chown %s: %w", k.K0sVars.KonnectivitySocketDir, err)
	}

	k.log = logrus.WithFields(logrus.Fields{"component": "konnectivity"})
	if k.executablePath, err = assets.StageExecutable(k.K0sVars.BinDir, "konnectivity-server"); err != nil {
		k.EmitWithPayload("failed to stage konnectivity-server", err)
		return fmt.Errorf("failed to stage konnectivity-server binary: %w", err)

	}
	defer k.Emit("successfully initialized konnectivity component")

	k.clusterConfig = k0scontext.GetNodeConfig(ctx)

	return nil
}

// Run ..
func (k *Konnectivity) Start(ctx context.Context) error {
	serverCount, serverCountChanged := k.ServerCount()

	if err := k.runServer(ctx, serverCount); err != nil {
		k.EmitWithPayload("failed to start konnectivity server", err)
		return fmt.Errorf("failed to start konnectivity server: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		var retry <-chan time.Time
		for {
			select {
			case <-serverCountChanged:
				prevServerCount := serverCount
				serverCount, serverCountChanged = k.ServerCount()
				// restart only if the server count actually changed
				if serverCount == prevServerCount {
					continue
				}

			case <-retry:
				k.Emit("retrying to start konnectivity server")
				k.log.Info("Retrying to start konnectivity server")

			case <-ctx.Done():
				k.Emit("stopped konnectivity server")
				k.log.Info("stopping konnectivity server reconfig loop")
				return
			}

			retry = nil

			if err := k.runServer(ctx, serverCount); err != nil {
				k.EmitWithPayload("failed to start konnectivity server", err)
				k.log.WithError(err).Errorf("Failed to start konnectivity server")
				retry = time.After(10 * time.Second)
				continue
			}
		}
	}()

	k.stop = func() { cancel(); <-done }

	return nil
}

func (k *Konnectivity) serverArgs(count uint) []string {
	return stringmap.StringMap{
		"--uds-name":                 filepath.Join(k.K0sVars.KonnectivitySocketDir, "konnectivity-server.sock"),
		"--cluster-cert":             filepath.Join(k.K0sVars.CertRootDir, "server.crt"),
		"--cluster-key":              filepath.Join(k.K0sVars.CertRootDir, "server.key"),
		"--kubeconfig":               k.K0sVars.KonnectivityKubeConfigPath,
		"--mode":                     "grpc",
		"--server-port":              "0",
		"--agent-port":               strconv.FormatInt(int64(k.clusterConfig.Spec.Konnectivity.AgentPort), 10),
		"--admin-port":               strconv.FormatInt(int64(k.clusterConfig.Spec.Konnectivity.AdminPort), 10),
		"--health-bind-address":      "localhost",
		"--health-port":              "8092",
		"--agent-namespace":          metav1.NamespaceSystem,
		"--agent-service-account":    "konnectivity-agent",
		"--authentication-audience":  "system:konnectivity-server",
		"--logtostderr":              "true",
		"--stderrthreshold":          "1",
		"--v":                        k.LogLevel,
		"--enable-profiling":         "false",
		"--delete-existing-uds-file": "true",
		"--server-count":             strconv.FormatUint(uint64(count), 10),
		"--server-id":                k.K0sVars.InvocationID,
		"--proxy-strategies":         "destHost,defaultRoute,default",
		"--cipher-suites":            constant.AllowedTLS12CipherSuiteNames(),
	}.ToArgs()
}

func (k *Konnectivity) runServer(ctx context.Context, count uint) error {
	// Stop supervisor
	if k.supervisor != nil {
		k.EmitWithPayload("restarting konnectivity server due to server count change",
			map[string]any{"serverCount": count})
		if err := k.supervisor.Stop(); err != nil {
			k.log.WithError(err).Error("Failed to stop executable")
		}
	}

	k.supervisor = &supervisor.Supervisor{
		Name:    "konnectivity",
		BinPath: k.executablePath,
		DataDir: k.K0sVars.DataDir,
		RunDir:  k.K0sVars.RunDir,
		Args:    k.serverArgs(count),
		UID:     k.uid,
	}
	err := k.supervisor.Supervise(ctx)
	if err != nil {
		k.supervisor = nil // not to make the next loop to try to stop it first
		return err
	}
	k.EmitWithPayload("started konnectivity server", map[string]any{"serverCount": count})

	return nil
}

// Ready implements manager.Ready.
func (k *Konnectivity) Ready() error {
	ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
	defer cancel()
	// This is somehow flipped: Check healthz instead of readyz.
	return k.health(ctx, "/healthz")
}

// Healthy implements prober.Healthz.
func (k *Konnectivity) Healthy() error {
	ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
	defer cancel()
	// This is somehow flipped: Check readyz instead of healthz.
	return k.health(ctx, "/readyz")
}

// Stop stops
func (k *Konnectivity) Stop() error {
	if k.stop != nil {
		logrus.Debug("stopping konnectivity component")
		k.stop()
	}
	if k.supervisor == nil {
		return nil
	}
	logrus.Debug("about to stop konnectivity supervisor")
	return k.supervisor.Stop()
}

func (k *Konnectivity) health(ctx context.Context, path string) error {
	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return fmt.Errorf("no redirects allowed: %s", req.URL)
		},
	}

	endpoint := url.URL{Scheme: "http", Host: "localhost:8092", Path: path}
	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected HTTP response status: %s", resp.Status)
	}

	return nil
}
