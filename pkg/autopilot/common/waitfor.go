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

package common

import (
	"context"
	"time"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2"
	apclient "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2/clientset"

	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

type WaitForCondition func(obj interface{}) bool

// WaitForPlanByName waits for the existence of a Plan having a specific name.
// On timeout, an error is returned.
func WaitForPlanByName(ctx context.Context, client apclient.Interface, name string, timeout time.Duration, conds ...WaitForCondition) (*apv1beta2.Plan, error) {
	obj, err := WaitForByName(
		ctx,
		func(name string) (interface{}, error) {
			return client.AutopilotV1beta2().Plans().Get(ctx, name, v1.GetOptions{})
		},
		name,
		timeout,
		conds...,
	)

	return obj.(*apv1beta2.Plan), err
}

// WaitForCRDByName waits for the existence of a CRD having a specific name.
// On timeout, an error is returned.
func WaitForCRDByName(ctx context.Context, client extclient.ApiextensionsV1Interface, name string, timeout time.Duration, conds ...WaitForCondition) (*extensionsv1.CustomResourceDefinition, error) {
	obj, err := WaitForByName(
		ctx,
		func(name string) (interface{}, error) {
			return client.CustomResourceDefinitions().Get(ctx, name, v1.GetOptions{})
		},
		name,
		timeout,
		conds...,
	)

	return obj.(*extensionsv1.CustomResourceDefinition), err
}

type Getter func(name string) (interface{}, error)

// WaitForName provides a generic way to wait for something with a given name, allowing
// user-specified conditionals.
func WaitForByName(ctx context.Context, getter Getter, name string, timeout time.Duration, conds ...WaitForCondition) (interface{}, error) {
	var obj interface{}
	return obj, wait.PollImmediate(500*time.Millisecond, timeout, func() (done bool, err error) {
		obj, err = getter(name)
		if err != nil {
			return false, nil
		}

		if len(conds) > 0 {
			// If any of the conditions fail, indicate that the poll should continue
			for _, cond := range conds {
				if !cond(obj) {
					return false, nil
				}
			}
		}

		return true, nil
	})
}
