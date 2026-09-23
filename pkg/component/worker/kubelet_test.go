// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/require"
)

func TestParseTaints(t *testing.T) {
	cases := []struct {
		name          string
		spec          string
		expectedTaint corev1.Taint
		expectedErr   bool
	}{
		{
			name:        "invalid spec format",
			spec:        "",
			expectedErr: true,
		},
		{
			name:        "invalid spec format",
			spec:        "foo=abc",
			expectedErr: true,
		},
		{
			name:        "invalid spec format",
			spec:        "foo=abc=xyz:NoSchedule",
			expectedErr: true,
		},
		{
			name:        "invalid spec format",
			spec:        "foo=abc:xyz:NoSchedule",
			expectedErr: true,
		},
		{
			name:        "invalid spec effect",
			spec:        "foo=abc:invalid_effect",
			expectedErr: true,
		},
		{
			name: "full taint",
			spec: "foo=abc:NoSchedule",
			expectedTaint: corev1.Taint{
				Key:    "foo",
				Value:  "abc",
				Effect: corev1.TaintEffectNoSchedule,
			},
			expectedErr: false,
		},
	}

	for _, c := range cases {
		taint, err := parseTaint(c.spec)
		if c.expectedErr && err == nil {
			t.Errorf("[%s] expected error for spec %s, but got nothing", c.name, c.spec)
		}
		if !c.expectedErr && err != nil {
			t.Errorf("[%s] expected no error for spec %s, but got: %v", c.name, c.spec, err)
		}
		require.Equal(t, c.expectedTaint, taint)
	}
}
