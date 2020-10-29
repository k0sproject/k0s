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

// Init does nothing
func (p *Component) Init() error {
	p.log = logrus.WithField("component", "telemetry")

	if segmentToken == "" {
		p.log.Info("no token, is telemetry disabled")
		return nil
	}

	p.interval = p.ClusterConfig.Telemetry.Interval
	p.stopCh = make(chan struct{})

	if err := p.initKubeClient(); err != nil {
		p.log.WithError(err).Error("can't init kube client")
		return err
	}

	p.log.Info("kube client has been init")
	p.analyticsClient = newSegmentClient(segmentToken)
	p.log.Info("segment client has been init")
	return nil
}

func (p *Component) initKubeClient() error {
	return retry.OnError(retry.DefaultRetry, func(err error) bool {
		return true
	}, p.retrieveKubeClient)
}

func (p *Component) retrieveKubeClient() error {
	client, err := kubeutil.Client(constant.AdminKubeconfigConfigPath)
	if err != nil {
		return err
	}
	p.kubernetesClient = client
	return nil
}

// Run runs work cycle
func (p *Component) Run() error {
	if segmentToken == "" {
		p.log.Info("no token, telemetry is disabled")
		return nil
	}
	go p.run()
	return nil
}

// Run does nothing
func (p *Component) Stop() error {
	if segmentToken == "" {
		p.log.Info("no token, telemetry is disabled")
		return nil
	}
	close(p.stopCh)
	if p.analyticsClient != nil {
		_ = p.analyticsClient.Close()
	}
	return nil
}

// Healthy checks health
func (p *Component) Healthy() error {
	return nil
}

func (p Component) run() {
	ticker := time.NewTicker(p.interval)
	for {
		select {
		case <-ticker.C:
			p.sendTelemetry()
		case <-p.stopCh:
			return
		}
	}
}
