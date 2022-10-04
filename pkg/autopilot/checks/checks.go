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
	"fmt"

	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/static"
)

func CanUpdate(clientFactory kubernetes.ClientFactoryInterface, newVersion string) error {
	removedAPIs, err := GetRemovedAPIsList()
	if err != nil {
		return err
	}

	discoveryClient, err := clientFactory.GetDiscoveryClient()
	if err != nil {
		return err
	}

	_, resources, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		logrus.Error(err)
		if len(resources) == 0 {
			return err
		}
	}

	restClientGetter := kubernetes.NewRESTClientGetter(clientFactory)
	resourceBuilder := resource.NewBuilder(restClientGetter).
		Unstructured().
		ContinueOnError().
		Flatten().
		AllNamespaces(true).
		Latest()

	for _, r := range resources {
		gv, _ := schema.ParseGroupVersion(r.GroupVersion)
		for _, ar := range r.APIResources {
			gvk := schema.GroupVersionKind{
				Group:   gv.Group,
				Version: gv.Version,
				Kind:    ar.Kind,
			}

			removedInVersion, ok := removedAPIs[gvk]
			if !ok {
				continue
			}

			if semver.Compare(newVersion, removedInVersion) >= 0 {
				res := resourceBuilder.ResourceTypeOrNameArgs(true, ar.Kind).Do()
				infos, err := res.Infos()
				if err != nil {
					return err
				}

				found := 0
				for _, i := range infos {
					if gvk == i.Mapping.GroupVersionKind {
						found++
					}
				}
				if found > 0 {
					err = fmt.Errorf("%s is removed in Kubernetes %s. There are %d resources of the type in the cluster", gvk.String(), semver.MajorMinor(newVersion), found)
					logrus.Error(err)
					return err
				}
			}
		}
	}

	return nil
}

type APIResource struct {
	Group     string `yaml:"group" json:"group"`
	Version   string `yaml:"version" json:"version"`
	Kind      string `yaml:"kind" json:"kind"`
	RemovedIn string `yaml:"removed_in" json:"removed_in"`
}

func GetRemovedAPIsList() (map[schema.GroupVersionKind]string, error) {
	b, err := static.Asset("misc/api-resources.yaml")
	if err != nil {
		return nil, err
	}
	var resources []APIResource
	err = yaml.Unmarshal(b, &resources)
	if err != nil {
		return nil, err
	}

	list := make(map[schema.GroupVersionKind]string)
	for _, r := range resources {
		s := schema.GroupVersionKind{
			Group:   r.Group,
			Version: r.Version,
			Kind:    r.Kind,
		}
		list[s] = r.RemovedIn
	}

	return list, nil
}
