//go:build unix

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/config"
)

type K0sStatus struct {
	Version                     string
	Pid                         int
	PPid                        int
	Role                        string
	SysInit                     string
	StubFile                    string
	Output                      string
	Workloads                   bool
	SingleNode                  bool
	Args                        []string
	WorkerToAPIConnectionStatus ProbeStatus
	KubeSchedulerStatus         ProbeStatus
	KubeControllerManagerStatus ProbeStatus
	ClusterConfig               *v1beta1.ClusterConfig
	K0sVars                     *config.CfgVars
}
type ProbeStatus struct {
	Message string
	Success bool
}

// GetStatus returns the status of the k0s process using the status socket
func GetStatusInfo(socketPath string) (*K0sStatus, error) {
	status := &K0sStatus{}
	if err := doHTTPRequestViaUnixSocket(socketPath, "status", status); err != nil {
		return nil, err
	}
	return status, nil
}

// GetComponentStatus returns the per-component events and health-checks
func GetComponentStatus(socketPath string, maxCount int) (*prober.State, error) {
	status := &prober.State{}
	if err := doHTTPRequestViaUnixSocket(socketPath,
		fmt.Sprintf("components?maxCount=%d", maxCount),
		status); err != nil {
		return nil, err
	}
	return status, nil
}

func doHTTPRequestViaUnixSocket(socketPath string, path string, tgt any) error {
	httpc := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
	}

	response, err := httpc.Get("http://localhost/" + path)
	if err != nil {
		return fmt.Errorf("status: can't get %q via %q: %w", path, socketPath, err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("status: can't get %q via %q: status code %d", path, socketPath, response.StatusCode)
	}

	if err := json.NewDecoder(response.Body).Decode(tgt); err != nil {
		return fmt.Errorf("status: can't get %q via %q: can't decode JSON: %w", path, socketPath, err)
	}

	return nil
}
