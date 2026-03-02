// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Stater interface {
	State(maxCount int) prober.State
}

type Status struct {
	StatusInformation K0sStatus
	Prober            Stater
	Socket            string
	L                 *logrus.Entry
	httpserver        http.Server
	CertManager       certManager

	WorkerClient       kubernetes.Interface
	AdminClient        kubernetes.Interface
	AdminClientFactory *kubeutil.ClientFactory
}

type certManager interface {
	GetRestConfig(ctx context.Context) (*rest.Config, error)
}

var _ manager.Component = (*Status)(nil)

const (
	defaultMaxEvents = 5
)

// Init initializes component
func (s *Status) Init(_ context.Context) error {
	s.L = logrus.WithFields(logrus.Fields{"component": "status"})
	mux := http.NewServeMux()
	mux.Handle("/status", &statusHandler{Status: s})
	mux.HandleFunc("/components", func(w http.ResponseWriter, r *http.Request) {
		maxCount, err := strconv.ParseInt(r.URL.Query().Get("maxCount"), 10, 32)
		if err != nil {
			maxCount = defaultMaxEvents
		}
		w.Header().Set("Content-Type", "application/json")
		if json.NewEncoder(w).Encode(s.Prober.State(int(maxCount))) != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	s.httpserver = http.Server{
		Handler: mux,
	}

	return nil
}

// Start runs the component
func (s *Status) Start(_ context.Context) error {
	listener, err := newStatusListener(s.Socket)
	if err != nil {
		s.L.Errorf("failed to create listener %s", err)
		return err
	}
	s.L.Infof("Listening address %s", s.Socket)
	go func() {
		if err := s.httpserver.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.L.Errorf("failed to start status server at %s: %s", s.Socket, err)
		}
	}()
	return nil
}

// Stop stops status component and removes the unix socket
func (s *Status) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.httpserver.Shutdown(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	cleanupStatusListener(s.Socket)
	return nil
}

type statusHandler struct {
	Status *Status
}

// ServerHTTP implementation of handler interface
func (sh *statusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	statusInfo := sh.getCurrentStatus(r.Context())

	w.Header().Set("Content-Type", "application/json")
	if json.NewEncoder(w).Encode(statusInfo) != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

const (
	defaultPollDuration = 1 * time.Second
	defaultPollTimeout  = 5 * time.Minute

	cniReady             = "AllReady"
	cniNotReady          = "NotReady"
	cniComponentNotReady = "ComponentsNotReady"
	cniNotFound          = "NotFound"
	cniError             = "Error"
)

func addCNICondition(conditions *[]Condition, status corev1.ConditionStatus, reason, message string) {
	*conditions = append(*conditions, Condition{
		Type:    "Ready",
		Status:  status,
		Reason:  reason,
		Message: message,
	})
}

func (sh *statusHandler) checkCNIDaemonSet(ds *appsv1.DaemonSet, conditions *[]Condition) {
	desired := ds.Status.DesiredNumberScheduled
	ready := ds.Status.NumberReady

	if desired > 0 && ready == desired {
		addCNICondition(conditions, corev1.ConditionTrue, cniReady, fmt.Sprintf("%s DaemonSet reports %d/%d ready", ds.Name, ready, desired))
	} else {
		addCNICondition(conditions, corev1.ConditionFalse, cniNotReady, fmt.Sprintf("%s DaemonSet reports %d/%d ready", ds.Name, ready, desired))
	}
}

func (sh *statusHandler) checkCalicoDeployment(deploy *appsv1.Deployment, conditions *[]Condition) {
	for _, c := range deploy.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable {
			if c.Status == corev1.ConditionTrue {
				addCNICondition(conditions, corev1.ConditionTrue, cniReady, "Calico kube-controllers deployment is available")
			} else {
				addCNICondition(conditions, corev1.ConditionFalse, cniComponentNotReady, "Calico kube-controllers deployment is not available")
			}
			return
		}
	}
}

func (sh *statusHandler) getCurrentStatus(ctx context.Context) K0sStatus {
	status := sh.Status.StatusInformation
	if !status.Workloads {
		return status
	}

	if sh.Status.WorkerClient == nil {
		kubeClient, err := sh.buildWorkerSideKubeAPIClient(ctx)
		if err != nil {
			status.WorkerToAPIConnectionStatus.Message = "failed to create kube-api client required for kube-api status reports, probably kubelet failed to init: " + err.Error()
		}
		sh.Status.WorkerClient = kubeClient
	}
	_, err := sh.Status.WorkerClient.AuthenticationV1().SelfSubjectReviews().Create(ctx, &authv1.SelfSubjectReview{}, v1.CreateOptions{})
	if err != nil {
		status.WorkerToAPIConnectionStatus.Message = err.Error()
	}
	status.WorkerToAPIConnectionStatus.Success = true

	status.Conditions = []Condition{}

	apiClient, err := sh.Status.AdminClientFactory.GetClient()
	if err != nil || apiClient == nil {
		return status
	}

	provider := ""
	if status.ClusterConfig != nil {
		provider = status.ClusterConfig.Spec.Network.Provider
	}

	switch provider {
	case constant.CNIProviderKubeRouter:
		ds, err := apiClient.AppsV1().DaemonSets(v1.NamespaceSystem).Get(ctx, "kube-router", v1.GetOptions{})
		if err == nil {
			sh.checkCNIDaemonSet(ds, &status.Conditions)
		} else if apierrors.IsNotFound(err) {
			addCNICondition(&status.Conditions, corev1.ConditionFalse, cniNotFound, "kube-router DaemonSet not found")
		} else {
			addCNICondition(&status.Conditions, corev1.ConditionUnknown, cniError, err.Error())
		}

	case constant.CNIProviderCalico:
		ds, err := apiClient.AppsV1().DaemonSets(v1.NamespaceSystem).Get(ctx, "calico-node", v1.GetOptions{})
		if err == nil {
			sh.checkCNIDaemonSet(ds, &status.Conditions)
			deploy, err := apiClient.AppsV1().Deployments(v1.NamespaceSystem).Get(ctx, "calico-kube-controllers", v1.GetOptions{})
			if err == nil {
				sh.checkCalicoDeployment(deploy, &status.Conditions)
			}
		} else if apierrors.IsNotFound(err) {
			addCNICondition(&status.Conditions, corev1.ConditionFalse, cniNotFound, "calico-node DaemonSet not found")
		} else {
			addCNICondition(&status.Conditions, corev1.ConditionUnknown, cniError, err.Error())
		}

	default:
		addCNICondition(&status.Conditions, corev1.ConditionUnknown, "CustomProvider", "")
	}

	return status
}

func (sh *statusHandler) buildWorkerSideKubeAPIClient(ctx context.Context) (client kubernetes.Interface, _ error) {
	timeout, cancel := context.WithTimeout(ctx, defaultPollTimeout)
	defer cancel()
	if err := wait.PollUntilWithContext(timeout, defaultPollDuration, func(ctx context.Context) (done bool, err error) {
		factory := kubeutil.ClientFactory{LoadRESTConfig: func() (*rest.Config, error) {
			return sh.Status.CertManager.GetRestConfig(ctx)
		}}

		client, err = factory.GetClient()
		return err == nil, nil
	}); err != nil {
		return nil, err
	}
	return client, nil
}
