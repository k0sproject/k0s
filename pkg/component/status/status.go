/*
Copyright 2021 k0s authors

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

package status

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/pkg/autopilot/client"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
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
	listener          net.Listener
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
	var err error
	s.httpserver = http.Server{
		Handler: mux,
	}
	err = dir.Init(s.StatusInformation.K0sVars.RunDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", s.Socket, err)
	}

	removeLeftovers(s.Socket)
	s.listener, err = net.Listen("unix", s.Socket)
	if err != nil {
		s.L.Errorf("failed to create listener %s", err)
		return err
	}
	s.L.Infof("Listening address %s", s.Socket)

	return nil
}

// removeLeftovers tries to remove leftover sockets that nothing is listening on
func removeLeftovers(socket string) {
	_, err := net.Dial("unix", socket)
	if err != nil {
		_ = os.Remove(socket)
	}
}

// Start runs the component
func (s *Status) Start(_ context.Context) error {
	go func() {
		if err := s.httpserver.Serve(s.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
	// Unix socket doesn't need to be explicitly removed because it's hadled
	// by httpserver.Shutdown
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
			status.WorkerToAPIConnectionStatus.Message = fmt.Sprintf("failed to create kube-api client required for kube-api status reports, probably kubelet failed to init: %s", err.Error())
			return status
		}
		sh.client = kubeClient
	}
	_, err := sh.client.CoreV1().Nodes().List(context.Background(), v1.ListOptions{})
	if err != nil {
		status.WorkerToAPIConnectionStatus.Message = err.Error()
		return status
	}
	status.WorkerToAPIConnectionStatus.Success = true
	return status
}

func (sh *statusHandler) buildWorkerSideKubeAPIClient(ctx context.Context) (kubernetes.Interface, error) {
	var restConfig *rest.Config
	var err error
	timeout, cancel := context.WithTimeout(ctx, defaultPollTimeout)
	defer cancel()
	if err := wait.PollUntilWithContext(timeout, defaultPollDuration, func(ctx context.Context) (done bool, err error) {
		if restConfig, err = sh.Status.CertManager.GetRestConfig(ctx); err != nil {
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	factory, err := client.NewClientFactory(restConfig)
	if err != nil {
		return nil, err
	}
	client, err := factory.GetClient()
	if err != nil {
		return nil, err
	}
	return client, nil
}
