package applier

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sync"

	jsonpatch "github.com/evanphx/json-patch"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

const (
	// Label stack label
	Label = "mke.mirantis.com/stack"

	// ChecksumAnnotation ...
	ChecksumAnnotation = "mke.mirantis.com/stack-checksum"

	// LastConfigAnnotation ...
	LastConfigAnnotation = "mke.mirantis.com/last-applied-configuration"
)

// Stack is a k8s resource bundle
type Stack struct {
	Name          string
	Resources     []*unstructured.Unstructured
	keepResources []string
	Client        dynamic.Interface
	Discovery     discovery.CachedDiscoveryInterface
}

// Apply applies stack resources
func (s *Stack) Apply(ctx context.Context, prune bool) error {
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(s.Discovery)
	sortedResources := []*unstructured.Unstructured{}
	for _, resource := range s.Resources {
		if resource.GetNamespace() == "" {
			sortedResources = append(sortedResources, resource)
		}
	}
	for _, resource := range s.Resources {
		if resource.GetNamespace() != "" {
			sortedResources = append(sortedResources, resource)
		}
	}
	for _, resource := range sortedResources {
		s.prepareResource(resource)
		mapping, err := mapper.RESTMapping(resource.GroupVersionKind().GroupKind(), resource.GroupVersionKind().Version)
		if err != nil {
			return fmt.Errorf("mapping error: %s", err)
		}
		var drClient dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			drClient = s.Client.Resource(mapping.Resource).Namespace(resource.GetNamespace())
		} else {
			drClient = s.Client.Resource(mapping.Resource)
		}
		serverResource, err := drClient.Get(ctx, resource.GetName(), metav1.GetOptions{})
		if apiErrors.IsNotFound(err) {
			_, err := drClient.Create(ctx, resource, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("cannot create resource %s: %s", resource.GetName(), err)
			}
		} else if err != nil {
			return fmt.Errorf("unknown api error: %s", err)
		} else {
			if serverResource.GetLabels()[LastConfigAnnotation] == "" {
				_, err = drClient.Update(ctx, resource, metav1.UpdateOptions{})
			} else {
				err = s.patchResource(ctx, drClient, serverResource, resource)
			}
		}
		s.keepResource(resource)
	}

	var err error
	if prune {
		err = s.prune(ctx, mapper)
	}

	return err
}

func (s *Stack) keepResource(resource *unstructured.Unstructured) {
	s.keepResources = append(s.keepResources, generateResourceID(*resource))
}

func (s *Stack) getAllAccessibleNamespaces(ctx context.Context) []string {
	namespaces := []string{}
	nsGroupVersionResource := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	nsList, err := s.Client.Resource(nsGroupVersionResource).List(ctx, metav1.ListOptions{})
	if apiErrors.IsForbidden(err) {
		for _, resource := range s.Resources {
			if ns := resource.GetNamespace(); ns != "" {
				namespaces = append(namespaces, ns)
			}
		}
	}
	if err != nil {
		return namespaces
	}

	for _, namespace := range nsList.Items {
		namespaces = append(namespaces, namespace.GetName())
	}

	return namespaces
}

func (s *Stack) prune(ctx context.Context, mapper *restmapper.DeferredDiscoveryRESTMapper) error {
	pruneableResources, err := s.findPruneableResources(ctx, mapper)
	if err != nil {
		return err
	}

	for _, resource := range pruneableResources {
		if resource.GetNamespace() != "" {
			err = s.deleteResource(ctx, mapper, resource)
			if err != nil {
				return err
			}
		}
	}
	for _, resource := range pruneableResources {
		if resource.GetNamespace() == "" {
			err = s.deleteResource(ctx, mapper, resource)
			if err != nil {
				return err
			}
		}
	}

	s.keepResources = []string{}

	return nil
}

func (s *Stack) findPruneableResources(ctx context.Context, mapper *restmapper.DeferredDiscoveryRESTMapper) ([]*unstructured.Unstructured, error) {
	pruneableResources := []*unstructured.Unstructured{}
	apiGroups, apiResourceLists, err := s.Discovery.ServerGroupsAndResources()
	if err != nil {
		return nil, err
	}
	groupVersionKinds := map[string]*schema.GroupVersionKind{}
	for _, apiResourceList := range apiResourceLists {
		for _, apiResource := range apiResourceList.APIResources {
			key := fmt.Sprintf("%s:%s", apiResourceList.GroupVersion, apiResource.Kind)
			if groupVersionKinds[key] == nil {
				apiGroup := findAPIGroupForAPIService(apiGroups, apiResourceList)
				if apiGroup != nil {
					groupVersionKinds[key] = &schema.GroupVersionKind{
						Group:   apiGroup.Name,
						Kind:    apiResource.Kind,
						Version: apiGroup.PreferredVersion.Version,
					}
				}
			}
		}
	}
	wg := sync.WaitGroup{}
	namespaces := s.getAllAccessibleNamespaces(ctx)
	for _, groupVersionKind := range groupVersionKinds {
		wg.Add(1)
		go func(groupVersionKind *schema.GroupVersionKind) {
			defer wg.Done()
			pruneableForGvk := s.findPruneableResourceForGroupVersionKind(ctx, mapper, groupVersionKind, namespaces)
			pruneableResources = append(pruneableResources, pruneableForGvk...)
		}(groupVersionKind)
	}
	wg.Wait()

	return pruneableResources, nil
}

func (s *Stack) deleteResource(ctx context.Context, mapper *restmapper.DeferredDiscoveryRESTMapper, resource *unstructured.Unstructured) error {
	propagationPolicy := metav1.DeletePropagationForeground
	drClient, err := s.clientForResource(mapper, resource)
	if err != nil {
		return err
	}
	err = drClient.Delete(ctx, resource.GetName(), metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if !apiErrors.IsNotFound(err) && !apiErrors.IsGone(err) {
		return err
	}
	return nil
}

func (s *Stack) clientForResource(mapper *restmapper.DeferredDiscoveryRESTMapper, resource *unstructured.Unstructured) (dynamic.ResourceInterface, error) {
	mapping, err := mapper.RESTMapping(resource.GroupVersionKind().GroupKind(), resource.GroupVersionKind().Version)
	if err != nil {
		return nil, fmt.Errorf("mapping error: %s", err)
	}
	var drClient dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		drClient = s.Client.Resource(mapping.Resource).Namespace(resource.GetNamespace())
	} else {
		drClient = s.Client.Resource(mapping.Resource)
	}

	return drClient, nil
}

func (s *Stack) findPruneableResourceForGroupVersionKind(ctx context.Context, mapper *restmapper.DeferredDiscoveryRESTMapper, groupVersionKind *schema.GroupVersionKind, namespaces []string) []*unstructured.Unstructured {
	pruneableResources := []*unstructured.Unstructured{}
	groupKind := schema.GroupKind{
		Group: groupVersionKind.Group,
		Kind:  groupVersionKind.Kind,
	}
	mapping, _ := mapper.RESTMapping(groupKind, groupVersionKind.Version)
	if mapping != nil {
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			drClient := s.Client.Resource(mapping.Resource)
			_, err := drClient.List(ctx, metav1.ListOptions{Limit: 1})
			if err == nil {
				// rights to fetch all namespaces at once
				for _, res := range s.getPruneableResources(ctx, drClient) {
					pruneableResources = append(pruneableResources, res)
				}
			} else {
				// need to query each namespace separately
				for _, namespace := range namespaces {
					for _, res := range s.getPruneableResources(ctx, drClient.Namespace(namespace)) {
						pruneableResources = append(pruneableResources, res)
					}
				}
			}
		} else {
			drClient := s.Client.Resource(mapping.Resource)
			for _, res := range s.getPruneableResources(ctx, drClient) {
				pruneableResources = append(pruneableResources, res)
			}
		}
	}

	return pruneableResources
}

func findAPIGroupForAPIService(apiGroups []*metav1.APIGroup, apiResource *metav1.APIResourceList) *metav1.APIGroup {
	for _, apiGroup := range apiGroups {
		gv := fmt.Sprintf("%s/%s", apiGroup.Name, apiGroup.PreferredVersion.Version)
		if gv == apiResource.GroupVersion {
			return apiGroup
		}
	}

	return nil
}

func (s *Stack) getPruneableResources(ctx context.Context, drClient dynamic.ResourceInterface) []*unstructured.Unstructured {
	pruneableResources := []*unstructured.Unstructured{}
	resourceList, err := drClient.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", Label, s.Name),
	})
	if err != nil {
		return []*unstructured.Unstructured{}
	}
	for _, resource := range resourceList.Items {
		if !s.isInStack(resource) && len(resource.GetOwnerReferences()) == 0 {
			pruneableResources = append(pruneableResources, &resource)
		}
	}

	return pruneableResources
}

func (s *Stack) isInStack(resource unstructured.Unstructured) bool {
	for _, id := range s.keepResources {
		if id == generateResourceID(resource) {
			return true
		}
	}
	return false
}

func (s *Stack) patchResource(ctx context.Context, drClient dynamic.ResourceInterface, serverResource *unstructured.Unstructured, localResource *unstructured.Unstructured) error {
	original := serverResource.GetLabels()[LastConfigAnnotation]
	if original == "" {
		return fmt.Errorf("%s does not have last-applied-configuration", localResource.GetSelfLink())
	}
	modified, _ := localResource.MarshalJSON()
	patch, err := jsonpatch.CreateMergePatch([]byte(original), modified)
	if err != nil {
		return err
	}
	_, err = drClient.Patch(ctx, localResource.GetName(), types.JSONPatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (s *Stack) prepareResource(resource *unstructured.Unstructured) {
	labels := resource.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[Label] = s.Name
	labels[ChecksumAnnotation] = resourceChecksum(resource)
	resource.SetLabels(labels)
}

func generateResourceID(resource unstructured.Unstructured) string {
	return fmt.Sprintf("%s:%s@%s", resource.GetKind(), resource.GetName(), resource.GetNamespace())
}

func resourceChecksum(resource *unstructured.Unstructured) string {
	json, err := resource.MarshalJSON()
	if err != nil {
		return ""
	}
	hasher := md5.New()
	hasher.Write(json)
	return hex.EncodeToString(hasher.Sum(nil))
}
