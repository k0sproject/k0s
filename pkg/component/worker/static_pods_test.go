// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"errors"
	"io"
	"net/http"
	"runtime"
	"testing"
	"time"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const dummyPod = `
apiVersion: v1
kind: Pod
metadata:
  name: dummy-test
  namespace: default
spec:
  containers:
  - image: nginx
    name: web
    ports:
    - containerPort: 80
      name: web
      protocol: TCP
`

func TestStaticPods_Provisioning(t *testing.T) {
	underTest := NewStaticPods()
	underTest.(*staticPods).log, _ = newTestLogger(t)

	t.Run("content_is_initlially_empty", func(t *testing.T) {
		assert.Equal(t, newList(t), getContent(t, underTest))
	})

	podUnderTest, err := underTest.ClaimStaticPod(metav1.NamespaceDefault, "dummy-test")
	require.NoError(t, err)

	t.Run("rejects_claims", func(t *testing.T) {
		for _, test := range []struct{ test, ns, name, err string }{
			{
				"pods_without_a_name",
				metav1.NamespaceDefault, "",
				`invalid name: "": `,
			},
			{
				"pods_without_a_namespace",
				"", "dummy-test",
				`invalid namespace: "": `,
			},
		} {
			t.Run(test.test, func(t *testing.T) {
				_, err := underTest.ClaimStaticPod(test.ns, test.name)
				if assert.Error(t, err) {
					assert.Contains(t, err.Error(), test.err)
				}
			})
		}
	})

	t.Run("rejects", func(t *testing.T) {
		_, err = underTest.ClaimStaticPod(metav1.NamespaceDefault, "dummy-test")
		if assert.Error(t, err) {
			assert.Equal(t, "default/dummy-test is already claimed", err.Error())
		}

		for _, test := range []struct {
			name string
			pod  any
			err  string
		}{
			{
				"non_pods",
				&corev1.Pod{
					TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
				},
				"not a Pod: v1/Secret",
			},
			{
				"pods_not_matching_the_claim",
				&corev1.Pod{
					TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
					ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "bar"},
				},
				`attempt to set the manifest to "foo/bar", whereas "default/dummy-test" was claimed`,
			},
			{
				"unknown_fields",
				`{"apiVersion": "v1", "kind": "Pod", "spec":{"foo": "bar"}}`,
				`error unmarshaling JSON: while decoding JSON: json: unknown field "foo"`,
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				err := podUnderTest.SetManifest(test.pod)
				if assert.Error(t, err) {
					assert.Equal(t, test.err, err.Error())
				}
				assert.Equal(t, newList(t), getContent(t, underTest))
			})
		}
	})

	t.Run("accepts", func(t *testing.T) {
		expected := newList(t, []byte(dummyPod))

		for _, test := range []struct {
			name string
			pod  any
		}{
			{"bytes", []byte(dummyPod)},
			{"strings", dummyPod},
		} {
			t.Run(test.name, func(t *testing.T) {
				assert.NoError(t, podUnderTest.SetManifest(test.pod))
				assert.Equal(t, expected, getContent(t, underTest))
			})
		}
	})

	t.Run("sets_pod_manifests", func(t *testing.T) {
		replaced := `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"dummy-test","namespace":"` + metav1.NamespaceDefault + `"}}`
		expected := newList(t, []byte(replaced))

		assert.NoError(t, podUnderTest.SetManifest(dummyPod))
		assert.NoError(t, podUnderTest.SetManifest(replaced))

		assert.Equal(t, expected, getContent(t, underTest))

		podUnderTest.Clear()

		assert.Equal(t, newList(t), getContent(t, underTest))

		assert.NoError(t, podUnderTest.SetManifest(replaced))

		assert.Equal(t, expected, getContent(t, underTest))
	})

	t.Run("drops_pods", func(t *testing.T) {
		podUnderTest.Drop()
		assert.Equal(t, newList(t), getContent(t, underTest))
		err := podUnderTest.SetManifest(dummyPod)
		if assert.Error(t, err) {
			assert.Equal(t, "already dropped", err.Error())
		}
		assert.Equal(t, newList(t), getContent(t, underTest))
	})
}

func TestStaticPods_Lifecycle(t *testing.T) {
	log, logs := newTestLogger(t)

	underTest := NewStaticPods().(*staticPods)
	underTest.log = log
	podUnderTest, err := underTest.ClaimStaticPod(metav1.NamespaceDefault, "dummy-test")
	require.NoError(t, err)
	assert.NoError(t, podUnderTest.SetManifest(dummyPod))

	t.Run("url_is_unavailable_before_init", func(t *testing.T) {
		_, err := underTest.ManifestURL()
		require.Error(t, err)
		assert.Equal(t, "static_pods component is not yet running", err.Error())
	})

	t.Run("fails_to_run_without_init", func(t *testing.T) {
		err := underTest.Start(t.Context())
		require.Error(t, err)
		require.Equal(t, "static_pods component is not yet initialized", err.Error())
	})

	t.Run("health_check_fails_without_init", func(t *testing.T) {
		err := underTest.Ready()
		require.Error(t, err)
		require.Equal(t, "static_pods component is not yet running", err.Error())
	})

	t.Run("fails_to_stop_without_init", func(t *testing.T) {
		err := underTest.Stop()
		require.Error(t, err)
		require.Equal(t, "static_pods component is not yet running", err.Error())
	})

	t.Run("init", func(t *testing.T) {
		require.NoError(t, underTest.Init(t.Context()))
	})

	t.Run("another_init_fails", func(t *testing.T) {
		err := underTest.Init(t.Context())
		if assert.Error(t, err) {
			assert.Equal(t, "static_pods component is already initialized", err.Error())
		}
	})

	t.Run("url_is_unavailable_after_init", func(t *testing.T) {
		_, err := underTest.ManifestURL()
		require.Error(t, err)
		assert.Equal(t, "static_pods component is not yet running", err.Error())
	})

	t.Run("health_check_fails_before_run", func(t *testing.T) {
		err := underTest.Ready()
		require.Error(t, err)
		require.Equal(t, "static_pods component is not yet running", err.Error())
	})

	t.Run("stop_before_run_fails", func(t *testing.T) {
		err := underTest.Stop()
		require.Error(t, err)
		assert.Equal(t, "static_pods component is not yet running", err.Error())
	})

	t.Run("runs", func(runT *testing.T) {
		if assert.NoError(runT, underTest.Start(t.Context())) {
			t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })

			var lastLog *logrus.Entry
			require.NoError(t, retry.Do(func() error {
				lastLog = logs.LastEntry()
				if lastLog == nil {
					return errors.New("not yet logged")
				}
				return nil
			}, retry.Attempts(5)))

			assert.Equal(t, "Serving HTTP requests", lastLog.Message)
			assert.Contains(t, lastLog.Data["local_addr"], "127.0.0.1")
		}
	})

	t.Run("another_run_fails", func(t *testing.T) {
		err := underTest.Start(t.Context())
		require.Error(t, err)
		assert.Equal(t, "static_pods component is already running", err.Error())
	})

	t.Run("health_check_works", func(t *testing.T) {
		err := underTest.Ready()
		assert.NoError(t, err)
		lastLog := logs.LastEntry()
		require.Equal(t, "Answering health check", lastLog.Message)
		assert.Contains(t, lastLog.Data["local_addr"], "127.0.0.1")
		assert.Contains(t, lastLog.Data["remote_addr"], "127.0.0.1")
	})

	t.Run("serves_content", func(t *testing.T) {
		dummyPod, err := yaml.YAMLToJSON([]byte(dummyPod))
		require.NoError(t, err)
		expectedContent := `{"apiVersion":"v1","kind":"PodList","items":[` + string(dummyPod) + "]}"

		url, err := underTest.ManifestURL()
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, url.String(), nil)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		t.Cleanup(cancel)

		req = req.WithContext(ctx)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.JSONEq(t, expectedContent, string(body))

		lastLog := logs.LastEntry()
		require.NotNil(t, lastLog)
		assert.Contains(t, lastLog.Message, "Writing content: ")
		assert.Contains(t, lastLog.Data["local_addr"], "127.0.0.1")
		assert.Contains(t, lastLog.Data["remote_addr"], "127.0.0.1")
	})

	t.Run("stops", func(t *testing.T) {
		require.NoError(t, underTest.Stop())
	})

	t.Run("health_check_fails_after_stopped", func(t *testing.T) {
		expectedErrMsg := "connection refused"
		if runtime.GOOS == "windows" {
			expectedErrMsg = "No connection could be made because the target machine actively refused it."
		}

		err := underTest.Ready()
		require.ErrorContains(t, err, expectedErrMsg)
	})

	t.Run("does_not_serve_content_anymore", func(t *testing.T) {
		expectedErrMsg := "connection refused"
		if runtime.GOOS == "windows" {
			expectedErrMsg = "No connection could be made because the target machine actively refused it."
		}

		url, err := underTest.ManifestURL()
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, url.String(), nil)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		t.Cleanup(cancel)

		resp, err := http.DefaultClient.Do(req.WithContext(ctx))
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), expectedErrMsg)
		} else {
			assert.NoError(t, resp.Body.Close())
		}
	})

	t.Run("stop_may_be_called_again", func(t *testing.T) {
		require.NoError(t, underTest.Stop())
	})

	t.Run("claimed_pod_may_be_dropped_after_stop", func(t *testing.T) {
		podUnderTest.Drop()
	})

	t.Run("reinit_fails", func(t *testing.T) {
		err := underTest.Init(t.Context())
		require.Error(t, err)
		assert.Equal(t, "static_pods component is already stopped", err.Error())
	})

	t.Run("rerun_fails", func(t *testing.T) {
		err := underTest.Start(t.Context())
		require.Error(t, err)
		assert.Equal(t, "static_pods component is already stopped", err.Error())
	})
}

func getContent(t *testing.T, underTest StaticPods) (content map[string]any) {
	require.NoError(t, yaml.Unmarshal(underTest.(*staticPods).content(), &content))
	return
}

func newList(t *testing.T, items ...[]byte) map[string]any {
	parsedItems := []any{}
	for _, item := range items {
		var parsedItem map[string]any
		require.NoError(t, yaml.Unmarshal(item, &parsedItem))
		parsedItems = append(parsedItems, parsedItem)
	}

	return map[string]any{
		"apiVersion": "v1",
		"kind":       "PodList",
		"items":      parsedItems,
	}
}

func newTestLogger(t *testing.T) (*logrus.Logger, *test.Hook) {
	t.Helper()
	log, logs := test.NewNullLogger()
	log.SetLevel(logrus.DebugLevel)
	t.Cleanup(func() {
		t.Helper()
		if !t.Failed() {
			return
		}
		entries := logs.AllEntries()
		for _, entry := range entries {
			t.Log("Captured log:", entry.Level, entry.Message, entry.Data)
		}
		if len(entries) == 0 {
			t.Log("No log entries captured")
		}
	})

	return log, logs
}
