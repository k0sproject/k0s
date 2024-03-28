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
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/avast/retry-go"
	"github.com/k0sproject/k0s/inttest/common"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
)

var _ manager.Component = (*EtcdMemberReconciler)(nil)

// TODO: DO we need REady probing for this component?
// var _ manager.Ready = (*EtcdMemberReconciler)(nil)

func NewEtcdMemberReconciler(kubeClientFactory kubeutil.ClientFactoryInterface, k0sVars *config.CfgVars, etcdConfig *v1beta1.EtcdConfig, leaderElector leaderelector.Interface) (*EtcdMemberReconciler, error) {

	return &EtcdMemberReconciler{
		clientFactory: kubeClientFactory,
		k0sVars:       k0sVars,
		etcdConfig:    etcdConfig,
		leaderElector: leaderElector,
	}, nil
}

// TODO Bolt in the common leader election stuff
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
		err = e.createMemberObject()
		if err != nil {
			log.WithError(err).Error("failed to create EtcdMember object")
		}
		var lastObservedVersion string
		watch.EtcdMembers(etcdMemberClient).
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
			IncludingDeletions().
			Until(ctx, func(member *etcdv1beta1.EtcdMember) (bool, error) {
				e.reconcileMember(ctx, member)
				// Never stop the watch
				return false, nil
			})
	}()

	return nil
}

func (e *EtcdMemberReconciler) Stop() error {
	return nil
}

type (
	crd     = extensionsv1.CustomResourceDefinition
	crdList = extensionsv1.CustomResourceDefinitionList
)

func (e *EtcdMemberReconciler) waitForCRD(ctx context.Context) error {
	rc := e.clientFactory.GetRESTConfig()

	ec, err := extclient.NewForConfig(rc)
	if err != nil {
		return err
	}
	log := logrus.WithField("component", "etcdMemberReconciler")
	log.Info("waiting to see EtcdMember CRD ready")
	return watch.FromClient[*crdList, crd](ec.CustomResourceDefinitions()).
		WithObjectName(fmt.Sprintf("%s.%s", "etcdmembers", "etcd.k0sproject.io")).
		WithErrorCallback(common.RetryWatchErrors(logrus.Infof)).
		Until(ctx, func(item *crd) (bool, error) {
			for _, cond := range item.Status.Conditions {
				if cond.Type == extensionsv1.Established {
					log.Infof("EtcdMember CRD status: %s", cond.Status)
					return cond.Status == extensionsv1.ConditionTrue, nil
				}
			}

			return false, nil
		})

}

func (e *EtcdMemberReconciler) createMemberObject() error {
	log := logrus.WithFields(logrus.Fields{"component": "etcdMemberReconciler", "phase": "createMemberObject"})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// find the member ID for this node
	etcdClient, err := etcd.NewClient(e.k0sVars.CertRootDir, e.k0sVars.EtcdCertDir, e.etcdConfig)
	if err != nil {
		return err
	}

	peerURL := fmt.Sprintf("https://%s:2380", e.etcdConfig.PeerAddress)

	memberID, err := etcdClient.GetPeerIDByAddress(ctx, peerURL)
	if err != nil {
		return err
	}

	// Convert the memberID to hex string
	memberIDStr := fmt.Sprintf("%x", memberID)

	name, err := os.Hostname()
	if err != nil {
		return err
	}
	em := &etcdv1beta1.EtcdMember{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Finalizers: []string{EtcdFinalizer},
		},
		PeerAddress: e.etcdConfig.PeerAddress,
		MemberID:    memberIDStr,
	}

	log.WithField("name", name).WithField("memberID", memberID).Info("creating EtcdMember object")

	em, err = e.etcdMemberClient.Create(ctx, em, v1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			// Update the object if it already exists
			em, err = e.etcdMemberClient.Update(ctx, em, v1.UpdateOptions{})
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

const EtcdFinalizer = "etcd.k0sproject.io/etcd-reconciler"

func (e *EtcdMemberReconciler) reconcileMember(ctx context.Context, member *etcdv1beta1.EtcdMember) {
	log := logrus.WithFields(logrus.Fields{
		"component":   "etcdMemberReconciler",
		"phase":       "reconcile",
		"name":        member.Name,
		"memberID":    member.MemberID,
		"peerAddress": member.PeerAddress,
	})

	if !e.leaderElector.IsLeader() {
		log.Debug("not the leader, skipping reconcile")
		return
	}

	var err error

	log.Debug("reconciling EtcdMember: %+v", member)

	if member.DeletionTimestamp.IsZero() {
		log.Debug("object is not being deleted, no action needed")
		return
	}

	// TODO Do we need to verify the conditions for removing peer? Or rely on Etcd client to error out if a peer cannot be removed?
	// TODO I need to split up the logic a bit so the error handling is easier to understand/handle
	defer func() {
		if err != nil {
			log.WithError(err).Error("reconcile failed, creating event")
			kc, derr := e.clientFactory.GetClient()
			if derr != nil {
				log.WithError(err).Error("failed to get kube client")
				return
			}
			event := &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "etcd-member-reconciler-",
					Namespace:    "kube-system",
				},
				InvolvedObject: corev1.ObjectReference{
					APIVersion: member.APIVersion,
					Kind:       "EtcdMember",
					Name:       member.Name,
					Namespace:  "kube-system",
				},
				Reason:        "ReconcileFailed",
				Message:       err.Error(),
				Type:          "Warning",
				LastTimestamp: metav1.NewTime(time.Now()),
				Source: corev1.EventSource{
					Component: "k0s-etcd-member-reconciler",
				},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			rerr := retry.Do(func() error {
				log.Debug("trying to create event")
				_, derr = kc.CoreV1().Events("kube-system").Create(ctx, event, v1.CreateOptions{})
				if derr != nil {
					log.WithError(derr).Error("failed to create event")
				}
				return derr
			}, retry.Context(ctx), retry.Attempts(5))
			if rerr != nil {
				log.WithError(rerr).Error("failed to create event")
				return
			}
			log.Debug("event created")
			return
		}
	}()

	etcdClient, err := etcd.NewClient(e.k0sVars.CertRootDir, e.k0sVars.EtcdCertDir, e.etcdConfig)
	if err != nil {
		log.WithError(err).Warn("failed to create etcd client")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Convert the memberID to uint64
	memberID, err := strconv.ParseUint(member.MemberID, 16, 64)
	if err != nil {
		log.WithError(err).Error("failed to parse memberID")
		return
	}

	if err = etcdClient.DeleteMember(ctx, memberID); err != nil {
		logrus.
			WithError(err).
			Errorf("Failed to delete etcd peer from cluster")
		return
	}
	log.Info("peer deleted from etcd cluster, removing finalizer")
	controllerutil.RemoveFinalizer(member, EtcdFinalizer)
	_, err = e.etcdMemberClient.Update(ctx, member, v1.UpdateOptions{})
	if err != nil {
		log.WithError(err).Error("failed to update EtcdMember")
	}
}
