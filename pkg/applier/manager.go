package applier

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/Mirantis/mke/pkg/constant"
	kubernetes2 "github.com/Mirantis/mke/pkg/kubernetes"
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
	tickerDone           chan struct{}
	watcherDone          chan struct{}
	cancelLeaderElection context.CancelFunc
	log                  *logrus.Entry
}

// Init initializes the Manager
func (m *Manager) Init() error {
	m.bundlePath = filepath.Join(constant.DataDir, "manifests")
	err := util.InitDirectory(m.bundlePath, 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create manifest bundle dir %s", m.bundlePath)
	}
	m.log = logrus.WithField("component", "applier-manager")

	m.applier, err = NewApplier(m.bundlePath)
	return err
}

func (m *Manager) retrieveKubeClient() error {
	client, err := kubernetes2.Client(constant.AdminKubeconfigConfigPath)
	if err != nil {
		return err
	}

	m.client = client

	return nil
}

// Run runs the Manager
func (m *Manager) Run() error {
	log := m.log

	// Make the done channels
	m.tickerDone = make(chan struct{})
	m.watcherDone = make(chan struct{})

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
	close(m.tickerDone)
	close(m.watcherDone)
	m.cancelLeaderElection()
	return nil
}

func (m *Manager) watchLeaseEvents(events *leaderelection.LeaseEvents) {
	log := m.log

	for {
		select {
		case <-events.AcquiredLease:
			log.Info("acquired leader lease")
			changesDetected := &atomic.Value{}
			// to make first tick to sync everything and retry until it succeeds
			changesDetected.Store(true)
			go m.runFSWatcher(changesDetected)
			go m.runApplier(changesDetected)
		case <-events.LostLease:
			log.Info("lost leader lease")
			m.tickerDone <- struct{}{}
			m.watcherDone <- struct{}{}
		}
	}
}

func (m *Manager) runApplier(changesDetected *atomic.Value) {
	log := m.log
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			changes := changesDetected.Load().(bool)
			if !changes {
				continue // Wait for next check
			}
			// Do actual apply
			if err := m.applier.Apply(); err != nil {
				log.Warnf("failed to apply manifests: %s", err.Error())
			} else {
				// Only set if the apply succeeds, will make it retry on every tick in case of failures
				changesDetected.Store(false)
			}
		case <-m.tickerDone:
			log.Info("manifest ticker done")
			return
		}
	}
}

func (m *Manager) runFSWatcher(changesDetected *atomic.Value) {
	log := m.log
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Errorf("failed to create fs watcher for %s: %s", m.bundlePath, err.Error())
		return
	}
	defer watcher.Close()

	watcher.Add(m.bundlePath)
	for {
		select {
		// watch for events
		case event := <-watcher.Events:
			log.Debugf("manifest change (%s) %s", event.Op.String(), event.Name)
			changesDetected.Store(true)
			// watch for errors
		case err := <-watcher.Errors:
			log.Warnf("watch error: %s", err.Error())
		case <-m.watcherDone:
			log.Info("manifest watcher done")
			return
		}
	}
}
