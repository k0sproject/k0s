package controller

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/k0sproject/k0s/pkg/component"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cfgClient "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/clientset"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
)

var (
	resourceType = v1.TypeMeta{APIVersion: "k0s.k0sproject.io/v1beta1", Kind: "clusterconfigs"}
	cOpts        = v1.CreateOptions{TypeMeta: resourceType}
	getOpts      = v1.GetOptions{TypeMeta: resourceType}
)

// ClusterConfigReconciler reconciles a ClusterConfig object
type ClusterConfigReconciler struct {
	ClusterConfig    *v1beta1.ClusterConfig
	ComponentManager *component.Manager

	configClient    cfgClient.ClusterConfigInterface
	kubeConfig      string
	leaderElector   LeaderElector
	log             *logrus.Entry
	resourceVersion string

	tickerDone chan struct{}
}

// NewClusterConfigReconciler creates a new clusterConfig reconciler
func NewClusterConfigReconciler(c *v1beta1.ClusterConfig, leaderElector LeaderElector, k0sVars constant.CfgVars, mgr *component.Manager) *ClusterConfigReconciler {
	d := atomic.Value{}
	d.Store(true)

	return &ClusterConfigReconciler{
		ClusterConfig:    c,
		ComponentManager: mgr,

		kubeConfig:    k0sVars.AdminKubeConfigPath,
		leaderElector: leaderElector,
		log:           logrus.WithFields(logrus.Fields{"component": "clusterConfig-reconciler"}),
	}
}

func (r *ClusterConfigReconciler) Init() error {
	return nil
}

func (r *ClusterConfigReconciler) Run() error {
	c, err := cfgClient.NewForConfig(r.kubeConfig)
	if err != nil {
		return fmt.Errorf("can't create kubernetes typed client for cluster config: %v", err)
	}
	r.configClient = c.ClusterConfigs(constant.ClusterConfigNamespace)
	r.tickerDone = make(chan struct{})

	// check if a CR already exists, and if so, populate the current resourceVersion
	cfg, err := r.configClient.Get(context.Background(), "k0s", getOpts)
	if err == nil {
		r.resourceVersion = cfg.ResourceVersion
	}

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err := r.Reconcile()
				if err != nil {
					r.log.Warnf("cluster-config reconciliation failed: %s", err.Error())
				}
			case <-r.tickerDone:
				r.log.Info("cluster-config reconciler done")
				return
			}
		}
	}()

	return nil
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
func (r *ClusterConfigReconciler) Reconcile() error {
	clusterConfig, err := r.configClient.Get(context.Background(), "k0s", getOpts)
	if err != nil {
		if errors.IsNotFound(err) {
			// ClusterConfig CR cannot be found, which means we can create it
			r.log.Debugf("didn't find cluster-config object: %v", err)
			err := r.copyRunningConfigToCR()
			if err != nil {
				r.log.Errorf("failed to save cluster-config  %v\n", err)
			}
		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
			r.log.Errorf("error getting cluster-config %v\n", statusError.ErrStatus.Message)
		}
		r.log.Errorf("failed to reconcile config status: %v", err)
		return err
	}
	// watch the clusterConfig resource for changes
	if clusterConfig.ResourceVersion > r.resourceVersion {
		r.log.Debugf("detected change in cluster-config custom resource: previous resourceVersion: %s, new resourceVersion: %s", r.resourceVersion, clusterConfig.ResourceVersion)
		r.resourceVersion = clusterConfig.ResourceVersion
		err = r.ComponentManager.Reconcile()
		if err != nil {
			return err
		}
	}
	r.log.Debugf("reconciling cluster-config (nothing to do!)")
	return nil
}

// Stop stops
func (r *ClusterConfigReconciler) Stop() error {
	if r.tickerDone != nil {
		close(r.tickerDone)
	}
	return nil
}

func (r *ClusterConfigReconciler) Healthy() error {
	return nil
}

func (r *ClusterConfigReconciler) copyRunningConfigToCR() error {
	if !r.leaderElector.IsLeader() {
		r.log.Debug("I am not the leader, not reconciling cluster configuration")
		return nil
	}
	clusterWideConfig := clusterConfigMinusNodeConfig(r.ClusterConfig)
	clusterConfig, err := r.configClient.Create(context.Background(), clusterWideConfig, cOpts)
	if err != nil {
		return err
	}
	r.resourceVersion = clusterConfig.ResourceVersion
	r.log.Info("successfully wrote cluster-config to API")
	return nil
}

// HACK: the current ClusterConfig struct holds both bootstrapping config & cluster-wide config
// this hack strips away the node-specific bootstrapping config so that we write a "clean" config to the CR
// This function accepts a standard ClusterConfig and returns the same config minus the node specific info:
//		- APISpec
//		- StorageSpec
//		- Network.ServiceCIDR
// TODO: separate bootstrapping configuration from node-specific configuration
func clusterConfigMinusNodeConfig(config *v1beta1.ClusterConfig) *v1beta1.ClusterConfig {
	clusterSpec := &v1beta1.ClusterSpec{
		ControllerManager: config.Spec.ControllerManager,
		Scheduler:         config.Spec.Scheduler,
		Network: &v1beta1.Network{
			Calico:     config.Spec.Network.Calico,
			DualStack:  config.Spec.Network.DualStack,
			KubeProxy:  config.Spec.Network.KubeProxy,
			KubeRouter: config.Spec.Network.KubeRouter,
			PodCIDR:    config.Spec.Network.PodCIDR,
			Provider:   config.Spec.Network.Provider,
		},
		PodSecurityPolicy: config.Spec.PodSecurityPolicy,
		WorkerProfiles:    config.Spec.WorkerProfiles,
		Telemetry:         config.Spec.Telemetry,
		Install:           config.Spec.Install,
		Images:            config.Spec.Images,
		Extensions:        config.Spec.Extensions,
		Konnectivity:      config.Spec.Konnectivity,
	}

	return &v1beta1.ClusterConfig{
		ObjectMeta: config.ObjectMeta,
		TypeMeta:   config.TypeMeta,
		DataDir:    config.DataDir,
		Spec:       clusterSpec,
		Status:     config.Status,
	}
}
