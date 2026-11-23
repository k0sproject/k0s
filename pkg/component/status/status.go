// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type Stater interface {
	State(maxCount int) prober.State
}

type Status struct {
	StatusInformation       K0sStatus
	Prober                  Stater
	Socket                  string
	L                       *logrus.Entry
	httpserver              http.Server
	GetWorkerClientFactory  func() kubeutil.ClientFactoryInterface
	ControllerClientFactory kubeutil.ClientFactoryInterface
	// HasWindowsNodes is the Windows stack counter. A nil bool means the count is not known yet.
	HasWindowsNodes func() (*bool, <-chan struct{})
	network         atomic.Pointer[v1beta1.Network]
}

var _ manager.Component = (*Status)(nil)

const statusCheckTimeout = 5 * time.Second

func (s *Status) ClusterConfigReconciler() manager.Component {
	return &clusterConfigUpdater{status: s}
}

type clusterConfigUpdater struct {
	status *Status
}

var (
	_ manager.Component  = (*clusterConfigUpdater)(nil)
	_ manager.Reconciler = (*clusterConfigUpdater)(nil)
)

func (c *clusterConfigUpdater) Init(context.Context) error  { return nil }
func (c *clusterConfigUpdater) Start(context.Context) error { return nil }
func (c *clusterConfigUpdater) Stop() error                 { return nil }

func (c *clusterConfigUpdater) Reconcile(_ context.Context, cfg *v1beta1.ClusterConfig) error {
	var network *v1beta1.Network
	if cfg != nil && cfg.Spec != nil {
		network = cfg.Spec.Network
	}
	c.status.network.Store(network)
	return nil
}

const defaultMaxEvents = 5

// Init initializes component
func (s *Status) Init(_ context.Context) error {
	s.L = logrus.WithFields(logrus.Fields{"component": "status"})
	mux := http.NewServeMux()
	mux.Handle("/status", &statusHandler{Status: s})
	mux.HandleFunc("/components", func(w http.ResponseWriter, r *http.Request) {
		maxCount, err := strconv.ParseInt(r.URL.Query().Get("maxCount"), 10, 32)
		if err != nil {
			maxCount = defaultMaxEvents
		}
		w.Header().Set("Content-Type", "application/json")
		if json.NewEncoder(w).Encode(s.Prober.State(int(maxCount))) != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	s.httpserver = http.Server{
		Handler: mux,
	}

	return nil
}

// Start runs the component
func (s *Status) Start(_ context.Context) error {
	listener, err := newStatusListener(s.Socket)
	if err != nil {
		s.L.Errorf("failed to create listener %s", err)
		return err
	}
	s.L.Infof("Listening address %s", s.Socket)
	go func() {
		if err := s.httpserver.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.L.Errorf("failed to start status server at %s: %s", s.Socket, err)
		}
	}()
	return nil
}

// Stop stops status component and removes the unix socket
func (s *Status) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.httpserver.Shutdown(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	cleanupStatusListener(s.Socket)
	return nil
}

type statusHandler struct {
	Status *Status
}

// ServerHTTP implementation of handler interface
func (sh *statusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	statusInfo := sh.getCurrentStatus(r.Context())

	w.Header().Set("Content-Type", "application/json")
	if json.NewEncoder(w).Encode(statusInfo) != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (sh *statusHandler) getCurrentStatus(ctx context.Context) K0sStatus {
	ctx, cancel := context.WithTimeout(ctx, statusCheckTimeout)
	defer cancel()

	status := sh.Status.StatusInformation
	var wg sync.WaitGroup
	if status.Workloads {
		wg.Go(func() { sh.probeWorkerAPI(ctx, &status) })
	}
	if sh.Status.ControllerClientFactory != nil {
		wg.Go(func() { sh.setCNICondition(ctx, &status) })
	}
	wg.Wait()
	return status
}

func (sh *statusHandler) probeWorkerAPI(ctx context.Context, status *K0sStatus) {
	client, err := sh.Status.GetWorkerClientFactory().GetClient()
	if err != nil {
		status.WorkerToAPIConnectionStatus = ProbeStatus{
			Message: "failed to get client: " + err.Error(),
		}
		return
	}

	_, err = client.Discovery().RESTClient().Get().AbsPath("/version").Do(ctx).Raw()
	if err != nil {
		status.WorkerToAPIConnectionStatus = ProbeStatus{Message: err.Error()}
		return
	}
	status.WorkerToAPIConnectionStatus = ProbeStatus{Success: true}
}

// setCNICondition reports managed CNI health from DaemonSet and Deployment status.
// A CNI conf file on disk is not used: kubelet treats that file as enough to
// mark the node Ready even when the CNI is not operational.
func (sh *statusHandler) setCNICondition(ctx context.Context, status *K0sStatus) {
	network := sh.Status.network.Load()
	if network == nil {
		status.Conditions = []Condition{cniCondition(
			corev1.ConditionUnknown,
			reasonError,
			"network configuration has not been reconciled yet",
		)}
		return
	}

	switch network.Provider {
	case constant.CNIProviderKubeRouter, constant.CNIProviderCalico:
	default:
		return
	}

	client, err := sh.Status.ControllerClientFactory.GetClient()
	if err != nil {
		status.Conditions = []Condition{cniCondition(corev1.ConditionUnknown, reasonError, err.Error())}
		return
	}

	var cond Condition
	switch network.Provider {
	case constant.CNIProviderKubeRouter:
		cond = daemonSetCNICondition(ctx, client, kubeRouterDaemonSet)
	case constant.CNIProviderCalico:
		cond = calicoCNICondition(ctx, client, network, sh.Status.HasWindowsNodes)
	default:
		return
	}
	status.Conditions = []Condition{cond}
}
