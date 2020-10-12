package applier

import (
	"context"
	"time"

	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/debounce"
	kubeutil "github.com/Mirantis/mke/pkg/kubernetes"
	"github.com/Mirantis/mke/pkg/leaderelection"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/fsnotify.v1"
	"k8s.io/client-go/kubernetes"
)

// Manager is the Component interface wrapper for Applier
type Manager struct {
	client               kubernetes.Interface
	applier              Applier
	bundlePath           string
	cancelWatcher        context.CancelFunc
	cancelLeaderElection context.CancelFunc
	log                  *logrus.Entry
}

// Init initializes the Manager
func (m *Manager) Init() error {
	err := util.InitDirectory(constant.ManifestsDir, constant.ManifestsDirMode)
	if err != nil {
		return errors.Wrapf(err, "failed to create manifest bundle dir %s", constant.ManifestsDir)
	}
	m.log = logrus.WithField("component", "applier-manager")

	m.applier, err = NewApplier(constant.ManifestsDir)
	return err
}

func (m *Manager) retrieveKubeClient() error {
	client, err := kubeutil.Client(constant.AdminKubeconfigConfigPath)
	if err != nil {
		return err
	}

	m.client = client

	return nil
}

// Run runs the Manager
func (m *Manager) Run() error {
	log := m.log

	for m.client == nil {
		log.Debug("retrieving kube client config")
		_ = m.retrieveKubeClient()
		time.Sleep(time.Second)
	}

	leasePool, err := leaderelection.NewLeasePool(m.client, "mke-manifest-applier", leaderelection.WithLogger(log))

	if err != nil {
		return err
	}

	electionEvents := &leaderelection.LeaseEvents{
		AcquiredLease: make(chan struct{}),
		LostLease:     make(chan struct{}),
	}

	go m.watchLeaseEvents(electionEvents)
	go func() {
		_, cancel, _ := leasePool.Watch(leaderelection.WithOutputChannels(electionEvents))
		m.cancelLeaderElection = cancel
	}()

	return nil
}

// Stop stops the Manager
func (m *Manager) Stop() error {
	m.cancelLeaderElection()
	return nil
}

func (m *Manager) watchLeaseEvents(events *leaderelection.LeaseEvents) {
	log := m.log

	for {
		select {
		case <-events.AcquiredLease:
			log.Info("acquired leader lease")
			ctx, cancel := context.WithCancel(context.Background())
			m.cancelWatcher = cancel
			go m.runFSWatcher(ctx)
		case <-events.LostLease:
			log.Info("lost leader lease")
			if m.cancelWatcher != nil {
				m.cancelWatcher()
			}
		}
	}
}

func (m *Manager) runFSWatcher(ctx context.Context) {
	log := logrus.WithField("component", "applier-manager")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Errorf("failed to create fs watcher for %s: %s", constant.ManifestsDir, err.Error())
		return
	}
	defer watcher.Close()

	// Apply once after becoming leader, to make everything sync even if there's no FS events
	log.Debug("Running initial apply after we've become the leader")
	if err := m.applier.Apply(); err != nil {
		log.Warnf("failed to apply manifests: %s", err.Error())
	}

	debouncer := debounce.New(5*time.Second, watcher.Events, func(arg fsnotify.Event) {
		log.Debug("debouncer triggering, applying...")
		if err := m.applier.Apply(); err != nil {
			log.Warnf("failed to apply manifests: %s", err.Error())
		}
	})
	defer debouncer.Stop()
	go debouncer.Start()

	err = watcher.Add(constant.ManifestsDir)
	if err != nil {
		log.Warnf("Failed to start watcher: %s", err.Error())
	}
	for {
		select {
		case err := <-watcher.Errors:
			log.Warnf("watch error: %s", err.Error())
		case <-ctx.Done():
			log.Info("manifest watcher done")
			return
		}
	}
}
