package applier

import (
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/fsnotify.v1"
)

// Manager is the Component interface wrapper for Applier
type Manager struct {
	applier     Applier
	bundlePath  string
	tickerDone  chan struct{}
	watcherDone chan struct{}
}

// Init initializes the Manager
func (m *Manager) Init() error {
	m.bundlePath = filepath.Join(constant.DataDir, "manifests")
	err := util.InitDirectory(m.bundlePath, 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create manifest bundle dir %s", m.bundlePath)
	}

	m.applier, err = NewApplier(m.bundlePath)
	return err
}

// Run runs the Manager
func (m *Manager) Run() error {
	log := logrus.WithField("component", "applier-manager")

	// Make the done channels
	m.tickerDone = make(chan struct{})
	m.watcherDone = make(chan struct{})

	var changesDetected atomic.Value
	// to make first tick to sync everything and retry until it succeeds
	changesDetected.Store(true)

	go func() {
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
	}()

	go func() {
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
	}()

	return nil
}

// Stop stops the Manager
func (m *Manager) Stop() error {
	close(m.tickerDone)
	close(m.watcherDone)
	return nil
}
