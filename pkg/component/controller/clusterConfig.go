package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/static"

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
	YamlConfig       *v1beta1.ClusterConfig
	ComponentManager *component.Manager

	configClient                cfgClient.ClusterConfigInterface
	kubeConfig                  string
	leaderElector               LeaderElector
	log                         *logrus.Entry
	lastReconciledConfigVersion string
	saver                       manifestsSaver

	tickerDone chan struct{}
}

// NewClusterConfigReconciler creates a new clusterConfig reconciler
func NewClusterConfigReconciler(cfgFile string, leaderElector LeaderElector, k0sVars constant.CfgVars, mgr *component.Manager, s manifestsSaver) (*ClusterConfigReconciler, error) {
	cfg, err := config.GetYamlFromFile(cfgFile, k0sVars)
	if err != nil {
		return nil, err
	}
	return &ClusterConfigReconciler{
		ComponentManager: mgr,
		YamlConfig:       cfg,

		kubeConfig:    k0sVars.AdminKubeConfigPath,
		leaderElector: leaderElector,
		log:           logrus.WithFields(logrus.Fields{"component": "clusterConfig-reconciler"}),
		saver:         s,
	}, nil
}

func (r *ClusterConfigReconciler) Init() error {
	err := r.writeCRD()
	if err != nil {
		return fmt.Errorf("failed to write api-config CRD to API: %v", err)
	}
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
	// cfg, err := r.configClient.Get(context.Background(), "k0s", getOpts)
	// if err == nil {
	// 	r.resourceVersion = cfg.ResourceVersion
	// }

	go func() {
		ticker := time.NewTicker(5 * time.Second)
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
			clusterConfig, err = r.copyRunningConfigToCR()
			if err != nil {
				r.log.Errorf("failed to save cluster-config  %v\n", err)
				return err
			}
		} else {
			r.log.Errorf("error getting cluster-config: %v", err)
			return err
		}
	}
	// watch the clusterConfig resource for changes
	if clusterConfig.ResourceVersion != r.lastReconciledConfigVersion {
		r.log.Debugf("detected change in cluster-config custom resource: previous resourceVersion: %s, new resourceVersion: %s", r.lastReconciledConfigVersion, clusterConfig.ResourceVersion)
		err = r.ComponentManager.Reconcile(clusterConfig)
		if err != nil {
			return err
		}
		r.lastReconciledConfigVersion = clusterConfig.ResourceVersion
	}
	r.log.Debugf("reconciling cluster-config done")
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

func (r *ClusterConfigReconciler) copyRunningConfigToCR() (*v1beta1.ClusterConfig, error) {
	clusterWideConfig := config.ClusterConfigMinusNodeConfig(r.YamlConfig)
	clusterConfig, err := r.configClient.Create(context.Background(), clusterWideConfig, cOpts)
	if err != nil {
		return nil, err
	}
	if !r.leaderElector.IsLeader() {
		r.log.Debug("I am not the leader, not writing cluster configuration")
		return clusterConfig, nil
	}

	r.log.Info("successfully wrote cluster-config to API")
	return clusterConfig, nil
}

func (r *ClusterConfigReconciler) writeCRD() error {
	crd, err := static.AssetDir("manifests/v1beta1/CustomResourceDefinition")
	if err != nil {
		r.log.Errorf("error retrieving api-config manifests: %s. will retry", err.Error())
	}
	for _, filename := range crd {
		content, err := static.Asset(fmt.Sprintf("manifests/v1beta1/CustomResourceDefinition/%s", filename))
		if err != nil {
			return fmt.Errorf("failed to fetch crd `%s`: %v", filename, err)
		}
		err = r.saver.Save(filename, content)
		if err != nil {
			return fmt.Errorf("error writing api-config CRD, will NOT retry: %v", err)
		}
	}
	return nil
}
