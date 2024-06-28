/*
Copyright 2024 k0s authors

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

package nllb

import (
	"context"
	_ "embed"
	"fmt"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/stretchr/testify/assert"
	testifysuite "github.com/stretchr/testify/suite"
	"sigs.k8s.io/yaml"
)

type suite struct {
	common.BootlooseSuite
}

//go:embed rings.yaml
var rings string

func (s *suite) TestStackApplier() {
	ctx, cancel := context.WithCancelCause(s.Context())
	s.T().Cleanup(func() { cancel(nil) })

	k0sConfig, err := yaml.Marshal(&v1beta1.ClusterConfig{
		Spec: &v1beta1.ClusterSpec{
			Storage: &v1beta1.StorageSpec{Type: v1beta1.KineStorageType},
		},
	})
	s.Require().NoError(err)

	s.WriteFileContent(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--disable-components=control-api,konnectivity-server,kube-controller-manager,kube-scheduler"))
	s.MakeDir(s.ControllerNode(0), "/var/lib/k0s/manifests/rings")
	s.PutFile(s.ControllerNode(0), "/var/lib/k0s/manifests/rings/rings.yaml", rings)

	kubeconfig, err := s.GetKubeConfig(s.ControllerNode(0))
	s.Require().NoError(err)
	client, err := dynamic.NewForConfig(kubeconfig)
	s.Require().NoError(err)

	sgv := schema.GroupVersion{Group: "k0s.example.com", Version: "v1"}

	s.T().Run("hobbit", func(t *testing.T) {
		t.Cleanup(func() {
			if t.Failed() {
				cancel(fmt.Errorf("%s failed", t.Name()))
			}
		})
		t.Parallel()
		species := client.Resource(sgv.WithResource("species"))
		assert.NoError(t, watch.Unstructured(species).
			WithObjectName("hobbit").
			WithErrorCallback(retryWatchErrors(t.Logf)).
			Until(ctx, func(item *unstructured.Unstructured) (bool, error) {
				speciesName, found, err := unstructured.NestedString(item.Object, "spec", "characteristics")
				if assert.NoError(t, err) && assert.True(t, found, "no characteristics found: %v", item.Object) {
					assert.Equal(t, "hairy feet", speciesName)
				}
				return true, nil
			}))
	})

	s.T().Run("frodo", func(t *testing.T) {
		t.Cleanup(func() {
			if t.Failed() {
				cancel(fmt.Errorf("%s failed", t.Name()))
			}
		})
		t.Parallel()
		characters := client.Resource(sgv.WithResource("characters"))
		assert.NoError(t, watch.Unstructured(characters.Namespace("shire")).
			WithObjectName("frodo").
			WithErrorCallback(retryWatchErrors(t.Logf)).
			Until(ctx, func(item *unstructured.Unstructured) (bool, error) {
				speciesName, found, err := unstructured.NestedString(item.Object, "spec", "speciesRef", "name")
				if assert.NoError(t, err) && assert.True(t, found, "no species found: %v", item.Object) {
					assert.Equal(t, "hobbit", speciesName)
				}
				return true, nil
			}))
	})
}

func retryWatchErrors(logf common.LogfFn) watch.ErrorCallback {
	commonRetry := common.RetryWatchErrors(logf)
	return func(err error) (time.Duration, error) {
		if retryDelay, err := commonRetry(err); err == nil {
			return retryDelay, nil
		}
		if apierrors.IsNotFound(err) {
			return 350 * time.Millisecond, nil
		}
		return 0, err
	}
}

func TestStackApplierSuite(t *testing.T) {
	s := suite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     0,
		},
	}
	testifysuite.Run(t, &s)
}
