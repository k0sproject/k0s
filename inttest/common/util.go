package common

import (
	"context"
	"fmt"
	"time"

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
	return wait.PollImmediate(100*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		ds, err := kc.AppsV1().DaemonSets("kube-system").Get(context.TODO(), "kube-router", v1.GetOptions{})
		if err != nil {
			fmt.Printf("error while getting kube-router DS: %s\n", err.Error())
			return false, nil
		}

		return ds.Status.NumberReady == ds.Status.DesiredNumberScheduled, nil
	})
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

// WaitForPod waits for pod be running
func WaitForPod(kc *kubernetes.Clientset, name string) error {
	return wait.PollImmediate(100*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		ds, err := kc.CoreV1().Pods("kube-system").Get(context.TODO(), name, v1.GetOptions{})
		if err != nil {
			return false, nil
		}

		return ds.Status.Phase == "Running", nil
	})
}
