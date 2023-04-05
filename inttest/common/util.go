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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/k0sproject/k0s/pkg/constant"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

// Poll tries a condition func until it returns true, an error or the specified
// context is canceled or expired.
func Poll(ctx context.Context, condition wait.ConditionWithContextFunc) error {
	return wait.PollImmediateUntilWithContext(ctx, 100*time.Millisecond, condition)
}

func fallbackPoll(condition wait.ConditionWithContextFunc) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return Poll(ctx, condition)
}

// WaitForCalicoReady waits to see all calico pods healthy
func WaitForCalicoReady(kc *kubernetes.Clientset) error {
	return WaitForDaemonSet(kc, "calico-node")
}

// WaitForKubeRouterReady waits to see all kube-router pods healthy.
func WaitForKubeRouterReady(kc *kubernetes.Clientset) error {
	return fallbackPoll(waitForKubeRouterReady(kc))
}

// WaitForKubeRouterReady waits to see all kube-router pods healthy as long as
// the context isn't canceled.
func WaitForKubeRouterReadyWithContext(ctx context.Context, kc *kubernetes.Clientset) error {
	return Poll(ctx, waitForKubeRouterReady(kc))
}

func waitForKubeRouterReady(kc *kubernetes.Clientset) wait.ConditionWithContextFunc {
	return waitForDaemonSet(kc, "kube-router")
}

// WaitForCoreDNSReady waits to see all coredns pods healthy as long as the context isn't canceled.
// It also waits to see all the related svc endpoints to be ready to make sure coreDNS is actually
// ready to serve requests.
func WaitForCoreDNSReady(ctx context.Context, kc *kubernetes.Clientset) error {
	err := Poll(ctx, waitForDeployment(kc, "coredns"))
	if err != nil {
		return err
	}
	// Wait till we see the svc endpoints ready
	return wait.PollImmediateUntilWithContext(ctx, 100*time.Millisecond, func(ctx context.Context) (bool, error) {
		epSlices, err := kc.DiscoveryV1().EndpointSlices("kube-system").List(ctx, metav1.ListOptions{
			LabelSelector: "k8s-app=kube-dns",
		})

		// NotFound is ok, it might not be created yet
		if err != nil && !apierrors.IsNotFound(err) {
			return true, err
		} else if err != nil {
			return false, nil
		}

		if len(epSlices.Items) < 1 {
			return false, nil
		}

		// Check that all addresses show ready conditions
		for _, epSlice := range epSlices.Items {
			for _, endpoint := range epSlice.Endpoints {
				if !(*endpoint.Conditions.Ready && *endpoint.Conditions.Serving) {
					// endpoint not ready&serving yet
					return false, nil
				}
			}
		}

		return true, nil
	})
}

func WaitForMetricsReady(c *rest.Config) error {
	apiServiceClientset, err := clientset.NewForConfig(c)
	if err != nil {
		return err
	}

	return fallbackPoll(func(ctx context.Context) (done bool, err error) {
		apiService, err := apiServiceClientset.ApiregistrationV1().APIServices().Get(ctx, "v1beta1.metrics.k8s.io", v1.GetOptions{})
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

// WaitForDaemonSet waits for daemon set be ready.
func WaitForDaemonSet(kc *kubernetes.Clientset, name string) error {
	return fallbackPoll(waitForDaemonSet(kc, name))
}

// WaitForDaemonSetWithContext waits for daemon set be ready as long as the
// given context isn't canceled.
func WaitForDaemonSetWithContext(ctx context.Context, kc *kubernetes.Clientset, name string) error {
	return Poll(ctx, waitForDaemonSet(kc, name))
}

func waitForDaemonSet(kc *kubernetes.Clientset, name string) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (done bool, err error) {
		ds, err := kc.AppsV1().DaemonSets("kube-system").Get(ctx, name, v1.GetOptions{})
		if err != nil {
			return false, nil
		}

		return ds.Status.NumberAvailable == ds.Status.DesiredNumberScheduled, nil
	}
}

// WaitForDeployment waits for a deployment to become ready.
func WaitForDeployment(kc *kubernetes.Clientset, name string) error {
	return fallbackPoll(waitForDeployment(kc, name))
}

func WaitForDefaultStorageClass(ctx context.Context, kc *kubernetes.Clientset) error {
	return Poll(ctx, waitForDefaultStorageClass(kc))
}

func waitForDefaultStorageClass(kc *kubernetes.Clientset) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (done bool, err error) {
		sc, err := kc.StorageV1().StorageClasses().Get(ctx, "openebs-hostpath", v1.GetOptions{})
		if err != nil {
			return false, nil
		}

		return sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true", nil
	}
}

// WaitForDeploymentWithContext waits for a deployment to become ready as long
// as the given context isn't canceled.
func WaitForDeploymentWithContext(ctx context.Context, kc *kubernetes.Clientset, name string) error {
	return Poll(ctx, waitForDeployment(kc, name))
}

func waitForDeployment(kc *kubernetes.Clientset, name string) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (done bool, err error) {
		dep, err := kc.AppsV1().Deployments("kube-system").Get(ctx, name, v1.GetOptions{})
		if err != nil {
			return false, nil
		}

		return *dep.Spec.Replicas == dep.Status.ReadyReplicas, nil
	}
}

// WaitForPod waits for pod be running
func WaitForPod(kc *kubernetes.Clientset, name, namespace string) error {
	return fallbackPoll(waitForPod(kc, name, namespace))
}

// WaitForPod waits until a pod is running as long as the given context isn't
// canceled.
func WaitForPodWithContext(ctx context.Context, kc *kubernetes.Clientset, name, namespace string) error {
	return Poll(ctx, waitForPod(kc, name, namespace))
}

func waitForPod(kc *kubernetes.Clientset, name, namespace string) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (done bool, err error) {
		ds, err := kc.CoreV1().Pods(namespace).Get(ctx, name, v1.GetOptions{})
		if err != nil {
			return false, nil
		}

		return ds.Status.Phase == "Running", nil
	}
}

// WaitForPodLogs picks the first Ready pod from the list of pods in given namespace and gets the logs of it
func WaitForPodLogs(kc *kubernetes.Clientset, namespace string) error {
	return fallbackPoll(func(ctx context.Context) (done bool, err error) {
		pods, err := kc.CoreV1().Pods(namespace).List(ctx, v1.ListOptions{
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

func WaitForLease(ctx context.Context, kc *kubernetes.Clientset, name string, namespace string) error {

	return Poll(ctx, func(ctx context.Context) (done bool, err error) {
		lease, err := kc.CoordinationV1().Leases(namespace).Get(ctx, name, v1.GetOptions{})
		if err != nil && apierrors.IsNotFound(err) {
			return false, nil // Not found, keep polling
		} else if err != nil {
			return false, err
		}

		// Verify that there's a valid holder on the lease
		return *lease.Spec.HolderIdentity != "", nil
	})
}

// VerifyKubeletMetrics checks whether we see container and image labels in kubelet metrics.
// It does it via polling as it takes some time for kubelet to start reporting metrics.
func VerifyKubeletMetrics(ctx context.Context, kc *kubernetes.Clientset, node string) error {

	return Poll(ctx, func(ctx context.Context) (done bool, err error) {

		path := fmt.Sprintf("/api/v1/nodes/%s/proxy/metrics/cadvisor", node)
		metrics, err := kc.CoreV1().RESTClient().Get().AbsPath(path).Param("format", "text").DoRaw(ctx)
		if err != nil {
			return false, nil // do not return the error so we keep on polling
		}

		image := fmt.Sprintf("%s:%s", constant.KubeRouterCNIImage, constant.KubeRouterCNIImageVersion)
		containerRegex := regexp.MustCompile(fmt.Sprintf(`container_cpu_usage_seconds_total{container="kube-router".*image="%s"`, image))

		scanner := bufio.NewScanner(bytes.NewReader(metrics))
		for scanner.Scan() {
			line := scanner.Text()
			if containerRegex.MatchString(line) {
				return true, nil
			}
		}

		return false, nil
	})
}
