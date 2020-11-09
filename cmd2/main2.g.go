package main

import (
	"context"
	"github.com/Mirantis/mke/pkg/apis/helm.k0sproject.io/clientset"
	"github.com/Mirantis/mke/pkg/apis/helm.k0sproject.io/v1beta1"
	"github.com/Mirantis/mke/pkg/component/server"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/davecgh/go-spew/spew"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	//"k8s.io/client-go/util/workqueue"

	// +kubebuilder:scaffold:imports
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	stopCh := make(chan struct{})
	client, err := clientset.NewForConfig(constant.AdminKubeconfigConfigPath)
	check(err)
	ha := server.HelmAddons{
		Client: client,
		L:      logrus.WithField("component", "cli"),
	}

	go ha.CrdControlLoop()
	<-stopCh
}

var (
	setupLog = ctrl.Log.WithName("setup")
)

type reconciler struct {
	client clientset.ChartV1Beta1Interface
	scheme *runtime.Scheme
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	spew.Dump(ctx, req)

	return ctrl.Result{}, nil
}

func main1() {
	client, err := clientset.NewForConfig(constant.AdminKubeconfigConfigPath)
	check(err)
	ctrl.SetLogger(zap.New())

	config, err := clientcmd.BuildConfigFromFlags("", constant.AdminKubeconfigConfigPath)

	check(err)

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// in a real controller, we'd create a new scheme for this
	err = api.AddToScheme(mgr.GetScheme())
	if err != nil {
		setupLog.Error(err, "unable to add scheme")
		os.Exit(1)
	}

	err = ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Chart{}).
		Owns(&corev1.Pod{}).
		Complete(&reconciler{
			client: client,
			scheme: mgr.GetScheme(),
		})
	if err != nil {
		setupLog.Error(err, "unable to create controller")
		os.Exit(1)
	}

	err = ctrl.NewWebhookManagedBy(mgr).
		For(&v1beta1.Chart{}).
		Complete()
	if err != nil {
		setupLog.Error(err, "unable to create webhook")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
