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
package status

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/pkg/install"
	"github.com/sirupsen/logrus"
)

type Status struct {
	StatusInformation install.K0sStatus
	Socket            string
	L                 *logrus.Entry
	httpserver        http.Server
	listener          net.Listener
	runCtx            context.Context
}

// Healthy dummy implementation
func (s *Status) Healthy() error { return nil }

// Init initializes component
func (s *Status) Init(_ context.Context) error {
	s.L = logrus.WithFields(logrus.Fields{"component": "status"})

	var err error
	s.httpserver = http.Server{
		Handler: &statusHandler{Status: s},
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

// Run runs the component
func (s *Status) Run(ctx context.Context) error {
	s.runCtx = ctx
	go func() {
		if err := s.httpserver.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			s.L.Errorf("failed to start status server at %s: %s", s.Socket, err)
		}
	}()
	return nil
}

// Stop stops status component and removes the unix socket
func (s *Status) Stop() error {
	if err := s.httpserver.Shutdown(s.runCtx); err != nil {
		return err
	}
	return os.Remove(s.Socket)
}

// Reconcile detects changes in configuration and applies them to the component
func (s *Status) Reconcile() error {
	logrus.Debug("reconcile method called for: Status")
	return nil
}

type statusHandler struct {
	Status *Status
}

// ServerHTTP implementation of handler interface
func (sh *statusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if json.NewEncoder(w).Encode(sh.Status.StatusInformation) != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
