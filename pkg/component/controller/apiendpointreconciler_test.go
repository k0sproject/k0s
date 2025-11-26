// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/stretchr/testify/assert"
)

var expectedIPv4Addresses = []string{
	"240.0.0.2",
	"240.0.0.3",
}

var expectedIPv6Addresses = []string{
	"2001:db8::1",
	"2001:db8::2",
}

func TestBasicReconcilerWithNoLeader(t *testing.T) {
	fakeFactory := testutil.NewFakeClientFactory()

	config := getFakeConfig()

	leaderStatus := func() leaderelection.Status { return leaderelection.StatusPending }
	r := NewEndpointReconciler(config, leaderStatus, fakeFactory, fakeResolver{}, v1beta1.PrimaryFamilyIPv4)

	ctx := t.Context()
	assert.NoError(t, r.Init(ctx))

	assert.NoError(t, r.reconcileEndpoints(ctx))
	client, err := fakeFactory.GetClient()
	assert.NoError(t, err)
	_, err = client.CoreV1().Endpoints(metav1.NamespaceDefault).Get(ctx, "kubernetes", metav1.GetOptions{})
	// The reconciler should not make any modification as we're not the leader so the endpoint should not get created
	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))
	// verifyEndpointAddresses(t, expectedAddresses, fakeFactory)
}

func TestBasicReconcilerWithNoExistingEndpoint(t *testing.T) {
	tests := []struct {
		afnet             v1beta1.PrimaryAddressFamilyType
		expectedAddresses []string
	}{
		{
			afnet:             v1beta1.PrimaryFamilyIPv4,
			expectedAddresses: expectedIPv4Addresses,
		},
		{
			afnet:             v1beta1.PrimaryFamilyIPv6,
			expectedAddresses: expectedIPv6Addresses,
		},
	}

	for _, test := range tests {
		fakeFactory := testutil.NewFakeClientFactory()
		config := getFakeConfig()

		leaderStatus := func() leaderelection.Status { return leaderelection.StatusLeading }
		r := NewEndpointReconciler(config, leaderStatus, fakeFactory, fakeResolver{}, test.afnet)

		ctx := t.Context()
		assert.NoError(t, r.Init(ctx))

		assert.NoError(t, r.reconcileEndpoints(ctx))
		verifyEndpointAddresses(t, test.expectedAddresses, fakeFactory.Client)
	}
}

func TestBasicReconcilerWithEmptyEndpointSubset(t *testing.T) {
	fakeFactory := testutil.NewFakeClientFactory()

	existingEp := corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Endpoints",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubernetes",
		},
		Subsets: []corev1.EndpointSubset{},
	}
	fakeClient, err := fakeFactory.GetClient()
	assert.NoError(t, err)
	ctx := t.Context()
	_, err = fakeClient.CoreV1().Endpoints(metav1.NamespaceDefault).Create(ctx, &existingEp, metav1.CreateOptions{})
	assert.NoError(t, err)
	config := getFakeConfig()

	leaderStatus := func() leaderelection.Status { return leaderelection.StatusLeading }
	r := NewEndpointReconciler(config, leaderStatus, fakeFactory, fakeResolver{}, v1beta1.PrimaryFamilyIPv4)

	assert.NoError(t, r.Init(ctx))

	assert.NoError(t, r.reconcileEndpoints(ctx))
	verifyEndpointAddresses(t, expectedIPv4Addresses, fakeFactory.Client)
}

func TestReconcilerWithNoNeedForUpdate(t *testing.T) {
	fakeFactory := testutil.NewFakeClientFactory()
	existingEp := corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Endpoints",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubernetes",
			Annotations: map[string]string{
				"foo": "bar",
			},
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: stringsToEndpointAddresses(expectedIPv4Addresses),
			},
		},
	}

	fakeClient, _ := fakeFactory.GetClient()

	ctx := t.Context()
	_, err := fakeClient.CoreV1().Endpoints(metav1.NamespaceDefault).Create(ctx, &existingEp, metav1.CreateOptions{})
	assert.NoError(t, err)

	config := getFakeConfig()

	leaderStatus := func() leaderelection.Status { return leaderelection.StatusLeading }
	r := NewEndpointReconciler(config, leaderStatus, fakeFactory, fakeResolver{}, v1beta1.PrimaryFamilyIPv4)

	assert.NoError(t, r.Init(ctx))

	assert.NoError(t, r.reconcileEndpoints(ctx))
	e := verifyEndpointAddresses(t, expectedIPv4Addresses, fakeFactory.Client)
	assert.Equal(t, "bar", e.Annotations["foo"])
}

func TestReconcilerWithNeedForUpdate(t *testing.T) {
	fakeFactory := testutil.NewFakeClientFactory()
	existingEp := corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Endpoints",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
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

	ctx := t.Context()
	_, err := fakeClient.CoreV1().Endpoints(metav1.NamespaceDefault).Create(ctx, &existingEp, metav1.CreateOptions{})
	assert.NoError(t, err)

	config := getFakeConfig()

	leaderStatus := func() leaderelection.Status { return leaderelection.StatusLeading }
	r := NewEndpointReconciler(config, leaderStatus, fakeFactory, fakeResolver{}, v1beta1.PrimaryFamilyIPv4)
	assert.NoError(t, r.Init(ctx))

	assert.NoError(t, r.reconcileEndpoints(ctx))
	e := verifyEndpointAddresses(t, expectedIPv4Addresses, fakeFactory.Client)
	assert.Equal(t, "bar", e.Annotations["foo"])
}

func verifyEndpointAddresses(t *testing.T, expectedAddresses []string, clients kubernetes.Interface) *corev1.Endpoints {
	ep, err := clients.CoreV1().Endpoints(metav1.NamespaceDefault).Get(t.Context(), "kubernetes", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedAddresses, endpointAddressesToStrings(ep.Subsets[0].Addresses))

	return ep
}

type fakeResolver struct{}

func (fr fakeResolver) LookupIP(ctx context.Context, afnet string, host string) ([]net.IP, error) {
	switch afnet {
	case "ip4":
		return []net.IP{
			net.ParseIP("240.0.0.2"),
			net.ParseIP("240.0.0.3"),
		}, nil
	case "ip6":
		return []net.IP{
			net.ParseIP("2001:db8::1"),
			net.ParseIP("2001:db8::2"),
		}, nil
	default:
		return nil, fmt.Errorf("unknown address family %q", afnet)
	}
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
