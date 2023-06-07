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

package common

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"syscall"
	"time"

	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"

	"github.com/sirupsen/logrus"
)

// Poll tries a condition func until it returns true, an error or the specified
// context is canceled or expired.
func Poll(ctx context.Context, condition wait.ConditionWithContextFunc) error {
	return wait.PollImmediateUntilWithContext(ctx, 100*time.Millisecond, condition)
}

// WaitForKubeRouterReady waits to see all kube-router pods healthy as long as
// the context isn't canceled.
func WaitForKubeRouterReady(ctx context.Context, kc *kubernetes.Clientset) error {
	return WaitForDaemonSet(ctx, kc, "kube-router")
}

// WaitForCoreDNSReady waits to see all coredns pods healthy as long as the context isn't canceled.
// It also waits to see all the related svc endpoints to be ready to make sure coreDNS is actually
// ready to serve requests.
func WaitForCoreDNSReady(ctx context.Context, kc *kubernetes.Clientset) error {
	err := WaitForDeployment(ctx, kc, "coredns", "kube-system")
	if err != nil {
		return fmt.Errorf("wait for deployment: %w", err)
	}
	// Wait till we see the svc endpoints ready
	return wait.PollImmediateUntilWithContext(ctx, 100*time.Millisecond, func(ctx context.Context) (bool, error) {
		epSlices, err := kc.DiscoveryV1().EndpointSlices("kube-system").List(ctx, metav1.ListOptions{
			LabelSelector: "k8s-app=kube-dns",
		})

		// NotFound is ok, it might not be created yet
		if err != nil && !apierrors.IsNotFound(err) {
			return true, fmt.Errorf("wait for coredns: list: %w", err)
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

func WaitForMetricsReady(ctx context.Context, c *rest.Config) error {
	clientset, err := aggregatorclient.NewForConfig(c)
	if err != nil {
		return err
	}

	watchAPIServices := watch.FromClient[*apiregistrationv1.APIServiceList, apiregistrationv1.APIService]
	return watchAPIServices(clientset.ApiregistrationV1().APIServices()).
		WithObjectName("v1beta1.metrics.k8s.io").
		WithErrorCallback(RetryWatchErrors(logrus.Infof)).
		Until(ctx, func(service *apiregistrationv1.APIService) (bool, error) {
			for _, c := range service.Status.Conditions {
				if c.Type == apiregistrationv1.Available {
					if c.Status == apiregistrationv1.ConditionTrue {
						return true, nil
					}

					break
				}
			}

			return false, nil
		})
}

func WaitForNodeReadyStatus(ctx context.Context, clients kubernetes.Interface, nodeName string, status corev1.ConditionStatus) error {
	return watch.Nodes(clients.CoreV1().Nodes()).
		WithObjectName(nodeName).
		Until(ctx, func(node *corev1.Node) (done bool, err error) {
			for _, cond := range node.Status.Conditions {
				if cond.Type == corev1.NodeReady {
					if cond.Status == status {
						return true, nil
					}

					break
				}
			}

			return false, nil
		})
}

// WaitForDaemonset waits for the DaemonlSet with the given name to have
// as many ready replicas as defined in the spec.
func WaitForDaemonSet(ctx context.Context, kc *kubernetes.Clientset, name string) error {
	return watch.DaemonSets(kc.AppsV1().DaemonSets("kube-system")).
		WithObjectName(name).
		WithErrorCallback(RetryWatchErrors(logrus.Infof)).
		Until(ctx, func(ds *appsv1.DaemonSet) (bool, error) {
			return ds.Status.NumberAvailable == ds.Status.DesiredNumberScheduled, nil
		})
}

// WaitForDeployment waits for the Deployment with the given name to become
// available as long as the given context isn't canceled.
func WaitForDeployment(ctx context.Context, kc *kubernetes.Clientset, name, namespace string) error {
	return watch.Deployments(kc.AppsV1().Deployments(namespace)).
		WithObjectName(name).
		WithErrorCallback(RetryWatchErrors(logrus.Infof)).
		Until(ctx, func(deployment *appsv1.Deployment) (bool, error) {
			for _, c := range deployment.Status.Conditions {
				if c.Type == appsv1.DeploymentAvailable {
					if c.Status == corev1.ConditionTrue {
						return true, nil
					}

					break
				}
			}

			return false, nil
		})
}

// WaitForStatefulSet waits for the StatefulSet with the given name to have
// as many ready replicas as defined in the spec.
func WaitForStatefulSet(ctx context.Context, kc *kubernetes.Clientset, name, namespace string) error {
	return watch.StatefulSets(kc.AppsV1().StatefulSets(namespace)).
		WithObjectName(name).
		WithErrorCallback(RetryWatchErrors(logrus.Infof)).
		Until(ctx, func(s *appsv1.StatefulSet) (bool, error) {
			return s.Status.ReadyReplicas == *s.Spec.Replicas, nil
		})
}

func WaitForDefaultStorageClass(ctx context.Context, kc *kubernetes.Clientset) error {
	return Poll(ctx, waitForDefaultStorageClass(kc))
}

func waitForDefaultStorageClass(kc *kubernetes.Clientset) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (done bool, err error) {
		sc, err := kc.StorageV1().StorageClasses().Get(ctx, "openebs-hostpath", metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		return sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true", nil
	}
}

// WaitForPod waits for the given pod to become ready as long as the given
// context isn't canceled.
func WaitForPod(ctx context.Context, kc *kubernetes.Clientset, name, namespace string) error {
	return watch.Pods(kc.CoreV1().Pods(namespace)).
		WithObjectName(name).
		WithErrorCallback(RetryWatchErrors(logrus.Infof)).
		Until(ctx, func(pod *corev1.Pod) (bool, error) {
			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady {
					if cond.Status == corev1.ConditionTrue {
						return true, nil
					}

					break
				}
			}

			return false, nil
		})
}

// WaitForPodLogs waits until it can stream the logs of the first running pod
// that comes along in the given namespace as long as the given context isn't
// canceled.
func WaitForPodLogs(ctx context.Context, kc *kubernetes.Clientset, namespace string) error {
	return Poll(ctx, func(ctx context.Context) (done bool, err error) {
		pods, err := kc.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			Limit:         100,
			FieldSelector: fields.OneTermEqualSelector("status.phase", string(corev1.PodRunning)).String(),
		})
		if err != nil {
			return false, err // stop polling with error in case the pod listing fails
		}
		if len(pods.Items) < 1 {
			return false, nil
		}

		pod := &pods.Items[0]
		logs, err := kc.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: pod.Spec.Containers[0].Name}).Stream(ctx)
		if err != nil {
			return false, nil // do not return the error so we keep on polling
		}
		defer logs.Close()

		return true, nil
	})
}

func WaitForLease(ctx context.Context, kc *kubernetes.Clientset, name string, namespace string) (string, error) {
	var holderIdentity string
	watchLeases := watch.FromClient[*coordinationv1.LeaseList, coordinationv1.Lease]
	if err := watchLeases(kc.CoordinationV1().Leases(namespace)).
		WithObjectName(name).
		WithErrorCallback(RetryWatchErrors(logrus.Infof)).
		Until(
			ctx, func(lease *coordinationv1.Lease) (bool, error) {
				holderIdentity = *lease.Spec.HolderIdentity
				// Verify that there's a valid holder on the lease
				return holderIdentity != "", nil
			},
		); err != nil {
		return "", err
	}

	return holderIdentity, nil
}

func RetryWatchErrors(logf func(format string, args ...any)) watch.ErrorCallback {
	return func(err error) (time.Duration, error) {
		if retryDelay, e := watch.IsRetryable(err); e == nil {
			logf("Encountered transient watch error, retrying in %s: %v", retryDelay, err)
			return retryDelay, nil
		}

		retryDelay := 1 * time.Second

		switch {
		case errors.Is(err, syscall.ECONNRESET):
			logf("Encountered connection reset while watching, retrying in %s: %v", retryDelay, err)
			return retryDelay, nil

		case errors.Is(err, syscall.ECONNREFUSED):
			logf("Encountered connection refused while watching, retrying in %s: %v", retryDelay, err)
			return retryDelay, nil

		case errors.Is(err, io.EOF):
			logf("Encountered EOF while watching, retrying in %s: %v", retryDelay, err)
			return retryDelay, nil
		}

		return 0, err
	}
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
