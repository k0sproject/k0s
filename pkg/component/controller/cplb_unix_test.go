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
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CPLBUnixSuite struct {
	suite.Suite
}

func (s *CPLBUnixSuite) TestDelayLoopString() {
	tests := []struct {
		name     string
		duration metav1.Duration
		output   string
	}{
		{
			name:     "2 seconds",
			duration: metav1.Duration{Duration: 2 * time.Second},
			output:   "2",
		},
		{
			name:     "1234 microseconds",
			duration: metav1.Duration{Duration: 1234 * time.Microsecond},
			output:   "0.001234",
		},
		{
			name:     "1.5 seconds",
			duration: metav1.Duration{Duration: 1500 * time.Millisecond},
			output:   "1.5",
		},
		{
			name:     "2 hours",
			duration: metav1.Duration{Duration: 2 * time.Hour},
			output:   "7200",
		},
	}
	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			s.Equal(tt.output, delayLoopString(tt.duration))
		})
	}
}

func TestCPLUnixSuite(t *testing.T) {
	cplUnixSuite := &CPLBUnixSuite{}

	suite.Run(t, cplUnixSuite)
}
