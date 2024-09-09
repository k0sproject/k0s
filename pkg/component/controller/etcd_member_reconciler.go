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

package controller

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	etcdv1beta1 "github.com/k0sproject/k0s/pkg/apis/etcd/v1beta1"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	etcdmemberclient "github.com/k0sproject/k0s/pkg/client/clientset/typed/etcd/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/etcd"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
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
	clientFactory    kubeutil.ClientFactoryInterface
	k0sVars          *config.CfgVars
	etcdConfig       *v1beta1.EtcdConfig
	etcdMemberClient etcdmemberclient.EtcdMemberInterface
	leaderElector    leaderelector.Interface
}

func (e *EtcdMemberReconciler) Init(_ context.Context) error {
	return nil
}

func (e *EtcdMemberReconciler) Start(ctx context.Context) error {
	log := logrus.WithField("component", "etcdMemberReconciler")

	etcdMemberClient, err := e.clientFactory.GetEtcdMemberClient()
	if err != nil {
		return err
	}
	e.etcdMemberClient = etcdMemberClient

	// Run the watch in go routine so it keeps running till the context ends
	go func() {
		err = e.waitForCRD(ctx)
		if err != nil {
			log.WithError(err).Errorf("didn't see EtcdMember CRD ready in time")
			return
		}

		// Create the object for this node
		err = e.createMemberObject(ctx)
		if err != nil {
			log.WithError(err).Error("failed to create EtcdMember object")
		}
		var lastObservedVersion string
		err = watch.EtcdMembers(etcdMemberClient).
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
				e.reconcileMember(ctx, member)
				// Never stop the watch
				return false, nil
			})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				log.WithError(err).Info("watch terminated")
			} else {
				log.WithError(err).Error("watch terminated unexpectedly")
			}
		}
	}()

	return nil
}

func (e *EtcdMemberReconciler) Stop() error {
	return nil
}

func (e *EtcdMemberReconciler) waitForCRD(ctx context.Context) error {
	rc, err := e.clientFactory.GetRESTConfig()
	if err != nil {
		return err
	}
	ec, err := extclient.NewForConfig(rc)
	if err != nil {
		return err
	}
	var lastObservedVersion string
	log := logrus.WithField("component", "etcdMemberReconciler")
	log.Info("waiting to see EtcdMember CRD ready")
	return watch.CRDs(ec.CustomResourceDefinitions()).
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
		Until(ctx, func(item *extensionsv1.CustomResourceDefinition) (bool, error) {
			lastObservedVersion = item.ResourceVersion
			for _, cond := range item.Status.Conditions {
				if cond.Type == extensionsv1.Established {
					log.Infof("EtcdMember CRD status: %s", cond.Status)
					return cond.Status == extensionsv1.ConditionTrue, nil
				}
			}

			return false, nil
		})

}

func (e *EtcdMemberReconciler) createMemberObject(ctx context.Context) error {
	log := logrus.WithFields(logrus.Fields{"component": "etcdMemberReconciler", "phase": "createMemberObject"})
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// find the member ID for this node
	etcdClient, err := etcd.NewClient(e.k0sVars.CertRootDir, e.k0sVars.EtcdCertDir, e.etcdConfig)
	if err != nil {
		return err
	}

	peerURL := fmt.Sprintf("https://%s", net.JoinHostPort(e.etcdConfig.PeerAddress, "2380"))

	memberID, err := etcdClient.GetPeerIDByAddress(ctx, peerURL)
	if err != nil {
		return err
	}

	// Convert the memberID to hex string
	memberIDStr := fmt.Sprintf("%x", memberID)
	name, err := e.etcdConfig.GetNodeName()

	if err != nil {
		return err
	}
	var em *etcdv1beta1.EtcdMember

	log.WithField("name", name).WithField("memberID", memberID).Info("creating EtcdMember object")

	// Check if the object already exists
	em, err = e.etcdMemberClient.Get(ctx, name, metav1.GetOptions{})
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

			em, err = e.etcdMemberClient.Create(ctx, em, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			em.Status = etcdv1beta1.Status{
				PeerAddress: e.etcdConfig.PeerAddress,
				MemberID:    memberIDStr,
			}
			em.Status.SetCondition(etcdv1beta1.ConditionTypeJoined, etcdv1beta1.ConditionTrue, "Member joined", time.Now())
			_, err = e.etcdMemberClient.UpdateStatus(ctx, em, metav1.UpdateOptions{})
			if err != nil {
				log.WithError(err).Error("failed to update member status")
			}
			return nil
		} else {
			return err
		}
	}

	em.Status.PeerAddress = e.etcdConfig.PeerAddress
	em.Status.MemberID = memberIDStr
	em.Spec.Leave = false

	log.Debug("EtcdMember object already exists, updating it")
	// Update the object if it already exists
	em, err = e.etcdMemberClient.Update(ctx, em, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	em.Status.SetCondition(etcdv1beta1.ConditionTypeJoined, etcdv1beta1.ConditionTrue, "Member joined", time.Now())
	_, err = e.etcdMemberClient.UpdateStatus(ctx, em, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (e *EtcdMemberReconciler) reconcileMember(ctx context.Context, member *etcdv1beta1.EtcdMember) {
	log := logrus.WithFields(logrus.Fields{
		"component":   "etcdMemberReconciler",
		"phase":       "reconcile",
		"name":        member.Name,
		"memberID":    member.Status.MemberID,
		"peerAddress": member.Status.PeerAddress,
	})

	if !e.leaderElector.IsLeader() {
		log.Debug("not the leader, skipping reconcile")
		return
	}

	log.Debugf("reconciling EtcdMember: %+v", member)

	if !member.Spec.Leave {
		log.Debug("member not marked for leave, no action needed")
		return
	}

	etcdClient, err := etcd.NewClient(e.k0sVars.CertRootDir, e.k0sVars.EtcdCertDir, e.etcdConfig)
	if err != nil {
		log.WithError(err).Warn("failed to create etcd client")
		member.Status.ReconcileStatus = "Failed"
		member.Status.Message = err.Error()
		if _, err = e.etcdMemberClient.UpdateStatus(ctx, member, metav1.UpdateOptions{}); err != nil {
			log.WithError(err).Error("failed to update member state")
		}

		return
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Verify that the member is actually still present in etcd
	members, err := etcdClient.ListMembers(ctx)
	if err != nil {
		member.Status.ReconcileStatus = "Failed"
		member.Status.Message = err.Error()
		if _, err = e.etcdMemberClient.UpdateStatus(ctx, member, metav1.UpdateOptions{}); err != nil {
			log.WithError(err).Error("failed to update member state")
		}

		return
	}

	// Member marked for leave but no member found in etcd, mark for leaved
	_, ok := members[member.Name]
	if !ok {
		log.Debug("member marked for leave but not in actual member list, updating state to reflect that")
		member.Status.SetCondition(etcdv1beta1.ConditionTypeJoined, etcdv1beta1.ConditionFalse, member.Status.Message, time.Now())
		member, err = e.etcdMemberClient.UpdateStatus(ctx, member, metav1.UpdateOptions{})
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

	if err = etcdClient.DeleteMember(ctx, memberID); err != nil {
		logrus.
			WithError(err).
			Errorf("Failed to delete etcd peer from cluster")
		member.Status.ReconcileStatus = "Failed"
		member.Status.Message = err.Error()
		_, err = e.etcdMemberClient.UpdateStatus(ctx, member, metav1.UpdateOptions{})
		if err != nil {
			log.WithError(err).Error("failed to update EtcdMember status")
		}
		return
	}

	// Peer removed successfully, update status
	log.Info("reconcile succeeded")
	member.Status.ReconcileStatus = "Success"
	member.Status.Message = "Member removed from cluster"
	member.Status.SetCondition(etcdv1beta1.ConditionTypeJoined, etcdv1beta1.ConditionFalse, member.Status.Message, time.Now())
	_, err = e.etcdMemberClient.UpdateStatus(ctx, member, metav1.UpdateOptions{})
	if err != nil {
		log.WithError(err).Error("failed to update EtcdMember status")
	}
}
