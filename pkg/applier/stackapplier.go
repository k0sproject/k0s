package applier

import (
	"time"

	"k8s.io/client-go/util/retry"

	"github.com/Mirantis/mke/pkg/debounce"
	"github.com/sirupsen/logrus"
	"gopkg.in/fsnotify.v1"
)

// StackApplier handles each directory as a Stack and watches for changes
type StackApplier struct {
	Path string

	fsWatcher *fsnotify.Watcher
	applier   Applier
	log       *logrus.Entry
	done      chan bool
}

// NewStackApplier crates new stack applier to manage a stack
func NewStackApplier(path string) (*StackApplier, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	err = watcher.Add(path)
	if err != nil {
		return nil, err
	}
	applier := NewApplier(path)
	log := logrus.WithField("component", "applier-"+applier.Name)
	log.WithField("path", path).Debug("created stack applier")

	return &StackApplier{
		Path:      path,
		fsWatcher: watcher,
		applier:   applier,
		log:       log,
		done:      make(chan bool, 1),
	}, nil
}

// Start both the initial apply and also the watch for a single stack
func (s *StackApplier) Start() error {
	debouncer := debounce.New(5*time.Second, s.fsWatcher.Events, func(arg fsnotify.Event) {
		s.log.Debug("debouncer triggering, applying...")
		err := retry.OnError(retry.DefaultRetry, func(err error) bool {
			return true
		}, s.applier.Apply)
		if err != nil {
			s.log.Warnf("failed to apply manifests: %s", err.Error())
		}
	})
	defer debouncer.Stop()
	go debouncer.Start()

	// apply all changes on start
	s.fsWatcher.Events <- fsnotify.Event{}

	<-s.done

	return nil
}

// Stop stops the stack applier and removes the stack
func (s *StackApplier) Stop() error {
	s.log.WithField("stack", s.Path).Info("stopping and deleting stack")
	s.done <- true
	close(s.done)

	return nil
}

// DeleteStack deletes the associated stack
func (s *StackApplier) DeleteStack() error {
	return s.applier.Delete()
}

// Health-check interface
func (s *StackApplier) Healthy() error { return nil }
