package common

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// Poll tries a condition func until it returns true, an error or the specified
// context is canceled or expired.
func Poll(ctx context.Context, condition wait.ConditionWithContextFunc) error {
	return wait.PollImmediateUntilWithContext(ctx, 100*time.Millisecond, condition)
}
