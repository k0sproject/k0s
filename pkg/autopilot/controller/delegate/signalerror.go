// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package delegate

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

// SignalErrorAnnotation is the annotation key used on both ControlNode and Node
// objects to persist the most recent signal processing failure.
const SignalErrorAnnotation = "k0sproject.io/autopilot-last-error"

// maxMessageLen is the maximum byte length for the message field before truncation.
const maxMessageLen = 1024

// SignalError holds details about a signal processing failure, stored as a JSON
// annotation on both ControlNode and Node objects.
type SignalError struct {
	PlanID    string      `json:"planID"`
	Reason    string      `json:"reason"`
	Message   string      `json:"message"`
	Timestamp metav1.Time `json:"timestamp"`
}

// truncate returns s truncated to max bytes, appending "..." when truncated.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// writeSignalErrorAnnotation writes a SignalError as a JSON annotation on obj.
// message is truncated before encoding to keep the annotation size bounded.
// reason is a short constant (e.g. "FailedDownload") and is not truncated.
func writeSignalErrorAnnotation(obj crcli.Object, planID, reason, message string) error {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	e := SignalError{
		PlanID:    planID,
		Reason:    reason,
		Message:   truncate(message, maxMessageLen),
		Timestamp: metav1.Now(),
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	annotations[SignalErrorAnnotation] = string(raw)
	obj.SetAnnotations(annotations)
	return nil
}

// readSignalErrorAnnotation reads the SignalError annotation from obj and
// returns "reason: message", or "" if absent or unparseable.
func readSignalErrorAnnotation(obj crcli.Object) string {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return ""
	}
	raw, ok := annotations[SignalErrorAnnotation]
	if !ok {
		return ""
	}
	var e SignalError
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return ""
	}
	return e.Reason + ": " + e.Message
}

// clearSignalErrorAnnotation removes the SignalError annotation from obj in-memory.
func clearSignalErrorAnnotation(obj crcli.Object) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return
	}
	delete(annotations, SignalErrorAnnotation)
	obj.SetAnnotations(annotations)
}
