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

package controller

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
)

var expectedAddresses = []string{
	"2001:db8::1",
	"2001:db8::2",
	"240.0.0.2",
	"240.0.0.3",
}

func TestBasicReconcilerWithNoLeader(t *testing.T) {
	fakeFactory := testutil.NewFakeClientFactory()

	config := getFakeConfig()

	r := NewEndpointReconciler(config, &leaderelector.Dummy{Leader: false}, fakeFactory, fakeResolver{})

	ctx := context.TODO()
	assert.NoError(t, r.Init(ctx))

	assert.NoError(t, r.reconcileEndpoints(ctx))
	client, err := fakeFactory.GetClient()
	assert.NoError(t, err)
	_, err = client.CoreV1().Endpoints("default").Get(ctx, "kubernetes", v1.GetOptions{})
	// The reconciler should not make any modification as we're not the leader so the endpoint should not get created
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
	// verifyEndpointAddresses(t, expectedAddresses, fakeFactory)
}

func TestBasicReconcilerWithNoExistingEndpoint(t *testing.T) {
	fakeFactory := testutil.NewFakeClientFactory()
	config := getFakeConfig()

	r := NewEndpointReconciler(config, &leaderelector.Dummy{Leader: true}, fakeFactory, fakeResolver{})

	ctx := context.TODO()
	assert.NoError(t, r.Init(ctx))

	assert.NoError(t, r.reconcileEndpoints(ctx))
	verifyEndpointAddresses(t, expectedAddresses, fakeFactory.Client)
}

func TestBasicReconcilerWithEmptyEndpointSubset(t *testing.T) {
	fakeFactory := testutil.NewFakeClientFactory()

	existingEp := corev1.Endpoints{
		TypeMeta: v1.TypeMeta{
			Kind:       "Endpoints",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "kubernetes",
		},
		Subsets: []corev1.EndpointSubset{},
	}
	fakeClient, err := fakeFactory.GetClient()
	assert.NoError(t, err)
	ctx := context.TODO()
	_, err = fakeClient.CoreV1().Endpoints("default").Create(ctx, &existingEp, v1.CreateOptions{})
	assert.NoError(t, err)
	config := getFakeConfig()

	r := NewEndpointReconciler(config, &leaderelector.Dummy{Leader: true}, fakeFactory, fakeResolver{})

	assert.NoError(t, r.Init(ctx))

	assert.NoError(t, r.reconcileEndpoints(ctx))
	verifyEndpointAddresses(t, expectedAddresses, fakeFactory.Client)
}

func TestReconcilerWithNoNeedForUpdate(t *testing.T) {
	fakeFactory := testutil.NewFakeClientFactory()
	existingEp := corev1.Endpoints{
		TypeMeta: v1.TypeMeta{
			Kind:       "Endpoints",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "kubernetes",
			Annotations: map[string]string{
				"foo": "bar",
			},
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: stringsToEndpointAddresses(expectedAddresses),
			},
		},
	}

	fakeClient, _ := fakeFactory.GetClient()

	ctx := context.TODO()
	_, err := fakeClient.CoreV1().Endpoints("default").Create(ctx, &existingEp, v1.CreateOptions{})
	assert.NoError(t, err)

	config := getFakeConfig()

	r := NewEndpointReconciler(config, &leaderelector.Dummy{Leader: true}, fakeFactory, fakeResolver{})

	assert.NoError(t, r.Init(ctx))

	assert.NoError(t, r.reconcileEndpoints(ctx))
	e := verifyEndpointAddresses(t, expectedAddresses, fakeFactory.Client)
	assert.Equal(t, "bar", e.ObjectMeta.Annotations["foo"])
}

func TestReconcilerWithNeedForUpdate(t *testing.T) {
	fakeFactory := testutil.NewFakeClientFactory()
	existingEp := corev1.Endpoints{
		TypeMeta: v1.TypeMeta{
			Kind:       "Endpoints",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "kubernetes",
			Annotations: map[string]string{
				"foo": "bar",
			},
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: stringsToEndpointAddresses([]string{"1.2.3.4", "1.1.1.1"}),
			},
		},
	}

	fakeClient, _ := fakeFactory.GetClient()

	ctx := context.TODO()
	_, err := fakeClient.CoreV1().Endpoints("default").Create(ctx, &existingEp, v1.CreateOptions{})
	assert.NoError(t, err)

	config := getFakeConfig()

	r := NewEndpointReconciler(config, &leaderelector.Dummy{Leader: true}, fakeFactory, fakeResolver{})
	assert.NoError(t, r.Init(ctx))

	assert.NoError(t, r.reconcileEndpoints(ctx))
	e := verifyEndpointAddresses(t, expectedAddresses, fakeFactory.Client)
	assert.Equal(t, "bar", e.ObjectMeta.Annotations["foo"])
}

func verifyEndpointAddresses(t *testing.T, expectedAddresses []string, clients kubernetes.Interface) *corev1.Endpoints {
	ep, err := clients.CoreV1().Endpoints("default").Get(context.TODO(), "kubernetes", v1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedAddresses, endpointAddressesToStrings(ep.Subsets[0].Addresses))

	return ep
}

type fakeResolver struct{}

func (fr fakeResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return []net.IPAddr{
		{IP: net.ParseIP("2001:db8::1")},
		{IP: net.ParseIP("2001:db8::2")},
		{IP: net.ParseIP("240.0.0.2")},
		{IP: net.ParseIP("240.0.0.3")},
	}, nil
}

func getFakeConfig() *v1beta1.ClusterConfig {
	return &v1beta1.ClusterConfig{
		Spec: &v1beta1.ClusterSpec{
			API: &v1beta1.APISpec{
				Address:         "240.0.0.1",
				ExternalAddress: "fake.k0s",
			},
		},
	}

}
