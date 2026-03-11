// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"helm.sh/helm/v3/pkg/kube"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	restfake "k8s.io/client-go/rest/fake"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/validation"
	"k8s.io/utils/ptr"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Wait(t *testing.T) {
	log, _ := test.NewNullLogger()

	pendingJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "name"},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To[int32](0),
			Completions:  ptr.To[int32](1),
		},
		Status: batchv1.JobStatus{Succeeded: 0},
	}

	t.Run("JobReadinessHandling", func(t *testing.T) {
		for _, tt := range []struct {
			name      string
			underTest func(*client, kube.ResourceList, time.Duration) error
			verify    func(*testing.T, *resourceTransport, error)
		}{
			{"ForWait", (*client).Wait, func(t *testing.T, rt *resourceTransport, err error) {
				require.NoError(t, err)
				assert.Equal(t, uint32(0), rt.requestsSeen.Load())
			}},
			{"ForWaitWithJobs", (*client).WaitWithJobs, func(t *testing.T, rt *resourceTransport, err error) {
				require.Equal(t, context.DeadlineExceeded, err)
				assert.Equal(t, uint32(3), rt.requestsSeen.Load())
			}},
		} {
			t.Run(tt.name, func(t *testing.T) {
				resources := []*resource.Info{
					{Namespace: pendingJob.Name, Name: pendingJob.Namespace, Object: pendingJob},
				}
				transport := &resourceTransport{inner: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if !assert.Equal(t, http.MethodGet, req.Method) {
						return nil, assert.AnError
					}
					if !assert.Equal(t, "/apis/batch/v1/namespaces/name/jobs/ns", req.URL.Path) {
						return nil, assert.AnError
					}
					return resourceResponse(req, pendingJob), nil
				})}

				synctest.Test(t, func(t *testing.T) {
					underTest := &client{
						Client: &kube.Client{Factory: &fakeKubeFactory{transport}, Log: t.Logf, Namespace: "ns"},
						ctx:    t.Context(),
						log:    log,
					}
					err := tt.underTest(underTest, resources, 5*time.Second)
					tt.verify(t, transport, err)
				})
			})
		}
	})

	t.Run("Context", func(t *testing.T) {
		for _, tt := range []struct {
			name      string
			underTest func(*client, kube.ResourceList, time.Duration) error
			obj       runtime.Object
		}{
			{"ForWaitCanceled", (*client).Wait, &corev1.Pod{}},
			{"ForWaitWithJobsCanceled", (*client).WaitWithJobs, pendingJob},
		} {
			t.Run(tt.name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					ctx, cancel := context.WithCancelCause(t.Context())
					cancel(assert.AnError)
					resources := []*resource.Info{
						{Namespace: "ns", Name: "name", Object: tt.obj},
					}

					transport := &resourceTransport{inner: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
						return resourceResponse(req, tt.obj), nil
					})}

					underTest := &client{
						Client: &kube.Client{Factory: &fakeKubeFactory{transport}, Log: t.Logf, Namespace: "ns"},
						ctx:    ctx,
						log:    log,
					}

					err := tt.underTest(underTest, resources, 5*time.Second)
					require.Error(t, err)
					assert.ErrorIs(t, err, context.Canceled)
				})
			})
		}
	})

	t.Run("WaitStopsOnTerminalError", func(t *testing.T) {
		for _, tt := range []struct {
			name      string
			underTest func(*client, kube.ResourceList, time.Duration) error
		}{
			{"ForWait", (*client).Wait},
			{"ForWaitWithJobs", (*client).WaitWithJobs},
		} {
			t.Run(tt.name, func(t *testing.T) {
				resources := []*resource.Info{
					{Namespace: "ns", Name: "name", Object: &corev1.Pod{}},
				}
				podNotFound := apierrors.NewNotFound(schema.GroupResource{Group: corev1.GroupName, Resource: "pods"}, "name")

				transport := &resourceTransport{inner: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if !assert.Equal(t, http.MethodGet, req.Method) {
						return nil, assert.AnError
					}
					if !assert.Equal(t, "/api/v1/namespaces/ns/pods/name", req.URL.Path) {
						return nil, assert.AnError
					}
					return nil, podNotFound
				})}

				synctest.Test(t, func(t *testing.T) {
					underTest := &client{
						Client: &kube.Client{Factory: &fakeKubeFactory{transport}, Log: t.Logf, Namespace: "ns"},
						ctx:    t.Context(),
						log:    log,
					}

					err := tt.underTest(underTest, resources, 5*time.Second)
					require.ErrorIs(t, err, podNotFound)
					assert.Equal(t, uint32(1), transport.requestsSeen.Load())
				})
			})
		}
	})
}

func TestClient_WaitForDelete(t *testing.T) {
	log, _ := test.NewNullLogger()

	t.Run("Deleted", func(t *testing.T) {
		resources := []*resource.Info{
			makeNotFoundInfo(t, "pods", &corev1.Pod{}, "pod-1"),
			makeNotFoundInfo(t, "pods", &corev1.Pod{}, "pod-2"),
		}

		underTest := &client{ctx: t.Context(), log: log}
		err := underTest.WaitForDelete(resources, 5*time.Second)

		require.NoError(t, err)
		for _, r := range resources {
			transport := r.Client.(*restfake.RESTClient).Client.Transport.(*resourceTransport)
			assert.Equalf(t, uint32(1), transport.requestsSeen.Load(), "Unexpected request count for %s", r.Name)
		}
	})

	t.Run("Context", func(t *testing.T) {
		t.Run("Timeout", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				existingPod := makePodInfo(t, nil)
				transport := existingPod.Client.(*restfake.RESTClient).Client.Transport.(*resourceTransport)
				resources := []*resource.Info{existingPod}

				underTest := &client{ctx: t.Context(), log: log}
				err := underTest.WaitForDelete(resources, 5*time.Second)

				assert.Equal(t, err, context.DeadlineExceeded)
				assert.Equal(t, uint32(3), transport.requestsSeen.Load())
			})

			t.Run("Canceled", func(t *testing.T) {
				ctx, cancel := context.WithCancelCause(t.Context())
				cancel(assert.AnError)
				resources := []*resource.Info{makePodInfo(t, nil)}

				underTest := &client{ctx: ctx, log: log}
				err := underTest.WaitForDelete(resources, 5*time.Second)

				require.Error(t, err)
				assert.ErrorIs(t, err, assert.AnError)
			})
		})
	})
}

func makeNotFoundInfo(t *testing.T, res string, obj runtime.Object, name string) *resource.Info {
	gr := schema.GroupResource{Group: gvkFor(t, obj).Group, Resource: res}
	return makeResourceInfo(t, res, obj, apierrors.NewNotFound(gr, name))
}

func makePodInfo(t *testing.T, err error) *resource.Info {
	return makeResourceInfo(t, "pods", &corev1.Pod{}, err)
}

func makeResourceInfo(t *testing.T, res string, obj runtime.Object, err error) *resource.Info {
	gvk := gvkFor(t, obj)
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) { return nil, err })
	if err == nil {
		var u unstructured.Unstructured
		require.NoError(t, scheme.Scheme.Convert(obj, &u, nil))
		u.SetNamespace("ns")
		u.SetName("name")

		obj, err = scheme.Scheme.New(gvk)
		require.NoError(t, err)
		require.NoError(t, scheme.Scheme.Convert(&u, obj, nil))

		rt = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return resourceResponse(req, obj), nil
		})
	}

	return &resource.Info{
		Namespace: "ns", Name: "name",
		Object: obj,
		Mapping: &meta.RESTMapping{
			Scope:            meta.RESTScopeNamespace,
			Resource:         gvk.GroupVersion().WithResource(res),
			GroupVersionKind: gvk,
		},
		Client: &restfake.RESTClient{
			NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
			GroupVersion:         corev1.SchemeGroupVersion,
			VersionedAPIPath:     "/api",
			Client: &http.Client{
				Transport: &resourceTransport{inner: rt},
			},
		},
	}
}

func gvkFor(t *testing.T, obj runtime.Object) schema.GroupVersionKind {
	kinds, _, err := scheme.Scheme.ObjectKinds(obj)
	require.NoError(t, err)
	require.NotEmpty(t, kinds)
	return kinds[0]
}

type resourceTransport struct {
	requestsSeen atomic.Uint32
	inner        http.RoundTripper
}

func (c *resourceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Let namespaces always exist
	if req.Method == http.MethodGet {
		if resourcePath, name := path.Split(req.URL.Path); resourcePath == "/api/v1/namespaces/" && name != "" {
			return resourceResponse(req, &corev1.Namespace{
				TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Namespace"},
				ObjectMeta: metav1.ObjectMeta{Name: name},
			}), nil
		}
	}

	c.requestsSeen.Add(1)
	return c.inner.RoundTrip(req)
}

func resourceResponse(req *http.Request, r runtime.Object) *http.Response {
	body, _ := json.Marshal(r)
	return &http.Response{
		Proto:         req.Proto,
		ProtoMajor:    req.ProtoMajor,
		ProtoMinor:    req.ProtoMinor,
		Request:       req,
		TLS:           req.TLS,
		StatusCode:    http.StatusOK,
		ContentLength: int64(len(body)),
		Body:          io.NopCloser(bytes.NewReader(body)),
	}
}

type fakeKubeFactory struct {
	transport http.RoundTripper
}

// DynamicClient implements [kube.Factory].
func (f *fakeKubeFactory) DynamicClient() (dynamic.Interface, error) {
	panic("unimplemented")
}

// KubernetesClientSet implements [kube.Factory].
func (f *fakeKubeFactory) KubernetesClientSet() (*kubernetes.Clientset, error) {
	return kubernetes.NewForConfig(&rest.Config{Transport: f.transport})
}

// NewBuilder implements [kube.Factory].
func (f *fakeKubeFactory) NewBuilder() *resource.Builder {
	panic("unimplemented")
}

// ToRawKubeConfigLoader implements [kube.Factory].
func (f *fakeKubeFactory) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	panic("unimplemented")
}

// Validator implements [kube.Factory].
func (f *fakeKubeFactory) Validator(validationDirective string) (validation.Schema, error) {
	panic("unimplemented")
}
