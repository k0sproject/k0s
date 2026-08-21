// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import "testing"

// TestNewCRDStack_WithStackName verifies that an explicitly provided stack
// name via WithStackName is preserved, even when WithCRDAssetsDir is not
// also given. Previously, the defaulting logic in NewCRDStack overwrote any
// explicitly set stack name back to the bundle name whenever the assets dir
// option was left unset, silently discarding WithStackName.
func TestNewCRDStack_WithStackName(t *testing.T) {
	stack := NewCRDStack(nil, nil, "etcd", WithStackName(EtcdMemberStackName))

	if stack.stackName != EtcdMemberStackName {
		t.Errorf("expected stack name %q, got %q", EtcdMemberStackName, stack.stackName)
	}
	if stack.assetsDir != "etcd" {
		t.Errorf("expected assets dir %q, got %q", "etcd", stack.assetsDir)
	}
}

// TestNewCRDStack_Defaults verifies that both the stack name and the assets
// dir default to the bundle name when no options are given.
func TestNewCRDStack_Defaults(t *testing.T) {
	stack := NewCRDStack(nil, nil, "helm")

	if stack.stackName != "helm" {
		t.Errorf("expected stack name %q, got %q", "helm", stack.stackName)
	}
	if stack.assetsDir != "helm" {
		t.Errorf("expected assets dir %q, got %q", "helm", stack.assetsDir)
	}
}
