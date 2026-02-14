// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0scloudprovider

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cloudproviderapi "k8s.io/cloud-provider/api"
)

type populateAddressTestData struct {
	name   string
	input  *v1.Node
	output []v1.NodeAddress
}

// populateInternalAddress

var testDataFindInternalAddress = []populateAddressTestData{
	{
		name: "From Provided",
		input: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
				},
			},
			Status: v1.NodeStatus{
				Addresses: []v1.NodeAddress{},
			},
		},
		output: []v1.NodeAddress{
			{Type: v1.NodeInternalIP, Address: "1.2.3.4"},
		},
	},
	{
		name: "From Status",
		input: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{},
			},
			Status: v1.NodeStatus{
				Addresses: []v1.NodeAddress{
					{Type: v1.NodeInternalIP, Address: "1.2.3.4"},
				},
			},
		},
		output: []v1.NodeAddress{
			{Type: v1.NodeInternalIP, Address: "1.2.3.4"},
		},
	},
	{
		name: "From Provided Preferred",
		input: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
				},
			},
			Status: v1.NodeStatus{
				Addresses: []v1.NodeAddress{
					{Type: v1.NodeInternalIP, Address: "5.6.7.8"},
				},
			},
		},
		output: []v1.NodeAddress{
			{Type: v1.NodeInternalIP, Address: "1.2.3.4"},
		},
	},
	{
		name: "Missing",
		input: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{},
			},
			Status: v1.NodeStatus{
				Addresses: []v1.NodeAddress{},
			},
		},
		output: []v1.NodeAddress{},
	},
	{
		name:   "nil Node",
		input:  nil,
		output: []v1.NodeAddress{},
	},
}

// TestPopulateInternalAddress runs tests against a suite of expected input/output data.
func TestPopulateInternalAddress(t *testing.T) {
	for _, tt := range testDataFindInternalAddress {
		t.Run(tt.name, func(t *testing.T) {
			addrs := make([]v1.NodeAddress, 0)
			populateInternalAddress(&addrs, tt.input)

			if !reflect.DeepEqual(addrs, tt.output) {
				t.Errorf("got %q, expected %q", addrs, tt.output)
			}
		})
	}
}

// populateExternalAddress

var testDataPopulateExternalAddress = []populateAddressTestData{
	{
		name: "Equality single address",
		input: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					ExternalIPAnnotation: "1.2.3.4",
				},
			},
			Status: v1.NodeStatus{
				Addresses: []v1.NodeAddress{},
			},
		},
		output: []v1.NodeAddress{
			{Type: v1.NodeExternalIP, Address: "1.2.3.4"},
		},
	},
	{
		name: "Equality multiple addresses",
		input: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					ExternalIPAnnotation: "1.2.3.4,2041:0000:140F::875B:131B",
				},
			},
			Status: v1.NodeStatus{
				Addresses: []v1.NodeAddress{},
			},
		},
		output: []v1.NodeAddress{
			{Type: v1.NodeExternalIP, Address: "1.2.3.4"},
			{Type: v1.NodeExternalIP, Address: "2041:0000:140F::875B:131B"},
		},
	},
	{
		name: "Missing",
		input: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{},
			},
			Status: v1.NodeStatus{
				Addresses: []v1.NodeAddress{},
			},
		},
		output: []v1.NodeAddress{},
	},
}

// TestPopulateExternalAddress runs tests against a suite of expected input/output data.
func TestPopulateExternalAddress(t *testing.T) {
	for _, tt := range testDataPopulateExternalAddress {
		t.Run(tt.name, func(t *testing.T) {
			addrs := make([]v1.NodeAddress, 0)
			populateExternalAddress(&addrs, tt.input)

			if !reflect.DeepEqual(addrs, tt.output) {
				t.Errorf("got %q, expected %q", addrs, tt.output)
			}
		})
	}
}
