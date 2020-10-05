package applier

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/mod/sumdb/dirhash"
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
	watcher.Add(path)
	applier := NewApplier(path)
	log := logrus.WithField("component", "applier-"+applier.Name)

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

	// to make first tick to sync everything and retry until it succeeds
	var hash atomic.Value
	hash.Store("")

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				files, err := filepath.Glob(path.Join(s.Path, "*.yaml"))
				if err != nil {
					s.log.Warnf("failed to list stack files: %s", err.Error())
					continue
				}

				currentHash, err := dirhash.Hash1(files, func(file string) (io.ReadCloser, error) {
					return os.Open(file)
				})
				if err != nil {
					s.log.Warnf("error calculating checksum for stack: %s", err.Error())
					continue
				}

				previousHash := hash.Load().(string)

				// s.log.Debugf("current hash: %s ; previous: %s", currentHash, previousHash)

				if previousHash != currentHash {
					// Do actual apply
					if err := s.applier.Apply(); err != nil {
						s.log.Warnf("failed to apply stack: %s", err.Error())
					} else {
						s.log.Infof("successfully applied stack %s", s.Path)
						// Only set if the apply succeeds, will make it retry on every tick in case of failures
						hash.Store(currentHash)
					}
				}
			case <-s.done:
				s.log.Info("manifest ticker done")
				return
			}
		}
	}()

	// go func() {
	// 	for {
	// 		select {
	// 		// watch for events
	// 		case event, ok := <-s.fsWatcher.Events:
	// 			if !ok {
	// 				return
	// 			}
	// 			s.log.Debugf("manifest change (%s) %s", event.Op.String(), event.Name)
	// 			changesDetected.Store(true)
	// 		// watch for errors
	// 		case err, ok := <-s.fsWatcher.Errors:
	// 			if !ok {
	// 				return
	// 			}
	// 			s.log.Warnf("watch error: %s", err.Error())
	// 		}
	// 	}
	// }()

	return nil
}

// Stop stops the stack applier and removes the stack
func (s *StackApplier) Stop() error {
	s.log.Infof("stopping and deleting stack %s", s.Path)
	s.done <- true
	close(s.done)

	return nil
}

// DeleteStack deletes the associated stack
func (s *StackApplier) DeleteStack() error {
	return s.applier.Delete()
	// return retry.Do(func() error {
	// 	err := s.applier.Delete()
	// 	if err != nil {
	// 		s.log.Warnf("error deleting stack %s, will retry few times: %s", s.applier.Name, err.Error())
	// 	}
	// 	return err
	// }, retry.Attempts(3))
}
