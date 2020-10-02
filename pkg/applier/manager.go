package applier

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/fsnotify.v1"
)

// Manager is the Component interface wrapper for Applier
type Manager struct {
	client      kubernetes.Interface
	applier     Applier
	bundlePath  string
	tickerDone  chan struct{}
	watcherDone chan struct{}
}

// Init initializes the Manager
func (m *Manager) Init() error {
	m.bundlePath = filepath.Join(constant.DataDir, "manifests")
	err := util.InitDirectory(m.bundlePath, 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create manifest bundle dir %s", m.bundlePath)
	}

	m.applier, err = NewApplier(m.bundlePath)
	return err
}

func (m *Manager) ensureKubeClient() error {
	cfg, err := clientcmd.BuildConfigFromFlags("", constant.AdminKubeconfigConfigPath)
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	m.client = client

	return nil
}

// Run runs the Manager
func (m *Manager) Run() error {
	log := logrus.WithField("component", "applier-manager")

	// Make the done channels
	m.tickerDone = make(chan struct{})
	m.watcherDone = make(chan struct{})

	for m.client == nil {
		log.Debug("retrieving kube client config")
		_ = m.ensureKubeClient()
		time.Sleep(time.Second)
	}
	hasLease := &atomic.Value{}
	hasLease.Store(false)

	hostname, err := os.Hostname()

	if err != nil {
		return err
	}

	go electLeader(m.client.CoordinationV1(), "mke-manifest-applier", "kube-node-lease", hostname, hasLease)

	var changesDetected atomic.Value
	// to make first tick to sync everything and retry until it succeeds
	changesDetected.Store(true)

	// todo: we could probably move this whole thing out and run it in `onStartedLeading`
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				lease := hasLease.Load().(bool)
				if !lease {
					log.Debug("node does not hold manifest lease")
					continue
				}

				changes := changesDetected.Load().(bool)
				if !changes {
					continue // Wait for next check
				}
				// Do actual apply
				if err := m.applier.Apply(); err != nil {
					log.Warnf("failed to apply manifests: %s", err.Error())
				} else {
					// Only set if the apply succeeds, will make it retry on every tick in case of failures
					changesDetected.Store(false)
				}
			case <-m.tickerDone:
				log.Info("manifest ticker done")
				return
			}
		}
	}()

	// todo: move this along with the above block so we don't watch files unless we hold a lease to apply changes
	go func() {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Errorf("failed to create fs watcher for %s: %s", m.bundlePath, err.Error())
			return
		}
		defer watcher.Close()

		watcher.Add(m.bundlePath)
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

// Stop stops the Manager
func (m *Manager) Stop() error {
	close(m.tickerDone)
	close(m.watcherDone)
	return nil
}

func electLeader(client v1.LeasesGetter, name, namespace, id string, elected *atomic.Value) error {
	log := logrus.WithField("component", "applier-manager")

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Client: client,
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}
	lec := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   60 * time.Second,
		RenewDeadline:   15 * time.Second,
		RetryPeriod:     5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				log.Info("acquired leader lease")
				elected.Store(true)
			},
			OnStoppedLeading: func() {
				log.Info("lost leader lease")
				elected.Store(false)
			},
			OnNewLeader: nil,
		},
	}
	le, err := leaderelection.NewLeaderElector(lec)
	if err != nil {
		return err
	}
	if lec.WatchDog != nil {
		lec.WatchDog.SetLeaderElection(le)
	}

	le.Run(context.TODO())

	return nil
}
