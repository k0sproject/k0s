package leaderelection

import (
	"context"
	"github.com/cloudflare/cfssl/log"
	"github.com/denisbrodbeck/machineid"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"time"
)

// the LeasePool represents a single lease accessed by multiple clients (considered part of the "pool")
type LeasePool struct {
	events *LeaseEvents
	config LeaseConfiguration
	client kubernetes.Interface
}

type LeaseEvents struct {
	AcquiredLease chan struct{}
	LostLease     chan struct{}
}

// the configuration allows passing through various options to customise
// the lease. We set sensible defaults for everything, but this makes
// it easier to reuse, extract, and test the leader election/lease.
type LeaseConfiguration struct {
	name          string
	identity      string
	namespace     string
	duration      time.Duration
	renewDeadline time.Duration
	retryPeriod   time.Duration
	log           *logrus.Entry
	ctx           context.Context
}

type LeaseOpt func(config LeaseConfiguration) LeaseConfiguration

func WithDuration(duration time.Duration) LeaseOpt {
	return func(config LeaseConfiguration) LeaseConfiguration {
		config.duration = duration
		return config
	}
}

func WithRenewDeadline(deadline time.Duration) LeaseOpt {
	return func(config LeaseConfiguration) LeaseConfiguration {
		config.renewDeadline = deadline
		return config
	}
}

func withRetryPeriod(retryPeriod time.Duration) LeaseOpt {
	return func(config LeaseConfiguration) LeaseConfiguration {
		config.retryPeriod = retryPeriod
		return config
	}
}

func WithLogger(logger *logrus.Entry) LeaseOpt {
	return func(config LeaseConfiguration) LeaseConfiguration {
		config.log = logger
		return config
	}
}

func WithContext(ctx context.Context) LeaseOpt {
	return func(config LeaseConfiguration) LeaseConfiguration {
		config.ctx = ctx
		return config
	}
}

func WithIdentity(identity string) LeaseOpt {
	return func(config LeaseConfiguration) LeaseConfiguration {
		config.identity = identity
		return config
	}
}

func WithNamespace(namespace string) LeaseOpt {
	return func(config LeaseConfiguration) LeaseConfiguration {
		config.namespace = namespace
		return config
	}
}

func NewLeasePool(client kubernetes.Interface, name string, opts ...LeaseOpt) (*LeasePool, error) {

	leaseConfig := LeaseConfiguration{
		log:           logrus.NewEntry(logrus.New()),
		duration:      60 * time.Second,
		renewDeadline: 15 * time.Second,
		retryPeriod:   5 * time.Second,
		ctx:           context.TODO(),
		namespace:     "kube-node-lease",
		name:          name,
	}

	// we default to the machine ID unless the user explicitly set an identity
	if leaseConfig.identity == "" {
		machineID, err := machineid.ProtectedID("mirantis-mke")

		if err != nil {
			return nil, err
		}

		leaseConfig.identity = machineID
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

type WatchOptions struct {
	channels *LeaseEvents
}

type WatchOpt func(options WatchOptions) WatchOptions

// this explicit option allows us to pass through channels with
// a size greater than 0, which makes testing a lot easier.
func WithOutputChannels(channels *LeaseEvents) WatchOpt {
	return func(options WatchOptions) WatchOptions {
		options.channels = channels
		return options
	}
}

func (p *LeasePool) Watch(opts ...WatchOpt) (*LeaseEvents, context.CancelFunc, error) {
	if p.events != nil {
		return p.events, nil, nil
	}

	watchOptions := WatchOptions{
		channels: &LeaseEvents{
			AcquiredLease: make(chan struct{}),
			LostLease: make(chan struct{}),
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
				log.Info("acquired leader lease")
				p.events.AcquiredLease <- struct{}{}
			},
			OnStoppedLeading: func() {
				log.Info("lost leader lease")
				p.events.LostLease <- struct{}{}
			},
			OnNewLeader: nil,
		},
	}
	le, err := leaderelection.NewLeaderElector(lec)
	if err != nil {
		return nil, nil, err
	}
	if lec.WatchDog != nil {
		lec.WatchDog.SetLeaderElection(le)
	}

	ctx, cancel := context.WithCancel(p.config.ctx)
	go le.Run(ctx)

	return p.events, cancel, nil
}
