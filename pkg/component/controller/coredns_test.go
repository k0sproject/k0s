// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"bytes"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoreDNS_RenderWithPatch(t *testing.T) {
	cfg := coreDNSConfig{
		Replicas:      1,
		ClusterDomain: "cluster.local",
		ClusterDNSIPs: []string{"10.96.0.10"},
		Image:         "coredns:latest",
		PullPolicy:    "IfNotPresent",
	}
	tw := templatewriter.TemplateWriter{
		Name:     "coredns",
		Template: coreDNSTemplate,
		Data:     cfg,
		Patches: v1beta1.Patches{{
			Target: v1beta1.PatchTarget{Kind: "Deployment", Name: "coredns"},
			Patch:  v1beta1.PatchSpec{Type: v1beta1.MergePatchType, Content: `{"metadata":{"annotations":{"patched":"true"}}}`},
		}},
	}
	var buf bytes.Buffer
	require.NoError(t, tw.WriteToBuffer(&buf))
	assert.Contains(t, buf.String(), "patched")
}

func Test_replicaCount(t *testing.T) {
	tests := []struct {
		name  string
		nodes int
		want  int
	}{
		{
			"one replica even for zero nodes",
			0,
			1,
		},
		{
			"one replica for one node",
			1,
			1,
		},
		{
			"2 replicas for two nodes (1 + ceil(2/10)) ",
			2,
			2,
		},
		{
			"2 replicas for 10 nodes (1 + 10/10)",
			10,
			2,
		},
		{
			"3 replicas for 15 nodes (1 + ceil(15/10))",
			15,
			3,
		},
		{
			"3 replicas for 20 nodes (1 + (20/10))",
			20,
			3,
		},
		{
			"11 replicas for 100 nodes (1 + (100/10) )",
			100,
			11,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := replicaCount(tt.nodes); got != tt.want {
				t.Errorf("replicaCount() = %v, want %v", got, tt.want)
			}
		})
	}
}
