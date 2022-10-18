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

package watch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/utils/pointer"
)

// Watcher offers a convenient way of watching Kubernetes resources. An
// ephemeral alternative to Reflectors and Indexers, useful for one-shot tasks
// when no caching is required. It performs an initial list of all the resources
// and then starts watching them. In case the watch needs to be restarted
// (a.k.a. resource too old), the watcher will re-list all the resources.
type Watcher[T any] struct {
	List  func(ctx context.Context, opts metav1.ListOptions) (resourceVersion string, items []T, err error)
	Watch func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)

	includeDeletions bool
	fieldSelector    string
	errorCallback    func(error) error
}

// Provider represents the backend for [Watcher].
// It is compatible with client-go's typed interfaces.
type Provider[L any] interface {
	List(ctx context.Context, opts metav1.ListOptions) (L, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
}

// FromClient creates a [Watcher] from the given client-go client. Note that the
// types L and I need to be connected in a way that L is a pointer type to a
// struct that has an `Items` field of type []I. This function will panic if
// this is not the case. Refer to [FromProvider] in order to provide a custom
// way of obtaining items from the list type.
func FromClient[L metav1.ListInterface, I any](client Provider[L]) *Watcher[I] {
	itemsFromList, err := itemsFromList[L, I]()
	if err != nil {
		panic(err)
	}
	return FromProvider(client, itemsFromList)
}

// FromProvider creates a [Watcher] from the given [Provider] and the
// corresponding itemsFromList function.
func FromProvider[L interface{ GetResourceVersion() string }, I any](provider Provider[L], itemsFromList func(L) []I) *Watcher[I] {
	return &Watcher[I]{
		List: func(ctx context.Context, opts metav1.ListOptions) (string, []I, error) {
			list, err := provider.List(ctx, opts)
			if err != nil {
				return "", nil, err
			}
			return list.GetResourceVersion(), itemsFromList(list), nil
		},
		Watch: provider.Watch,
	}
}

// IncludingDeletions will include deleted items in watches.
func (w *Watcher[T]) IncludingDeletions() *Watcher[T] {
	w.includeDeletions = true
	return w
}

// ExcludingDeletions will suppress deleted items from watches.
// This is the default.
func (w *Watcher[T]) ExcludingDeletions() *Watcher[T] {
	w.includeDeletions = false
	return w
}

// WithObjectName sets this Watcher's field selector in a way to only match
// objects with the given name.
func (w *Watcher[T]) WithObjectName(name string) *Watcher[T] {
	return w.WithFieldSelector(fields.OneTermEqualSelector(metav1.ObjectNameField, name))
}

// WithFieldSelector sets the given field selector for this Watcher. Refer to
// the [concept] for a general introduction to field selectors. To gain an
// overview of the supported values, have a look at the usages of
// [k8s.io/apimachinery/pkg/runtime.Scheme.AddFieldLabelConversionFunc] in the
// [Kubernetes codebase].
//
// [concept]: https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/
// [Kubernetes codebase]: https://sourcegraph.com/search?q=lang:go+AddFieldLabelConversionFunc(...)+repo:^github\.com/kubernetes/kubernetes%24+-file:_test\.go%24+select:content&patternType=structural
func (w *Watcher[T]) WithFieldSelector(selector fields.Selector) *Watcher[T] {
	w.fieldSelector = selector.String()
	return w
}

// WithoutFieldSelector clears any field selector from this Watcher.
// This is the default.
func (w *Watcher[T]) WithoutFieldSelector() *Watcher[T] {
	w.fieldSelector = ""
	return w
}

// WithErrorCallback sets this Watcher's error callback. It's invoked every time
// an error occurs and determines if the watch should continue or terminate:
//   - The returned error is nil: continue watching
//   - The returned error is not nil: terminate watch with that error
//
// If the error callback is not set or nil, the default behavior is to terminate.
func (w *Watcher[T]) WithErrorCallback(callback func(error) error) *Watcher[T] {
	w.errorCallback = callback
	return w
}

// Until runs a watch until condition returns true. It will error out in case
// the context gets canceled or the condition returns an error.
func (w *Watcher[T]) Until(ctx context.Context, condition func(*T) (bool, error)) error {
	listCondition := func(items []T) (bool, error) {
		for i := range items {
			ok, err := condition(&items[i])
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}

		return false, nil
	}

	return retry(ctx, w.errorCallback, func(ctx context.Context) error {
		return w.run(ctx, listCondition, condition)
	})
}

func itemsFromList[L metav1.ListInterface, I any]() (func(L) []I, error) {
	// List types from client-go don't provide any methods to get their items.
	// Hence provide a way to get the items via reflection.

	index, err := func() ([]int, error) {
		var list L
		var items []I
		listType := reflect.TypeOf(list)
		if listType.Kind() != reflect.Pointer {
			return nil, fmt.Errorf("not a pointer type: %s", listType)
		}
		itemsType := reflect.TypeOf(items)
		itemsField, ok := listType.Elem().FieldByName("Items")
		if !ok || itemsField.Type != itemsType {
			return nil, fmt.Errorf(`expected an "Items" field of type %s in %s`, itemsType, listType)
		}
		return itemsField.Index, nil
	}()
	if err != nil {
		return nil, err
	}

	return func(l L) []I {
		// The compatibility of the types has been checked above.
		// This will not panic at runtime.
		return reflect.ValueOf(l).Elem().FieldByIndex(index).Interface().([]I)
	}, nil
}

type noRetry struct{ error }

var errResourceTooOld = errors.New("resource too old")

func (w *Watcher[T]) run(ctx context.Context, listCallback func([]T) (bool, error), watchCallback func(*T) (bool, error)) error {
	var initialResourceVersion string
	{
		resourceVersion, items, err := w.List(ctx, metav1.ListOptions{
			FieldSelector:  w.fieldSelector,
			TimeoutSeconds: pointer.Int64(30),
		})
		if err != nil {
			return err
		}
		if ok, err := listCallback(items); err != nil {
			return noRetry{err}
		} else if ok {
			return nil
		}

		initialResourceVersion = resourceVersion
	}

	watcher, err := watchtools.NewRetryWatcher(initialResourceVersion, &cache.ListWatch{
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			opts.FieldSelector = w.fieldSelector
			opts.TimeoutSeconds = pointer.Int64(30)
			return w.Watch(ctx, opts)
		},
	})
	if err != nil {
		return err
	}
	defer watcher.Stop()

	for {
		var suppressDeletions bool
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return errors.New("result channel closed unexpectedly")
			}

			switch event.Type {
			case watch.Bookmark:
				continue // nothing to do, handled by RetryWatcher

			case watch.Deleted:
				suppressDeletions = !w.includeDeletions
				fallthrough

			case watch.Added, watch.Modified:
				item, ok := any(event.Object).(*T)
				if !ok {
					var example T
					err = error(&apierrors.UnexpectedObjectError{Object: event.Object})
					return fmt.Errorf("got an event of type %q, expecting an object of type %T: (%T) %w", event.Type, &example, event.Object, err)
				}

				if suppressDeletions && isDeleted(item) {
					continue
				}

				if ok, err := watchCallback(item); err != nil {
					return noRetry{err}
				} else if ok {
					return nil
				}

			case watch.Error:
				fallthrough // go to default case for error handling

			default:
				err := apierrors.FromObject(event.Object)
				var statusErr *apierrors.StatusError
				if errors.As(err, &statusErr) && statusErr.ErrStatus.Code == http.StatusGone {
					return errResourceTooOld
				}

				return err
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func isDeleted(resource any) bool {
	deletable, ok := resource.(interface{ GetDeletionTimestamp() *metav1.Time })
	return ok && deletable.GetDeletionTimestamp() != nil
}

func retry(ctx context.Context, errorCallback func(error) error, runWatch func(context.Context) error) error {
	for {
		err := runWatch(ctx)
		if err == nil {
			// No error means the callbacks returned success. The watch is done.
			return nil
		}

		if err == errResourceTooOld {
			// Start over without delay.
			continue
		}

		if err == ctx.Err() {
			// The context has been canceled. Good bye.
			return err
		}

		if err, ok := err.(noRetry); ok {
			return err.error
		}

		// Ask the callback about any other errors.
		if errorCallback != nil {
			err = errorCallback(err)
		}

		if err != nil {
			return err
		}

		// Retry in 10 secs.
		select {
		case <-time.After(10 * time.Second):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
