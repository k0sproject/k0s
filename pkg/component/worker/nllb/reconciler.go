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

package nllb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	k0snet "github.com/k0sproject/k0s/internal/pkg/net"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/worker"
	workerconfig "github.com/k0sproject/k0s/pkg/component/worker/config"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"golang.org/x/exp/slices"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/sirupsen/logrus"
)

// Reconciler reconciles a static Pod on a worker node that implements
// node-local load balancing.
type Reconciler struct {
	log                        logrus.FieldLogger
	dataDir                    string
	runtimeDir                 string
	workerProfileName          string
	workerProfile              workerconfig.Profile
	regularKubeconfigPath      string
	loadBalancedKubeconfigPath string
	loadBalancer               backend

	mu    sync.Mutex
	state reconcilerState

	// valid when started
	stop func()
}

var (
	_ manager.Component = (*Reconciler)(nil)
	_ manager.Ready     = (*Reconciler)(nil)
)

type reconcilerState string

var (
	reconcilerCreated     reconcilerState = "created"
	reconcilerInitialized reconcilerState = "initialized"
	reconcilerStarted     reconcilerState = "started"
	reconcilerStopped     reconcilerState = "stopped"
)

// backend represents a load balancer backend that's managed by a [Reconciler].
type backend interface {
	init(context.Context) error
	start(ctx context.Context, profile workerconfig.Profile, apiServers []k0snet.HostPort) error

	getAPIServerAddress() (*k0snet.HostPort, error)
	updateAPIServers([]k0snet.HostPort) error

	stop()
}

// NewReconciler creates a component that reconciles a static Pod that
// implements node-local load balancing.
func NewReconciler(
	k0sVars *config.CfgVars,
	staticPods worker.StaticPods,
	workerProfileName string,
	workerProfile workerconfig.Profile,
) (*Reconciler, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		if runtime.GOOS == "windows" {
			runtimeDir = filepath.Join(k0sVars.DataDir)
		} else {
			runtimeDir = "/run/k0s"
		}
	} else {
		runtimeDir = filepath.Join(runtimeDir, "k0s")
	}
	runtimeDir = filepath.Join(runtimeDir, "nllb")

	var loadBalancer backend
	switch workerProfile.NodeLocalLoadBalancing.Type {
	case v1beta1.NllbTypeEnvoyProxy:
		loadBalancer = &envoyProxy{
			log:        logrus.WithFields(logrus.Fields{"component": "nllb.envoyProxy"}),
			dir:        filepath.Join(runtimeDir, "envoy"),
			staticPods: staticPods,
		}
	default:
		return nil, fmt.Errorf("unsupported node-local load balancing type: %q", workerProfile.NodeLocalLoadBalancing.Type)
	}

	r := &Reconciler{
		log:                        logrus.WithFields(logrus.Fields{"component": "nllb.Reconciler"}),
		dataDir:                    k0sVars.DataDir,
		runtimeDir:                 runtimeDir,
		workerProfileName:          workerProfileName,
		workerProfile:              workerProfile,
		regularKubeconfigPath:      k0sVars.KubeletAuthConfigPath,
		loadBalancedKubeconfigPath: filepath.Join(runtimeDir, "kubeconfig.yaml"),
		loadBalancer:               loadBalancer,

		state: reconcilerCreated,
	}

	return r, nil
}

func (r *Reconciler) GetKubeletKubeconfigPath() string {
	return r.loadBalancedKubeconfigPath
}

// NewClient returns a new Kubernetes client, backed by the node-local load balancer.
func (r *Reconciler) NewClient() (kubernetes.Interface, error) {
	return kubeutil.NewClientFromFile(r.loadBalancedKubeconfigPath)
}

func (r *Reconciler) Init(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != reconcilerCreated {
		return fmt.Errorf("cannot initialize, not created: %s", r.state)
	}
	if err := dir.Init(r.runtimeDir, 0700); err != nil {
		return err
	}
	if err := r.loadBalancer.init(ctx); err != nil {
		return err
	}

	r.state = reconcilerInitialized
	return nil
}

func (r *Reconciler) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != reconcilerInitialized {
		return fmt.Errorf("cannot start, not initialized: %s", r.state)
	}

	kubeconfig, err := readKubeconfig(r.regularKubeconfigPath)
	if err != nil {
		return err
	}

	apiServers := r.workerProfile.APIServerAddresses
	if len(apiServers) < 1 {
		apiServer, err := getAPIServerAddress(kubeconfig)
		if err != nil {
			return fmt.Errorf("failed to load initial API server address from %q: %w", r.regularKubeconfigPath, err)
		}
		apiServers = []k0snet.HostPort{*apiServer}
	}

	if err := r.loadBalancer.start(ctx, r.workerProfile, apiServers); err != nil {
		return fmt.Errorf("failed to start load balancer: %w", err)
	}

	lbAddr, err := r.loadBalancer.getAPIServerAddress()
	if err != nil {
		r.loadBalancer.stop()
		return fmt.Errorf("failed to obtain local address for node-local load balancing: %w", err)
	}

	if err := writePatchedKubeconfig(r.loadBalancedKubeconfigPath, kubeconfig, *lbAddr); err != nil {
		return fmt.Errorf("failed to write load-balanced kubeconfig file: %w", err)
	}

	reconcilerCtx, cancelReconciler := context.WithCancel(context.Background())
	reconcilerDone := make(chan struct{})

	go func() {
		defer close(reconcilerDone)
		r.runReconcileLoop(reconcilerCtx)
		r.log.Info("Reconciliation loop done")
	}()

	stop := func() {
		cancelReconciler()
		<-reconcilerDone
		r.loadBalancer.stop()
	}

	r.stop = stop
	r.state = reconcilerStarted
	return nil
}

func (r *Reconciler) Ready() error {
	if err := func() error {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.state != reconcilerStarted {
			return fmt.Errorf("cannot check for readiness, not started: %s", r.state)
		}
		return nil
	}(); err != nil {
		return err
	}

	// req, err := http.NewRequest(http.MethodGet, healthCheckURL, nil)
	// if err != nil {
	// 	return err
	// }

	// ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
	// defer cancel()

	// resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	// if err != nil {
	// 	return err
	// }
	// resp.Body.Close()
	// if resp.StatusCode != http.StatusNoContent {
	// 	return fmt.Errorf("unexpected HTTP response status: %s", resp.Status)
	// }

	return nil
}

func (r *Reconciler) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state == reconcilerStopped {
		return nil
	}

	if r.state != reconcilerStarted {
		return fmt.Errorf("cannot stop: %s", r.state)
	}

	r.stop()
	if err := os.Remove(r.loadBalancedKubeconfigPath); err != nil && !os.IsNotExist(err) {
		r.log.WithError(err).Warnf("Failed to remove load-balanced kubeconfig from disk")
	}

	r.stop = nil
	r.state = reconcilerStopped
	return nil
}

func (r *Reconciler) runReconcileLoop(ctx context.Context) {
	updates := make(chan workerconfig.Profile, 1)

	go func() {
		wait.UntilWithContext(ctx, func(ctx context.Context) {
			client, err := kubeutil.NewClientFromFile(r.loadBalancedKubeconfigPath)
			if err != nil {
				r.log.WithError(err).Error("Failed to create load-balanced Kubernetes client")
				return
			}

			err = workerconfig.WatchProfile(ctx, r.log, client, r.dataDir, r.workerProfileName,
				func(profile workerconfig.Profile) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case updates <- profile:
						return nil
					}
				},
			)
			if err != nil && !errors.Is(err, ctx.Err()) {
				r.log.WithError(err).Error("Failed to watch worker profiles")
			}
		}, 10*time.Second)
	}()

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	var desiredAPIServers, actualAPIServers []k0snet.HostPort
	for {
		select {
		case <-ctx.Done():
			return

		case profile := <-updates:
			if len(profile.APIServerAddresses) < 1 {
				r.log.Error("Refusing to remove all upstream API server addresses")
				continue
			}

			desiredAPIServers = slices.Clone(profile.APIServerAddresses)
			slices.SortFunc(desiredAPIServers, func(l, r k0snet.HostPort) bool {
				return l.String() < r.String()
			})

		case <-ticker.C:
			// Retry failed reconciliations every minute
		}

		if slices.Equal(desiredAPIServers, actualAPIServers) {
			continue
		}

		if err := r.loadBalancer.updateAPIServers(desiredAPIServers); err != nil {
			r.log.WithError(err).Error("Failed to update API server addresses")
		} else {
			actualAPIServers = desiredAPIServers
			r.log.Info("Updated API server addresses")
		}
	}
}

func readKubeconfig(path string) (*clientcmdapi.Config, error) {
	kubeconfig, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return nil, err
	}

	// Resolve non-absolute paths in case the kubeconfig gets written to another folder.
	err = clientcmd.ResolveLocalPaths(kubeconfig)
	if err != nil {
		return nil, err
	}

	if err := clientcmdapi.MinifyConfig(kubeconfig); err != nil {
		return nil, err
	}

	return kubeconfig, err
}

func getAPIServerAddress(kubeconfig *clientcmdapi.Config) (*k0snet.HostPort, error) {
	if len(kubeconfig.CurrentContext) < 1 {
		return nil, errors.New("current-context unspecified")
	}
	ctx, ok := kubeconfig.Contexts[kubeconfig.CurrentContext]
	if !ok {
		return nil, fmt.Errorf("current-context not found: %q", kubeconfig.CurrentContext)
	}
	cluster, ok := kubeconfig.Clusters[ctx.Cluster]
	if !ok {
		return nil, fmt.Errorf("cluster not found: %q", ctx.Cluster)
	}
	server, err := url.Parse(cluster.Server)
	if err != nil {
		return nil, fmt.Errorf("invalid server %q for cluster %q: %w", cluster.Server, ctx.Cluster, err)
	}

	var defaultPort uint16
	switch server.Scheme {
	case "https":
		defaultPort = 443
	case "http":
		defaultPort = 80
	default:
		return nil, fmt.Errorf("unsupported URL scheme %q for server %q for cluster %q", server.Scheme, cluster.Server, ctx.Cluster)
	}

	address, err := k0snet.ParseHostPortWithDefault(server.Host, defaultPort)
	if err != nil {
		return nil, fmt.Errorf("invalid server %q for cluster %q: %w", cluster.Server, ctx.Cluster, err)
	}

	return address, nil
}

func writePatchedKubeconfig(path string, kubeconfig *clientcmdapi.Config, server k0snet.HostPort) error {
	kubeconfig = kubeconfig.DeepCopy()
	if err := clientcmdapi.MinifyConfig(kubeconfig); err != nil {
		return err
	}

	cluster := kubeconfig.Clusters[kubeconfig.Contexts[kubeconfig.CurrentContext].Cluster]
	clusterServer, err := url.Parse(cluster.Server)
	if err != nil {
		return fmt.Errorf("invalid server: %w", err)
	}
	clusterServer.Host = server.String()
	cluster.Server = clusterServer.String()

	bytes, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return err
	}

	return file.WriteContentAtomically(path, bytes, constant.CertSecureMode)
}

func getLoopbackIP(ctx context.Context) (net.IP, error) {
	localIPs, err := net.DefaultResolver.LookupIPAddr(ctx, "localhost")
	if err != nil {
		err = fmt.Errorf("failed to resolve localhost: %w", err)
	} else {
		for _, addr := range localIPs {
			if addr.IP.IsLoopback() {
				return addr.IP, nil
			}
		}

		err = fmt.Errorf("no loopback IPs found for localhost: %v", localIPs)
	}

	return net.IP{127, 0, 0, 1}, err
}
