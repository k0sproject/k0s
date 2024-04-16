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
	"reflect"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/utils/ptr"
)

type VersionedResource interface {
	GetResourceVersion() string
}

// Watcher offers a convenient way of watching Kubernetes resources. An
// ephemeral alternative to Reflectors and Indexers, useful for one-shot tasks
// when no caching is required. It performs an initial list of all the resources
// and then starts watching them. In case the watch needs to be restarted
// (a.k.a. resource version too old), Watcher will re-list all the resources.
// The Watcher will restart the watch API call from time to time at the last
// seen resource version, so that stale HTTP connections won't make the watch go
// stale, too.
type Watcher[T any] struct {
	List  func(ctx context.Context, opts metav1.ListOptions) (resourceVersion string, items []T, err error)
	Watch func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)

	includeDeletions bool
	fieldSelector    string
	labelSelector    string
	errorCallback    ErrorCallback
}

// Condition is a func that gets called by [Watcher] for each updated item. The
// watch will terminate successfully if it returns true, continue if it returns
// false or terminate with the returned error.
type Condition[T any] func(item *T) (done bool, err error)

// ErrorCallback is a func that, if specified, will be called by the [Watcher]
// whenever it encounters some error. Whenever the returned error is nil, the
// Watcher will wait for the specified duration and retry the last call.
// Otherwise the Watcher will return the returned error.
type ErrorCallback = func(error) (retryDelay time.Duration, err error)

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
func FromProvider[L VersionedResource, I any](provider Provider[L], itemsFromList func(L) []I) *Watcher[I] {
	return &Watcher[I]{
		List: func(ctx context.Context, opts metav1.ListOptions) (string, []I, error) {
			list, err := provider.List(ctx, opts)
			if err != nil {
				return "", nil, err
			}
			return list.GetResourceVersion(), itemsFromList(list), nil
		},
		Watch: provider.Watch,

		fieldSelector: fields.Everything().String(),
		labelSelector: labels.Everything().String(),
	}
}

// IsRetryable checks if the given error might make sense to be retried in the
// context of watching Kubernetes resources. Returns the retry delay and no
// error if it's retryable, or the passed in error otherwise.
func IsRetryable(err error) (time.Duration, error) {
	// Only consider errors that suggest a client delay ...
	if delaySecs, ok := apierrors.SuggestsClientDelay(err); ok {
		// ... and whose reason indicates that retries might make sense.
		switch apierrors.ReasonForError(err) {
		case metav1.StatusReasonTooManyRequests,
			metav1.StatusReasonServerTimeout,
			metav1.StatusReasonTimeout,
			metav1.StatusReasonServiceUnavailable:
			return time.Duration(delaySecs) * time.Second, nil
		}
	}

	return 0, err
}

// Ensure that [IsRetryable] is a valid error callback.
var _ ErrorCallback = IsRetryable

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

// WithFieldSelector sets the given field selector for this Watcher. The default
// is to match everything:
//
//	watcher.FromClient(...).WithFieldSelector(fields.Everything())
//
// Refer to the [concept] for a general introduction to field selectors. To gain
// an overview of the supported values, have a look at the usages of
// [k8s.io/apimachinery/pkg/runtime.Scheme.AddFieldLabelConversionFunc] in the
// [Kubernetes codebase].
//
// [concept]: https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/
// [Kubernetes codebase]: https://sourcegraph.com/search?q=lang:go+AddFieldLabelConversionFunc%28...%29+repo:%5Egithub%5C.com/kubernetes/kubernetes%24+-file:_test%5C.go%24+select:content&patternType=structural
func (w *Watcher[T]) WithFieldSelector(selector fields.Selector) *Watcher[T] {
	w.fieldSelector = selector.String()
	return w
}

// WithLabelSelector sets the given label selector for this Watcher. The default
// is to match everything:
//
//	watcher.FromClient(...).WithLabelSelector(labels.Everything())
func (w *Watcher[T]) WithLabelSelector(selector labels.Selector) *Watcher[T] {
	w.labelSelector = selector.String()
	return w
}

// WithLabels sets this Watcher's label selector to match exactly the given Set.
// A nil and empty Sets are considered equivalent to labels.Everything(). It
// does not perform any validation, which means the server will reject the
// request if the Set contains invalid values.
func (w *Watcher[T]) WithLabels(l labels.Set) *Watcher[T] {
	return w.WithLabelSelector(labels.SelectorFromSet(l))
}

// WithErrorCallback sets this Watcher's error callback. It's invoked every time
// an error occurs and determines if the watch should continue or terminate:
//   - The returned error is nil: continue watching
//   - The returned error is not nil: terminate watch with that error
//
// If the error callback is not set or nil, the default behavior is to terminate.
func (w *Watcher[T]) WithErrorCallback(callback ErrorCallback) *Watcher[T] {
	w.errorCallback = callback
	return w
}

// Until runs a watch until condition returns true. It will error out in case
// the context gets canceled or the condition returns an error.
func (w *Watcher[T]) Until(ctx context.Context, condition Condition[T]) error {
	return retry(ctx, w.errorCallback, func(ctx context.Context) error {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		return w.run(ctx, condition)
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

// conditionError indicates that an error originated from a [Condition]. Those
// errors will never be presented to the error callback, but terminate the watch
// immediately.
type conditionError struct{ error }

type startWatch struct {
	resourceVersion string
}

func (w *Watcher[T]) run(ctx context.Context, condition Condition[T]) error {
	startWatch, err := w.list(ctx, condition)
	if err != nil {
		return err
	}

	for startWatch != nil {
		startWatch, err = w.watch(ctx, startWatch.resourceVersion, condition)
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *Watcher[T]) list(ctx context.Context, condition Condition[T]) (*startWatch, error) {
	const maxListDurationSecs = 30
	ctx, cancel := context.WithTimeout(ctx, (maxListDurationSecs+10)*time.Second)
	defer cancel()
	resourceVersion, items, err := w.List(ctx, metav1.ListOptions{
		FieldSelector:  w.fieldSelector,
		LabelSelector:  w.labelSelector,
		TimeoutSeconds: ptr.To(int64(maxListDurationSecs)),
	})
	if err != nil {
		return nil, err
	}

	for i := range items {
		done, err := condition(&items[i])
		if err != nil {
			return nil, conditionError{err}
		}
		if done {
			return nil, nil // terminate successfully
		}
	}

	if !isResourceVersionValid(resourceVersion) {
		return nil, fmt.Errorf("list returned invalid resource version: %q", resourceVersion)
	}

	return &startWatch{resourceVersion}, nil
}

func (w *Watcher[T]) watch(ctx context.Context, resourceVersion string, condition Condition[T]) (*startWatch, error) {
	const maxWatchDurationSecs = 120
	watcher, err := w.Watch(ctx, metav1.ListOptions{
		ResourceVersion:     resourceVersion,
		AllowWatchBookmarks: true,
		FieldSelector:       w.fieldSelector,
		LabelSelector:       w.labelSelector,
		TimeoutSeconds:      ptr.To(int64(maxWatchDurationSecs)),
	})
	if err != nil {
		return nil, err
	}
	defer watcher.Stop()

	watchTimeout := time.NewTimer((maxWatchDurationSecs + 10) * time.Second)
	defer watchTimeout.Stop()

	startWatch := &startWatch{resourceVersion}
	for startWatch != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case <-watchTimeout.C:
			return nil, apierrors.NewTimeoutError("server unexpectedly didn't close the watch", 1)

		case event, ok := <-watcher.ResultChan():
			if !ok {
				// The server closed the watch remotely.
				// This usually happens after maxWatchDurationSecs have passed.
				return startWatch, nil
			}

			startWatch, err = w.processWatchEvent(&event, condition)
			if err != nil {
				return nil, err
			}
		}
	}

	return nil, nil // terminate successfully
}

func (w *Watcher[T]) processWatchEvent(event *watch.Event, condition Condition[T]) (*startWatch, error) {
	switch event.Type {
	case watch.Added, watch.Modified, watch.Deleted:
		if w.includeDeletions || event.Type != watch.Deleted {
			item, ok := any(event.Object).(*T)
			if !ok {
				var example T
				var err error = &apierrors.UnexpectedObjectError{Object: event.Object}
				return nil, fmt.Errorf("got an event of type %q, expecting an object of type %T: (%T) %w", event.Type, &example, event.Object, err)
			}

			if done, err := condition(item); err != nil {
				return nil, conditionError{err}
			} else if done {
				return nil, nil // terminate successfully
			}
		}

		fallthrough // update resource version

	case watch.Bookmark:
		nextResourceVersion, err := getResourceVersion(event.Object)
		if err != nil {
			return nil, err
		}
		return &startWatch{nextResourceVersion}, nil

	case watch.Error:
		return nil, fmt.Errorf("watch error: %w", apierrors.FromObject(event.Object))

	default:
		return nil, fmt.Errorf("unexpected watch event (%s): %w", event.Type, apierrors.FromObject(event.Object))
	}
}

func getResourceVersion(resource runtime.Object) (string, error) {
	if rv, ok := resource.(VersionedResource); ok {
		resourceVersion := rv.GetResourceVersion()
		if !isResourceVersionValid(resourceVersion) {
			var err error = &apierrors.UnexpectedObjectError{Object: resource}
			return "", fmt.Errorf("invalid resource version: %w", err)
		}
		return resourceVersion, nil
	}

	var err error = &apierrors.UnexpectedObjectError{Object: resource}
	return "", fmt.Errorf("failed to get resource version: %w", err)
}

func isResourceVersionValid(resourceVersion string) bool {
	// https://github.com/kubernetes/kubernetes/issues/74022
	switch resourceVersion {
	case "", "0":
		return false
	default:
		return true
	}
}

func retry(ctx context.Context, errorCallback ErrorCallback, runWatch func(context.Context) error) error {
	for {
		err := runWatch(ctx)
		if err == nil {
			// No error means the user-specified condition returned success.
			// The watch is done.
			return nil
		}

		var condErr conditionError
		if errors.As(err, &condErr) {
			// The user-specified condition returned an error.
			return condErr.error
		}

		if ctx.Err() != nil {
			// The context has been closed. Good bye.
			return err
		}

		if apierrors.IsResourceExpired(err) {
			// Start over without delay (resource version too old)
			continue
		}

		// Ask the error callback about any other errors.
		if errorCallback != nil {
			retryDelay, err := errorCallback(err)
			if err != nil {
				return err
			}

			// Retry after some timeout.
			timer := time.NewTimer(retryDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
				continue
			}
		}

		return err
	}
}
