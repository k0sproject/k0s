// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"time"

	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

// How long to keep retrying a conflicting signaling status update. Six attempts
// spread over about 0.8 seconds, which gives a lagging cache time to catch up
// with whatever caused the conflict. Note that the sleeps happen between the
// attempts, so there are five of them, 25ms doubling up to 400ms.
//
// Deliberately no Cap: [wait.Backoff] stops stepping once the next interval
// would exceed it, which would cost an attempt without saving any time.
var updateStatusBackoff = wait.Backoff{
	Steps:    6,
	Duration: 25 * time.Millisecond,
	Factor:   2.0,
	Jitter:   0.1,
}

// UpdateSignalStatus records newStatus, which must not be nil, as the signaling
// status of the signal node identified by key. It reports whether the status
// was actually recorded, which is false when the transition turned out to no
// longer be applicable. Callers with a side effect to match against the write,
// such as reporting an event, need to check it.
//
// Reconcilers download, drain or replace binaries before recording the
// resulting status, based on a snapshot of the signal node taken when the
// reconcile started. That snapshot is likely stale by the time the work is
// done, so it must not be written back wholesale. Writing it as-is would
// overwrite signaling that arrived while the work was running, such as a newer
// plan for the same node. Writing it with optimistic concurrency instead fails
// whenever something else modified the object meanwhile, and since a failed
// write makes the reconciler redo the work, every retry loses the same race
// again.
//
// So re-read the node and record the status on the fresh object, but only if
// that is still applicable: the node has to carry the same signaling request,
// and be in one of the validFrom statuses, where the empty string stands for a
// node without a status yet. This is the precondition the reconciler checked
// before it started, re-evaluated against the current state.
//
// validFrom is a set rather than an exact match because the status can move on
// without invalidating the work. For airgap updates the download and signal
// reconcilers are triggered by the same status-less event, so the signal
// reconciler moves the node to Downloading while the download is already
// running.
//
// Reads may be served from a cache that lags behind the API, so a write can
// still conflict. That is what the retries are for, and the optimistic
// concurrency keeps a lagging read from overwriting anything. The retries are
// more patient than [retry.DefaultRetry] because a conflict means the cache has
// yet to observe the write that caused it, and giving up after a few
// milliseconds would only make the reconciler redo all of its work.
func UpdateSignalStatus(
	ctx context.Context,
	logger *logrus.Entry,
	client crcli.Client,
	delegate apdel.ControllerDelegate,
	key types.NamespacedName,
	actedUpon apsigv2.SignalData,
	validFrom []string,
	newStatus *apsigv2.Status,
) (recorded bool, _ error) {
	err := retry.RetryOnConflict(updateStatusBackoff, func() error {
		recorded = false // this attempt may follow one that got as far as the write

		signalNode := delegate.CreateObject()
		if err := client.Get(ctx, key, signalNode); err != nil {
			return fmt.Errorf("unable to get signal node: %w", err)
		}

		var current apsigv2.SignalData
		if err := current.Unmarshal(signalNode.GetAnnotations()); err != nil {
			return fmt.Errorf("unable to unmarshal signal data: %w", err)
		}

		// A different request altogether: recording the status now would undo
		// whatever superseded the one that was acted upon. That change will
		// have triggered a reconcile of its own, so there's nothing to do here.
		if !isSameRequest(current, actedUpon) {
			logger.Infof("Not recording status '%s': the signaling request changed while it was being processed", newStatus.Status)
			return nil
		}

		// The same request, but no longer in a state this transition applies
		// to, i.e. it has already moved past it. Recording the status now would
		// take the state machine backwards.
		var currentStatus string
		if current.Status != nil {
			currentStatus = current.Status.Status
		}
		if !slices.Contains(validFrom, currentStatus) {
			logger.Infof("Not recording status '%s': the signal node has moved on to '%s'", newStatus.Status, currentStatus)
			return nil
		}

		current.Status = newStatus
		if err := current.Marshal(signalNode.GetAnnotations()); err != nil {
			return fmt.Errorf("unable to marshal signal data: %w", err)
		}

		logger.Infof("Updating signaling response to '%s'", newStatus.Status)
		if err := client.Update(ctx, signalNode, &crcli.UpdateOptions{}); err != nil {
			return err
		}

		recorded = true
		return nil
	})

	return recorded, err
}

// isSameRequest reports whether both describe the same signaling request,
// disregarding the status, which is what the transitions in here change.
func isSameRequest(a, b apsigv2.SignalData) bool {
	a.Status, b.Status = nil, nil
	return reflect.DeepEqual(a, b)
}
