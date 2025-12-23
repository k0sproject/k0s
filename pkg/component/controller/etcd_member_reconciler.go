// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go"
	etcdv1beta1 "github.com/k0sproject/k0s/pkg/apis/etcd/v1beta1"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	etcdclient "github.com/k0sproject/k0s/pkg/client/clientset/typed/etcd/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/etcd"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
	"github.com/k0sproject/k0s/pkg/leaderelection"
	"github.com/sirupsen/logrus"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	nodeutil "k8s.io/component-helpers/node/util"
)

var _ manager.Component = (*EtcdMemberReconciler)(nil)

func NewEtcdMemberReconciler(kubeClientFactory kubeutil.ClientFactoryInterface, k0sVars *config.CfgVars, etcdConfig *v1beta1.EtcdConfig, leaderElector leaderelector.Interface, controllerCount func() uint) (*EtcdMemberReconciler, error) {

	return &EtcdMemberReconciler{
		clientFactory:   kubeClientFactory,
		k0sVars:         k0sVars,
		etcdConfig:      etcdConfig,
		leaderElector:   leaderElector,
		controllerCount: controllerCount,
	}, nil
}

type EtcdMemberReconciler struct {
	clientFactory   kubeutil.ClientFactoryInterface
	k0sVars         *config.CfgVars
	etcdConfig      *v1beta1.EtcdConfig
	leaderElector   leaderelector.Interface
	controllerCount func() uint
	stop            func()
}

func (e *EtcdMemberReconciler) Init(_ context.Context) error {
	return nil
}

// resync does a full resync of the etcd members when the leader changes
// This is needed to ensure all the member objects are in sync with the actual etcd cluster
// We might get stale state if we remove the current leader as the leader will essentially
// remove itself from the etcd cluster and after that tries to update the member object.
func (e *EtcdMemberReconciler) resync(ctx context.Context, client etcdclient.EtcdMemberInterface, log logrus.FieldLogger) bool {
	// Loop through all the members and run reconcile on them
	// Use high timeout as etcd/api could be a bit slow when the leader changes
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	members, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		log.WithError(err).Error("Failed to list etcd members")
		return false
	}

	var failed bool
	for _, member := range members.Items {
		if !e.reconcileMember(ctx, client, &member) {
			failed = true
		}
	}

	return !failed
}

func (e *EtcdMemberReconciler) Start(ctx context.Context) error {
	log := logrus.WithField("component", "EtcdMemberReconciler")

	err := e.waitForCRD(ctx)
	if err != nil {
		return fmt.Errorf("didn't see EtcdMember CRD ready in time: %w", err)
	}

	client, err := e.clientFactory.GetEtcdMemberClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		wait.UntilWithContext(ctx, func(ctx context.Context) {
			e.reconcile(ctx, log, client)
		}, 1*time.Minute)
	}()

	e.stop = func() {
		cancel(errors.New("etcd member reconciler is stopping"))
		<-done
	}

	return nil
}

func (e *EtcdMemberReconciler) reconcile(ctx context.Context, log logrus.FieldLogger, client etcdclient.EtcdMemberInterface) {
	// Create the object for this node
	// Need to be done in retry loop as during the initial startup the etcd might not be stable
	err := retry.Do(
		func() error {
			return e.createMemberObject(ctx, client)
		},
		retry.Delay(3*time.Second),
		retry.Attempts(5),
		retry.Context(ctx),
		retry.LastErrorOnly(true),
		retry.RetryIf(func(retryErr error) bool {
			log.Debugf("retrying createMemberObject: %v", retryErr)
			// During etcd cluster bootstrap, it's common to see k8s giving 500 errors due to etcd timeouts
			return apierrors.IsInternalError(retryErr)
		}),
	)
	if err != nil {
		log.WithError(err).Error("failed to create EtcdMember object for this controller")
	}

	leaderelection.RunLeaderTasks(ctx, e.leaderElector.CurrentStatus, func(ctx context.Context) {
		e.watchAndResync(ctx, client, log)
	})
}

func (e *EtcdMemberReconciler) watchAndResync(ctx context.Context, client etcdclient.EtcdMemberInterface, log logrus.FieldLogger) {
	trigger := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		e.watchEtcdMembers(ctx, client, log, trigger)
	}()
	defer func() { <-done }()

	var retry <-chan time.Time

	for {
		select {
		case <-retry:
		case <-trigger:
			if retry != nil {
				continue
			}
		case <-ctx.Done():
			return
		}

		retry = nil
		if !e.resync(ctx, client, log) {
			log.Debug("Retrying in ten seconds")
			retry = time.After(10 * time.Second)
		}
	}
}

func (e *EtcdMemberReconciler) watchEtcdMembers(ctx context.Context, client etcdclient.EtcdMemberInterface, log logrus.FieldLogger, trigger chan<- struct{}) {
	var lastObservedVersion string
	err := watch.EtcdMembers(client).
		WithErrorCallback(func(err error) (time.Duration, error) {
			retryDelay, e := watch.IsRetryable(err)
			if e == nil {
				log.WithError(err).Debugf(
					"Encountered transient error while watching etcd members"+
						", last observed resource version was %q"+
						", retrying in %s",
					lastObservedVersion, retryDelay,
				)
				return retryDelay, nil
			}
			log.WithError(e).Error("bailing out watch")
			return 0, err
		}).
		Until(ctx, func(member *etcdv1beta1.EtcdMember) (bool, error) {
			lastObservedVersion = member.ResourceVersion
			log.Debugf("watch triggered on %s", member.Name)
			select {
			case trigger <- struct{}{}:
			default:
			}
			// Never stop the watch
			return false, nil
		})

	if canceled := context.Cause(ctx); errors.Is(err, canceled) {
		log.WithError(err).Info("Watch terminated")
	} else {
		log.WithError(err).Error("Watch terminated unexpectedly")
	}
}

func (e *EtcdMemberReconciler) Stop() error {
	if e.stop != nil {
		e.stop()
	}
	return nil
}

func (e *EtcdMemberReconciler) waitForCRD(ctx context.Context) error {
	ctx, cancel := context.WithTimeoutCause(ctx, 2*time.Minute, errors.New("EtcdMember CRD did not become established in time"))
	defer cancel()

	client, err := e.clientFactory.GetAPIExtensionsClient()
	if err != nil {
		return err
	}
	var lastObservedVersion string
	log := logrus.WithField("component", "etcdMemberReconciler")
	log.Info("waiting to see EtcdMember CRD ready")
	return watch.CRDs(client.ApiextensionsV1().CustomResourceDefinitions()).
		WithObjectName(fmt.Sprintf("%s.%s", "etcdmembers", "etcd.k0sproject.io")).
		WithErrorCallback(func(err error) (time.Duration, error) {
			if retryAfter, e := watch.IsRetryable(err); e == nil {
				log.WithError(err).Infof(
					"Transient error while watching etcdmember CRD"+
						", last observed version is %q"+
						", starting over after %s ...",
					lastObservedVersion, retryAfter,
				)
				return retryAfter, nil
			}

			retryAfter := 10 * time.Second
			log.WithError(err).Errorf(
				"Failed to watch for etcdmember CRD"+
					", last observed version is %q"+
					", starting over after %s ...",
				lastObservedVersion, retryAfter,
			)
			return retryAfter, nil
		}).
		Until(ctx, func(item *apiextensionsv1.CustomResourceDefinition) (bool, error) {
			lastObservedVersion = item.ResourceVersion
			for _, cond := range item.Status.Conditions {
				if cond.Type == apiextensionsv1.Established {
					log.Infof("EtcdMember CRD status: %s", cond.Status)
					return cond.Status == apiextensionsv1.ConditionTrue, nil
				}
			}

			return false, nil
		})

}

func (e *EtcdMemberReconciler) createMemberObject(ctx context.Context, client etcdclient.EtcdMemberInterface) error {
	log := logrus.WithFields(logrus.Fields{"component": "etcdMemberReconciler", "phase": "createMemberObject"})
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// find the member ID for this node
	etcdClient, err := etcd.NewClient(e.k0sVars.CertRootDir, e.k0sVars.EtcdCertDir, e.etcdConfig)
	if err != nil {
		return err
	}
	defer etcdClient.Close()

	memberID, err := etcdClient.GetPeerIDByAddress(ctx, e.etcdConfig.GetPeerURL())
	if err != nil {
		return err
	}

	// Convert the memberID to hex string
	memberIDStr := strconv.FormatUint(memberID, 16)
	memberName := e.etcdConfig.GetMemberName()
	name, err := nodeutil.GetHostname(memberName)
	if err != nil {
		return fmt.Errorf("failed to get name for etcd member: %w", err)
	}
	var em *etcdv1beta1.EtcdMember

	log.WithField("name", name).WithField("memberID", memberID).Info("creating EtcdMember object")

	// Check if the object already exists
	em, err = client.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("EtcdMember object not found, creating it")
			em = &etcdv1beta1.EtcdMember{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: etcdv1beta1.EtcdMemberSpec{
					Leave: false,
				},
			}

			em, err = client.Create(ctx, em, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			em.Status = etcdv1beta1.Status{
				PeerAddress: e.etcdConfig.PeerAddress,
				MemberID:    memberIDStr,
			}
			em.Status.SetCondition(etcdv1beta1.ConditionTypeJoined, etcdv1beta1.ConditionTrue, "Member joined", time.Now())
			_, err = client.UpdateStatus(ctx, em, metav1.UpdateOptions{})
			if err != nil {
				log.WithError(err).Error("failed to update member status")
			}
			return nil
		} else {
			return err
		}
	}

	em.Spec.Leave = false

	log.Debug("EtcdMember object already exists, updating it")
	// Update the object if it already exists
	em, err = client.Update(ctx, em, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	em.Status.PeerAddress = e.etcdConfig.PeerAddress
	em.Status.MemberID = memberIDStr
	em.Status.SetCondition(etcdv1beta1.ConditionTypeJoined, etcdv1beta1.ConditionTrue, "Member joined", time.Now())
	_, err = client.UpdateStatus(ctx, em, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (e *EtcdMemberReconciler) reconcileMember(ctx context.Context, client etcdclient.EtcdMemberInterface, member *etcdv1beta1.EtcdMember) bool {
	log := logrus.WithFields(logrus.Fields{
		"component":   "etcdMemberReconciler",
		"phase":       "reconcile",
		"name":        member.Name,
		"memberID":    member.Status.MemberID,
		"peerAddress": member.Status.PeerAddress,
	})

	log.Debugf("reconciling EtcdMember: %s", member.Name)

	if !member.Spec.Leave {
		log.Debug("member not marked for leave, no action needed")
		return true
	}

	if e.etcdConfig.PeerAddress == member.Status.PeerAddress {
		if count := e.controllerCount(); count < 2 {
			msg := "The only active controller cannot leave the etcd cluster" +
				"; either bring another controller online or clear spec.leave"
			log.Error(msg)
			member.Status.Message = msg
			member.Status.ReconcileStatus = ""
			if _, err := client.UpdateStatus(ctx, member, metav1.UpdateOptions{}); err != nil {
				log.WithError(err).Error("Failed to update member state")
			}
			return false
		}

		if le, ok := e.leaderElector.(interface{ YieldLease() }); ok {
			msg := "Waiting for another controller to take over"
			log.Info(msg)
			member.Status.Message = msg
			member.Status.ReconcileStatus = ""
			if _, err := client.UpdateStatus(ctx, member, metav1.UpdateOptions{}); err != nil {
				log.WithError(err).Error("Failed to update member state")
			}
			le.YieldLease()
		} else {
			msg := "Requested to leave the etcd cluster, but cannot yield the lease"
			log.Error(msg)
			member.Status.Message = msg
			member.Status.ReconcileStatus = etcdv1beta1.ReconcileStatusFailed
			if _, err := client.UpdateStatus(ctx, member, metav1.UpdateOptions{}); err != nil {
				log.WithError(err).Error("Failed to update member state")
			}
		}

		return false
	}

	etcdClient, err := etcd.NewClient(e.k0sVars.CertRootDir, e.k0sVars.EtcdCertDir, e.etcdConfig)
	if err != nil {
		log.WithError(err).Warn("failed to create etcd client")
		member.Status.ReconcileStatus = etcdv1beta1.ReconcileStatusFailed
		member.Status.Message = err.Error()
		if _, err = client.UpdateStatus(ctx, member, metav1.UpdateOptions{}); err != nil {
			log.WithError(err).Error("failed to update member state")
		}

		return false
	}
	defer etcdClient.Close()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Verify that the member is actually still present in etcd
	members, err := etcdClient.ListMembers(ctx)
	if err != nil {
		member.Status.ReconcileStatus = etcdv1beta1.ReconcileStatusFailed
		member.Status.Message = err.Error()
		if _, err = client.UpdateStatus(ctx, member, metav1.UpdateOptions{}); err != nil {
			log.WithError(err).Error("failed to update member state")
		}

		return false
	}

	// Member marked for leave but no member found in etcd, mark for leaved
	_, ok := members[member.Name]
	if !ok {
		log.Debug("member marked for leave but not in actual member list, updating state to reflect that")
		member.Status.SetCondition(etcdv1beta1.ConditionTypeJoined, etcdv1beta1.ConditionFalse, member.Status.Message, time.Now())
		member.Status.ReconcileStatus = etcdv1beta1.ReconcileStatusSuccess
		member, err = client.UpdateStatus(ctx, member, metav1.UpdateOptions{})
		if err != nil {
			log.WithError(err).Error("failed to update EtcdMember status")
			return false
		}
	}

	joinStatus := member.Status.GetCondition(etcdv1beta1.ConditionTypeJoined)
	if joinStatus != nil && joinStatus.Status == etcdv1beta1.ConditionFalse && !ok {
		log.Debug("member already left, no action needed")
		return true
	}

	// Convert the memberID to uint64
	memberID, err := strconv.ParseUint(member.Status.MemberID, 16, 64)
	if err != nil {
		log.WithError(err).Error("failed to parse memberID")
		return false
	}

	err = retry.Do(func() error {
		return etcdClient.DeleteMember(ctx, memberID)
	},
		retry.Delay(5*time.Second),
		retry.LastErrorOnly(true),
		retry.Attempts(5),
		retry.Context(ctx),
		retry.RetryIf(func(err error) bool {
			// In case etcd reports unhealthy cluster, retry
			msg := err.Error()
			switch {
			case strings.Contains(msg, "unhealthy cluster"):
				return true
			case strings.Contains(msg, "leader changed"):
				return true
			}
			return false
		}),
	)

	if err != nil {
		logrus.
			WithError(err).
			Errorf("Failed to delete etcd peer from cluster")
		member.Status.ReconcileStatus = etcdv1beta1.ReconcileStatusFailed
		member.Status.Message = err.Error()
		_, err = client.UpdateStatus(ctx, member, metav1.UpdateOptions{})
		if err != nil {
			log.WithError(err).Error("failed to update EtcdMember status")
		}
		return false
	}

	// Peer removed successfully, update status
	log.Info("reconcile succeeded")
	member.Status.ReconcileStatus = etcdv1beta1.ReconcileStatusSuccess
	member.Status.Message = "Member removed from cluster"
	member.Status.SetCondition(etcdv1beta1.ConditionTypeJoined, etcdv1beta1.ConditionFalse, member.Status.Message, time.Now())
	_, err = client.UpdateStatus(ctx, member, metav1.UpdateOptions{})
	if err != nil {
		log.WithError(err).Error("failed to update EtcdMember status")
		return false
	}

	return true
}
