/*
Copyright 2021 k0s authors

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
	"bytes"
	"context"
	"os"
	"path"
	"path/filepath"

	"k8s.io/client-go/util/retry"

	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

// Applier manages all the "static" manifests and applies them on the k8s API
type Applier struct {
	Name string
	Dir  string

	log             *logrus.Entry
	clientFactory   kubernetes.ClientFactoryInterface
	client          dynamic.Interface
	discoveryClient discovery.CachedDiscoveryInterface
}

// NewApplier creates new Applier
func NewApplier(dir string, kubeClientFactory kubernetes.ClientFactoryInterface) Applier {
	name := filepath.Base(dir)
	log := logrus.WithFields(logrus.Fields{
		"component": "applier",
		"bundle":    name,
	})

	return Applier{
		log:           log,
		Dir:           dir,
		Name:          name,
		clientFactory: kubeClientFactory,
	}
}

func (a *Applier) init() error {
	c, err := a.clientFactory.GetDynamicClient()
	if err != nil {
		return err
	}
	discoveryClient, err := a.clientFactory.GetDiscoveryClient()
	if err != nil {
		return err
	}

	a.client = c
	a.discoveryClient = discoveryClient

	return nil
}

// just a wrapper for the retry as we need to init "lazily" from both apply and delete directions
func (a *Applier) lazyInit() error {
	if a.client == nil || a.discoveryClient == nil {
		err := retry.OnError(retry.DefaultBackoff, func(err error) bool {
			return true
		}, a.init)
		if err != nil {
			return err
		}
	}

	return nil
}

// Apply resources
func (a *Applier) Apply() error {
	err := a.lazyInit()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(path.Join(a.Dir, "*.yaml"))
	if err != nil {
		return err
	}
	resources, err := a.parseFiles(files)
	if err != nil {
		return err
	}
	stack := Stack{
		Name:      a.Name,
		Resources: resources,
		Client:    a.client,
		Discovery: a.discoveryClient,
	}
	a.log.Debug("applying stack")
	err = stack.Apply(context.Background(), true)
	if err != nil {
		a.log.WithError(err).Warn("stack apply failed")
		a.discoveryClient.Invalidate()
	} else {
		a.log.Debug("successfully applied stack")
	}

	return err
}

// Delete deletes the entire stack by applying it with empty set of resources
func (a *Applier) Delete() error {
	err := a.lazyInit()
	if err != nil {
		return err
	}
	stack := Stack{
		Name:      a.Name,
		Resources: []*unstructured.Unstructured{},
		Client:    a.client,
		Discovery: a.discoveryClient,
	}
	logrus.Debugf("about to delete a stack %s with empty apply", a.Name)
	err = stack.Apply(context.Background(), true)
	return err
}

func (a *Applier) parseFiles(files []string) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured
	for _, file := range files {
		// TODO Probably better to pass in the file stream into decoder and not to read it fully to mem first
		source, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}

		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(source), 4096)
		var resource map[string]interface{}
		for decoder.Decode(&resource) == nil {
			item := &unstructured.Unstructured{
				Object: resource,
			}
			if item.GetAPIVersion() != "" && item.GetKind() != "" {
				resources = append(resources, item)
				resource = nil
			}
		}
	}

	return resources, nil
}
