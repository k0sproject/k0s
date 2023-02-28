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

package controller

import (
	"context"
	"fmt"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/static"
)

var _ manager.Component = (*CRD)(nil)

// CRD unpacks bundled CRD definitions to the filesystem
type CRD struct {
	saver   manifestsSaver
	bundles []string
}

// NewCRD build new CRD
func NewCRD(s manifestsSaver, bundles []string) *CRD {
	return &CRD{
		saver:   s,
		bundles: bundles,
	}
}

// Init  (c CRD) Init(_ context.Context) error {
func (c CRD) Init(_ context.Context) error {
	return nil
}

// Run unpacks manifests from bindata
func (c CRD) Start(_ context.Context) error {
	for _, bundle := range c.bundles {
		crds, err := static.AssetDir(fmt.Sprintf("manifests/%s/CustomResourceDefinition", bundle))
		if err != nil {
			return fmt.Errorf("can't unbundle CRD `%s` manifests: %v", bundle, err)
		}

		for _, filename := range crds {
			manifestName := fmt.Sprintf("%s-crd-%s", bundle, filename)
			content, err := static.Asset(fmt.Sprintf("manifests/%s/CustomResourceDefinition/%s", bundle, filename))
			if err != nil {
				return fmt.Errorf("failed to fetch crd `%s`: %v", filename, err)
			}
			if err := c.saver.Save(manifestName, content); err != nil {
				return fmt.Errorf("failed to save CRD `%s` manifest `%s` to FS: %v", bundle, manifestName, err)
			}
		}

	}

	return nil
}

func (c CRD) Stop() error {
	return nil
}
