// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package fakeclient

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"time"
	"unsafe"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic/fake"
	metadatafake "k8s.io/client-go/metadata/fake"
	"k8s.io/client-go/testing"
)

// Creates a new fake clientset backed by the given ObjectTracker.
// This should only be used with the auto-generated fake clientsets.
func NewClientset[T any, PT interface {
	*T
	testing.FakeClient
}](discovery *discoveryfake.FakeDiscovery, tracker testing.ObjectTracker) *T {
	p := PT(new(T))

	// This wire code is adopted from the generated fake clients.
	p.AddReactor("*", "*", testing.ObjectReaction(tracker))
	p.AddWatchReactor("*", func(action testing.Action) (bool, watch.Interface, error) {
		var opts metav1.ListOptions
		if watchAction, ok := action.(testing.WatchActionImpl); ok {
			opts = watchAction.ListOptions
		}
		watch, err := tracker.Watch(action.GetResource(), action.GetNamespace(), opts)
		if err != nil {
			return false, nil, err
		}
		return true, watch, nil
	})

	// Set the fake clientset's discovery and tracker.
	ty := reflect.TypeFor[T]()
	for f := range ty.Fields() {
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

// Creates a new fake metadata client that is backed by the given client. Note
// that this client doesn't support write operations besides patching. Use the
// given client directly for writing instead.
func NewMetadataClient(client *fake.FakeDynamicClient) *metadatafake.FakeMetadataClient {
	// The fake metadata client lists with a made-up group and an empty kind.
	// That resulting kind needs to be known to the dynamic client's scheme.
	// Core v1 "List" is: it's registered as an unstructured list, like any
	// other kind ending in "List". The resulting object is externalized per
	// item anyways, so the actual list kind doesn't matter.
	listKind := corev1.SchemeGroupVersion.WithKind("")

	dynTracker := client.Tracker()
	metadataClient := NewClientset[metadatafake.FakeMetadataClient](nil, &TransformingObjectTracker{
		Inner:    dynTracker,
		ListKind: func(schema.GroupVersionKind) schema.GroupVersionKind { return listKind },
		Internalize: func(runtime.Object) (runtime.Object, error) {
			panic("the fake metadata client is read-and-patch only")
		},
		Externalize: func(o runtime.Object, _ schema.GroupVersionKind) (runtime.Object, error) {
			return toPartialObjectMetadata(o)
		},
	})

	// Apply patches via the dynamic client's tracker, so that they're applied
	// to the full objects, like a real API server would do. Applying them via
	// the metadata client's tracker would apply them to the metadata-only view
	// instead, and then overwrite the full objects with that partial view.
	// NB: The API server applies patches to the full objects and doesn't
	// restrict them to metadata fields in any way, so a patch sent via a
	// metadata client may also change non-metadata fields.
	patchReaction := testing.ObjectReaction(dynTracker)
	metadataClient.PrependReactor("patch", "*", func(action testing.Action) (bool, runtime.Object, error) {
		handled, obj, err := patchReaction(action)
		if !handled || err != nil {
			return handled, obj, err
		}

		partial, err := toPartialObjectMetadata(obj)
		return true, partial, err
	})

	return metadataClient
}

type TransformingObjectTracker struct {
	Inner       testing.ObjectTracker
	ListKind    func(schema.GroupVersionKind) schema.GroupVersionKind // Optional.
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
func (t *TransformingObjectTracker) Delete(gvr schema.GroupVersionResource, ns, name string, opts ...metav1.DeleteOptions) error {
	return t.Inner.Delete(gvr, ns, name, opts...)
}

// Get implements testing.ObjectTracker.
func (t *TransformingObjectTracker) Get(gvr schema.GroupVersionResource, ns, name string, opts ...metav1.GetOptions) (runtime.Object, error) {
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
	if t.ListKind != nil {
		gvk = t.ListKind(gvk)
	}

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
	return t.internalized(applyConfiguration, func(obj runtime.Object) error {
		err := t.Inner.Apply(gvr, applyConfiguration, ns, opts...)

		// Translate an apply of a non-existent object into a create.
		if errors.IsNotFound(err) {
			return t.Inner.Create(gvr, obj, ns, metav1.CreateOptions{})
		}

		return err
	})
}

// Watch implements testing.ObjectTracker.
func (t *TransformingObjectTracker) Watch(gvr schema.GroupVersionResource, ns string, opts ...metav1.ListOptions) (watch.Interface, error) {
	w, err := t.Inner.Watch(gvr, ns, opts...)
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

func toPartialObjectMetadata(obj runtime.Object) (runtime.Object, error) {
	if meta.IsListType(obj) {
		var list metav1.List
		if listMeta, err := meta.ListAccessor(obj); err == nil {
			list.ResourceVersion = listMeta.GetResourceVersion()
			list.Continue = listMeta.GetContinue()
			list.RemainingItemCount = listMeta.GetRemainingItemCount()
		}

		if err := meta.EachListItem(obj, func(obj runtime.Object) error {
			partial, err := toPartialObjectMetadata(obj)
			if err != nil {
				return err
			}
			list.Items = append(list.Items, runtime.RawExtension{Object: partial})
			return nil
		}); err != nil {
			return obj, err
		}

		return &list, nil
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return obj, err
	}

	partial := meta.AsPartialObjectMetadata(accessor)
	partial.SetGroupVersionKind(metav1.SchemeGroupVersion.WithKind("PartialObjectMetadata"))
	return partial, nil
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
	listGVK.Kind += "List"

	list, err := scheme.New(listGVK)
	if err != nil {
		return obj, err
	}

	if err := meta.SetList(list, items); err != nil {
		return obj, err
	}

	return list, nil
}
