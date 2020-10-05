package applier

import (
	"path"
	"path/filepath"

	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/fsnotify.v1"
)

// Manager is the Component interface wrapper for Applier
type Manager struct {
	bundlePath  string
	watcherDone chan struct{}

	stackAppliers map[string]*StackApplier

	log *logrus.Entry

	watcher *fsnotify.Watcher
}

// Init initializes the Manager
func (m *Manager) Init() error {
	m.log = logrus.WithField("component", "applier-manager")
	m.stackAppliers = make(map[string]*StackApplier)

	m.bundlePath = filepath.Join(constant.DataDir, "manifests")
	err := util.InitDirectory(m.bundlePath, 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create manifest bundle dir %s", m.bundlePath)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		m.log.Errorf("failed to create fs watcher for %s: %s", m.bundlePath, err.Error())
		return err
	}
	watcher.Add(m.bundlePath)
	m.watcher = watcher

	return nil
}

// Run runs the Manager
func (m *Manager) Run() error {
	// Make the done channels
	m.watcherDone = make(chan struct{})

	dirs, err := util.GetAllDirs(m.bundlePath)
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		stackDir := path.Join(m.bundlePath, dir)
		sa, err := NewStackApplier(stackDir)
		if err != nil {
			return err
		}
		m.stackAppliers[stackDir] = sa
		err = sa.Start()
		if err != nil {
			m.log.Warnf("error creating applier for stack %s", dir)
		}
	}

	go func() {
		for {
			select {
			// watch for events
			case event, ok := <-m.watcher.Events:
				if !ok {
					return
				}
				switch event.Op {
				case fsnotify.Create:
					if util.IsDirectory(event.Name) {
						m.log.Infof("creating new applier for %s", event.Name)
						sa, err := NewStackApplier(event.Name)
						err = sa.Start()
						if err != nil {
							m.log.Warnf("error creating applier for stack %s", event.Name)
							sa.Stop()
						}
						m.stackAppliers[event.Name] = sa
					}
				case fsnotify.Remove:
					m.log.Infof("removing applier for %s", event.Name)
					sa, ok := m.stackAppliers[event.Name]
					if ok {
						err = sa.Stop()
						err = sa.DeleteStack()
						delete(m.stackAppliers, event.Name)
						if err != nil {
							m.log.Warnf("failed to stop and delete a stack applier %s: %s", event.Name, err.Error())
						}
					} else {
						m.log.Warnf("attempted to remove non-initialized stack applier for %s", event.Name)
					}
				}
			// watch for errors
			case err, ok := <-m.watcher.Errors:
				if !ok {
					return
				}
				m.log.Warnf("watch error: %s", err.Error())
			}
		}
	}()

	return nil
}

// Stop stops the Manager
func (m *Manager) Stop() error {
	m.watcher.Close()
	return nil
}
