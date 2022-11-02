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

	"github.com/k0sproject/k0s/inttest/common"
	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2"
	apclient "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2/clientset"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sirupsen/logrus"
)

// WaitForPlanByName waits for the existence of a Plan having a specific name.
// On timeout, an error is returned.
func WaitForPlanByName(ctx context.Context, client apclient.Interface, name string, timeout time.Duration, conds ...func(obj *apv1beta2.Plan) bool) (*apv1beta2.Plan, error) {
	return WaitForByName[*apv1beta2.PlanList](ctx, client.AutopilotV1beta2().Plans(), name, timeout, conds...)
}

// Some shortcuts for very long type names.
type (
	CRD     = extensionsv1.CustomResourceDefinition
	CRDList = extensionsv1.CustomResourceDefinitionList
)

// WaitForCRDByName waits for the existence of a CRD having a specific name.
// On timeout, an error is returned.
func WaitForCRDByName(ctx context.Context, client extensionsclient.ApiextensionsV1Interface, name string, timeout time.Duration, conds ...func(*CRD) bool) (*CRD, error) {
	return WaitForByName[*CRDList](ctx, client.CustomResourceDefinitions(), name, timeout, conds...)
}

// WaitForName provides a generic way to wait for something with a given name, allowing
// user-specified conditionals.
func WaitForByName[L metav1.ListInterface, I any](ctx context.Context, client watch.Provider[L], name string, timeout time.Duration, conds ...func(*I) bool) (*I, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var match *I
	err := watch.FromClient[L, I](client).
		WithObjectName(name).
		WithErrorCallback(common.RetryWatchErrors(logrus.Infof)).
		Until(ctx, func(item *I) (bool, error) {
			// If any of the conditions fail, indicate that the poll should continue
			for _, cond := range conds {
				if !cond(item) {
					return false, nil
				}
			}

			match = item
			return true, nil
		})
	if err != nil {
		return nil, err
	}
	return match, nil
}
