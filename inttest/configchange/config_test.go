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

package configchange

import (
	"context"
	"fmt"
	"testing"
	"time"

	cfgClient "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/clientset/typed/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
)

type ConfigSuite struct {
	common.FootlooseSuite
}

func TestConfigSuite(t *testing.T) {
	s := ConfigSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}

func (s *ConfigSuite) TestK0sGetsUp() {

	s.NoError(s.InitController(0, "--enable-dynamic-config"))
	s.NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)
	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.NoError(err)
	s.T().Log("waiting to see kube-router pods ready")
	s.NoError(common.WaitForKubeRouterReady(s.Context(), kc), "kube-router did not start")

	// Cluster is up-and-running, we can now start testing the config changes

	s.NoError(s.clearConfigEvents(kc))

	cfgClient, err := s.getConfigClient()
	s.Require().NoError(err)

	eventWatch, err := kc.CoreV1().Events("kube-system").Watch(context.Background(), metav1.ListOptions{FieldSelector: "involvedObject.name=k0s"})
	s.Require().NoError(err)
	defer eventWatch.Stop()

	s.T().Run("changing cni should fail", func(t *testing.T) {
		originalConfig, err := cfgClient.Get(s.Context(), "k0s", metav1.GetOptions{})
		s.Require().NoError(err)
		newConfig := originalConfig.DeepCopy()
		newConfig.Spec.Network.Provider = constant.CNIProviderCalico
		newConfig.Spec.Network.Calico = v1beta1.DefaultCalico()
		newConfig.Spec.Network.KubeRouter = nil
		_, err = cfgClient.Update(s.Context(), newConfig, metav1.UpdateOptions{})
		s.Require().NoError(err)

		// Check that we see proper event for failed reconcile
		event, err := s.waitForReconcileEvent(eventWatch)
		s.Require().NoError(err)

		s.Equal("Warning", event.Type)
		s.Equal("FailedReconciling", event.Reason)
		s.Equal("cannot change CNI provider from kuberouter to calico", event.Message)
	})

	s.T().Run("setting bad ip address should fail", func(t *testing.T) {
		originalConfig, err := cfgClient.Get(context.Background(), "k0s", metav1.GetOptions{})
		s.Require().NoError(err)
		newConfig := originalConfig.DeepCopy()
		newConfig.Spec.Network = v1beta1.DefaultNetwork()
		newConfig.Spec.Network.PodCIDR = "invalid ip address"
		_, err = cfgClient.Update(context.Background(), newConfig, metav1.UpdateOptions{})
		s.Require().NoError(err)

		// Check that we see proper event for failed reconcile
		event, err := s.waitForReconcileEvent(eventWatch)
		s.Require().NoError(err)

		s.T().Logf("the event is %+v", event)
		s.Equal("Warning", event.Type)
		s.Equal("FailedReconciling", event.Reason)
	})

	s.T().Run("changing kuberouter MTU should work", func(t *testing.T) {
		originalConfig, err := cfgClient.Get(context.Background(), "k0s", metav1.GetOptions{})
		s.Require().NoError(err)
		newConfig := originalConfig.DeepCopy()
		newConfig.Spec.Network = v1beta1.DefaultNetwork()
		newConfig.Spec.Network.KubeRouter.AutoMTU = false
		newConfig.Spec.Network.KubeRouter.MTU = 1300

		// Get the resource version for current kuberouter configmap
		cml, err := kc.CoreV1().ConfigMaps("kube-system").List(s.Context(), metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("metadata.name", "kube-router-cfg").String(),
		})
		s.Require().NoError(err)

		// Get the resource version for the current ds
		ds, err := kc.AppsV1().DaemonSets("kube-system").List(s.Context(), metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("metadata.name", "kube-router").String(),
		})

		s.Require().NoError(err)

		_, err = cfgClient.Update(s.Context(), newConfig, metav1.UpdateOptions{})
		s.Require().NoError(err)
		if event, err := s.waitForReconcileEvent(eventWatch); s.NoError(err) {
			s.Equal("Normal", event.Type)
			s.Equal("SuccessfulReconcile", event.Reason)
		}

		// Verify MTU setting have been propagated properly
		// It takes a while to actually apply the changes through stack applier
		// Start the watch only from last version so we only get changed cm(s) and not the original one
		configMapWatch, err := kc.CoreV1().ConfigMaps("kube-system").Watch(s.Context(), metav1.ListOptions{
			FieldSelector:   fields.OneTermEqualSelector("metadata.name", "kube-router-cfg").String(),
			ResourceVersion: cml.ResourceVersion,
		})
		s.Require().NoError(err)

		daemonSetWatchChannel, err := kc.AppsV1().DaemonSets("kube-system").Watch(s.Context(), metav1.ListOptions{
			FieldSelector:   fields.OneTermEqualSelector("metadata.name", "kube-router").String(),
			ResourceVersion: ds.ResourceVersion,
		})
		s.Require().NoError(err)
		defer configMapWatch.Stop()
		timeout := time.After(20 * time.Second)
		for i := 0; i < 2; i++ {
			select {
			case event := <-configMapWatch.ResultChan():
				cm := event.Object.(*corev1.ConfigMap)
				cniConf := cm.Data["cni-conf.json"]
				s.Contains(cniConf, `"mtu": 1300`)
			case event := <-daemonSetWatchChannel.ResultChan():
				ds := event.Object.(*appsv1.DaemonSet)
				s.Require().Contains(ds.Spec.Template.Spec.Containers[0].Args, "--auto-mtu=false")
			case <-timeout:
				s.Require().Fail("timed out while waiting for ConfigMap change")
			}
		}

	})
}

func (s *ConfigSuite) waitForReconcileEvent(eventWatch watch.Interface) (*corev1.Event, error) {
	timeout := time.After(20 * time.Second)
	select {
	case e := <-eventWatch.ResultChan():
		event := e.Object.(*corev1.Event)
		return event, nil
	case <-timeout:
		return nil, fmt.Errorf("timeout waiting for reconcile event")
	}
}

func (s *ConfigSuite) clearConfigEvents(kc *kubernetes.Clientset) error {
	return kc.CoreV1().Events("kube-system").DeleteCollection(s.Context(), metav1.DeleteOptions{}, metav1.ListOptions{FieldSelector: "involvedObject.name=k0s"})
}

// Get the ClusterConfig client from the controller node's kubeconfig.
func (s *ConfigSuite) getConfigClient() (cfgClient.ClusterConfigInterface, error) {
	config, err := s.GetKubeConfig(s.ControllerNode(0))
	if err != nil {
		return nil, fmt.Errorf("can't get kubeconfig: %w", err)
	}
	c, err := cfgClient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("can't create kubernetes typed client for cluster config: %w", err)
	}
	return c.ClusterConfigs(constant.ClusterConfigNamespace), nil
}
