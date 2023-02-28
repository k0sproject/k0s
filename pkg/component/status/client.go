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
	"fmt"
	"net"
	"net/http"

	config "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/constant"
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
	ClusterConfig               *config.ClusterConfig
	K0sVars                     constant.CfgVars
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

func doHTTPRequestViaUnixSocket(socketPath string, path string, tgt interface{}) error {
	httpc := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	response, err := httpc.Get("http://localhost/" + path)
	if err != nil {
		return fmt.Errorf("status: can't do http request: %v %v", socketPath, path)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("status: unexpected http status code: %v %v", socketPath, path)
	}

	if err := json.NewDecoder(response.Body).Decode(tgt); err != nil {
		return fmt.Errorf("status: can't decode json: %v", err)
	}
	return nil
}
