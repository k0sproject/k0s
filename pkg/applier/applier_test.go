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
	ioutil.WriteFile(fmt.Sprintf("%s/test.yaml", dir), []byte(template), 0400)
	ioutil.WriteFile(fmt.Sprintf("%s/test-pod.yaml", dir), []byte(template2), 0400)
	assert.Nil(t, err)
	a, err := NewApplier(dir)
	assert.Nil(t, err)

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
	assert.Nil(t, err)
	gv, _ := schema.ParseResourceArg("configmaps.v1.")
	r, err := a.client.Resource(*gv).Namespace("kube-system").Get(context.Background(), "applier-test", metav1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, "applier", r.GetLabels()["component"])
	podgv, _ := schema.ParseResourceArg("pods.v1.")
	r, err = a.client.Resource(*podgv).Namespace("kube-system").Get(context.Background(), "applier-test", metav1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, "Pod", r.GetKind())
	assert.Equal(t, "applier", r.GetLabels()["component"])
}
