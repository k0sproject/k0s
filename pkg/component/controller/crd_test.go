// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCRDStack_WithStackName(t *testing.T) {
	stack := NewCRDStack(nil, nil, "the-bundle-name", WithStackName("the-stack-name"))

	assert.Equal(t, "the-stack-name", stack.stackName, "stack name should be taken from WithStackName")
}

func TestNewCRDStack_Defaults(t *testing.T) {
	stack := NewCRDStack(nil, nil, "the-bundle-name")

	assert.Equal(t, "the-bundle-name", stack.stackName, "stack name should default to bundle name")
}
