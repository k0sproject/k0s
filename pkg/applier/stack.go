/*
Copyright 2020 k0s authors

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

package applier

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"k8s.io/utils/ptr"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/sirupsen/logrus"
)

// Stack is a k8s resource bundle
type Stack struct {
	Name          string
	Resources     []*unstructured.Unstructured
	keepResources []string
	Clients       kubernetes.ClientFactoryInterface

	log *logrus.Entry
}

// Apply applies stack resources by creating or updating the resources. If prune is requested,
// the previously applied stack resources which are not part of the current stack are removed from k8s api
func (s *Stack) Apply(ctx context.Context, prune bool) error {
	s.log = logrus.WithField("stack", s.Name)

	discoveryClient, err := s.Clients.GetDiscoveryClient()
	if err != nil {
		return err
	}
	dynamicClient, err := s.Clients.GetDynamicClient()
	if err != nil {
		return err
	}

	s.log.Debugf("applying with %d resources", len(s.Resources))
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	var sortedResources []*unstructured.Unstructured
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

	var errs []error
	for _, resource := range sortedResources {
		s.prepareResource(resource)
		mapping, err := getRESTMapping(mapper, ptr.To(resource.GroupVersionKind()))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		var drClient dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			drClient = dynamicClient.Resource(mapping.Resource).Namespace(resource.GetNamespace())
		} else {
			drClient = dynamicClient.Resource(mapping.Resource)
		}
		serverResource, err := drClient.Get(ctx, resource.GetName(), metav1.GetOptions{})
		if apiErrors.IsNotFound(err) {
			created, err := drClient.Create(ctx, resource, metav1.CreateOptions{})
			if err != nil {
				err = fmt.Errorf("cannot create resource %s: %w", resource.GetName(), err)
				errs = append(errs, err)
				continue
			}
			if isCRD(created) {
				s.waitForCRD(ctx, created.GetName())
				mapper.Reset() // so that the created CRD gets rediscovered
			}
		} else if err != nil {
			err = fmt.Errorf("unknown api error: %w", err)
			errs = append(errs, err)
			continue
		} else { // The resource already exists, we need to update/patch it
			localChecksum := resource.GetAnnotations()[ChecksumAnnotation]
			if serverResource.GetAnnotations()[ChecksumAnnotation] == localChecksum {
				s.log.Debug("resource checksums match, no need to update")
				s.keepResource(resource)
				continue
			}
			if serverResource.GetAnnotations()[LastConfigAnnotation] == "" {
				s.log.Debug("doing plain update as no last-config label present")
				resource.SetResourceVersion(serverResource.GetResourceVersion())
				resource, err = drClient.Update(ctx, resource, metav1.UpdateOptions{})
			} else {
				s.log.Debug("patching resource")
				resource, err = s.patchResource(ctx, drClient, serverResource, resource)
			}
			if err != nil {
				err = fmt.Errorf("can't update resource: %w", err)
				errs = append(errs, err)
				continue
			}
			if isCRD(resource) {
				s.waitForCRD(ctx, resource.GetName())
				mapper.Reset() // so that the changed CRD gets rediscovered
			}
		}
		s.keepResource(resource)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	if !prune {
		return nil
	}

	return s.prune(ctx, mapper)
}

// waitForCRD waits 5 seconds for a CRD to become established on a best-effort basis.
func (s *Stack) waitForCRD(ctx context.Context, crdName string) {
	client, err := s.Clients.GetAPIExtensionsClient()
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_ = watch.CRDs(client.ApiextensionsV1().CustomResourceDefinitions()).
		WithObjectName(crdName).
		WithErrorCallback(watch.IsRetryable).
		Until(ctx, func(item *apiextensionsv1.CustomResourceDefinition) (bool, error) {
			for _, cond := range item.Status.Conditions {
				if cond.Type == apiextensionsv1.Established {
					return cond.Status == apiextensionsv1.ConditionTrue, nil
				}
			}
			return false, nil
		})
}

func (s *Stack) keepResource(resource *unstructured.Unstructured) {
	resourceID := generateResourceID(*resource)
	logrus.WithField("stack", s.Name).Debugf("marking resource to be kept: %s", resourceID)
	s.keepResources = append(s.keepResources, resourceID)
}

func (s *Stack) prune(ctx context.Context, mapper meta.ResettableRESTMapper) error {
	pruneableResources, err := s.findPruneableResources(ctx, mapper)
	if err != nil {
		return err
	}
	if len(pruneableResources) == 0 {
		return nil
	}

	s.log.Debug("starting to delete resources, namespaced resources first")
	for _, resource := range pruneableResources {
		resourceID := generateResourceID(resource)
		if resource.GetNamespace() != "" {
			s.log.Debugf("deleting resource %s", resourceID)
			err = s.deleteResource(ctx, mapper, resource)
			if err != nil {
				return err
			}
		}
	}
	for _, resource := range pruneableResources {
		resourceID := generateResourceID(resource)
		if resource.GetNamespace() == "" {
			s.log.Debugf("deleting resource %s", resourceID)
			err = s.deleteResource(ctx, mapper, resource)
			if err != nil {
				return err
			}
		}
	}
	s.log.Debug("resources pruned succesfully")
	s.keepResources = []string{}

	return nil
}

// ignoredResources defines a list of resources which as ignored in prune phase
// The reason for ignoring these are:
// - v1:Endpoints inherit the stack label but do not have owner ref set --> each apply would prune all stack related endpoints
// - discovery.k8s.io/v1:EndpointSlice inherit the stack label but do not have owner ref set --> each apply would prune all stack related endpointsslices
// Defined is the form of api-group/version:kind. The core group kinds are defined as v1:<kind>
var ignoredResources = []string{
	"v1:Endpoints",
	"discovery.k8s.io/v1:EndpointSlice",
}

func (s *Stack) findPruneableResources(ctx context.Context, mapper meta.ResettableRESTMapper) ([]unstructured.Unstructured, error) {
	var pruneableResources []unstructured.Unstructured

	client, err := s.Clients.GetDiscoveryClient()
	if err != nil {
		return nil, err
	}

	apiResourceLists, err := client.ServerPreferredResources()
	if err != nil {
		// Client-Go emits an error when an API service is registered but unimplemented.
		// We trap that error here but since the discovery client continues
		// building the API object, it is correctly populated with all valid APIs.
		// See https://github.com/kubernetes/kubernetes/issues/72051#issuecomment-521157642
		// Common cause for this is metrics API which often gives 503s during discovery
		if discovery.IsGroupDiscoveryFailedError(err) {
			s.log.Debugf("error in api discovery for pruning: %s", err.Error())
		} else {
			return nil, fmt.Errorf("failed to list api groups for pruning: %w", err)
		}
	}

	groupVersionKinds := map[string]*schema.GroupVersionKind{}
	for _, apiResourceList := range apiResourceLists {
		for _, apiResource := range apiResourceList.APIResources {
			key := fmt.Sprintf("%s:%s", apiResourceList.GroupVersion, apiResource.Kind)
			if !slices.Contains(apiResource.Verbs, "delete") {
				continue
			}
			if slices.Contains(ignoredResources, key) {
				s.log.Debugf("skipping resource %s from prune", key)
				continue
			}
			if groupVersionKinds[key] == nil {
				// We need to parse the GV from apiResourceList, for some reason the group and version infos are empty on the apiResource level
				gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
				if err != nil {
					return nil, fmt.Errorf("api discovery returned unparseable group-version: %s", apiResourceList.GroupVersion)
				}
				groupVersionKinds[key] = &schema.GroupVersionKind{
					Group:   gv.Group,
					Kind:    apiResource.Kind,
					Version: gv.Version,
				}
			}
		}
	}

	s.log.Debug("starting to find prunable resources")
	start := time.Now()
	wg := sync.WaitGroup{}
	mu := sync.Mutex{} // The shield against concurrent appends for pruneable resources

	// Let's parallelize each group-version-kind finding
	for _, groupVersionKind := range groupVersionKinds {
		wg.Add(1)
		go func(groupVersionKind *schema.GroupVersionKind) {
			defer wg.Done()
			pruneableForGvk := s.findPruneableResourceForGroupVersionKind(ctx, mapper, groupVersionKind)
			if len(pruneableForGvk) > 0 {
				mu.Lock()
				pruneableResources = append(pruneableResources, pruneableForGvk...)
				mu.Unlock()
			}
			s.log.Debugf("found %d prunable resources for kind %s", len(pruneableForGvk), groupVersionKind)
		}(groupVersionKind)
	}
	wg.Wait()
	s.log.Debugf("found %d prunable resources", len(pruneableResources))
	s.log.Debugf("finding prunable resources took %s", time.Since(start).String())
	return pruneableResources, nil
}

func (s *Stack) deleteResource(ctx context.Context, mapper meta.ResettableRESTMapper, resource unstructured.Unstructured) error {
	propagationPolicy := metav1.DeletePropagationForeground
	drClient, err := s.clientForResource(mapper, resource)
	if err != nil {
		return fmt.Errorf("failed to get dynamic client for resource %s: %w", resource.GetSelfLink(), err)
	}
	err = drClient.Delete(ctx, resource.GetName(), metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil && !apiErrors.IsNotFound(err) && !apiErrors.IsGone(err) {
		return fmt.Errorf("deleting resource failed: %w", err)
	}
	return nil
}

func getRESTMapping(mapper meta.ResettableRESTMapper, gvk *schema.GroupVersionKind) (*meta.RESTMapping, error) {
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)

	// If the error indicates that the resource's kind is unknown, it may be
	// that the corresponding CRD has already been applied, but the RESTMapper
	// is still operating on stale cached data. Force a reset of the mapper and
	// retry the call once.
	var noMatchErr *meta.NoKindMatchError
	if errors.As(err, &noMatchErr) {
		mapper.Reset()
		mapping, err = mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	}

	if err != nil {
		return nil, fmt.Errorf("mapping error: %w", err)
	}

	return mapping, nil
}

func (s *Stack) clientForResource(mapper meta.ResettableRESTMapper, resource unstructured.Unstructured) (dynamic.ResourceInterface, error) {
	mapping, err := getRESTMapping(mapper, ptr.To(resource.GroupVersionKind()))
	if err != nil {
		return nil, fmt.Errorf("mapping error: %w", err)
	}

	client, err := s.Clients.GetDynamicClient()
	if err != nil {
		return nil, err
	}

	var drClient dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		drClient = client.Resource(mapping.Resource).Namespace(resource.GetNamespace())
	} else {
		drClient = client.Resource(mapping.Resource)
	}

	return drClient, nil
}

func (s *Stack) findPruneableResourceForGroupVersionKind(ctx context.Context, mapper meta.ResettableRESTMapper, groupVersionKind *schema.GroupVersionKind) []unstructured.Unstructured {
	mapping, _ := getRESTMapping(mapper, groupVersionKind)
	// FIXME error handling...
	if mapping != nil {
		client, err := s.Clients.GetDynamicClient()
		if err != nil {
			return nil
		}
		// We're running this with full admin rights, we should have capability to get stuff with single call
		drClient := client.Resource(mapping.Resource)
		return s.getPruneableResources(ctx, drClient)
	}

	return nil
}

func (s *Stack) getPruneableResources(ctx context.Context, drClient dynamic.ResourceInterface) []unstructured.Unstructured {
	var pruneableResources []unstructured.Unstructured
	listOpts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", NameLabel, s.Name),
	}
	resourceList, err := drClient.List(ctx, listOpts)
	if err != nil {
		// FIXME why no error propagation !??!
		return nil
	}
	for _, resource := range resourceList.Items {
		// We need to filter out objects that do not actually have the stack label set
		// There are some cases where we get "extra" results, e.g.: https://github.com/kubernetes-sigs/metrics-server/issues/604
		if !s.isInStack(resource) && len(resource.GetOwnerReferences()) == 0 && resource.GetLabels()[NameLabel] == s.Name {
			s.log.Debugf("adding prunable resource: %s", generateResourceID(resource))
			pruneableResources = append(pruneableResources, resource)
		}
	}

	return pruneableResources
}

func (s *Stack) isInStack(resource unstructured.Unstructured) bool {
	resourceID := generateResourceID(resource)
	for _, id := range s.keepResources {
		if id == resourceID {
			return true
		}
	}
	return false
}

func (s *Stack) patchResource(ctx context.Context, drClient dynamic.ResourceInterface, serverResource *unstructured.Unstructured, localResource *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	original := serverResource.GetAnnotations()[LastConfigAnnotation]
	if original == "" {
		return nil, fmt.Errorf("%s does not have last-applied-configuration", localResource.GetSelfLink())
	}
	modified, _ := localResource.MarshalJSON()

	patch, err := jsonpatch.CreateMergePatch([]byte(original), modified)
	if err != nil {
		return nil, fmt.Errorf("failed to create jsonpatch data: %w", err)
	}
	resource, err := drClient.Patch(ctx, localResource.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to patch resource: %w", err)
	}

	return resource, nil
}

func (s *Stack) prepareResource(resource *unstructured.Unstructured) {
	checksum := resourceChecksum(resource)
	lastAppliedConfig, _ := resource.MarshalJSON()

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
	annotations[ChecksumAnnotation] = checksum
	annotations[LastConfigAnnotation] = string(lastAppliedConfig)
	resource.SetAnnotations(annotations)
}

func isCRD(resource *unstructured.Unstructured) bool {
	gvk := resource.GroupVersionKind()
	return gvk.Group == apiextensionsv1.GroupName && gvk.Kind == "CustomResourceDefinition"
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
