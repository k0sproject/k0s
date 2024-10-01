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

	"github.com/k0sproject/k0s/pkg/kubernetes"

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
			gv := gv // Copy over the default GroupVersion from the list
			// Apply resource-specific overrides
			if ar.Group != "" {
				gv.Group = ar.Group
			}
			if ar.Version != "" {
				gv.Version = ar.Version
			}

			removedInVersion := removedInVersion(gv.WithKind(ar.Kind))
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
				return fmt.Errorf("%s.%s %s has been removed in Kubernetes %s, but there are %d such resources in the cluster", ar.Name, gv.Group, gv.Version, removedInVersion, found)
			}
		}
	}

	return nil
}
