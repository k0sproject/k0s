/*
Copyright 2020 Mirantis, Inc.

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
package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coordinationv1client "k8s.io/client-go/kubernetes/typed/coordination/v1"
)

var (
	MinLeaseHolders = 1
)

type LeaseLock struct {
	Client coordinationv1client.LeasesGetter
	Config *LeaseConfig
	Log    *logrus.Entry

	lease           *coordinationv1.Lease
	observedSpec    *coordinationv1.LeaseSpec
	observedRawSpec []byte
	observedTime    time.Time
}

type LeaseConfig struct {
	HolderIdentity string
	Name           string
	Namespace      string
	ServiceName    string

	// LeaseDuration is the duration of a lease, before the lease can be re-acquired. A client needs to wait a full LeaseDuration without observing a change to
	// the record before it can attempt to take over.
	LeaseDuration time.Duration

	// RenewDeadline is the duration that the lease holder will retry refreshing leadership before giving up.
	RenewDeadline time.Duration

	// RetryPeriod is the duration the lease clients should wait
	// between tries of actions.
	RetryPeriod time.Duration
}

// getLease gets the LeaseSpec for a for the relevant LeaseMeta Name & Namespace
func (l *LeaseLock) getLease(ctx context.Context) (*coordinationv1.LeaseSpec, []byte, error) {
	var err error
	l.lease, err = l.Client.Leases(l.Config.Namespace).Get(ctx, l.Config.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	leaseByte, err := json.Marshal(l.lease.Spec)
	if err != nil {
		return nil, nil, err
	}
	return &l.lease.Spec, leaseByte, nil
}

// createLease creates the LeaseSpec for a for the relevant LeaseMeta Name, Namespace & LeaseSpec
func (l *LeaseLock) createLease(ctx context.Context, leaseSpec coordinationv1.LeaseSpec) error {
	var err error
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      l.Config.Name,
			Namespace: l.Config.Namespace,
			Labels:    map[string]string{"service": l.Config.ServiceName},
		},
		Spec: leaseSpec,
	}

	l.lease, err = l.Client.Leases(l.Config.Namespace).Create(ctx, lease, metav1.CreateOptions{})
	return err
}

// updateLease updates the LeaseSpec for a for the relevant LeaseMeta Name, Namespace & LeaseSpec
func (l *LeaseLock) updateLease(ctx context.Context, leaseSpec coordinationv1.LeaseSpec) error {
	var err error
	if l.lease == nil {
		return fmt.Errorf("lease not initialized, call get or create first")
	}
	l.lease.Spec = leaseSpec

	lease, err := l.Client.Leases(l.Config.Namespace).Update(ctx, l.lease, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	l.lease = lease
	return nil
}

func leaseConfigToLeaseSpec(lec LeaseConfig) *coordinationv1.LeaseSpec {
	leaseDurationSeconds := int32(lec.LeaseDuration / time.Second)
	holderIdentity, err := os.Hostname()
	if err != nil {
		return nil
	}

	return &coordinationv1.LeaseSpec{
		HolderIdentity:       &holderIdentity,
		AcquireTime:          &metav1.MicroTime{Time: time.Now()},
		LeaseDurationSeconds: &leaseDurationSeconds,
		RenewTime:            &metav1.MicroTime{Time: time.Now()},
	}
}

func (l *LeaseLock) LeaseRunner(ctx context.Context) {
	err := l.validateLeaseConfig()
	if err != nil {
		l.Log.Error(err)
	}

	defer runtime.HandleCrash()
	if !l.acquire(ctx) {
		return // ctx signalled done
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	l.renew(ctx)
}

// acquire loops calling tryAcquireOrRenew and returns true immediately when tryAcquireOrRenew succeeds.
// Returns false if ctx signals done.
func (l *LeaseLock) acquire(ctx context.Context) bool {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	succeeded := false
	desc := fmt.Sprintf("%v/%v", l.Config.Namespace, l.Config.Name)
	l.Log.Infof("attempting to acquire lease %v...", desc)
	wait.JitterUntil(func() {
		succeeded = l.tryAcquireOrRenew(ctx)
		if !succeeded {
			l.Log.Infof("failed to acquire lease %v. will attempt to re-acquire.", desc)
			return
		}
		l.Log.Infof("successfully acquired lease %v", desc)
		cancel()
	}, l.Config.RetryPeriod, 1.2, true, ctx.Done())
	return succeeded
}

func (l *LeaseLock) renew(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wait.Until(func() {
		timeoutCtx, timeoutCancel := context.WithTimeout(ctx, l.Config.RenewDeadline)
		defer timeoutCancel()
		err := wait.PollImmediateUntil(l.Config.RetryPeriod, func() (bool, error) {
			return l.tryAcquireOrRenew(timeoutCtx), nil
		}, timeoutCtx.Done())

		desc := fmt.Sprintf("%v/%v", l.Config.Namespace, l.Config.Name)
		if err == nil {
			l.Log.Infof("successfully renewed lease %v", desc)
			return
		}
		// renewal failed
		l.Log.Infof("failed to renew lease %v: %v", desc, err)
		cancel()
	}, l.Config.RetryPeriod, ctx.Done())
}

func (l *LeaseLock) tryAcquireOrRenew(ctx context.Context) bool {
	desiredLeaseSpec := leaseConfigToLeaseSpec(*l.Config)

	// 1. obtain or create the lease
	oldLeaseSpec, oldLeaseRawSpec, err := l.getLease(ctx)

	desc := fmt.Sprintf("%v/%v", l.Config.Namespace, l.Config.Name)

	if err != nil {
		if !errors.IsNotFound(err) {
			l.Log.Errorf("error retrieving resource lease %v: %v", desc, err)
			return false
		}
		if err = l.createLease(ctx, *desiredLeaseSpec); err != nil {
			l.Log.Errorf("error initially creating lease lock: %v", err)
			return false
		}
		l.observedSpec = desiredLeaseSpec
		l.observedTime = time.Now()

		return true
	}

	// 2. record exists, check identity & time
	if !bytes.Equal(l.observedRawSpec, oldLeaseRawSpec) {
		l.observedSpec = oldLeaseSpec
		l.observedRawSpec = oldLeaseRawSpec
		l.observedTime = time.Now()
	}

	expiredTime := l.observedTime.Add(l.Config.LeaseDuration)

	// check if the expired Time is after current time (I.E. not yet expired)
	if len(*oldLeaseSpec.HolderIdentity) > 0 && expiredTime.After(time.Now()) {
		l.Log.Debugf("lease is held by %v and has not yet expired (expiration time: %v)", *oldLeaseSpec.HolderIdentity, expiredTime)
		return false
	}

	// 3. update the lease
	desiredLeaseSpec.AcquireTime = oldLeaseSpec.AcquireTime // leave the "acquired" field unchanged

	// update the lock itself
	if err = l.updateLease(ctx, *desiredLeaseSpec); err != nil {
		l.Log.Errorf("Failed to update lease: %v", err)
		return false
	}

	l.observedSpec = desiredLeaseSpec
	l.observedTime = time.Now()
	return true
}

// fetch a list of all the leases that contain the label service=l.Config.ServiceName
func (l *LeaseLock) listLease(ctx context.Context) *coordinationv1.LeaseList {
	label := map[string]string{"service": l.Config.ServiceName}
	desc := fmt.Sprintf("%v/%v", l.Config.Namespace, l.Config.Name)

	ls := &metav1.LabelSelector{}
	if err := metav1.Convert_Map_string_To_string_To_v1_LabelSelector(&label, ls, nil); err != nil {
		logrus.Debugf("failed to parse label for listing lease %v", desc)
	}

	opts := metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(ls),
	}

	list, err := l.Client.Leases(l.Config.Namespace).List(ctx, opts)
	if err != nil {
		logrus.Errorf("failed to fetch lease holders for %v", desc)
	}
	return list
}

// CountValidLeaseHolders fetches the leaseList and examines if a lease is still valid.
// if it is, it adds it to the validLeaseHolder count
func (l *LeaseLock) CountValidLeaseHolders(ctx context.Context) int {
	var validLeases []coordinationv1.Lease
	list := l.listLease(ctx)

	for i := range list.Items {
		lease := list.Items[i]
		leaseDurationSeconds := lease.Spec.LeaseDurationSeconds
		leaseDuration := time.Duration(*leaseDurationSeconds) * time.Second
		lastRenewed := lease.Spec.RenewTime
		expires := lastRenewed.Add(leaseDuration).Add(l.Config.RenewDeadline)

		if expires.After(time.Now()) {
			logrus.Debugf("lease for %v still valid, adding to valid lease count", *lease.Spec.HolderIdentity)
			validLeases = append(validLeases, lease)
		}
	}
	count := len(validLeases)
	if count < MinLeaseHolders {
		return MinLeaseHolders
	}
	return count
}

func (l *LeaseLock) validateLeaseConfig() error {
	l.Log.Debug("Validating lease config")
	var JitterFactor = 1.2
	if l.Config.LeaseDuration <= l.Config.RenewDeadline {
		return fmt.Errorf("leaseDuration must be greater than renewDeadline")
	}
	if l.Config.RenewDeadline <= time.Duration(JitterFactor*float64(l.Config.RetryPeriod)) {
		return fmt.Errorf("RenewDeadline must be greater than retryPeriod*JitterFactor")
	}
	if l.Config.LeaseDuration < 1 {
		return fmt.Errorf("LeaseDuration must be greater than zero")
	}
	if l.Config.RenewDeadline < 1 {
		return fmt.Errorf("RenewDeadline must be greater than zero")
	}
	if l.Config.RetryPeriod < 1 {
		return fmt.Errorf("RetryPeriod must be greater than zero")
	}
	l.Log.Info("LeaseConfig is valid. Moving on!")
	return nil
}
