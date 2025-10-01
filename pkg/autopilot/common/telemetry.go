// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"github.com/k0sproject/k0s/pkg/telemetry"
	"github.com/segmentio/analytics-go/v3"
)

const eventName = "autopilotTransitionEvent"

var reporter = telemetry.NewDefaultSegmentClient()

type Event struct {
	ClusterID string
	OldStatus string
	NewStatus string
}

func ReportEvent(evt *Event) error {
	if reporter == nil {
		return nil
	}
	return reporter.Enqueue(analytics.Track{
		Event:      eventName,
		Properties: evt.asProperties(),
	})
}

func (e *Event) asProperties() analytics.Properties {
	return analytics.Properties{
		"clusterID": e.ClusterID,
		"oldStatus": e.OldStatus,
		"newStatus": e.NewStatus,
	}
}
