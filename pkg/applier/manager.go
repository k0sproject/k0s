package applier

import (
	"context"
	"path"
	"time"

	"github.com/Mirantis/mke/pkg/constant"
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
	stacks               map[string]*StackApplier
}

// Init initializes the Manager
func (m *Manager) Init() error {
	err := util.InitDirectory(constant.ManifestsDir, constant.ManifestsDirMode)
	if err != nil {
		return errors.Wrapf(err, "failed to create manifest bundle dir %s", constant.ManifestsDir)
	}
	m.log = logrus.WithField("component", "applier-manager")
	m.stacks = make(map[string]*StackApplier)
	m.bundlePath = constant.ManifestsDir

	m.applier = NewApplier(constant.ManifestsDir)
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
			go func() {
				_ = m.runWatchers(ctx)
			}()
		case <-events.LostLease:
			log.Info("lost leader lease")
			if m.cancelWatcher != nil {
				m.cancelWatcher()
			}
		}
	}
}

func (m *Manager) runWatchers(ctx context.Context) error {
	log := logrus.WithField("component", "applier-manager")

	dirs, err := util.GetAllDirs(m.bundlePath)
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		if err := m.createStack(path.Join(m.bundlePath, dir)); err != nil {
			log.WithError(err).Error("failed to create stack")
			return err
		}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.WithError(err).Error("failed to create watcher")
		return err
	}
	defer watcher.Close()

	err = watcher.Add(m.bundlePath)
	if err != nil {
		log.Warnf("Failed to start watcher: %s", err.Error())
	}
	for {
		select {
		case err, ok := <-watcher.Errors:
			if !ok {
				return err
			}

			log.Warnf("watch error: %s", err.Error())
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			switch event.Op {
			case fsnotify.Create:
				if util.IsDirectory(event.Name) {
					if err := m.createStack(event.Name); err != nil {
						return err
					}
				}
			case fsnotify.Remove:
				_ = m.removeStack(event.Name)
			}
		case <-ctx.Done():
			log.Info("manifest watcher done")
			return nil
		}
	}
}

func (m *Manager) createStack(name string) error {
	// safeguard in case the fswatcher would trigger an event for an already existing watcher
	if _, ok := m.stacks[name]; ok {
		return nil
	}
	m.log.WithField("stack", name).Info("registering new stack")
	sa, err := NewStackApplier(name)
	if err != nil {
		return err
	}

	go func() {
		_ = sa.Start()
	}()

	m.stacks[name] = sa
	return nil
}

func (m *Manager) removeStack(name string) error {
	sa, ok := m.stacks[name]

	if !ok {
		m.log.
			WithField("path", name).
			Debug("attempted to remove non-existent stack, probably not a directory")
		return nil
	}
	err := sa.Stop()
	if err != nil {
		m.log.WithField("stack", name).WithError(err).Warn("failed to stop stack applier")
		return err
	}
	err = sa.DeleteStack()
	if err != nil {
		m.log.WithField("stack", name).WithError(err).Warn("failed to stop and delete a stack applier")
		return err
	}

	delete(m.stacks, name)

	return nil
}
