/*
Copyright 2024 k0s authors

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

package fakeclient

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"time"
	"unsafe"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/testing"
)

// Creates a new fake clientset backed by the given ObjectTracker.
// This should only be used with the auto-generated fake clientsets.
// Discovery() will return nil.
func NewClientset[T any](discovery *discoveryfake.FakeDiscovery, tracker testing.ObjectTracker) *T {
	p := new(T)

	// Assume that a pointer to T is a fake client.
	c := any(p).(testing.FakeClient)

	// This wire code is adopted from the generated fake clients.
	c.AddReactor("*", "*", testing.ObjectReaction(tracker))
	c.AddWatchReactor("*", func(action testing.Action) (bool, watch.Interface, error) {
		watch, err := tracker.Watch(action.GetResource(), action.GetNamespace())
		if err != nil {
			return false, nil, err
		}
		return true, watch, nil
	})

	// Set the fake clientset's discovery and tracker.
	ty := reflect.TypeOf(p).Elem()
	for i := 0; i < ty.NumField(); i++ {
		f := ty.Field(i)
		if f.Name == "discovery" && f.Type == reflect.TypeFor[*discoveryfake.FakeDiscovery]() {
			*(**discoveryfake.FakeDiscovery)(unsafe.Add(unsafe.Pointer(p), f.Offset)) = discovery
		} else if f.Name == "tracker" && f.Type == reflect.TypeFor[testing.ObjectTracker]() {
			*(*testing.ObjectTracker)(unsafe.Add(unsafe.Pointer(p), f.Offset)) = tracker
		}
	}

	return p
}

func TypedObjectTrackerFrom(scheme *runtime.Scheme, dynamicClient *fake.FakeDynamicClient) *TransformingObjectTracker {
	return &TransformingObjectTracker{
		Inner: dynamicClient.Tracker(),
		Internalize: func(o runtime.Object) (runtime.Object, error) {
			return toUnstructured(scheme, o)
		},
		Externalize: func(o runtime.Object, gvk schema.GroupVersionKind) (runtime.Object, error) {
			return fromUnstructured(scheme, o, gvk)
		},
	}
}

type TransformingObjectTracker struct {
	Inner       testing.ObjectTracker
	Internalize func(runtime.Object) (runtime.Object, error)
	Externalize func(runtime.Object, schema.GroupVersionKind) (runtime.Object, error)
}

var _ testing.ObjectTracker = (*TransformingObjectTracker)(nil)

// Add implements testing.ObjectTracker.
func (t *TransformingObjectTracker) Add(obj runtime.Object) error {
	return t.internalized(obj, t.Inner.Add)
}

// Create implements testing.ObjectTracker.
func (t *TransformingObjectTracker) Create(gvr schema.GroupVersionResource, obj runtime.Object, ns string, opts ...metav1.CreateOptions) error {
	return t.internalized(obj, func(obj runtime.Object) error { return t.Inner.Create(gvr, obj, ns, opts...) })
}

// Delete implements testing.ObjectTracker.
func (t *TransformingObjectTracker) Delete(gvr schema.GroupVersionResource, ns string, name string, opts ...metav1.DeleteOptions) error {
	return t.Inner.Delete(gvr, ns, ns, opts...)
}

// Get implements testing.ObjectTracker.
func (t *TransformingObjectTracker) Get(gvr schema.GroupVersionResource, ns string, name string, opts ...metav1.GetOptions) (runtime.Object, error) {
	obj, err := t.Inner.Get(gvr, ns, name, opts...)
	if err != nil {
		return nil, err
	}

	external, err := t.Externalize(obj, obj.GetObjectKind().GroupVersionKind())
	if err != nil {
		return obj, fmt.Errorf("failed to externalize object: %w", err)
	}

	return external, nil
}

// List implements testing.ObjectTracker.
func (t *TransformingObjectTracker) List(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, ns string, opts ...metav1.ListOptions) (runtime.Object, error) {
	obj, err := t.Inner.List(gvr, gvk, ns, opts...)
	if err != nil {
		return nil, err
	}

	external, err := t.Externalize(obj, gvk)
	if err != nil {
		return obj, fmt.Errorf("failed to externalize object: %w", err)
	}

	// Set a fake resource version, so that watches work.
	if versioned, ok := external.(interface {
		GetResourceVersion() string
		SetResourceVersion(string)
	}); ok {
		if versioned.GetResourceVersion() == "" {
			versioned.SetResourceVersion(strconv.FormatInt(time.Now().UnixMilli(), 10))
		}
	}

	return external, nil
}

// Update implements testing.ObjectTracker.
func (t *TransformingObjectTracker) Update(gvr schema.GroupVersionResource, obj runtime.Object, ns string, opts ...metav1.UpdateOptions) error {
	return t.internalized(obj, func(obj runtime.Object) error { return t.Inner.Update(gvr, obj, ns, opts...) })
}

// Patch implements testing.ObjectTracker.
func (t *TransformingObjectTracker) Patch(gvr schema.GroupVersionResource, obj runtime.Object, ns string, opts ...metav1.PatchOptions) error {
	return t.internalized(obj, func(obj runtime.Object) error { return t.Inner.Patch(gvr, obj, ns, opts...) })
}

// Apply implements testing.ObjectTracker.
func (t *TransformingObjectTracker) Apply(gvr schema.GroupVersionResource, applyConfiguration runtime.Object, ns string, opts ...metav1.PatchOptions) error {
	return t.internalized(applyConfiguration, func(obj runtime.Object) error { return t.Inner.Apply(gvr, applyConfiguration, ns, opts...) })
}

// Watch implements testing.ObjectTracker.
func (t *TransformingObjectTracker) Watch(gvr schema.GroupVersionResource, ns string, opts ...metav1.ListOptions) (watch.Interface, error) {
	w, err := t.Inner.Watch(gvr, ns)
	if err != nil {
		return w, err
	}

	internal := w.ResultChan()
	external := make(chan watch.Event)

	go func() {
		defer close(external)
		for e := range internal {
			if e.Object != nil {
				gvk := e.Object.GetObjectKind().GroupVersionKind()
				if external, err := t.Externalize(e.Object, gvk); err == nil {
					e.Object = external
				} else {
					e = watch.Event{
						Type: watch.Error,
						Object: &metav1.Status{
							Status:  metav1.StatusFailure,
							Message: err.Error(),
							Code:    http.StatusInternalServerError,
						},
					}
				}
			}

			external <- e
		}
	}()

	return &watcher{external, w.Stop}, nil
}

func (t *TransformingObjectTracker) internalized(obj runtime.Object, f func(runtime.Object) error) error {
	internal, err := t.Internalize(obj)
	if err != nil {
		return fmt.Errorf("failed to internalize object: %w", err)
	}
	return f(internal)
}

type watcher struct {
	result <-chan watch.Event
	stop   func()
}

// ResultChan implements watch.Interface.
func (w *watcher) ResultChan() <-chan watch.Event { return w.result }

// Stop implements watch.Interface.
func (w *watcher) Stop() { w.stop() }

func toUnstructured(scheme *runtime.Scheme, obj runtime.Object) (runtime.Object, error) {
	var u unstructured.Unstructured
	if err := scheme.Convert(obj, &u, nil); err != nil {
		return obj, err
	}

	if meta.IsListType(obj) {
		if u.IsList() {
			return u.ToList()
		}
		return obj, fmt.Errorf("not an unstructured list: %T", obj)
	}

	return &u, nil
}

func fromUnstructured(scheme *runtime.Scheme, obj runtime.Object, gvk schema.GroupVersionKind) (runtime.Object, error) {
	if !meta.IsListType(obj) {
		external, err := scheme.New(gvk)
		if err != nil {
			return obj, err
		}

		err = scheme.Convert(obj, external, nil)
		if err != nil {
			return obj, err
		}

		external.GetObjectKind().SetGroupVersionKind(gvk)
		return external, nil
	}

	var items []runtime.Object
	if err := meta.EachListItem(obj, func(obj runtime.Object) error {
		if typed, err := fromUnstructured(scheme, obj, gvk); err != nil {
			return err
		} else {
			items = append(items, typed)
			return nil
		}
	}); err != nil {
		return obj, err
	}

	listGVK := gvk
	listGVK.Kind = listGVK.Kind + "List"

	list, err := scheme.New(listGVK)
	if err != nil {
		return obj, err
	}

	if err := meta.SetList(list, items); err != nil {
		return obj, err
	}

	return list, nil
}
