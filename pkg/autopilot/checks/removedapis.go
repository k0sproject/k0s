// Copyright 2024 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checks

import (
	"cmp"
	"sort"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type removedAPI struct {
	group, version, kind, removedInVersion string
}

// Returns the Kubernetes version in which candidate has been removed, if any.
func removedInVersion(candidate schema.GroupVersionKind) string {
	if idx, found := sort.Find(len(removedGVKs), func(i int) int {
		if cmp := cmp.Compare(candidate.Group, removedGVKs[i].group); cmp != 0 {
			return cmp
		}
		if cmp := cmp.Compare(candidate.Version, removedGVKs[i].version); cmp != 0 {
			return cmp
		}
		return cmp.Compare(candidate.Kind, removedGVKs[i].kind)
	}); found {
		return removedGVKs[idx].removedInVersion
	}

	return ""
}

// Sorted array of removed APIs.
var removedGVKs = [59]removedAPI{
	{"admissionregistration.k8s.io", "v1beta1", "MutatingWebhookConfiguration", "v1.22.0"},
	{"admissionregistration.k8s.io", "v1beta1", "ValidatingWebhookConfiguration", "v1.22.0"},
	{"apiextensions.k8s.io", "v1beta1", "CustomResourceDefinition", "v1.22.0"},
	{"apiregistration.k8s.io", "v1beta1", "APIService", "v1.22.0"},
	{"apps", "v1beta1", "Deployment", "v1.16.0"},
	{"apps", "v1beta1", "ReplicaSet", "v1.16.0"},
	{"apps", "v1beta1", "StatefulSet", "v1.16.0"},
	{"apps", "v1beta2", "DaemonSet", "v1.16.0"},
	{"apps", "v1beta2", "Deployment", "v1.16.0"},
	{"apps", "v1beta2", "ReplicaSet", "v1.16.0"},
	{"apps", "v1beta2", "StatefulSet", "v1.16.0"},
	{"authentication.k8s.io", "v1beta1", "TokenReview", "v1.22.0"},
	{"autoscaling", "v2beta1", "HorizontalPodAutoscaler", "v1.25.0"},
	{"autoscaling", "v2beta1", "HorizontalPodAutoscalerList", "v1.25.0"},
	{"autoscaling", "v2beta2", "HorizontalPodAutoscaler", "v1.26.0"},
	{"autoscaling", "v2beta2", "HorizontalPodAutoscalerList", "v1.26.0"},
	{"batch", "v1beta1", "CronJob", "v1.25.0"},
	{"batch", "v1beta1", "CronJobList", "v1.25.0"},
	{"certificates.k8s.io", "v1beta1", "CertificateSigningRequest", "v1.22.0"},
	{"coordination.k8s.io", "v1beta1", "Lease", "v1.22.0"},
	{"discovery.k8s.io", "v1beta1", "EndpointSlice", "v1.25.0"},
	{"events.k8s.io", "v1beta1", "Event", "v1.25.0"},
	{"extensions", "v1beta1", "DaemonSet", "v1.16.0"},
	{"extensions", "v1beta1", "Deployment", "v1.16.0"},
	{"extensions", "v1beta1", "Ingress", "v1.22.0"},
	{"extensions", "v1beta1", "NetworkPolicy", "v1.16.0"},
	{"extensions", "v1beta1", "PodSecurityPolicy", "v1.16.0"},
	{"extensions", "v1beta1", "ReplicaSet", "v1.16.0"},
	{"flowcontrol.apiserver.k8s.io", "v1beta1", "FlowControl", "v1.26.0"},
	{"k0s.k0sproject.example.com", "v1beta1", "RemovedCRD", "v99.99.99"}, // This is a test entry
	{"networking.k8s.io", "v1beta1", "Ingress", "v1.22.0"},
	{"networking.k8s.io", "v1beta1", "IngressClass", "v1.22.0"},
	{"policy", "v1beta1", "PodDisruptionBudget", "v1.25.0"},
	{"policy", "v1beta1", "PodDisruptionBudgetList", "v1.25.0"},
	{"policy", "v1beta1", "PodSecurityPolicy", "v1.25.0"},
	{"rbac.authorization.k8s.io", "v1alpha1", "ClusterRole", "v1.22.0"},
	{"rbac.authorization.k8s.io", "v1alpha1", "ClusterRoleBinding", "v1.22.0"},
	{"rbac.authorization.k8s.io", "v1alpha1", "ClusterRoleBindingList", "v1.22.0"},
	{"rbac.authorization.k8s.io", "v1alpha1", "ClusterRoleList", "v1.22.0"},
	{"rbac.authorization.k8s.io", "v1alpha1", "Role", "v1.22.0"},
	{"rbac.authorization.k8s.io", "v1alpha1", "RoleBinding", "v1.22.0"},
	{"rbac.authorization.k8s.io", "v1alpha1", "RoleBindingList", "v1.22.0"},
	{"rbac.authorization.k8s.io", "v1alpha1", "RoleList", "v1.22.0"},
	{"rbac.authorization.k8s.io", "v1beta1", "ClusterRole", "v1.22.0"},
	{"rbac.authorization.k8s.io", "v1beta1", "ClusterRoleBinding", "v1.22.0"},
	{"rbac.authorization.k8s.io", "v1beta1", "ClusterRoleBindingList", "v1.22.0"},
	{"rbac.authorization.k8s.io", "v1beta1", "ClusterRoleList", "v1.22.0"},
	{"rbac.authorization.k8s.io", "v1beta1", "Role", "v1.22.0"},
	{"rbac.authorization.k8s.io", "v1beta1", "RoleBinding", "v1.22.0"},
	{"rbac.authorization.k8s.io", "v1beta1", "RoleBindingList", "v1.22.0"},
	{"rbac.authorization.k8s.io", "v1beta1", "RoleList", "v1.22.0"},
	{"scheduling.k8s.io", "v1alpha1", "PriorityClass", "v1.17.0"},
	{"scheduling.k8s.io", "v1beta1", "PriorityClass", "v1.22.0"},
	{"storage.k8s.io", "v1beta1", "CSIDriver", "v1.22.0"},
	{"storage.k8s.io", "v1beta1", "CSINode", "v1.22.0"},
	{"storage.k8s.io", "v1beta1", "CSIStorageCapacity", "v1.27.0"},
	{"storage.k8s.io", "v1beta1", "CSIStorageCapacity", "v1.27.0"},
	{"storage.k8s.io", "v1beta1", "StorageClass", "v1.22.0"},
	{"storage.k8s.io", "v1beta1", "VolumeAttachment", "v1.22.0"},
}
