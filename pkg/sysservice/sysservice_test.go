// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew_UnsupportedInitSystemIsDetectable(t *testing.T) {
	// k0s reset has to tell "no init system that k0s can manage" apart from a
	// real failure, so that it doesn't report an error on hosts where k0s was
	// never installed as a service in the first place.
	_, err := New("k0scontroller")
	if err == nil {
		t.Skip("this host runs a supported init system")
	}
	assert.ErrorIs(t, err, ErrUnsupportedInitSystem)
}
