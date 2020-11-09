package controller

import (
	"context"
	"github.com/Mirantis/mke/pkg/apis/helm.k0sproject.io/clientset"
	"github.com/Mirantis/mke/pkg/apis/helm.k0sproject.io/v1beta1"
	"github.com/davecgh/go-spew/spew"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/go-logr/logr"
)

// ChartReconciler reconciles a Chart object
type ChartReconciler struct {
	Client clientset.ChartV1Beta1Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=helm.k0sproject.io,resources=charts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=helm.k0sproject.io,resources=charts/status,verbs=get;update;patch

func (r *ChartReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("chart", req.NamespacedName)

	// your logic here
	spew.Dump(req)
	return ctrl.Result{}, nil
}

func (r *ChartReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Chart{}).
		Complete(r)
}
