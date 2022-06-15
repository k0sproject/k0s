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
package defaultstorage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/tests/smoke/common"
)

type DefaultStorageSuite struct {
	common.FootlooseSuite
}

func (s *DefaultStorageSuite) TestK0sGetsUp() {
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))
	s.MakeDir(s.ControllerNode(0), "/var/lib/k0s/manifests/test")
	s.PutFile(s.ControllerNode(0), "/var/lib/k0s/manifests/test/pvc.yaml", pvcManifest)
	s.PutFile(s.ControllerNode(0), "/var/lib/k0s/manifests/test/deployment.yaml", deploymentManifest)
	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0), "")
	s.NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	err = common.WaitForDeployment(kc, "nginx")
	s.NoError(err)

	pods, err := kc.CoreV1().Pods("kube-system").List(context.TODO(), v1.ListOptions{
		Limit: 100,
	})
	s.NoError(err)

	podCount := len(pods.Items)

	s.T().Logf("found %d pods in kube-system", podCount)
	s.Greater(podCount, 0, "expecting to see few pods in kube-system namespace")

	pv, err := kc.CoreV1().PersistentVolumes().List(context.TODO(), v1.ListOptions{})
	s.NoError(err)
	s.Greater(len(pv.Items), 0, "At least one persistent volume must be created for the deployment with claims")
}

func TestDefaultStorageSuite(t *testing.T) {
	s := DefaultStorageSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
		},
	}
	suite.Run(t, &s)
}

const pvcManifest = `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: nginx-pvc
  namespace: kube-system
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: openebs-hostpath
  resources:
    requests:
      storage: 5Gi
`

const deploymentManifest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: kube-system
  labels:
    app: nginx
spec:
  selector:
    matchLabels:
      app: nginx
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx 
        name: nginx
        volumeMounts:
        - name: persistent-storage
          mountPath: /var/lib/nginx
      volumes:
      - name: persistent-storage
        persistentVolumeClaim:
          claimName: nginx-pvc
`

const k0sConfig = `
spec:
  extensions:
    storage:
      type: openebs_local_storage
`
