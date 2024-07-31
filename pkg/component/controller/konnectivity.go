/*
Copyright 2020 k0s authors

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

	"github.com/sirupsen/logrus"

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
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

// Konnectivity implements the component interface for konnectivity server
type Konnectivity struct {
	K0sVars    *config.CfgVars
	LogLevel   string
	SingleNode bool
	// used for lease lock
	KubeClientFactory          kubeutil.ClientFactoryInterface
	NodeConfig                 *v1beta1.ClusterConfig
	K0sControllersLeaseCounter *K0sControllersLeaseCounter

	supervisor      *supervisor.Supervisor
	uid             int
	serverCount     int
	serverCountChan <-chan int
	stopFunc        context.CancelFunc
	clusterConfig   *v1beta1.ClusterConfig
	log             *logrus.Entry

	*prober.EventEmitter
}

var _ manager.Component = (*Konnectivity)(nil)
var _ manager.Ready = (*Konnectivity)(nil)
var _ prober.Healthz = (*Konnectivity)(nil)

// Init ...
func (k *Konnectivity) Init(ctx context.Context) error {
	var err error
	k.uid, err = users.GetUID(constant.KonnectivityServerUser)
	if err != nil {
		k.EmitWithPayload("error getting UID for", err)
		logrus.Warn("running konnectivity as root: ", err)
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
	if err := assets.Stage(k.K0sVars.BinDir, "konnectivity-server", constant.BinDirMode); err != nil {
		k.EmitWithPayload("failed to stage konnectivity-server", err)
		return fmt.Errorf("failed to stage konnectivity-server binary %w", err)

	}
	defer k.Emit("successfully initialized konnectivity component")

	k.clusterConfig = k0scontext.GetNodeConfig(ctx)

	return nil
}

// Run ..
func (k *Konnectivity) Start(ctx context.Context) error {
	// Buffered chan to send updates for the count of servers
	k.serverCountChan = k.K0sControllersLeaseCounter.Subscribe()

	// To make the server start, add "dummy" 0 into the channel
	if err := k.runServer(0); err != nil {
		k.EmitWithPayload("failed to run konnectivity server", err)
		return fmt.Errorf("failed to run konnectivity server: %w", err)
	}

	go k.watchControllerCountChanges(ctx)

	return nil
}

func (k *Konnectivity) serverArgs(count int) []string {
	return stringmap.StringMap{
		"--uds-name":                 filepath.Join(k.K0sVars.KonnectivitySocketDir, "konnectivity-server.sock"),
		"--cluster-cert":             filepath.Join(k.K0sVars.CertRootDir, "server.crt"),
		"--cluster-key":              filepath.Join(k.K0sVars.CertRootDir, "server.key"),
		"--kubeconfig":               k.K0sVars.KonnectivityKubeConfigPath,
		"--mode":                     "grpc",
		"--server-port":              "0",
		"--agent-port":               fmt.Sprintf("%d", k.clusterConfig.Spec.Konnectivity.AgentPort),
		"--admin-port":               fmt.Sprintf("%d", k.clusterConfig.Spec.Konnectivity.AdminPort),
		"--health-bind-address":      "localhost",
		"--health-port":              "8092",
		"--agent-namespace":          "kube-system",
		"--agent-service-account":    "konnectivity-agent",
		"--authentication-audience":  "system:konnectivity-server",
		"--logtostderr":              "true",
		"--stderrthreshold":          "1",
		"--v":                        k.LogLevel,
		"--enable-profiling":         "false",
		"--delete-existing-uds-file": "true",
		"--server-count":             strconv.Itoa(count),
		"--server-id":                k.K0sVars.InvocationID,
		"--proxy-strategies":         "destHost,default",
		"--cipher-suites":            constant.AllowedTLS12CipherSuiteNames(),
	}.ToArgs()
}

// runs the supervisor and restarts if the calculated server count changes
func (k *Konnectivity) watchControllerCountChanges(ctx context.Context) {
	// previousArgs := stringmap.StringMap{}
	for {
		k.log.Debug("waiting for server count change")
		select {
		case <-ctx.Done():
			k.Emit("stopped konnectivity server")
			logrus.Info("stopping konnectivity server reconfig loop")
			return
		case count := <-k.serverCountChan:
			if k.clusterConfig == nil {
				k.Emit("skipping konnectivity server start, cluster config not yet available")
				continue
			}
			// restart only if the count actually changes and we've got the global config
			if count != k.serverCount {
				if err := k.runServer(count); err != nil {
					k.EmitWithPayload("failed to run konnectivity server", err)
					logrus.Errorf("failed to run konnectivity server: %s", err)
					continue
				}
			}
			k.serverCount = count
		}
	}
}

func (k *Konnectivity) runServer(count int) error {
	// Stop supervisor
	if k.supervisor != nil {
		k.EmitWithPayload("restarting konnectivity server due to server count change",
			map[string]interface{}{"serverCount": count})
		k.supervisor.Stop()
	}

	k.supervisor = &supervisor.Supervisor{
		Name:    "konnectivity",
		BinPath: assets.BinPath("konnectivity-server", k.K0sVars.BinDir),
		DataDir: k.K0sVars.DataDir,
		RunDir:  k.K0sVars.RunDir,
		Args:    k.serverArgs(count),
		UID:     k.uid,
	}
	err := k.supervisor.Supervise()
	if err != nil {
		k.EmitWithPayload("failed to run konnectivity server", err)
		k.log.Errorf("failed to start konnectivity supervisor: %s", err)
		k.supervisor = nil // not to make the next loop to try to stop it first
		return err
	}
	k.serverCount = count
	k.EmitWithPayload("started konnectivity server", map[string]interface{}{"serverCount": count})

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
	if k.stopFunc != nil {
		logrus.Debug("closing konnectivity component context")
		k.stopFunc()
	}
	if k.supervisor == nil {
		return nil
	}
	logrus.Debug("about to stop konnectivity supervisor")
	k.supervisor.Stop()
	return nil
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
