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
	"k8s.io/client-go/kubernetes"
)

// The LeasePool represents a single lease accessed by multiple clients (considered part of the "pool")
//
// Deprecated: Use [Client] instead.
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
	name        string
	identity    string
	namespace   string
	retryPeriod time.Duration
	log         logrus.FieldLogger
}

// A LeaseOpt is a function that modifies a LeaseConfiguration
type LeaseOpt func(config LeaseConfiguration) LeaseConfiguration

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
//
// Deprecated: Use [NewClient] instead.
func NewLeasePool(client kubernetes.Interface, name, identity string, opts ...LeaseOpt) (*LeasePool, error) {

	leaseConfig := LeaseConfiguration{
		log:         logrus.StandardLogger(),
		retryPeriod: 5 * time.Second,
		namespace:   "kube-node-lease",
		name:        name,
		identity:    identity,
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

	client, err := NewClient(&LeaseConfig{
		Namespace: p.config.namespace,
		Name:      p.config.name,
		Identity:  p.config.identity,
		Client:    p.client.CoordinationV1(),
		internalConfig: internalConfig{
			retryPeriod: p.config.retryPeriod,
		},
	})

	if err != nil {
		return nil, err
	}

	go client.Run(ctx, func(status Status) {
		if status == StatusLeading {
			p.config.log.Info("Acquired leader lease")
			p.events.AcquiredLease <- struct{}{}
		} else {
			p.config.log.Info("Lost leader lease")
			p.events.LostLease <- struct{}{}
		}
	})

	return p.events, nil
}
