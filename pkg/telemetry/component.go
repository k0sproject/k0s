package telemetry

import (
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"

	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// Component is a telemetry component for k0s component manager
type Component struct {
	ClusterConfig     *v1beta1.ClusterConfig
	K0sVars           constant.CfgVars
	Version           string
	KubeClientFactory kubeutil.ClientFactory

	kubernetesClient kubernetes.Interface
	analyticsClient  analyticsClient

	log    *logrus.Entry
	stopCh chan struct{}
}

var interval = time.Minute * 10

// Init set up for external service clients (segment, k8s api)
func (c *Component) Init() error {
	c.log = logrus.WithField("component", "telemetry")

	if segmentToken == "" {
		c.log.Info("no token, telemetry is disabled")
		return nil
	}

	c.stopCh = make(chan struct{})
	c.log.Info("kube client has been init")
	c.analyticsClient = newSegmentClient(segmentToken)
	c.log.Info("segment client has been init")
	return nil
}

func (c *Component) retrieveKubeClient(ch chan struct{}) {
	client, err := c.KubeClientFactory.GetClient()
	if err != nil {
		c.log.WithError(err).Warning("can't init kube client")
		return
	}
	c.kubernetesClient = client
	close(ch)
}

// Run runs work cycle
func (c *Component) Run() error {
	if segmentToken == "" {
		c.log.Info("no token, telemetry is disabled")
		return nil
	}
	initedCh := make(chan struct{})
	wait.Until(func() {
		c.retrieveKubeClient(initedCh)
	}, time.Second, initedCh)
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
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			c.sendTelemetry()
		case <-c.stopCh:
			return
		}
	}
}
