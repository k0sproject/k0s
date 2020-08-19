package server

import (
	"os"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/supervisor"
)

const apiYamls = `
apiVersion: apiregistration.k8s.io/v1beta1
kind: APIService
metadata:
  name: v1beta1.mke.k8s.io
spec:
  service:
    name: mke-api
    namespace: kube-system
  group: mke.k8s.io
  version: v1beta1
  # TODO Fix to use proper certs etc.
  insecureSkipTLSVerify: true
  groupPriorityMinimum: 100
  versionPriority: 100
---
apiVersion: v1
kind: Service
metadata:
  name: mke-api
  namespace: kube-system
spec:
---
apiVersion: v1
kind: Endpoint
metadata:
  name: mke-api
  namespace: kube-system
subsets:
- addresses:
  - ip: 127.0.0.1
  ports:
  - name: https
    port: 8000
    protocol: TCP
`

// MkeControlApi
type MkeControlApi struct {
	ClusterConfig *config.ClusterConfig

	supervisor supervisor.Supervisor
}

func (m *MkeControlApi) Init() error {
	// We need to create a serving cert for the api

	return nil
}

// Run runs mke control api as separate process
func (m *MkeControlApi) Run() error {
	// TODO: Make the api process to use some other user
	m.supervisor = supervisor.Supervisor{
		Name:    "mke control api",
		BinPath: os.Args[0],
		Args: []string{
			"api",
		},
	}

	m.supervisor.Supervise()
	return nil
}

// Stop stops mke api
func (m *MkeControlApi) Stop() error {
	return m.supervisor.Stop()
}
