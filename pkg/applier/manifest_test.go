// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package applier

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	configMap := func(name string) *corev1.ConfigMap {
		return &corev1.ConfigMap{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: name},
		}
	}
	objects := []runtime.Object{configMap("a"), configMap("b")}
	require.NoError(t, WriteManifest(path, objects))
	data, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)
	assert.Equal(t, "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\n  namespace: ns\n---\n"+
		"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: b\n  namespace: ns\n", string(data))

	// An unchanged manifest isn't written again, so the applier doesn't see a change.
	past := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(path, past, past))
	require.NoError(t, WriteManifest(path, objects))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, info.ModTime().Equal(past))
}
