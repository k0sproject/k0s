/*
Copyright 2022 k0s authors

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

package watch_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiwatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/utils/ptr"
)

func TestWatcher(t *testing.T) {
	ctx := func() context.Context {
		ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
		t.Cleanup(cancel)
		return ctx
	}()

	someConfigMap := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "someConfigMap"}}

	t.Run("Cancelation", func(t *testing.T) {
		t.Run("ContextInitiallyCanceled", func(t *testing.T) {
			t.Parallel()
			provider, underTest := newTestWatcher()
			provider.nextList.ListMeta.ResourceVersion = t.Name()
			provider.watch = func(metav1.ListOptions) error {
				provider.ch = openEventChanWith()
				provider.watch = forbiddenWatch(t)
				return nil
			}

			ctx, cancel := context.WithCancel(context.TODO())
			cancel()

			err := underTest.
				WithErrorCallback(forbiddenErrorCallback(t)).
				Until(ctx, forbiddenCondition(t))

			assert.Same(t, err, context.Canceled)
			assert.Equal(t, 1, provider.callsToList)
			assert.Equal(t, 1, provider.callsToWatch)
			assert.Equal(t, 1, provider.callsToStop)
		})

		t.Run("InRetryDelay", func(t *testing.T) {
			t.Parallel()

			provider, underTest := newTestWatcher()
			provider.nextList.ResourceVersion = t.Name()
			provider.watch = func(metav1.ListOptions) error { return assert.AnError }
			var callsToErrorCallback int

			ctx, cancel := context.WithCancel(context.TODO())

			err := underTest.
				WithErrorCallback(func(err error) (time.Duration, error) {
					callsToErrorCallback++
					require.Same(t, assert.AnError, err)
					cancel()
					return 24 * time.Hour, nil
				}).
				Until(ctx, forbiddenCondition(t))

			assert.Same(t, err, context.Canceled)
			assert.Equal(t, 1, provider.callsToList)
			assert.Equal(t, 1, provider.callsToWatch)
			assert.Equal(t, 1, callsToErrorCallback)
			assert.Zero(t, provider.callsToStop)
		})
	})

	t.Run("InitialListError", func(t *testing.T) {
		t.Parallel()
		provider, underTest := newTestWatcher()
		provider.nextListErr = assert.AnError
		var callsToErrorCallback int

		err := underTest.
			WithErrorCallback(func(err error) (time.Duration, error) {
				callsToErrorCallback++
				assert.Same(t, assert.AnError, err)
				return 0, err
			}).
			Until(ctx, forbiddenCondition(t))

		assert.Same(t, assert.AnError, err)
		assert.Equal(t, 1, provider.callsToList)
		assert.Equal(t, 1, callsToErrorCallback)
		assert.Zero(t, provider.callsToWatch)
	})

	t.Run("ListConditionError", func(t *testing.T) {
		t.Parallel()
		provider, underTest := newTestWatcher()
		provider.nextList.Items = []corev1.ConfigMap{someConfigMap}
		var callsToCondition int

		err := underTest.
			WithErrorCallback(forbiddenErrorCallback(t)).
			Until(ctx, func(watched *corev1.ConfigMap) (bool, error) {
				assert.Zero(t, callsToCondition, "Condition called more than once")
				callsToCondition++

				return false, assert.AnError
			})

		assert.Same(t, assert.AnError, err)
		assert.Equal(t, 1, provider.callsToList)
		assert.Zero(t, provider.callsToWatch)
	})

	t.Run("SuccessAfterInitialList", func(t *testing.T) {
		t.Parallel()
		provider, underTest := newTestWatcher()
		provider.nextList.Items = []corev1.ConfigMap{someConfigMap}
		var callsToCondition int

		err := underTest.
			WithErrorCallback(forbiddenErrorCallback(t)).
			Until(ctx, func(watched *corev1.ConfigMap) (bool, error) {
				assert.Zero(t, callsToCondition, "Condition called more than once")
				callsToCondition++

				assert.Equal(t, &someConfigMap, watched)
				return true, nil
			})

		assert.NoError(t, err)
		assert.Equal(t, 1, callsToCondition)
		assert.Equal(t, 1, provider.callsToList)
		assert.Zero(t, provider.callsToWatch)
	})

	t.Run("WatchChannelClosed", func(t *testing.T) {
		t.Skip("doesn't apply in its current form")

		t.Parallel()
		provider, underTest := newTestWatcher()
		provider.nextList.ListMeta.ResourceVersion = t.Name()
		provider.watch = func(metav1.ListOptions) error {
			provider.ch = closedEventChanWith()
			provider.watch = forbiddenWatch(t)
			return nil
		}

		err := underTest.
			WithErrorCallback(forbiddenErrorCallback(t)).
			Until(ctx, forbiddenCondition(t))

		assert.ErrorContains(t, err, "result channel closed unexpectedly")
		assert.Equal(t, 1, provider.callsToList)
		assert.Equal(t, 1, provider.callsToWatch)
		assert.Equal(t, 1, provider.callsToStop)
	})

	t.Run("BadWatchEventTypes", func(t *testing.T) {
		t.Parallel()

		injectedErr := apierrors.NewBadRequest("injected")

		for _, test := range []struct {
			eventType    apiwatch.EventType
			errorMessage string
		}{
			{apiwatch.Error, "watch error"},
			{apiwatch.EventType("Bogus"), "unexpected watch event (Bogus)"},
		} {
			test := test
			t.Run(string(test.eventType), func(t *testing.T) {
				t.Parallel()
				provider, underTest := newTestWatcher()
				provider.nextList.ListMeta.ResourceVersion = t.Name()
				provider.watch = func(opts metav1.ListOptions) error {
					provider.ch = openEventChanWith(apiwatch.Event{
						Type:   test.eventType,
						Object: &injectedErr.ErrStatus,
					})
					provider.watch = forbiddenWatch(t)
					return nil
				}
				var callsToErrorCallback int

				assertErr := func(err error) {
					t.Helper()
					assert.ErrorContains(t, err, test.errorMessage)
					var statusErr *apierrors.StatusError
					if assert.ErrorAs(t, err, &statusErr) {
						assert.Equal(t, injectedErr, statusErr)
					}
				}

				err := underTest.
					WithErrorCallback(func(err error) (time.Duration, error) {
						callsToErrorCallback++
						assertErr(err)
						return 0, err
					}).
					Until(ctx, forbiddenCondition(t))

				assertErr(err)
				assert.Equal(t, 1, provider.callsToList)
				assert.Equal(t, 1, provider.callsToWatch)
				assert.Equal(t, 1, provider.callsToStop)
			})
		}
	})

	t.Run("SuccessAfterWatch", func(t *testing.T) {
		t.Parallel()
		provider, underTest := newTestWatcher()
		provider.nextList.ListMeta.ResourceVersion = t.Name()
		provider.watch = func(opts metav1.ListOptions) error {
			assert.Equal(t, t.Name(), opts.ResourceVersion)
			assert.True(t, opts.AllowWatchBookmarks)
			assert.Equal(t, opts.TimeoutSeconds, ptr.To(int64(120)))

			provider.ch = openEventChanWith(
				apiwatch.Event{
					Type:   apiwatch.Added,
					Object: &someConfigMap,
				})
			provider.watch = forbiddenWatch(t)

			return nil
		}
		var callsToCondition int

		err := underTest.Until(ctx, func(watched *corev1.ConfigMap) (bool, error) {
			assert.Equal(t, 1, provider.callsToWatch, "Expected a single call to watch prior to the condition call")
			assert.Zero(t, callsToCondition, "Condition called more than once")
			callsToCondition++

			assert.Same(t, &someConfigMap, watched)
			return true, nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 1, callsToCondition)
		assert.Equal(t, 1, provider.callsToList)
		assert.Equal(t, 1, provider.callsToWatch)
		assert.Equal(t, 1, provider.callsToStop)
	})

	t.Run("ResourceVersionTooOld", func(t *testing.T) {
		t.Parallel()
		provider, underTest := newTestWatcher()
		provider.nextList.ListMeta.ResourceVersion = t.Name()
		provider.watch = func(metav1.ListOptions) error {
			provider.ch = openEventChanWith(apiwatch.Event{
				Type:   apiwatch.Error,
				Object: &apierrors.NewResourceExpired("injected resource version too old").ErrStatus,
			})

			provider.watch = func(opts metav1.ListOptions) error {
				provider.ch = openEventChanWith(apiwatch.Event{
					Type:   apiwatch.Added,
					Object: &someConfigMap,
				})
				provider.watch = forbiddenWatch(t)

				return nil
			}

			return nil
		}
		var callsToCondition int

		err := underTest.
			WithErrorCallback(forbiddenErrorCallback(t)).
			Until(ctx, func(watched *corev1.ConfigMap) (bool, error) {
				assert.Equal(t, 2, provider.callsToWatch, "Expected two calls to watch prior to the condition call")
				assert.Zero(t, callsToCondition, "Condition called more than once")
				callsToCondition++

				assert.Same(t, &someConfigMap, watched)
				return true, nil
			})

		assert.NoError(t, err)
		assert.Equal(t, 1, callsToCondition)
		assert.Equal(t, 2, provider.callsToList)
		assert.Equal(t, 2, provider.callsToWatch)
		assert.Equal(t, 2, provider.callsToStop)
	})

	t.Run("WatchConditionError", func(t *testing.T) {
		t.Parallel()
		provider, underTest := newTestWatcher()
		provider.nextList.ListMeta.ResourceVersion = t.Name()
		provider.watch = func(opts metav1.ListOptions) error {
			provider.ch = openEventChanWith(apiwatch.Event{
				Type:   apiwatch.Added,
				Object: &someConfigMap,
			})
			provider.watch = forbiddenWatch(t)

			return nil
		}
		var conditionCalled int

		err := underTest.
			WithErrorCallback(forbiddenErrorCallback(t)).
			Until(ctx, func(watched *corev1.ConfigMap) (bool, error) {
				assert.Equal(t, 1, provider.callsToWatch, "Expected a single call to watch prior to the condition call")
				assert.Zero(t, conditionCalled, "Condition called more than once")
				conditionCalled++

				assert.Same(t, &someConfigMap, watched)
				return false, assert.AnError
			})

		assert.Same(t, err, assert.AnError)
		assert.Equal(t, 1, conditionCalled)
		assert.Equal(t, 1, provider.callsToList)
		assert.Equal(t, 1, provider.callsToWatch)
		assert.Equal(t, 1, provider.callsToStop)
	})

	t.Run("BogusObjectInWatchEvent", func(t *testing.T) {
		t.Parallel()

		bogusSecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: "bogusSecret",
			},
		}

		provider, underTest := newTestWatcher()
		provider.nextList.ListMeta.ResourceVersion = t.Name()
		provider.watch = func(opts metav1.ListOptions) error {
			provider.ch = openEventChanWith(apiwatch.Event{
				Type:   apiwatch.Added,
				Object: &bogusSecret,
			})
			provider.watch = forbiddenWatch(t)

			return nil
		}
		var callsToErrorCallback int

		assertErr := func(err error) {
			t.Helper()
			assert.ErrorContains(t, err, `got an event of type "ADDED", expecting an object of type *v1.ConfigMap: `)
			var wrappedErr *apierrors.UnexpectedObjectError
			if assert.ErrorAs(t, err, &wrappedErr) {
				assert.Equal(t, &bogusSecret, wrappedErr.Object)
			}
		}

		err := underTest.
			WithErrorCallback(func(err error) (time.Duration, error) {
				callsToErrorCallback++
				assertErr(err)
				return 0, err
			}).
			Until(ctx, forbiddenCondition(t))

		assertErr(err)
		assert.Equal(t, 1, provider.callsToList)
		assert.Equal(t, 1, provider.callsToWatch)
		assert.Equal(t, 1, callsToErrorCallback)
		assert.Equal(t, 1, provider.callsToStop)
	})

	t.Run("InvalidResourceVersion", func(t *testing.T) {
		t.Parallel()

		invalidConfigMap := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: "0",
			},
		}

		t.Run("InList", func(t *testing.T) {
			t.Parallel()
			provider, underTest := newTestWatcher()
			provider.nextList.ListMeta.ResourceVersion = "0"
			provider.nextList.Items = []corev1.ConfigMap{invalidConfigMap}
			var callsToErrorCallback int
			var callsToCondition int

			err := underTest.
				WithErrorCallback(func(err error) (time.Duration, error) {
					callsToErrorCallback++
					assert.ErrorContains(t, err, `list returned invalid resource version: "0"`)
					return 0, err
				}).
				Until(ctx, func(watched *corev1.ConfigMap) (bool, error) {
					assert.Zero(t, callsToCondition, "Condition called more than once")
					callsToCondition++

					assert.Equal(t, &invalidConfigMap, watched)
					return false, nil
				})

			assert.ErrorContains(t, err, `list returned invalid resource version: "0"`)
			assert.Equal(t, 1, callsToCondition)
			assert.Equal(t, 1, provider.callsToList)
			assert.Equal(t, 1, callsToErrorCallback)
			assert.Zero(t, provider.callsToWatch)
		})

		t.Run("InWatch", func(t *testing.T) {
			t.Parallel()

			provider, underTest := newTestWatcher()
			provider.nextList.ListMeta.ResourceVersion = t.Name()
			provider.watch = func(opts metav1.ListOptions) error {
				provider.ch = openEventChanWith(apiwatch.Event{
					Type:   apiwatch.Added,
					Object: &invalidConfigMap,
				})
				provider.watch = forbiddenWatch(t)

				return nil
			}
			var callsToErrorCallback int
			var callsToCondition int

			assertErr := func(err error) {
				t.Helper()
				assert.ErrorContains(t, err, `invalid resource version: `)
				var wrappedErr *apierrors.UnexpectedObjectError
				if assert.ErrorAs(t, err, &wrappedErr) {
					assert.Equal(t, &invalidConfigMap, wrappedErr.Object)
				}
			}

			err := underTest.
				WithErrorCallback(func(err error) (time.Duration, error) {
					callsToErrorCallback++
					assertErr(err)
					return 0, err
				}).
				Until(ctx, func(watched *corev1.ConfigMap) (bool, error) {
					assert.Zero(t, callsToCondition, "Condition called more than once")
					callsToCondition++

					assert.Same(t, &invalidConfigMap, watched)
					return false, nil
				})

			assertErr(err)
			assert.Equal(t, 1, callsToCondition)
			assert.Equal(t, 1, provider.callsToList)
			assert.Equal(t, 1, provider.callsToWatch)
			assert.Equal(t, 1, callsToErrorCallback)
			assert.Equal(t, 1, provider.callsToStop)
		})
	})

	t.Run("Bookmarking", func(t *testing.T) {
		t.Parallel()

		provider, underTest := newTestWatcher()
		provider.nextList.ListMeta.ResourceVersion = t.Name()
		var second, third func(opts metav1.ListOptions) error
		provider.watch = func(opts metav1.ListOptions) error {
			bookmark := unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"resourceVersion": "the bookmark",
					},
				},
			}

			provider.ch = closedEventChanWith(apiwatch.Event{
				Type:   apiwatch.Bookmark,
				Object: &bookmark,
			})
			provider.watch = second

			return nil
		}
		second = func(opts metav1.ListOptions) error {
			assert.Equal(t, "the bookmark", opts.ResourceVersion)
			assert.True(t, opts.AllowWatchBookmarks)

			provider.ch = closedEventChanWith(apiwatch.Event{
				Type:   apiwatch.Added,
				Object: &someConfigMap,
			})
			provider.watch = third

			return nil
		}
		third = func(opts metav1.ListOptions) error {
			assert.Equal(t, "someConfigMap", opts.ResourceVersion)
			assert.True(t, opts.AllowWatchBookmarks)

			provider.ch = openEventChanWith(apiwatch.Event{
				Type:   apiwatch.Modified,
				Object: &someConfigMap,
			})
			provider.watch = forbiddenWatch(t)

			return nil
		}
		var callsToCondition int

		err := underTest.
			WithErrorCallback(forbiddenErrorCallback(t)).
			Until(ctx, func(watched *corev1.ConfigMap) (bool, error) {
				defer func() { callsToCondition++ }()
				assert.Same(t, &someConfigMap, watched)
				switch callsToCondition {
				case 0:
					return false, nil
				default:
					require.Fail(t, "Unexpected call to condition")
					fallthrough
				case 1:
					return true, nil
				}
			})

		assert.NoError(t, err)
		assert.Equal(t, 1, provider.callsToList)
		assert.Equal(t, 3, provider.callsToWatch)
		assert.Equal(t, 3, provider.callsToStop)
	})

	t.Run("WatchError", func(t *testing.T) {
		t.Parallel()

		provider, underTest := newTestWatcher()
		provider.nextList.ListMeta.ResourceVersion = t.Name()
		provider.watch = func(metav1.ListOptions) error {
			provider.watch = forbiddenWatch(t)
			return assert.AnError
		}
		var callsToErrorCallback int

		err := underTest.
			WithErrorCallback(func(err error) (time.Duration, error) {
				callsToErrorCallback++
				assert.Same(t, assert.AnError, err)
				return 0, err
			}).
			Until(ctx, forbiddenCondition(t))

		assert.Same(t, assert.AnError, err)
		assert.Equal(t, 1, provider.callsToList)
		assert.Equal(t, 1, provider.callsToWatch)
		assert.Equal(t, 1, callsToErrorCallback)
		assert.Zero(t, provider.callsToStop)
	})

	t.Run("ErrorCallbackAllowsRetry", func(t *testing.T) {
		t.Parallel()

		retryError := errors.New("retry me")
		provider, underTest := newTestWatcher()
		provider.nextListErr = retryError
		var callsToErrorCallback int
		var callsToCondition int

		err := underTest.
			WithErrorCallback(func(err error) (retryAfter time.Duration, _ error) {
				underTest.WithErrorCallback(forbiddenErrorCallback(t))
				callsToErrorCallback++

				provider.nextList.Items = []corev1.ConfigMap{someConfigMap}
				provider.nextListErr = nil

				if assert.Equal(t, err, retryError) {
					return time.Duration(0), nil
				}
				return retryAfter, err
			}).
			Until(ctx, func(watched *corev1.ConfigMap) (bool, error) {
				assert.Equal(t, 1, callsToErrorCallback, "Expected a single call to error callback prior to the condition call")
				assert.Zero(t, callsToCondition, "Condition called more than once")
				callsToCondition++

				assert.Equal(t, &someConfigMap, watched)
				return true, nil
			})

		assert.NoError(t, err)
		assert.Equal(t, 2, provider.callsToList)
		assert.Equal(t, 1, callsToErrorCallback)
		assert.Equal(t, 1, callsToCondition)
		assert.Zero(t, provider.callsToWatch)
	})

	t.Run("ErrorCallbackTransformsErr", func(t *testing.T) {
		t.Parallel()

		listError := errors.New("list error")
		transformedError := errors.New("transformed error")
		provider, underTest := newTestWatcher()
		provider.nextListErr = listError
		var callsToErrorCallback int

		err := underTest.
			WithErrorCallback(func(err error) (retryAfter time.Duration, _ error) {
				underTest.WithErrorCallback(forbiddenErrorCallback(t))
				assert.Zero(t, callsToErrorCallback, "error callback called more than once")
				callsToErrorCallback++

				assert.Same(t, listError, err)
				return time.Duration(0), transformedError
			}).
			Until(ctx, forbiddenCondition(t))

		assert.Same(t, transformedError, err)
		assert.Equal(t, 1, provider.callsToList)
		assert.Equal(t, 1, callsToErrorCallback)
		assert.Zero(t, provider.callsToWatch)
	})
}

func forbiddenCondition(t *testing.T) watch.Condition[corev1.ConfigMap] {
	return func(*corev1.ConfigMap) (bool, error) {
		require.Fail(t, "Condition shouldn't be called.")
		return false, nil
	}
}

func forbiddenWatch(t *testing.T) func(metav1.ListOptions) error {
	return func(metav1.ListOptions) error {
		require.Fail(t, "Watch shouldn't be called.")
		return nil
	}
}

func forbiddenErrorCallback(t *testing.T) watch.ErrorCallback {
	return func(err error) (time.Duration, error) {
		require.Fail(t, "ErrorCallback shouldn't be called.", "Error was: %v", err)
		return 0, nil
	}
}

func openEventChanWith(events ...apiwatch.Event) chan apiwatch.Event {
	ch := make(chan apiwatch.Event, len(events))
	for _, event := range events {
		ch <- event
	}
	return ch
}

func closedEventChanWith(events ...apiwatch.Event) chan apiwatch.Event {
	ch := openEventChanWith(events...)
	close(ch)
	return ch
}

func newTestWatcher() (*mockProvider, *watch.Watcher[corev1.ConfigMap]) {
	provider := new(mockProvider)
	return provider, watch.FromClient[*corev1.ConfigMapList, corev1.ConfigMap](provider)
}

type mockProvider struct {
	callsToList int
	nextList    corev1.ConfigMapList
	nextListErr error

	callsToWatch int
	watch        func(metav1.ListOptions) error
	ch           chan apiwatch.Event

	callsToStop int
}

func (m *mockProvider) List(ctx context.Context, opts metav1.ListOptions) (*corev1.ConfigMapList, error) {
	defer func() { m.callsToList++ }()
	if m.nextListErr != nil {
		return nil, m.nextListErr
	}
	return &m.nextList, nil
}

func (m *mockProvider) Watch(ctx context.Context, opts metav1.ListOptions) (apiwatch.Interface, error) {
	defer func() { m.callsToWatch++ }()
	if err := m.watch(opts); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *mockProvider) ResultChan() <-chan apiwatch.Event {
	return m.ch
}

func (m *mockProvider) Stop() {
	m.callsToStop++
}
