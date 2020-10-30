package telemetry

import (
	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/constant"
	kubeutil "github.com/Mirantis/mke/pkg/kubernetes"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"time"
)

// Component is a telemetry component for MKE component manager
type Component struct {
	ClusterConfig *config.ClusterConfig

	kubernetesClient kubernetes.Interface
	analyticsClient  analyticsClient

	log      *logrus.Entry
	stopCh   chan struct{}
	interval time.Duration
}

// Init set up for external service clients (segment, k8s api)
func (c *Component) Init() error {
	c.log = logrus.WithField("component", "telemetry")

	if segmentToken == "" {
		c.log.Info("no token, telemetry is disabled")
		return nil
	}

	c.interval = c.ClusterConfig.Telemetry.Interval
	c.stopCh = make(chan struct{})

	if err := c.initKubeClient(); err != nil {
		c.log.WithError(err).Error("can't init kube client")
		return err
	}

	c.log.Info("kube client has been init")
	c.analyticsClient = newSegmentClient(segmentToken)
	c.log.Info("segment client has been init")
	return nil
}

func (c *Component) initKubeClient() error {
	return retry.OnError(retry.DefaultRetry, func(err error) bool {
		return true
	}, c.retrieveKubeClient)
}

func (c *Component) retrieveKubeClient() error {
	client, err := kubeutil.Client(constant.AdminKubeconfigConfigPath)
	if err != nil {
		return err
	}
	c.kubernetesClient = client
	return nil
}

// Run runs work cycle
func (c *Component) Run() error {
	if segmentToken == "" {
		c.log.Info("no token, telemetry is disabled")
		return nil
	}
	go c.run()
	return nil
}

// Run does nothing
func (c *Component) Stop() error {
	if segmentToken == "" {
		c.log.Info("no token, telemetry is disabled")
		return nil
	}
	close(c.stopCh)
	if c.analyticsClient != nil {
		_ = c.analyticsClient.Close()
	}
	return nil
}

// Healthy checks health
func (c *Component) Healthy() error {
	return nil
}

func (c Component) run() {
	ticker := time.NewTicker(c.interval)
	for {
		select {
		case <-ticker.C:
			c.sendTelemetry()
		case <-c.stopCh:
			return
		}
	}
}
