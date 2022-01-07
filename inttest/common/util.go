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
package common

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

// WaitForCalicoReady waits to see all calico pods healthy
func WaitForCalicoReady(kc *kubernetes.Clientset) error {
	return WaitForDaemonSet(kc, "calico-node")
}

// WaitForKubeRouterReady waits to see all kube-router pods healthy
func WaitForKubeRouterReady(kc *kubernetes.Clientset) error {
	return WaitForDaemonSet(kc, "kube-router")
}

func WaitForMetricsReady(c *rest.Config) error {
	apiServiceClientset, err := clientset.NewForConfig(c)
	if err != nil {
		return err
	}

	return wait.PollImmediate(100*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		apiService, err := apiServiceClientset.ApiregistrationV1().APIServices().Get(context.TODO(), "v1beta1.metrics.k8s.io", v1.GetOptions{})
		if err != nil {
			return false, nil
		}

		for _, c := range apiService.Status.Conditions {
			if c.Type == "Available" && c.Status == "True" {
				return true, nil
			}
		}

		return false, nil
	})
}

// WaitForDaemonSet waits for daemon set be ready
func WaitForDaemonSet(kc *kubernetes.Clientset, name string) error {
	return wait.PollImmediate(100*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		ds, err := kc.AppsV1().DaemonSets("kube-system").Get(context.TODO(), name, v1.GetOptions{})
		if err != nil {
			return false, nil
		}

		return ds.Status.NumberAvailable == ds.Status.DesiredNumberScheduled, nil
	})
}

// WaitForDaemonSet waits for daemon set be ready
func WaitForDeployment(kc *kubernetes.Clientset, name string) error {
	return wait.PollImmediate(100*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		dep, err := kc.AppsV1().Deployments("kube-system").Get(context.TODO(), name, v1.GetOptions{})
		if err != nil {
			return false, nil
		}

		return *dep.Spec.Replicas == dep.Status.ReadyReplicas, nil
	})
}

// WaitForPod waits for pod be running
func WaitForPod(kc *kubernetes.Clientset, name, namespace string) error {
	return wait.PollImmediate(100*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		ds, err := kc.CoreV1().Pods(namespace).Get(context.TODO(), name, v1.GetOptions{})
		if err != nil {
			return false, nil
		}

		return ds.Status.Phase == "Running", nil
	})
}

// WaitForPodLogs picks the first Ready pod from the list of pods in given namespace and gets the logs of it
func WaitForPodLogs(kc *kubernetes.Clientset, namespace string) error {
	return wait.PollImmediate(100*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		pods, err := kc.CoreV1().Pods(namespace).List(context.TODO(), v1.ListOptions{
			Limit: 100,
		})
		if err != nil {
			return false, err // stop polling with error in case the pod listing fails
		}
		var readyPod *corev1.Pod
		for _, p := range pods.Items {
			if p.Status.Phase == "Running" {
				readyPod = &p
			}
		}
		if readyPod == nil {
			return false, nil // do not return the error so we keep on polling
		}
		_, err = kc.CoreV1().Pods(readyPod.Namespace).GetLogs(readyPod.Name, &corev1.PodLogOptions{Container: readyPod.Spec.Containers[0].Name}).Stream(context.Background())
		if err != nil {
			return false, nil // do not return the error so we keep on polling
		}

		return true, nil
	})
}
