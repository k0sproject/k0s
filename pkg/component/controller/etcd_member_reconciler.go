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
)

var _ manager.Component = (*EtcdMemberReconciler)(nil)

func NewEtcdMemberReconciler(kubeClientFactory kubeutil.ClientFactoryInterface, k0sVars *config.CfgVars, etcdConfig *v1beta1.EtcdConfig, leaderElector leaderelector.Interface) (*EtcdMemberReconciler, error) {

	return &EtcdMemberReconciler{
		clientFactory: kubeClientFactory,
		k0sVars:       k0sVars,
		etcdConfig:    etcdConfig,
		leaderElector: leaderElector,
	}, nil
}

type EtcdMemberReconciler struct {
	clientFactory kubeutil.ClientFactoryInterface
	k0sVars       *config.CfgVars
	etcdConfig    *v1beta1.EtcdConfig
	leaderElector leaderelector.Interface
	stop          func()
}

func (e *EtcdMemberReconciler) Init(_ context.Context) error {
	return nil
}

// resync does a full resync of the etcd members when the leader changes
// This is needed to ensure all the member objects are in sync with the actual etcd cluster
// We might get stale state if we remove the current leader as the leader will essentially
// remove itself from the etcd cluster and after that tries to update the member object.
func (e *EtcdMemberReconciler) resync(ctx context.Context, client etcdclient.EtcdMemberInterface) error {
	// Loop through all the members and run reconcile on them
	// Use high timeout as etcd/api could be a bit slow when the leader changes
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	members, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, member := range members.Items {
		e.reconcileMember(ctx, client, &member)
	}
	return nil
}

func (e *EtcdMemberReconciler) Start(ctx context.Context) error {
	log := logrus.WithField("component", "EtcdMemberReconciler")

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
	err := e.waitForCRD(ctx)
	if err != nil {
		log.WithError(err).Errorf("didn't see EtcdMember CRD ready in time")
		return
	}

	// Create the object for this node
	// Need to be done in retry loop as during the initial startup the etcd might not be stable
	err = wait.PollUntilContextTimeout(ctx, 3*time.Second, 15*time.Second, true, func(ctx context.Context) (bool, error) {
		err := e.createMemberObject(ctx, client)
		if err != nil {
			// During etcd cluster bootstrap, it's common to see k8s giving 500 errors due to etcd timeouts
			if apierrors.IsInternalError(err) {
				log.Debugf("retrying createMemberObject: %v", err)
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	if err != nil {
		log.WithError(err).Error("failed to create EtcdMember object for this controller")
	}

	leaderelection.RunLeaderTasks(ctx, e.leaderElector.CurrentStatus, func(ctx context.Context) {
		var lastObservedVersion string
		err = watch.EtcdMembers(client).
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
				if err := e.resync(ctx, client); err != nil {
					log.WithError(err).Error("failed to resync etcd members")
				}
				// Never stop the watch
				return false, nil
			})

		if canceled := context.Cause(ctx); errors.Is(err, canceled) {
			log.WithError(err).Info("Watch terminated")
		} else {
			log.WithError(err).Error("Watch terminated unexpectedly")
		}
	})
}

func (e *EtcdMemberReconciler) Stop() error {
	if e.stop != nil {
		e.stop()
	}
	return nil
}

func (e *EtcdMemberReconciler) waitForCRD(ctx context.Context) error {
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

	memberID, err := etcdClient.GetPeerIDByAddress(ctx, e.etcdConfig.GetPeerURL())
	if err != nil {
		return err
	}

	// Convert the memberID to hex string
	memberIDStr := strconv.FormatUint(memberID, 16)
	name, err := e.etcdConfig.GetNodeName()

	if err != nil {
		return err
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

func (e *EtcdMemberReconciler) reconcileMember(ctx context.Context, client etcdclient.EtcdMemberInterface, member *etcdv1beta1.EtcdMember) {
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
		return
	}

	etcdClient, err := etcd.NewClient(e.k0sVars.CertRootDir, e.k0sVars.EtcdCertDir, e.etcdConfig)
	if err != nil {
		log.WithError(err).Warn("failed to create etcd client")
		member.Status.ReconcileStatus = etcdv1beta1.ReconcileStatusFailed
		member.Status.Message = err.Error()
		if _, err = client.UpdateStatus(ctx, member, metav1.UpdateOptions{}); err != nil {
			log.WithError(err).Error("failed to update member state")
		}

		return
	}

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

		return
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
		}
	}

	joinStatus := member.Status.GetCondition(etcdv1beta1.ConditionTypeJoined)
	if joinStatus != nil && joinStatus.Status == etcdv1beta1.ConditionFalse && !ok {
		log.Debug("member already left, no action needed")
		return
	}

	// Convert the memberID to uint64
	memberID, err := strconv.ParseUint(member.Status.MemberID, 16, 64)
	if err != nil {
		log.WithError(err).Error("failed to parse memberID")
		return
	}

	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 25*time.Second, true, func(ctx context.Context) (bool, error) {
		err := etcdClient.DeleteMember(ctx, memberID)
		if err != nil {
			// In case etcd reports unhealthy cluster, retry
			msg := err.Error()
			switch {
			case strings.Contains(msg, "unhealthy cluster"):
				return false, nil
			case strings.Contains(msg, "leader changed"):
				return false, nil
			}
			return false, err
		}
		return true, nil
	})

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
		return
	}

	// Peer removed successfully, update status
	log.Info("reconcile succeeded")
	member.Status.ReconcileStatus = etcdv1beta1.ReconcileStatusSuccess
	member.Status.Message = "Member removed from cluster"
	member.Status.SetCondition(etcdv1beta1.ConditionTypeJoined, etcdv1beta1.ConditionFalse, member.Status.Message, time.Now())
	_, err = client.UpdateStatus(ctx, member, metav1.UpdateOptions{})
	if err != nil {
		log.WithError(err).Error("failed to update EtcdMember status")
	}
}
