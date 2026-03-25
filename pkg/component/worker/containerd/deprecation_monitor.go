// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"context"
	"fmt"
	"time"

	introspectionAPI "github.com/containerd/containerd/api/services/introspection/v1"
	containerdclient "github.com/containerd/containerd/v2/client"
	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/pkg/component/manager"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
)

const (
	// componentName identifies this component in logs and events
	componentName = "containerd-deprecation-monitor"

	// Node condition type indicating containerd deprecation warnings
	nodeConditionContainerdNoDeprecations corev1.NodeConditionType = "ContainerdHasNoDeprecations"

	// Event reason for deprecation detection
	eventReasonDeprecationDetected = "ContainerdDeprecationDetected"

	// Reconciliation interval
	reconcileInterval = 5 * time.Minute
)

// DeprecationMonitor watches for containerd deprecation warnings and surfaces them on Node objects
type DeprecationMonitor struct {
	containerdSocketPath string
	certManager          certManager
	nodeName             apitypes.NodeName

	kubeClient    *kubernetes.Clientset
	eventRecorder record.EventRecorder
	log           *logrus.Entry
	stopCh        chan struct{}
}

var _ manager.Component = (*DeprecationMonitor)(nil)

type certManager interface {
	GetRestConfig(ctx context.Context) (*rest.Config, error)
}

// NewDeprecationMonitor creates a new deprecation monitor component
func NewDeprecationMonitor(containerdSocketPath string, certManager certManager, nodeName apitypes.NodeName) *DeprecationMonitor {
	return &DeprecationMonitor{
		containerdSocketPath: containerdSocketPath,
		certManager:          certManager,
		nodeName:             nodeName,
		log:                  logrus.WithField("component", componentName),
		stopCh:               make(chan struct{}),
	}
}

// Init initializes the kubernetes client
func (d *DeprecationMonitor) Init(ctx context.Context) error {
	return nil
}

// Start begins the reconciliation loop
func (d *DeprecationMonitor) Start(ctx context.Context) error {
	d.log.Info("Starting deprecation monitor")
	config, err := d.certManager.GetRestConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get kubernetes rest config: %w", err)
	}

	d.kubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Create event broadcaster and recorder for proper event deduplication
	eventBroadcaster := record.NewBroadcaster(record.WithContext(ctx))
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: d.kubeClient.CoreV1().Events(metav1.NamespaceDefault)})
	d.eventRecorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: componentName})

	go func() {
		d.reconcileLoop(ctx)
	}()

	d.log.WithFields(logrus.Fields{
		"containerdSocketPath": d.containerdSocketPath,
		"nodeName":             d.nodeName,
	}).Info("Deprecation monitor initialized successfully")
	return nil
}

// Stop stops the reconciliation loop
func (d *DeprecationMonitor) Stop() error {
	close(d.stopCh)
	return nil
}

// reconcileLoop runs the reconciliation loop
func (d *DeprecationMonitor) reconcileLoop(ctx context.Context) {
	// Wait for node to exist before starting reconciliation
	d.log.WithField("node", d.nodeName).Info("Waiting for node to be registered in Kubernetes API")

	if err := wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		_, err := d.kubeClient.CoreV1().Nodes().Get(ctx, string(d.nodeName), metav1.GetOptions{})
		if err != nil {
			d.log.WithError(err).Debug("Node not found yet, waiting...")
			return false, nil
		}
		return true, nil
	}); err != nil {
		d.log.WithError(err).Info("Stopped waiting for node")
		return
	}

	d.log.WithField("node", d.nodeName).Info("Node found, starting deprecation monitor reconciliation loop")

	// Do initial reconciliation immediately
	if err := d.reconcile(ctx); err != nil {
		d.log.WithError(err).Error("Initial reconciliation failed")
	}

	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.log.WithField("node", d.nodeName).Info("Stopping deprecation monitor reconciliation loop")
			return
		case <-ticker.C:
			if err := d.reconcile(ctx); err != nil {
				d.log.WithError(err).Error("Reconciliation failed")
			}
		case <-d.stopCh:
			d.log.WithField("node", d.nodeName).Info("Stopping deprecation monitor reconciliation loop")
			return
		}
	}
}

// reconcile performs a single reconciliation cycle
func (d *DeprecationMonitor) reconcile(ctx context.Context) error {
	d.log.Debug("Starting reconciliation cycle")

	// Query containerd for deprecation warnings
	warnings, err := d.getDeprecationWarnings(ctx)
	if err != nil {
		return fmt.Errorf("failed to get deprecation warnings: %w", err)
	}

	d.log.WithField("warningCount", len(warnings)).Info("Retrieved deprecation warnings from containerd")
	for _, w := range warnings {
		d.log.WithFields(logrus.Fields{
			"id":      w.ID,
			"message": w.Message,
		}).Debug("Deprecation warning")
	}

	// Update node condition
	hasDeprecations := len(warnings) > 0
	if err := d.updateNodeCondition(ctx, hasDeprecations); err != nil {
		return fmt.Errorf("failed to update node condition: %w", err)
	}

	// Emit events for all current deprecations
	if hasDeprecations {
		d.emitEventsForDeprecations(warnings)
	}

	return nil
}

// getDeprecationWarnings queries containerd's introspection API for deprecation warnings
func (d *DeprecationMonitor) getDeprecationWarnings(ctx context.Context) ([]*introspectionAPI.DeprecationWarning, error) {
	ctrdClient, err := containerdclient.New(d.containerdSocketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to containerd: %w", err)
	}
	defer ctrdClient.Close()

	introspectionClient := ctrdClient.IntrospectionService()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := introspectionClient.Server(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query server info: %w", err)
	}

	return resp.Deprecations, nil
}

// updateNodeCondition updates the ContainerdHasNoDeprecations condition on the node
func (d *DeprecationMonitor) updateNodeCondition(ctx context.Context, hasDeprecations bool) error {
	node, err := d.kubeClient.CoreV1().Nodes().Get(ctx, string(d.nodeName), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Prepare the new condition
	var status corev1.ConditionStatus
	var reason, message string

	if hasDeprecations {
		status = corev1.ConditionFalse
		reason = "DeprecationsDetected"
		message = "Containerd has detected deprecated configuration or runtime features"
	} else {
		status = corev1.ConditionTrue
		reason = "NoDeprecations"
		message = "No containerd deprecation warnings detected"
	}

	// Update or add the condition
	now := metav1.Now()
	conditionFound := false

	for i := range node.Status.Conditions {
		if node.Status.Conditions[i].Type == nodeConditionContainerdNoDeprecations {
			// Preserve LastTransitionTime if status hasn't changed
			if node.Status.Conditions[i].Status != status {
				node.Status.Conditions[i].LastTransitionTime = now
			}
			node.Status.Conditions[i].Status = status
			node.Status.Conditions[i].Reason = reason
			node.Status.Conditions[i].Message = message
			conditionFound = true
			break
		}
	}

	if !conditionFound {
		node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
			Type:               nodeConditionContainerdNoDeprecations,
			Status:             status,
			Reason:             reason,
			Message:            message,
			LastTransitionTime: now,
		})
	}

	// Update node status
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err = d.kubeClient.CoreV1().Nodes().UpdateStatus(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update node status: %w", err)
	}

	d.log.WithFields(logrus.Fields{
		"node":            d.nodeName,
		"hasDeprecations": hasDeprecations,
		"status":          status,
	}).Debug("Updated node condition")

	return nil
}

// emitEventsForDeprecations emits events for all current deprecation warnings
// Events have a TTL and will naturally expire if deprecations are fixed
// Using EventRecorder provides automatic deduplication and count aggregation
func (d *DeprecationMonitor) emitEventsForDeprecations(warnings []*introspectionAPI.DeprecationWarning) {
	// Get a reference to the node object for the event
	nodeRef := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(d.nodeName),
		},
	}

	for _, warning := range warnings {
		message := fmt.Sprintf("Containerd deprecation: %s - %s", warning.ID, warning.Message)

		// EventRecorder handles deduplication, count updates, and TTL automatically
		// Events with the same reason/message will be aggregated
		d.eventRecorder.Event(nodeRef, corev1.EventTypeWarning, eventReasonDeprecationDetected, message)

		d.log.WithFields(logrus.Fields{
			"node":           d.nodeName,
			"warningID":      warning.ID,
			"warningMessage": warning.Message,
		}).Debug("Recorded deprecation event")
	}
}
