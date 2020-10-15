package applier

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/Mirantis/mke/pkg/util"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	// NameLabel stack label
	NameLabel = "mke.mirantis.com/stack"

	// ChecksumAnnotation defines the annotation key to used for stack checksums
	ChecksumAnnotation = "mke.mirantis.com/stack-checksum"

	// LastConfigAnnotation defines the annotation to be used for last applied configs
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

// Apply applies stack resources by creating or updating the resources. If prune is requested,
// the previously applied stack resources which are not part of the current stack are removed from k8s api
func (s *Stack) Apply(ctx context.Context, prune bool) error {
	log := logrus.WithField("stack", s.Name)

	log.Debugf("applying with %d resources", len(s.Resources))
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
			log.Debugf("server resource labels: %v", serverResource.GetLabels())
			log.Debugf("server checsum: %s", serverResource.GetAnnotations()[ChecksumAnnotation])
			localChecksum := resourceChecksum(resource)
			log.Debugf("local checsum: %s", localChecksum)

			if serverResource.GetAnnotations()[ChecksumAnnotation] == localChecksum {
				log.Debug("resource checksums match, no need to update")
				return nil
			}
			if serverResource.GetAnnotations()[LastConfigAnnotation] == "" {
				log.Debug("doing plain update as no last-config label present")
				resource.SetResourceVersion(serverResource.GetResourceVersion())
				_, err = drClient.Update(ctx, resource, metav1.UpdateOptions{})
			} else {
				log.Debug("patching resource")
				err = s.patchResource(ctx, drClient, serverResource, resource)
			}
			if err != nil {
				return fmt.Errorf("can't update resource:%v", err)
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

// ignoredResources defines a list of resources which as ignored in prune phase
// The reason for ignoring these are:
// - v1:Endpoints inherit the stack label but do not have owner ref set --> each apply would prune all stack related endpoints
// Defined is the form of api-group/version:kind. The core group kinds are defined as v1:<kind>
var ignoredResources = []string{"v1:Endpoints"}

func (s *Stack) findPruneableResources(ctx context.Context, mapper *restmapper.DeferredDiscoveryRESTMapper) ([]*unstructured.Unstructured, error) {
	pruneableResources := []*unstructured.Unstructured{}
	apiResourceLists, err := s.Discovery.ServerPreferredResources()
	if err != nil {
		// Client-Go emits an error when an API service is registered but unimplemented.
		// We trap that error here but since the discovery client continues
		// building the API object, it is correctly populated with all valid APIs.
		// See https://github.com/kubernetes/kubernetes/issues/72051#issuecomment-521157642
		// Common cause for this is metrics API which often gives 503s during discovery
		if discovery.IsGroupDiscoveryFailedError(err) {
			logrus.Debugf("error in api discovery for pruning: %s", err.Error())
		} else {
			return nil, errors.Wrapf(err, "failed to list api groups for pruning")
		}
	}

	groupVersionKinds := map[string]*schema.GroupVersionKind{}
	for _, apiResourceList := range apiResourceLists {
		for _, apiResource := range apiResourceList.APIResources {
			key := fmt.Sprintf("%s:%s", apiResourceList.GroupVersion, apiResource.Kind)
			if !util.StringSliceContains(apiResource.Verbs, "delete") {
				continue
			}
			if util.StringSliceContains(ignoredResources, key) {
				logrus.Debugf("skipping resource %s from prune", key)
				continue
			}
			if groupVersionKinds[key] == nil {
				groupVersionKinds[key] = &schema.GroupVersionKind{
					Group:   apiResource.Group,
					Kind:    apiResource.Kind,
					Version: apiResource.Version,
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
	logrus.Debugf("found %d prunable resources", len(pruneableResources))
	return pruneableResources, nil
}

func (s *Stack) deleteResource(ctx context.Context, mapper *restmapper.DeferredDiscoveryRESTMapper, resource *unstructured.Unstructured) error {
	propagationPolicy := metav1.DeletePropagationForeground
	drClient, err := s.clientForResource(mapper, resource)
	if err != nil {
		return errors.Wrapf(err, "failed to get dynamic client for resource %s", resource.GetSelfLink())
	}
	err = drClient.Delete(ctx, resource.GetName(), metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if !apiErrors.IsNotFound(err) && !apiErrors.IsGone(err) {
		return errors.Wrapf(err, "deleting resource failed")
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
				pruneableResources = append(pruneableResources, s.getPruneableResources(ctx, drClient)...)
			} else {
				// need to query each namespace separately
				for _, namespace := range namespaces {
					pruneableResources = append(pruneableResources, s.getPruneableResources(ctx, drClient.Namespace(namespace))...)
				}
			}
		} else {
			drClient := s.Client.Resource(mapping.Resource)
			pruneableResources = append(pruneableResources, s.getPruneableResources(ctx, drClient)...)
		}
	}

	return pruneableResources
}

func (s *Stack) getPruneableResources(ctx context.Context, drClient dynamic.ResourceInterface) []*unstructured.Unstructured {
	pruneableResources := []*unstructured.Unstructured{}
	listOpts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", NameLabel, s.Name),
	}
	resourceList, err := drClient.List(ctx, listOpts)
	if err != nil {
		return []*unstructured.Unstructured{}
	}
	for _, resource := range resourceList.Items {
		// We need to filter out objects that do not actually have the stack label set
		// There are some cases where we get "extra" results, e.g.: https://github.com/kubernetes-sigs/metrics-server/issues/604
		if !s.isInStack(resource) && len(resource.GetOwnerReferences()) == 0 && resource.GetLabels()[NameLabel] == s.Name {
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
	log := logrus.WithField("stack", s.Name)

	original := serverResource.GetAnnotations()[LastConfigAnnotation]
	if original == "" {
		return fmt.Errorf("%s does not have last-applied-configuration", localResource.GetSelfLink())
	}
	modified, _ := localResource.MarshalJSON()

	patch, err := jsonpatch.CreateMergePatch([]byte(original), modified)
	log.Debugf("******* patch: %s", string(patch))
	if err != nil {
		return errors.Wrapf(err, "failed to create jsonpatch data")
	}
	_, err = drClient.Patch(ctx, localResource.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to patch resource")
	}

	return nil
}

func (s *Stack) prepareResource(resource *unstructured.Unstructured) {
	labels := resource.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[NameLabel] = s.Name
	resource.SetLabels(labels)

	annotations := resource.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[ChecksumAnnotation] = resourceChecksum(resource)
	config, _ := resource.MarshalJSON()
	annotations[LastConfigAnnotation] = string(config)
	resource.SetAnnotations(annotations)
}

func generateResourceID(resource unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s:%s@%s", resource.GetObjectKind().GroupVersionKind().Group, resource.GetKind(), resource.GetName(), resource.GetNamespace())
}

func resourceChecksum(resource *unstructured.Unstructured) string {
	json, err := resource.MarshalJSON()
	if err != nil {
		return ""
	}
	hasher := md5.New()
	// based on the implementation hasher.Write never returns err
	_, _ = hasher.Write(json)

	return hex.EncodeToString(hasher.Sum(nil))
}
