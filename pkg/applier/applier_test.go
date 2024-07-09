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

package applier_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kubeutil "github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/applier"
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
	require.NoError(t, os.WriteFile(fmt.Sprintf("%s/test-ns.yaml", dir), []byte(templateNS), 0400))
	require.NoError(t, os.WriteFile(fmt.Sprintf("%s/test-list.yaml", dir), []byte(template), 0400))
	require.NoError(t, os.WriteFile(fmt.Sprintf("%s/test-deploy.yaml", dir), []byte(templateDeployment), 0400))

	fakes := kubeutil.NewFakeClientFactory()
	a := applier.NewApplier(dir, fakes)

	ctx := context.Background()
	err := a.Apply(ctx)
	assert.NoError(t, err)
	gv, _ := schema.ParseResourceArg("configmaps.v1.")
	r, err := fakes.DynamicClient.Resource(*gv).Namespace("kube-system").Get(ctx, "applier-test", metav1.GetOptions{})
	if assert.NoError(t, err) {
		assert.Equal(t, "applier", r.GetLabels()["component"])
	}
	podgv, _ := schema.ParseResourceArg("pods.v1.")
	r, err = fakes.DynamicClient.Resource(*podgv).Namespace("kube-system").Get(ctx, "applier-test", metav1.GetOptions{})
	if assert.NoError(t, err) {
		assert.Equal(t, "Pod", r.GetKind())
		assert.Equal(t, "applier", r.GetLabels()["component"])
	}
	deployGV, _ := schema.ParseResourceArg("deployments.v1.apps")
	_, err = fakes.DynamicClient.Resource(*deployGV).Namespace("kube-system").Get(ctx, "nginx", metav1.GetOptions{})
	assert.NoError(t, err)

	// Attempt to delete the stack with a different applier
	a2 := applier.NewApplier(dir, fakes)
	assert.NoError(t, a2.Delete(ctx))
	// Check that the resources are deleted
	_, err = fakes.DynamicClient.Resource(*gv).Namespace("kube-system").Get(ctx, "applier-test", metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))

	_, err = fakes.DynamicClient.Resource(*podgv).Namespace("kube-system").Get(ctx, "applier-test", metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))

	_, err = fakes.DynamicClient.Resource(*deployGV).Namespace("kube-system").Get(ctx, "nginx", metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))

	gvNS, _ := schema.ParseResourceArg("namespaces.v1.")
	_, err = fakes.DynamicClient.Resource(*gvNS).Get(ctx, "kube-system", metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))
}
