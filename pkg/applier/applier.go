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
	"fmt"
	"path/filepath"

	"github.com/k0sproject/k0s/pkg/kubernetes"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"

	"github.com/sirupsen/logrus"
)

// manifestFilePattern is the glob pattern that all applicable manifest files need to match.
const manifestFilePattern = "*.yaml"

func FindManifestFilesInDir(dir string) ([]string, error) {
	return filepath.Glob(filepath.Join(dir, manifestFilePattern))
}

// Applier manages all the "static" manifests and applies them on the k8s API
type Applier struct {
	Name string
	Dir  string

	log           *logrus.Entry
	clientFactory kubernetes.ClientFactoryInterface
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

// Apply resources
func (a *Applier) Apply(ctx context.Context) error {
	files, err := FindManifestFilesInDir(a.Dir)
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
		Clients:   a.clientFactory,
	}
	a.log.Debug("applying stack")
	err = stack.Apply(ctx, true)
	if err != nil {
		a.log.WithError(err).Warn("stack apply failed")
	} else {
		a.log.Debug("successfully applied stack")
	}

	return err
}

// Delete deletes the entire stack by applying it with empty set of resources
func (a *Applier) Delete(ctx context.Context) error {
	stack := Stack{Name: a.Name, Clients: a.clientFactory}
	logrus.Debugf("about to delete a stack %s with empty apply", a.Name)
	return stack.Apply(ctx, true)
}

func (a *Applier) parseFiles(files []string) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured
	if len(files) == 0 {
		return resources, nil
	}

	objects, err := resource.NewLocalBuilder().
		Unstructured().
		Path(false, files...).
		Flatten().
		Do().
		Infos()
	if err != nil {
		return nil, fmt.Errorf("unable to build resources: %w", err)
	}
	for _, o := range objects {
		item := o.Object.(*unstructured.Unstructured)
		if item.GetAPIVersion() != "" && item.GetKind() != "" {
			resources = append(resources, item)
		}
	}

	return resources, nil
}
