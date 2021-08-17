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
}

// Healthy dummy implementation
func (s *Status) Healthy() error { return nil }

// Init initializes component
func (s *Status) Init() error {
	s.L = logrus.WithFields(logrus.Fields{"component": "status"})

	var err error
	s.httpserver = http.Server{
		Handler: &statusHandler{Status: s},
	}
	err = dir.Init(s.StatusInformation.K0sVars.RunDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", s.Socket, err)
	}

	s.listener, err = net.Listen("unix", s.Socket)
	if err != nil {
		s.L.Errorf("failed to create listener %s", err)
		return err
	}
	s.L.Infof("Listening address %s", s.Socket)

	return nil
}

// Run runs the component
func (s *Status) Run() error {
	go func() {
		if err := s.httpserver.Serve(s.listener); err != nil {
			s.L.Errorf("failed to start status server at %s: %s", s.Socket, err)
		}
	}()
	return nil
}

// Stop stops status component and removes the unix socket
func (s *Status) Stop() error {
	defer os.Remove(s.Socket)
	if err := s.httpserver.Shutdown(context.TODO()); err != nil {
		return err
	}
	return nil
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
