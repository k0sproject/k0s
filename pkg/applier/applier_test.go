/*
Copyright 2020 Mirantis, Inc.

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
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery/cached/memory"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic/fake"
	kubetesting "k8s.io/client-go/testing"
	"testing"
)

func TestApplierAppliesAllManifestsInADirectory(t *testing.T) {
	dir, err := ioutil.TempDir("", "applier-test-*")
	assert.NoError(t, err)
	template := `
kind: ConfigMap
apiVersion: v1
metadata:
  name: applier-test
  namespace: kube-system
  labels:
    component: applier
data:
  foo: bar
`
	template2 := `
kind: Pod
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
	assert.NoError(t, ioutil.WriteFile(fmt.Sprintf("%s/test.yaml", dir), []byte(template), 0400))
	assert.NoError(t, ioutil.WriteFile(fmt.Sprintf("%s/test-pod.yaml", dir), []byte(template2), 0400))

	a := NewApplier(dir)
	assert.NoError(t, err)

	a.client = fake.NewSimpleDynamicClient(runtime.NewScheme())
	fakeDiscoveryClient := &discoveryfake.FakeDiscovery{Fake: &kubetesting.Fake{}}
	fakeDiscoveryClient.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: corev1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "nodes", Namespaced: false, Kind: "Node"},
				{Name: "pods", Namespaced: true, Kind: "Pod"},
				{Name: "configmaps", Namespaced: true, Kind: "ConfigMap"},
				{Name: "replicationcontrollers", Namespaced: true, Kind: "ReplicationController"},
			},
		},
	}
	a.discoveryClient = memory.NewMemCacheClient(fakeDiscoveryClient)
	err = a.Apply()
	assert.NoError(t, err)
	gv, _ := schema.ParseResourceArg("configmaps.v1.")
	r, err := a.client.Resource(*gv).Namespace("kube-system").Get(context.Background(), "applier-test", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "applier", r.GetLabels()["component"])
	podgv, _ := schema.ParseResourceArg("pods.v1.")
	r, err = a.client.Resource(*podgv).Namespace("kube-system").Get(context.Background(), "applier-test", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "Pod", r.GetKind())
	assert.Equal(t, "applier", r.GetLabels()["component"])
}
