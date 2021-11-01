package controller

import (
	"context"
	"fmt"
	"os"
	"time"

	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/static"

	"github.com/k0sproject/k0s/pkg/component"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	cfgClient "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/clientset/typed/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"

	"github.com/k0sproject/k0s/pkg/component/controller/clusterconfig"
)

var (
	resourceType = v1.TypeMeta{APIVersion: "k0s.k0sproject.io/v1beta1", Kind: "clusterconfigs"}
	cOpts        = v1.CreateOptions{TypeMeta: resourceType}
	getOpts      = v1.GetOptions{TypeMeta: resourceType}
)

// ClusterConfigReconciler reconciles a ClusterConfig object
type ClusterConfigReconciler struct {
	YamlConfig        *v1beta1.ClusterConfig
	ComponentManager  *component.Manager
	KubeClientFactory kubeutil.ClientFactoryInterface

	configClient  cfgClient.ClusterConfigInterface
	kubeConfig    string
	leaderElector LeaderElector
	log           *logrus.Entry
	saver         manifestsSaver
	configSource  clusterconfig.ConfigSource
}

// NewClusterConfigReconciler creates a new clusterConfig reconciler
func NewClusterConfigReconciler(cfgFile string, leaderElector LeaderElector, k0sVars constant.CfgVars, mgr *component.Manager, s manifestsSaver, kubeClientFactory kubeutil.ClientFactoryInterface, configSource clusterconfig.ConfigSource) (*ClusterConfigReconciler, error) {
	cfg, err := v1beta1.GetYamlFromFile(cfgFile, k0sVars)
	if err != nil {
		return nil, err
	}

	configClient, err := kubeClientFactory.GetConfigClient()
	if err != nil {
		return nil, err
	}

	return &ClusterConfigReconciler{
		ComponentManager:  mgr,
		YamlConfig:        cfg,
		KubeClientFactory: kubeClientFactory,
		kubeConfig:        k0sVars.AdminKubeConfigPath,
		leaderElector:     leaderElector,
		log:               logrus.WithFields(logrus.Fields{"component": "clusterConfig-reconciler"}),
		saver:             s,
		configSource:      configSource,
		configClient:      configClient,
	}, nil
}

func (r *ClusterConfigReconciler) Init() error {
	// If we do not need to store the config in API we do not need the CRDs either
	if !r.configSource.NeedToStoreInitialConfig() {
		return nil
	}
	err := r.writeCRD()
	if err != nil {
		return fmt.Errorf("failed to write api-config CRD to API: %v", err)
	}
	return nil
}

func (r *ClusterConfigReconciler) Run(ctx context.Context) error {
	if r.configSource.NeedToStoreInitialConfig() {
		// We need to wait until we either succees getting the object or creating it
		err := wait.Poll(1*time.Second, 20*time.Second, func() (done bool, err error) {
			timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			// Create the config object if it does not exist already
			_, e := r.configClient.Get(timeoutCtx, constant.ClusterConfigObjectName, getOpts)
			if e != nil {
				if errors.IsNotFound(e) {
					// ClusterConfig CR cannot be found, which means we can create it
					r.log.Debugf("didn't find cluster-config object: %v", err)
					_, e = r.copyRunningConfigToCR(ctx)
					if e != nil {
						r.log.Errorf("failed to save cluster-config  %v\n", err)
						return false, nil
					}
				} else {
					r.log.Errorf("error getting cluster-config: %v", err)
					return false, nil
				}
			}
			return true, nil
		})
		if err != nil {
			return fmt.Errorf("not able to get or create the cluster config: %v", err)
		}
	}

	go func() {
		r.log.Debug("start listening changes from config source")
		for {
			select {
			case cfg, ok := <-r.configSource.ResultChan():
				if !ok {
					// Recv channel close, we can stop now
					r.log.Debug("config source closed channel")
					return
				}
				errors := cfg.Validate()
				var err error
				if len(errors) > 0 {
					err = fmt.Errorf("failed to validate config: %v", errors)
				} else {
					err = r.ComponentManager.Reconcile(ctx, cfg)
				}
				r.reportStatus(cfg, err)
				if err != nil {
					r.log.Errorf("cluster-config reconcile failed: %s", err.Error())
				}
				r.log.Debugf("reconciling cluster-config done")
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// Stop stops
func (r *ClusterConfigReconciler) Stop() error {
	// Nothing really to stop, the main ConfigSource "watch" channel go-routine is stopped
	// via the main Context's Done channel in the Run function
	return nil
}

func (r *ClusterConfigReconciler) Healthy() error {
	return nil
}

func (r *ClusterConfigReconciler) reportStatus(config *v1beta1.ClusterConfig, reconcileError error) {
	hostname, err := os.Hostname()
	if err != nil {
		r.log.Error("failed to get hostname:", err)
		hostname = ""
	}
	// TODO We need to design proper status field(s) to the cluster cfg object, now just send event
	client, err := r.KubeClientFactory.GetClient()
	if err != nil {
		r.log.Error("failed to get kube client:", err)
	}
	e := &corev1.Event{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "k0s.",
		},
		EventTime:      v1.NowMicro(),
		FirstTimestamp: v1.Now(),
		LastTimestamp:  v1.Now(),
		InvolvedObject: corev1.ObjectReference{
			Kind:            v1beta1.ClusterConfigKind,
			Namespace:       config.Namespace,
			Name:            config.Name,
			UID:             config.UID,
			APIVersion:      v1beta1.ClusterConfigAPIVersion,
			ResourceVersion: config.ResourceVersion,
		},
		Action:              "ConfigReconciling",
		ReportingController: "k0s-controller",
		ReportingInstance:   hostname,
	}
	if reconcileError != nil {
		e.Reason = "FailedReconciling"
		e.Message = reconcileError.Error()
		e.Type = corev1.EventTypeWarning
	} else {
		e.Reason = "SuccessfulReconcile"
		e.Message = "Succesfully reconciler cluster config"
		e.Type = corev1.EventTypeNormal
	}
	_, err = client.CoreV1().Events(constant.ClusterConfigNamespace).Create(context.TODO(), e, v1.CreateOptions{})
	if err != nil {
		r.log.Error("failed to create event for config reconcile:", err)
	}
}

func (r *ClusterConfigReconciler) copyRunningConfigToCR(baseCtx context.Context) (*v1beta1.ClusterConfig, error) {
	ctx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
	defer cancel()
	clusterWideConfig := r.YamlConfig.GetClusterWideConfig().StripDefaults().StripDefaults().CRValidator()
	clusterConfig, err := r.configClient.Create(ctx, clusterWideConfig, cOpts)
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
