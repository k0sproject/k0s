// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"strings"
	"testing"
)

func TestPatches_Validate(t *testing.T) {
	tests := []struct {
		name    string
		patches Patches
		wantErr bool
	}{
		{name: "empty is valid", patches: nil, wantErr: false},
		{
			name: "valid strategic patch",
			patches: Patches{{
				Target: PatchTarget{Kind: "Deployment", Name: "coredns"},
				Patch:  PatchSpec{Type: StrategicMergePatchType, Content: "spec: {}"},
			}},
			wantErr: false,
		},
		{
			name: "unknown type",
			patches: Patches{{
				Target: PatchTarget{Kind: "Deployment", Name: "coredns"},
				Patch:  PatchSpec{Type: "bogus", Content: "{}"},
			}},
			wantErr: true,
		},
		{
			name: "missing kind",
			patches: Patches{{
				Target: PatchTarget{Name: "coredns"},
				Patch:  PatchSpec{Type: MergePatchType, Content: "{}"},
			}},
			wantErr: true,
		},
		{
			name: "missing name",
			patches: Patches{{
				Target: PatchTarget{Kind: "Deployment"},
				Patch:  PatchSpec{Type: MergePatchType, Content: "{}"},
			}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.patches.Validate()
			if tt.wantErr && len(errs) == 0 {
				t.Fatalf("expected error, got none")
			}
			if !tt.wantErr && len(errs) > 0 {
				t.Fatalf("expected no error, got %v", errs)
			}
		})
	}
}

func TestClusterSpec_Validate_Patches(t *testing.T) {
	s := &ClusterSpec{
		MetricsServer: &MetricsServer{
			Patches: Patches{{
				Target: PatchTarget{Kind: "Deployment", Name: "metrics-server"},
				Patch:  PatchSpec{Type: "bogus", Content: "{}"},
			}},
		},
	}
	errs := s.Validate()
	found := false
	for _, e := range errs {
		if e != nil && strings.Contains(e.Error(), "metricsServer.patches[0]") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a patches validation error, got %v", errs)
	}
}
