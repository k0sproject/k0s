/*
Copyright 2024 k0s authors

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

package leaderelection

import (
	"cmp"
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/k0sproject/k0s/pkg/k0scontext"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coordinationv1client "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// Indicates if a leader election client has taken the lead or not.
type Status bool

const (
	StatusPending Status = false
	StatusLeading Status = true
)

// Returns a string representation suitable for logging.
func (s Status) String() string {
	if s {
		return "leading"
	}
	return "pending"
}

// Configures a leader election client backed by a coordination/v1 Lease resource.
type LeaseConfig struct {
	// The Lease resource that's backing the leader election client.
	Namespace, Name string

	// The unique name identifying this client across all participants in the election.
	Identity string

	// The Kubernetes client used to manage the Lease resource.
	Client coordinationv1client.LeasesGetter

	internalConfig
}

// Implements [Config].
func (c *LeaseConfig) buildLock() (resourcelock.Interface, error) {
	if c.Namespace == "" {
		return nil, fmt.Errorf("namespace may not be empty")
	}
	if c.Name == "" {
		return nil, fmt.Errorf("name may not be empty")
	}
	if c.Client == nil {
		return nil, fmt.Errorf("client may not be nil")
	}

	return &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Namespace: c.Namespace,
			Name:      c.Name,
		},
		Client: c.Client,
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: c.Identity,
		},
	}, nil
}

// A client configuration to be used with [NewClient].
//
// See:
//   - [LeaseConfig]
type Config interface {
	buildLock() (resourcelock.Interface, error)
	internal() internalConfig
}

// Default durations for leader election.
// Not publicly configurable at the moment.
const (
	defaultLeaseDuration = 60 * time.Second
	defaultRenewDeadline = 15 * time.Second
	defaultRetryPeriod   = 5 * time.Second
)

// Creates a new leader election client with the provided configuration.
func NewClient(c Config) (*Client, error) {
	lock, err := c.buildLock()
	if err != nil {
		return nil, err
	}

	ic := c.internal()
	leaderElector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   defaultLeaseDuration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) { k0scontext.Value[onStartedLeadingFunc](ctx)() },
			OnStoppedLeading: func() { /* handled in runLeaderElectionRound */ },
		},
		RenewDeadline: cmp.Or(ic.renewDeadline, defaultRenewDeadline),
		RetryPeriod:   cmp.Or(ic.retryPeriod, defaultRetryPeriod),
	})
	if err != nil {
		return nil, err
	}

	return &Client{leaderElector}, nil
}

// Internal, non-exposed leader election configuration settings.
type internalConfig struct {
	renewDeadline, retryPeriod time.Duration
}

// Implements [Config].
func (c *internalConfig) internal() internalConfig { return *c }

// A leader election client.
type Client struct {
	leaderElector *leaderelection.LeaderElector
}

// Executes the leader election process. The changed callback is called whenever
// the status changes. It will be called sequentially, but not necessarily from
// the same goroutine. Run returns when ctx is done.
func (c *Client) Run(ctx context.Context, changed func(Status)) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			c.runLeaderElectionRound(ctx, changed)
		}
	}
}

// Performs a single round of leader election.
// It returns after the lead got lost or when ctx is done.
func (c *Client) runLeaderElectionRound(ctx context.Context, changed func(Status)) {
	// The Kubernetes leader elector calls the OnStartedLeading callback
	// concurrently in a goroutine. It doesn't wait for the callback to return
	// when it calls OnStoppedLeading. As a result, both callbacks can
	// interleave, and in pathological situations, OnStartedLeading can even be
	// called _after_ OnStoppedLeading. Take this behavior into account here,
	// and make sure that this round's callback is guaranteed not to be called
	// at the same time, and that it always observes the correct order of leader
	// election states (possibly none).

	var changedCalled atomic.Bool
	done := make(chan struct{})

	leadTaken := func() {
		defer close(done)
		// Ensure that the leader elector hasn't returned yet.
		if !changedCalled.Swap(true) {
			changed(StatusLeading)
		}
	}

	// This is an alternative implementation of the OnStoppedLeading callback.
	// The callback doesn't have access to a context, so it can't get a value
	// from it like it's done for OnStartedLeading. OnStoppedLeading is
	// implemented via a defer call in the leader elector's Run method. This
	// behavior can be implemented here as well.
	defer func() {
		// Wait for the StatusLeading callback to finish and call changed with
		// StatusPending only if changed was called before.
		if changedCalled.Swap(true) {
			<-done
			changed(StatusPending)
		}
	}()

	// Run the leader elector with the appropriate callback.
	c.leaderElector.Run(k0scontext.WithValue(ctx, onStartedLeadingFunc(leadTaken)))
}

// A helper type to pass the current leader election round's callback into the
// leader elector's OnStartedLeading callback function.
type onStartedLeadingFunc func()
