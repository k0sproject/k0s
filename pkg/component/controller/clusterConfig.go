package controller

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cfgClient "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/clientset"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
)

var resourceType = v1.TypeMeta{APIVersion: "k0s.k0sproject.io/v1beta1", Kind: "clusterconfigs"}

// ClusterConfigReconciler reconciles a ClusterConfig object
type ClusterConfigReconciler struct {
	ClusterConfig *v1beta1.ClusterConfig

	configClient  *cfgClient.K0sV1beta1Client
	kubeConfig    string
	leaderElector LeaderElector
	log           *logrus.Entry

	tickerDone chan struct{}
}

// NewClusterConfigReconciler creates a new clusterConfig reconciler
func NewClusterConfigReconciler(c *v1beta1.ClusterConfig, leaderElector LeaderElector, k0sVars constant.CfgVars) *ClusterConfigReconciler {
	d := atomic.Value{}
	d.Store(true)

	return &ClusterConfigReconciler{
		ClusterConfig: c,

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
		return fmt.Errorf("can't create kubernetes typed Client for cluster config: %v", err)
	}
	r.configClient = c
	r.tickerDone = make(chan struct{})
	go r.Reconcile()
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
func (r *ClusterConfigReconciler) Reconcile() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			getOpts := v1.GetOptions{TypeMeta: resourceType}
			_, err := r.configClient.ClusterConfigs(constant.ClusterConfigNamespace).Get(context.Background(), "k0s", getOpts)
			if err != nil {
				if errors.IsNotFound(err) {
					// ClusterConfig CR cannot be found, which means we can create it
					err := r.copyRunningConfigToCR()
					if err != nil {
						r.log.Errorf("failed to save cluster config  %v\n", err)
					}
				} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
					r.log.Errorf("Error getting cluster config %v\n", statusError.ErrStatus.Message)
				}
				r.log.Errorf("failed to reconcile config status: %v", err)
				continue
			}
			/*
				if r.ClusterConfig.Spec != clusterConfig.Spec {
					// found a change in configuration
					r.log.Infof("detected change in cluster config. reconciling...")
				}*/

		case <-r.tickerDone:
			r.log.Info("clusterConfig reconciler done")
			return
		}
	}
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

// HACK: the current ClusterConfig struct holds both bootstrapping config & cluster-wide config
// this hack stripps away the node-specific bootstrapping config so that we write a "clean" config to the CR
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

func (r *ClusterConfigReconciler) copyRunningConfigToCR() error {
	clusterWideConfig := clusterConfigMinusNodeConfig(r.ClusterConfig)
	createOpts := v1.CreateOptions{TypeMeta: resourceType}
	_, err := r.configClient.ClusterConfigs(constant.ClusterConfigNamespace).Create(context.Background(), clusterWideConfig, createOpts)
	if err != nil {
		return err
	}
	return nil
}
