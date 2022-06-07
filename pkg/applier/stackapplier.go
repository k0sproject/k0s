/*
Copyright 2022 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package applier

import (
	"context"
	"time"

	"k8s.io/client-go/util/retry"

	"github.com/k0sproject/k0s/pkg/debounce"
	"github.com/k0sproject/k0s/pkg/kubernetes"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// StackApplier handles each directory as a Stack and watches for changes
type StackApplier struct {
	Path string

	fsWatcher *fsnotify.Watcher
	applier   Applier
	log       *logrus.Entry

	ctx    context.Context
	cancel context.CancelFunc
}

// NewStackApplier crates new stack applier to manage a stack
func NewStackApplier(ctx context.Context, path string, kubeClientFactory kubernetes.ClientFactoryInterface) (*StackApplier, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	err = watcher.Add(path)
	if err != nil {
		return nil, err
	}
	applier := NewApplier(path, kubeClientFactory)
	log := logrus.WithField("component", "applier-"+applier.Name)
	log.WithField("path", path).Debug("created stack applier")

	sa := &StackApplier{
		Path:      path,
		fsWatcher: watcher,
		applier:   applier,
		log:       log,
	}

	sa.ctx, sa.cancel = context.WithCancel(ctx)

	return sa, nil
}

// Start both the initial apply and also the watch for a single stack
func (s *StackApplier) Start() error {
	debouncer := debounce.Debouncer[fsnotify.Event]{
		Input:   s.fsWatcher.Events,
		Timeout: 1 * time.Second,
		Filter:  s.triggersApply,
		Callback: func(fsnotify.Event) {
			s.log.Debug("Debouncer triggering, applying...")
			err := retry.OnError(retry.DefaultRetry, func(err error) bool {
				return true
			}, func() error {
				return s.applier.Apply(s.ctx)
			})
			if err != nil {
				s.log.WithError(err).Error("Failed to apply manifests")
			}
		},
	}

	// Send an artificial event to ensure that an initial apply will happen.
	go func() { s.fsWatcher.Events <- fsnotify.Event{} }()

	_ = debouncer.Run(s.ctx)
	return nil
}

// Stop stops the stack applier.
func (s *StackApplier) Stop() {
	s.log.WithField("stack", s.Path).Info("Stopping stack")
	s.cancel()
}

// DeleteStack deletes the associated stack
func (s *StackApplier) DeleteStack(ctx context.Context) error {
	return s.applier.Delete(ctx)
}

// Health-check interface
func (s *StackApplier) Healthy() error { return nil }

func (*StackApplier) triggersApply(event fsnotify.Event) bool {
	// always let the initial apply happen
	if event == (fsnotify.Event{}) {
		return true
	}

	// ignore chmods (3845479a0)
	if event.Op == fsnotify.Chmod {
		return false
	}

	return true
}
