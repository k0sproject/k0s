package controller

import (
	"context"
	"sync/atomic"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	k8sutil "github.com/k0sproject/k0s/pkg/kubernetes"
)

// ClusterConfigReconciler reconciles a ClusterConfig object
type ClusterConfigReconciler struct {
	ClusterConfig *v1beta1.ClusterConfig

	kubeClientFactory k8sutil.ClientFactory
	leaderElector     LeaderElector
	log               *logrus.Entry
	scheme            *runtime.Scheme
	stopCh            chan struct{}
}

// NewClusterConfigReconciler creates a new clusterConfig reconciler
func NewClusterConfigReconciler(c *v1beta1.ClusterConfig, leaderElector LeaderElector, kubeClientFactory k8sutil.ClientFactory) *ClusterConfigReconciler {
	d := atomic.Value{}
	d.Store(true)
	return &ClusterConfigReconciler{
		ClusterConfig: c,

		kubeClientFactory: kubeClientFactory,
		leaderElector:     leaderElector,
		log:               logrus.WithFields(logrus.Fields{"component": "clusterConfig-reconciler"}),
		stopCh:            make(chan struct{}),
	}
}

//+kubebuilder:rbac:groups=k0s.k0sproject.io,resources=clusterconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k0s.k0sproject.io,resources=clusterconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k0s.k0sproject.io,resources=clusterconfigs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *ClusterConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.ClusterConfig{}).
		Complete(r)
}
