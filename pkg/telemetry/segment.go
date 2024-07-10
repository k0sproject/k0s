/*
Copyright 2020 k0s authors

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
