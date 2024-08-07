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

package leaderelection

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// The LeasePool represents a single lease accessed by multiple clients (considered part of the "pool")
type LeasePool struct {
	events *LeaseEvents
	config LeaseConfiguration
	client kubernetes.Interface
}

// LeaseEvents contains channels to inform the consumer when a lease is acquired and lost
type LeaseEvents struct {
	AcquiredLease chan struct{}
	LostLease     chan struct{}
}

// The LeaseConfiguration allows passing through various options to customise the lease.
type LeaseConfiguration struct {
	name          string
	identity      string
	namespace     string
	duration      time.Duration
	renewDeadline time.Duration
	retryPeriod   time.Duration
	log           logrus.FieldLogger
}

// A LeaseOpt is a function that modifies a LeaseConfiguration
type LeaseOpt func(config LeaseConfiguration) LeaseConfiguration

// WithDuration sets the duration of the lease (for new leases)
func WithDuration(duration time.Duration) LeaseOpt {
	return func(config LeaseConfiguration) LeaseConfiguration {
		config.duration = duration
		return config
	}
}

// WithRenewDeadline sets the renew deadline of the lease
func WithRenewDeadline(deadline time.Duration) LeaseOpt {
	return func(config LeaseConfiguration) LeaseConfiguration {
		config.renewDeadline = deadline
		return config
	}
}

// WithRetryPeriod specifies the retry period of the lease
func WithRetryPeriod(retryPeriod time.Duration) LeaseOpt {
	return func(config LeaseConfiguration) LeaseConfiguration {
		config.retryPeriod = retryPeriod
		return config
	}
}

// WithLogger allows the consumer to pass a different logrus entry with additional context
func WithLogger(logger logrus.FieldLogger) LeaseOpt {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return func(config LeaseConfiguration) LeaseConfiguration {
		config.log = logger
		return config
	}
}

// WithNamespace specifies which namespace the lease should be created in, defaults to kube-node-lease
func WithNamespace(namespace string) LeaseOpt {
	return func(config LeaseConfiguration) LeaseConfiguration {
		config.namespace = namespace
		return config
	}
}

// NewLeasePool creates a new LeasePool struct to interact with a lease
func NewLeasePool(client kubernetes.Interface, name, identity string, opts ...LeaseOpt) (*LeasePool, error) {

	leaseConfig := LeaseConfiguration{
		log:           logrus.StandardLogger(),
		duration:      60 * time.Second,
		renewDeadline: 15 * time.Second,
		retryPeriod:   5 * time.Second,
		namespace:     "kube-node-lease",
		name:          name,
		identity:      identity,
	}

	for _, opt := range opts {
		leaseConfig = opt(leaseConfig)
	}

	return &LeasePool{
		client: client,
		events: nil,
		config: leaseConfig,
	}, nil
}

type watchOptions struct {
	channels *LeaseEvents
}

// WatchOpt is a callback that alters the watchOptions configuration
type WatchOpt func(options watchOptions) watchOptions

// WithOutputChannels allows us to pass through channels with
// a size greater than 0, which makes testing a lot easier.
func WithOutputChannels(channels *LeaseEvents) WatchOpt {
	return func(options watchOptions) watchOptions {
		options.channels = channels
		return options
	}
}

// Watch is the primary function of LeasePool, and starts the leader election process
func (p *LeasePool) Watch(ctx context.Context, opts ...WatchOpt) (*LeaseEvents, error) {
	if p.events != nil {
		return p.events, nil
	}

	watchOptions := watchOptions{
		channels: &LeaseEvents{
			AcquiredLease: make(chan struct{}),
			LostLease:     make(chan struct{}),
		},
	}
	for _, opt := range opts {
		watchOptions = opt(watchOptions)
	}

	p.events = watchOptions.channels

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      p.config.name,
			Namespace: p.config.namespace,
		},
		Client: p.client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: p.config.identity,
		},
	}
	lec := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   p.config.duration,
		RenewDeadline:   p.config.renewDeadline,
		RetryPeriod:     p.config.retryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				p.config.log.Info("Acquired leader lease")
				p.events.AcquiredLease <- struct{}{}
			},
			OnStoppedLeading: func() {
				p.config.log.Info("Lost leader lease")
				p.events.LostLease <- struct{}{}
			},
			OnNewLeader: nil,
		},
	}
	le, err := leaderelection.NewLeaderElector(lec)
	if err != nil {
		return nil, err
	}
	if lec.WatchDog != nil {
		lec.WatchDog.SetLeaderElection(le)
	}

	go func() {
		for ctx.Err() == nil {
			le.Run(ctx)
		}
	}()

	return p.events, nil
}
