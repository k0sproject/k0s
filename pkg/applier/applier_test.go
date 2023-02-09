/*
Copyright 2020 k0s authors

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

package applier

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kubeutil "github.com/k0sproject/k0s/internal/testutil"
)

func TestApplierAppliesAllManifestsInADirectory(t *testing.T) {
	dir := t.TempDir()
	templateNS := `
apiVersion: v1
kind: Namespace
metadata:
  name:  kube-system
`
	template := `
apiVersion: v1
kind: List
items:
  - kind: ConfigMap
    apiVersion: v1
    metadata:
      name: applier-test
      namespace: kube-system
      labels:
        component: applier
    data:
      foo: bar
  - kind: Pod
    apiVersion: v1
    metadata:
      name: applier-test
      namespace: kube-system
      labels:
        component: applier
    spec:
      containers:
      - name: nginx
        image: nginx:1.15
`

	templateDeployment := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
       app: nginx
    spec:
      containers:
      - name: nginx
        image: docker.io/nginx:1-alpine
        resources:
          limits:
            memory: "64Mi"
            cpu: "100m"
        ports:
          - containerPort: 80
`
	assert.NoError(t, os.WriteFile(fmt.Sprintf("%s/test-ns.yaml", dir), []byte(templateNS), 0400))
	assert.NoError(t, os.WriteFile(fmt.Sprintf("%s/test-list.yaml", dir), []byte(template), 0400))
	assert.NoError(t, os.WriteFile(fmt.Sprintf("%s/test-deploy.yaml", dir), []byte(templateDeployment), 0400))

	fakes := kubeutil.NewFakeClientFactory()
	verbs := []string{"get", "list", "delete", "create"}
	fakes.RawDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: corev1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "nodes", Namespaced: false, Kind: "Node", Verbs: verbs},
				{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: verbs},
				{Name: "configmaps", Namespaced: true, Kind: "ConfigMap", Verbs: verbs},
				{Name: "namespaces", Namespaced: false, Kind: "Namespace", Verbs: verbs},
			},
		},
		{
			GroupVersion: appsv1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "deployments", Namespaced: true, Kind: "Deployment", Verbs: verbs},
			},
		},
	}

	a := NewApplier(dir, fakes)

	ctx := context.Background()
	err := a.Apply(ctx)
	assert.NoError(t, err)
	gv, _ := schema.ParseResourceArg("configmaps.v1.")
	r, err := a.client.Resource(*gv).Namespace("kube-system").Get(ctx, "applier-test", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "applier", r.GetLabels()["component"])
	podgv, _ := schema.ParseResourceArg("pods.v1.")
	r, err = a.client.Resource(*podgv).Namespace("kube-system").Get(ctx, "applier-test", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "Pod", r.GetKind())
	assert.Equal(t, "applier", r.GetLabels()["component"])
	deployGV, _ := schema.ParseResourceArg("deployments.v1.apps")
	_, err = a.client.Resource(*deployGV).Namespace("kube-system").Get(ctx, "nginx", metav1.GetOptions{})
	assert.NoError(t, err)

	// Attempt to delete the stack with a different applier
	a2 := NewApplier(dir, fakes)
	assert.NoError(t, a2.Delete(ctx))
	// Check that the resources are deleted
	_, err = a.client.Resource(*gv).Namespace("kube-system").Get(ctx, "applier-test", metav1.GetOptions{})
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))

	_, err = a.client.Resource(*podgv).Namespace("kube-system").Get(ctx, "applier-test", metav1.GetOptions{})
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))

	_, err = a.client.Resource(*deployGV).Namespace("kube-system").Get(ctx, "nginx", metav1.GetOptions{})
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))

	gvNS, _ := schema.ParseResourceArg("namespaces.v1.")
	_, err = a.client.Resource(*gvNS).Get(ctx, "kube-system", metav1.GetOptions{})
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}
