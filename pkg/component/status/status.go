// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Stater interface {
	State(maxCount int) prober.State
}

type Status struct {
	StatusInformation K0sStatus
	Prober            Stater
	Socket            string
	L                 *logrus.Entry
	httpserver        http.Server
	CertManager       certManager
}

type certManager interface {
	GetRestConfig(ctx context.Context) (*rest.Config, error)
}

var _ manager.Component = (*Status)(nil)

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
	client kubernetes.Interface
}

// ServerHTTP implementation of handler interface
func (sh *statusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	statusInfo := sh.getCurrentStatus(r.Context())

	w.Header().Set("Content-Type", "application/json")
	if json.NewEncoder(w).Encode(statusInfo) != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

const (
	defaultPollDuration = 1 * time.Second
	defaultPollTimeout  = 5 * time.Minute
)

func (sh *statusHandler) getCurrentStatus(ctx context.Context) K0sStatus {
	status := sh.Status.StatusInformation
	if !status.Workloads {
		return status
	}

	if sh.client == nil {
		kubeClient, err := sh.buildWorkerSideKubeAPIClient(ctx)
		if err != nil {
			status.WorkerToAPIConnectionStatus.Message = "failed to create kube-api client required for kube-api status reports, probably kubelet failed to init: " + err.Error()
			return status
		}
		sh.client = kubeClient
	}
	nodes, err := sh.client.CoreV1().Nodes().List(ctx, v1.ListOptions{})
	if err != nil {
		status.WorkerToAPIConnectionStatus.Message = err.Error()
		return status
	}
	status.WorkerToAPIConnectionStatus.Success = true

	status.CNI = &CNI{
		Health: "Healthy",
	}

	for _, node := range nodes.Items {
		for _, cond := range node.Status.Conditions {
			if cond.Type == "NetworkUnavailable" && cond.Status == "True" {
				status.CNI.Fail("Node %s reports NetworkUnavailable: %s", node.Name, cond.Message)
			}
		}
	}

	if status.ClusterConfig != nil && status.ClusterConfig.Spec.Network.Provider != "" {
		status.CNI.Provider = status.ClusterConfig.Spec.Network.Provider
	}

	pods, err := sh.client.CoreV1().Pods("kube-system").List(ctx, v1.ListOptions{
		LabelSelector: "k8s-app in (kube-router, calico-node, calico-kube-controllers)",
	})

	if err != nil {
		status.CNI.Fail("failed to list kube-system pods: %v", err)
	} else {
		for _, pod := range pods.Items {
			app := pod.Labels["k8s-app"]

			if status.CNI.Provider == "" {
				if strings.Contains(app, "calico") {
					status.CNI.Provider = "calico"
				} else if app == "kube-router" {
					status.CNI.Provider = "kuberouter"
				}
			}

			isReady := false
			if pod.Status.Phase == "Running" {
				for _, cond := range pod.Status.Conditions {
					if cond.Type == "Ready" && cond.Status == "True" {
						isReady = true
						break
					}
				}
			}

			status.CNI.Components = append(
				status.CNI.Components,
				fmt.Sprintf("%s (Ready: %t)", pod.Name, isReady),
			)

			if !isReady {
				status.CNI.Fail("Component %s is in %s state", pod.Name, pod.Status.Phase)
			}
		}
	}

	if len(status.CNI.Components) == 0 && status.CNI.Health == "Healthy" {
		status.CNI.Fail("No CNI components found")
	}

	return status
}

func (sh *statusHandler) buildWorkerSideKubeAPIClient(ctx context.Context) (client kubernetes.Interface, _ error) {
	timeout, cancel := context.WithTimeout(ctx, defaultPollTimeout)
	defer cancel()
	if err := wait.PollUntilWithContext(timeout, defaultPollDuration, func(ctx context.Context) (done bool, err error) {
		factory := kubeutil.ClientFactory{LoadRESTConfig: func() (*rest.Config, error) {
			return sh.Status.CertManager.GetRestConfig(ctx)
		}}

		client, err = factory.GetClient()
		return err == nil, nil
	}); err != nil {
		return nil, err
	}
	return client, nil
}
