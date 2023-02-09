/*
Copyright 2020 k0s authors

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
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/k0sproject/k0s/pkg/debounce"
	"github.com/k0sproject/k0s/pkg/kubernetes"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// StackApplier applies a stack whenever the files on disk change.
type StackApplier struct {
	log  logrus.FieldLogger
	path string

	doApply, doDelete func(context.Context) error
}

// NewStackApplier crates new stack applier to manage a stack
func NewStackApplier(path string, kubeClientFactory kubernetes.ClientFactoryInterface) *StackApplier {
	var mu sync.Mutex
	applier := NewApplier(path, kubeClientFactory)

	return &StackApplier{
		log:  logrus.WithField("component", "applier-"+applier.Name),
		path: path,

		doApply: func(ctx context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			return applier.Apply(ctx)
		},

		doDelete: func(ctx context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			return applier.Delete(ctx)
		},
	}
}

// Run executes the initial apply and watches the stack for updates.
func (s *StackApplier) Run(ctx context.Context) error {
	if ctx.Err() != nil {
		return nil // The context is already done.
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	debounceCtx, cancelDebouncer := context.WithCancel(ctx)
	defer cancelDebouncer()

	debouncer := debounce.Debouncer[fsnotify.Event]{
		Input:    watcher.Events,
		Timeout:  1 * time.Second,
		Filter:   s.triggersApply,
		Callback: func(fsnotify.Event) { s.apply(debounceCtx) },
	}

	// Send an artificial event to ensure that an initial apply will happen.
	go func() { watcher.Events <- fsnotify.Event{} }()

	// Consume and log any errors.
	go func() {
		for {
			err, ok := <-watcher.Errors
			if !ok {
				return
			}
			s.log.WithError(err).Error("Error while watching stack")
		}
	}()

	err = watcher.Add(s.path)
	if err != nil {
		return fmt.Errorf("failed to watch %q: %w", s.path, err)
	}

	_ = debouncer.Run(debounceCtx)
	return nil
}

func (*StackApplier) triggersApply(event fsnotify.Event) bool {
	// Always let the initial apply happen
	if event == (fsnotify.Event{}) {
		return true
	}

	// Only consider events on manifest files
	if match, _ := filepath.Match(manifestFilePattern, filepath.Base(event.Name)); !match {
		return false
	}

	return true
}

func (s *StackApplier) apply(ctx context.Context) {
	s.log.Info("Applying manifests")

	err := retry.Do(
		func() error { return s.doApply(ctx) },
		retry.OnRetry(func(attempt uint, err error) {
			s.log.WithError(err).Warnf("Failed to apply manifests in attempt #%d, retrying after backoff", attempt+1)
		}),
		retry.Context(ctx),
		retry.LastErrorOnly(true),
	)

	if err != nil {
		s.log.WithError(err).Error("Failed to apply manifests")
	}
}

// DeleteStack deletes the associated stack
func (s *StackApplier) DeleteStack(ctx context.Context) error {
	return s.doDelete(ctx)
}
