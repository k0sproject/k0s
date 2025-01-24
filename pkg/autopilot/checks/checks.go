// Copyright 2022 k0s authors
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
	"context"
	"fmt"
	"strings"

	"github.com/k0sproject/k0s/pkg/kubernetes"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/metadata"

	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
)

func CanUpdate(ctx context.Context, log logrus.FieldLogger, clientFactory kubernetes.ClientFactoryInterface, newVersion string) error {
	discoveryClient, err := clientFactory.GetDiscoveryClient()
	if err != nil {
		return err
	}

	_, resources, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		log.WithError(err).Warn("Error while discovering supported API groups and resources")
		if len(resources) == 0 {
			return err
		}
	}

	var metaClient metadata.Interface
	for _, r := range resources {
		gv, err := schema.ParseGroupVersion(r.GroupVersion)
		if err != nil {
			log.WithError(err).Warn("Skipping API version ", r.GroupVersion)
			continue
		}

		for _, ar := range r.APIResources {
			// Skip resources which don't have the same name and kind. This is to skip
			// subresources such as FlowSchema/Status
			if strings.Contains(ar.Name, "/") {
				continue
			}

			gv := gv // Copy over the default GroupVersion from the list
			// Apply resource-specific overrides
			if ar.Group != "" {
				gv.Group = ar.Group
			}
			if ar.Version != "" {
				gv.Version = ar.Version
			}

			removedInVersion, currentVersion := removedInVersion(gv.WithKind(ar.Kind))
			if removedInVersion == "" || semver.Compare(newVersion, removedInVersion) < 0 {
				continue
			}

			if metaClient == nil {
				restConfig, err := clientFactory.GetRESTConfig()
				if err != nil {
					return err
				}

				if metaClient, err = metadata.NewForConfig(restConfig); err != nil {
					return err
				}
			}

			metas, err := metaClient.Resource(gv.WithResource(ar.Name)).
				Namespace(metav1.NamespaceAll).
				List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}

			if found := len(metas.Items); found > 0 {
				if currentVersion == "" {
					return fmt.Errorf("%s.%s %s has been removed in Kubernetes %s, but there are %d such resources in the cluster", ar.Name, gv.Group, gv.Version, removedInVersion, found)
				}
				// If we find removed APIs, it could be because the APIserver is serving the same object with an older GVK
				// for compatibility reasons while the current good API still works.
				newGV := gv
				newGV.Version = currentVersion
				outdatedItems := []metav1.PartialObjectMetadata{}
				for _, item := range metas.Items {
					// Currently none of the deleted resources are namespaced, so we can skip the namespace check.
					// However we keep it in the list so that it breaks if we add a namespaced resource.
					_, err := metaClient.Resource(newGV.WithResource(ar.Name)).
						Get(ctx, item.GetName(), metav1.GetOptions{})
					if apierrors.IsNotFound(err) {
						outdatedItems = append(outdatedItems, item)
					} else if err != nil {
						return err
					}
				}
				if foundOutdated := len(outdatedItems); foundOutdated > 0 {
					return fmt.Errorf("%s.%s %s has been removed in Kubernetes %s, but there are %d such resources in the cluster", ar.Name, gv.Group, gv.Version, removedInVersion, found)
				}
			}
		}
	}

	return nil
}
