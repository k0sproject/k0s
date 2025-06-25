// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"github.com/segmentio/analytics-go"
)

var segmentToken = ""

const heartbeatEvent = "cluster-heartbeat"

func IsEnabled() bool {
	return segmentToken != ""
}

func NewDefaultSegmentClient() analytics.Client {
	if !IsEnabled() {
		return nil
	}

	return analytics.New(segmentToken)
}
