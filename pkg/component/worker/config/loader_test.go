/*
Copyright 2022 k0s authors

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

package config

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/constant"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadProfile(t *testing.T) {
	workerConfigMap := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("worker-config-fake-%s", constant.KubernetesMajorMinorVersion),
			Namespace: "kube-system",
		},
		Data: map[string]string{"kubeletConfiguration": `{"kind":"foo"}`},
	}

	clientFactory := func() (kubernetes.Interface, error) {
		return fake.NewSimpleClientset(&workerConfigMap), nil
	}

	workerProfile, err := func() (*Profile, error) {
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()
		timer := time.AfterFunc(10*time.Second, func() {
			assert.Fail(t, "Call to Loader.Load() took too long, check the logs for details")
			cancel()
		})
		defer timer.Stop()

		log := logrus.New()
		log.SetLevel(logrus.DebugLevel)

		workerConfig, err := loadProfile(
			ctx,
			log.WithField("test", t.Name()),
			clientFactory,
			"fake",
		)

		if t.Failed() {
			t.FailNow()
		}
		return workerConfig, err
	}()

	require.NoError(t, err)
	if assert.NotNil(t, workerProfile) {
		expected := kubeletv1beta1.KubeletConfiguration{
			TypeMeta: metav1.TypeMeta{Kind: "foo"},
		}
		assert.Equal(t, &expected, &workerProfile.KubeletConfiguration)
	}
}
