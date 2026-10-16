// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	k0snet "github.com/k0sproject/k0s/internal/pkg/net"
	k0spatches "github.com/k0sproject/k0s/internal/pkg/patches"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/worker"
	workerconfig "github.com/k0sproject/k0s/pkg/component/worker/config"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"sigs.k8s.io/yaml"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
)

// containerFileLabel is the SELinux label applied to files that need to be
// readable by containers. Containers typically run with the container_t type,
// which can read files labeled with container_file_t.
// This is needed in NLLB since we create files/dirs on the host and
// bind-mount them into the Envoy container, and thus they need to be readable by the container runtime.
// These come from https://github.com/containers/container-selinux
// which we instruct people to install in the k0s installation docs if they use SELinux.
const containerFileLabel string = "system_u:object_r:container_file_t:s0"

// eventReasonPatchFailed is the reason of the Node event that's recorded
// when the patches couldn't be applied to the node-local load balancer.
const eventReasonPatchFailed = "NodeLocalLoadBalancingPatchFailed"

// eventRetryDelay is the delay between attempts to record a Node event.
var eventRetryDelay = 1 * time.Second

// Reconciler reconciles a static Pod on a worker node that implements
// node-local load balancing.
type Reconciler struct {
	log                        logrus.FieldLogger
	nodeName                   apitypes.NodeName
	dataDir                    string
	runtimeDir                 string
	workerProfileName          string
	workerProfile              workerconfig.Profile
	regularKubeconfigPath      string
	loadBalancedKubeconfigPath string
	loadBalancer               backend

	// Creates the client used to record Node events.
	newEventClient func() (kubernetes.Interface, error)

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
	nodeName apitypes.NodeName,
) (*Reconciler, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		if runtime.GOOS == "windows" {
			runtimeDir = k0sVars.DataDir
		} else {
			runtimeDir = "/run/k0s"
		}
	} else {
		runtimeDir = filepath.Join(runtimeDir, "k0s")
	}
	runtimeDir = filepath.Join(runtimeDir, "nllb")

	r := &Reconciler{
		log:                        logrus.WithFields(logrus.Fields{"component": "nllb.Reconciler"}),
		nodeName:                   nodeName,
		dataDir:                    k0sVars.DataDir,
		runtimeDir:                 runtimeDir,
		workerProfileName:          workerProfileName,
		workerProfile:              workerProfile,
		regularKubeconfigPath:      k0sVars.KubeletAuthConfigPath,
		loadBalancedKubeconfigPath: filepath.Join(runtimeDir, "kubeconfig.yaml"),

		state: reconcilerCreated,
	}

	// Use the regular kubeconfig, as it doesn't depend on the load balancer.
	r.newEventClient = func() (kubernetes.Interface, error) {
		return kubeutil.NewClientFromFile(r.regularKubeconfigPath)
	}

	switch workerProfile.NodeLocalLoadBalancing.Type {
	case v1beta1.NllbTypeEnvoyProxy:
		r.loadBalancer = &envoyProxy{
			log:        logrus.WithFields(logrus.Fields{"component": "nllb.envoyProxy"}),
			dir:        filepath.Join(runtimeDir, "envoy"),
			staticPods: staticPods,
			patchPod:   r.patchPod,
		}
	case v1beta1.NllbTypeTraefik:
		r.loadBalancer = &traefik{
			log:        logrus.WithFields(logrus.Fields{"component": "nllb.traefik"}),
			dir:        filepath.Join(runtimeDir, "traefik"),
			staticPods: staticPods,
			patchPod:   r.patchPod,
		}
	default:
		return nil, fmt.Errorf("unsupported node-local load balancing type: %q", workerProfile.NodeLocalLoadBalancing.Type)
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
	if err := dir.InitWithOptions(r.runtimeDir).
		WithPermissions(0700).
		WithSELinuxLabel(containerFileLabel).Apply(); err != nil {
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
			slices.SortFunc(desiredAPIServers, func(l, r k0snet.HostPort) int {
				return strings.Compare(l.String(), r.String())
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

// podPatcher applies the user-supplied patches to a load balancer's pod
// manifest. It always returns a pod that can be provisioned.
type podPatcher func(ctx context.Context, pod *corev1.Pod, patches v1beta1.Patches) *corev1.Pod

// patchPod is the [podPatcher] that's used by the load balancers. If the
// patches can't be applied, the load balancer is provisioned without them, so
// that the worker and everything that depends on the load balancer keeps on
// working.
func (r *Reconciler) patchPod(ctx context.Context, pod *corev1.Pod, patches v1beta1.Patches) *corev1.Pod {
	patchedPod, err := patchPod(pod, patches)
	if err != nil {
		r.log.WithError(err).Error("Failed to apply patches to the node-local load balancer, running it without patches")
		if err := r.recordPatchFailedEvent(ctx, err); err != nil {
			r.log.WithError(err).Error("Failed to record Node event")
		}
		return pod
	}

	return patchedPod
}

// recordPatchFailedEvent records a Warning event on this Node, so that the
// failure is visible in the cluster, and not only in the worker logs.
func (r *Reconciler) recordPatchFailedEvent(ctx context.Context, cause error) error {
	client, err := r.newEventClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	now := metav1.Now()
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: string(r.nodeName) + ".",
			Namespace:    metav1.NamespaceDefault,
		},
		// Use the node name as UID. 'kubectl describe node' only finds events
		// whose UID is either the Node's actual UID, or the node name (which is
		// what older kubelets used). The actual UID isn't known if the Node
		// hasn't been registered yet, and events without any UID aren't found
		// at all.
		InvolvedObject: corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Node",
			Name:       string(r.nodeName),
			UID:        apitypes.UID(r.nodeName),
		},
		Type:   corev1.EventTypeWarning,
		Reason: eventReasonPatchFailed,
		Message: "Failed to apply patches to the node-local load balancer, running it without patches. " +
			"Fix the patches in the cluster configuration and restart the worker. Error: " + cause.Error(),
		Source:              corev1.EventSource{Component: "nllb", Host: string(r.nodeName)},
		ReportingController: "nllb",
		ReportingInstance:   string(r.nodeName),
		FirstTimestamp:      now,
		LastTimestamp:       now,
		Count:               1,
	}

	return retry.Do(
		func() error {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			_, err := client.CoreV1().Events(metav1.NamespaceDefault).Create(ctx, event, metav1.CreateOptions{})
			return err
		},
		retry.Context(ctx),
		retry.Attempts(3),
		retry.Delay(eventRetryDelay),
		retry.LastErrorOnly(true),
	)
}

func patchPod(pod *corev1.Pod, patches v1beta1.Patches) (*corev1.Pod, error) {
	if len(patches) == 0 {
		return pod, nil
	}

	// patches work via marshaled data, so convert to pod to yaml and back to apply the patches
	podBytes, err := yaml.Marshal(pod)
	if err != nil {
		return nil, err
	}

	patchedBytes, err := k0spatches.Apply(podBytes, patches)
	if err != nil {
		return nil, err
	}

	var patchedPod corev1.Pod
	if err := yaml.Unmarshal(patchedBytes, &patchedPod); err != nil {
		return nil, err
	}

	return &patchedPod, nil
}
