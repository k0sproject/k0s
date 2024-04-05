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

package worker

import (
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
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

func TestHasSystemdResolvedNameserver(t *testing.T) {
	t.Run("nonexistent_file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "resolv.conf")
		detected, err := hasSystemdResolvedNameserver(path)
		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.False(t, detected)
	})

	for _, test := range []struct {
		name     string
		content  string
		expected bool
	}{
		{"empty_file", "", false},
		{"no_nameservers", "search example.com\n", false},
		{"whitespace", "  nameserver\t127.0.0.53   ", false}, // no whitespace allowed in front of keywords
		{"trailing_nonsense", "nameserver\t127.0.0.53  you won't look at me, right?", true},
		{
			"multiple_nameservers_systemd_resolved_first",
			"nameserver 127.0.0.53\nsearch example.com\nnameserver 1.2.3.4",
			false,
		},
		{
			"multiple_nameservers_systemd_resolved_second",
			"nameserver 1.2.3.4\nnameserver 127.0.0.53\nsearch example.com",
			false,
		},
		{
			"commented_nameserver",
			"search example.com\nnameserver 127.0.0.53\n#nameserver 1.2.3.4",
			true,
		},
		{
			"comment_after_nameserver",
			"search example.com\nnameserver 127.0.0.53 # not 1.2.3.4",
			true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "resolv.conf")
			require.NoError(t, os.WriteFile(path, []byte(test.content), 0644))
			detected, err := hasSystemdResolvedNameserver(path)
			if assert.NoError(t, err) {
				assert.Equal(t, test.expected, detected)
			}
		})
	}
}
