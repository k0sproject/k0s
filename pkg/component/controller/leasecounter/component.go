// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package leasecounter

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/k0sproject/k0s/internal/leaderelection"
	"github.com/k0sproject/k0s/pkg/component/manager"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/sirupsen/logrus"
)

const (
	leaseTypeLabel                = "k0s.k0sproject.io/lease-type"
	leaseTypeLabelValueController = "controller"

	konnectivityServersStateAnnotation = "k0s.k0sproject.io/konnectivity-servers-state"
)

// A controller's observed konnectivity state.
type KonnectivityState struct {
	ServerCount      uint   // observed server count advertised to agents
	ServerCountRises uint64 // incremented on each server count rise
}

// A snapshot of an observed controller lease.
type Lease struct {
	Name         string
	Active       bool               // lease held and not expired
	Konnectivity *KonnectivityState // nil if not recorded
}

// The controller leases as observed by the lease counter.
type Leases struct {
	// The number of active controllers. Always includes this controller, as
	// it's running, so it counts even while its own lease isn't active, e.g.
	// right after startup. As a consequence, this might be one greater than
	// the number of active leases in [Leases.All].
	ActiveCount uint

	// This controller's own lease as observed, active or not.
	// Nil if it doesn't exist.
	Own *Lease

	// All controller leases, active or not.
	All []Lease
}

// Implements a component that manages a lease per controller. The
// per-controller leases are used to determine the amount of currently running
// controllers
type Component struct {
	NodeName          apitypes.NodeName
	InvocationID      string
	KubeClientFactory kubeutil.ClientFactoryInterface

	// Called whenever a change to the controller leases was observed. Not
	// deduplicated. Returns the konnectivity state to be recorded in this
	// controller's lease, or nil to record none.
	UpdateLeases func(Leases) *KonnectivityState

	log  logrus.FieldLogger
	stop func()
}

var _ manager.Component = (*Component)(nil)

// Init initializes the component needs
func (c *Component) Init(context.Context) error {
	c.log = logrus.WithField("component", "controllerlease")
	return nil
}

// Run runs the leader elector to keep the lease object up-to-date.
func (c *Component) Start(context.Context) error {
	kubeClient, err := c.KubeClientFactory.GetClient()
	if err != nil {
		return fmt.Errorf("can't create kubernetes rest client for lease pool: %w", err)
	}

	// Use the node name to make the lease names be clear to which controller they belong to
	leaseName := "k0s-ctrl-" + string(c.NodeName)

	client, err := leaderelection.NewClient(&leaderelection.LeaseConfig{
		Namespace: corev1.NamespaceNodeLease,
		Name:      leaseName,
		Labels: map[string]string{
			leaseTypeLabel: leaseTypeLabelValueController,
		},
		Identity: c.InvocationID,
		Client:   kubeClient.CoordinationV1(),
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Add(2)
	go func() { defer wg.Done(); c.runLeaderElection(ctx, client) }()
	go func() { defer wg.Done(); c.runLeaseCounter(ctx, kubeClient, leaseName) }()
	c.stop = func() { cancel(); wg.Wait() }

	return nil
}

// Stop stops the component
func (c *Component) Stop() error {
	if c.stop != nil {
		c.stop()
	}
	return nil
}

func (c *Component) runLeaderElection(ctx context.Context, client *leaderelection.Client) {
	c.log.Info("Trying to acquire the controller lease")
	client.Run(ctx, func(status leaderelection.Status) {
		if status == leaderelection.StatusLeading {
			c.log.Info("Holding the controller lease")
		} else {
			c.log.Error("Lost the controller lease")
		}
	})
}

// Updates the controller leases every 10 secs.
func (c *Component) runLeaseCounter(ctx context.Context, clients kubernetes.Interface, ownLeaseName string) {
	c.log.Debug("Starting controller lease counter every 10 secs")

	var ownLeaseInactive bool
	wait.UntilWithContext(ctx, func(ctx context.Context) {
		c.log.Debug("Counting active controller leases")
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		leases, err := controllerLeases(ctx, clients)
		if err != nil {
			c.log.WithError(err).Error("Failed to count controller lease holders")
			return
		}

		var (
			own   *Lease
			count uint
		)
		all := make([]Lease, len(leases))
		for i := range leases {
			l := &leases[i]
			all[i] = Lease{
				Name:         l.Name,
				Active:       isActive(l),
				Konnectivity: fromKonnectivityServersStateAnnotation(l.Annotations[konnectivityServersStateAnnotation]),
			}
			if l.Name == ownLeaseName {
				own = &all[i]
			}
			if all[i].Active {
				count++
			}
		}

		inactive := own == nil || !own.Active
		if inactive {
			// This controller is running, so it counts even while its lease isn't active.
			count++
		}
		if inactive != ownLeaseInactive {
			ownLeaseInactive = inactive
			if inactive {
				c.log.Info("Counting this controller although its lease isn't active")
			} else {
				c.log.Info("This controller's lease is active")
			}
		}

		desired := c.UpdateLeases(Leases{ActiveCount: count, Own: own, All: all})

		// Record what the update asked for.
		if own != nil {
			actual := own.Konnectivity
			if desired != actual && (desired == nil || actual == nil || *desired != *actual) {
				if err := patchKonnectivityState(ctx, clients, own.Name, desired); err != nil {
					log := c.log.WithError(err)
					if desired != nil {
						log = log.WithField("desired", toKonnectivityServersStateAnnotation(*desired))
					}
					if actual != nil {
						log = log.WithField("actual", toKonnectivityServersStateAnnotation(*actual))
					}
					log.Error("Failed to patch the konnectivity state in this controller's lease, retrying after the next counting")
				}
			}
		}
	}, 10*time.Second)
}

func patchKonnectivityState(ctx context.Context, clients kubernetes.Interface, name string, state *KonnectivityState) error {
	var encoded any
	if state != nil {
		encoded = toKonnectivityServersStateAnnotation(*state)
	}

	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				konnectivityServersStateAnnotation: encoded,
			},
		},
	})
	if err != nil {
		return err
	}

	leases := clients.CoordinationV1().Leases(corev1.NamespaceNodeLease)
	_, err = leases.Patch(ctx, name, apitypes.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

func toKonnectivityServersStateAnnotation(s KonnectivityState) string {
	return fmt.Sprintf("%d/%d", s.ServerCount, s.ServerCountRises)
}

func fromKonnectivityServersStateAnnotation(value string) *KonnectivityState {
	countVal, risesVal, _ := strings.Cut(value, "/")
	count, err := strconv.ParseUint(countVal, 10, 64)
	if err != nil {
		return nil
	}
	rises, err := strconv.ParseUint(risesVal, 10, 64)
	if err != nil {
		return nil
	}

	return &KonnectivityState{
		ServerCount:      uint(min(count, math.MaxUint)),
		ServerCountRises: rises,
	}
}
