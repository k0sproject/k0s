package applier

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/Mirantis/mke/pkg/constant"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/fsnotify.v1"
)

type Manager struct {
	applier Applier

	tickerDone  chan struct{}
	watcherDone chan struct{}
}

func (m *Manager) Run() error {
	log := logrus.WithField("component", "applier-manager")
	bundlePath := filepath.Join(constant.DataDir, "manifests")
	err := os.MkdirAll(bundlePath, 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create manifest bundle dir %s", bundlePath)
	}

	m.applier, err = NewApplier(bundlePath)

	if err := m.applier.Apply(); err != nil {
		log.Warnf("initial manifest sync failed: %s", err.Error())
	}

	// Make the done channels
	m.tickerDone = make(chan struct{}, 1)
	m.watcherDone = make(chan struct{}, 1)

	var changesDetected atomic.Value
	changesDetected.Store(false)

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
					// Not much we can do
					log.Warnf("failed to apply manifests: %s", err.Error())
				}
				changesDetected.Store(false)
			case <-m.tickerDone:
				log.Info("manifest ticker done")
				return
			}
		}
	}()

	go func() {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Errorf("failed to create fs watcher for %s: %s", bundlePath, err.Error())
			return
		}
		defer watcher.Close()

		watcher.Add(bundlePath)
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

func (m *Manager) Stop() error {

	close(m.tickerDone)
	close(m.watcherDone)
	return nil
}
