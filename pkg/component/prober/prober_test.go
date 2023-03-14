/*
Copyright 2022 k0s authors

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

package prober

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestHealthChecks(t *testing.T) {

	t.Run("prober_stores_not_more_than_n_last_results_for_each_component", func(t *testing.T) {
		prober := testProber(9)
		prober.Register("test", &mockComponent{})
		prober.Register("test2", &mockComponent{})
		prober.Run(context.Background())
		st := prober.State(maxEvents)
		assert.Len(t, prober.withHealthComponents, 2)
		assert.Len(t, st.HealthProbes, 2, "should have 2 components in the state")
		assert.Len(t, st.HealthProbes["test"], 3, "should have 3 results for test component even after 9 iterations")
		assert.Len(t, st.HealthProbes["test2"], 3, "should have 3 results for test2 component even after 9 iterations")
	})

	t.Run("prober_stores_and_overrides_error_results", func(t *testing.T) {
		prober := testProber(5)

		prober.Register("test", &mockComponent{
			errors: []error{nil, nil, fmt.Errorf("test1 error"), nil, nil},
		})

		prober.Register("test2", &mockComponent{
			errors: []error{nil, fmt.Errorf("test2 error"), nil, nil, nil},
		})

		prober.Register("test3", &mockComponent{
			errors: []error{nil, nil, nil, nil, fmt.Errorf("test3 error")},
		})
		prober.Run(context.Background())
		st := prober.State(maxEvents)
		assert.Len(t, prober.withHealthComponents, 3)
		assert.Len(t, st.HealthProbes, 3, "should have 3 components in the state")
		assert.Len(t, st.HealthProbes["test"], 3, "should have 5 result for test component")
		assert.Len(t, st.HealthProbes["test2"], 3, "should have 5 result for test2 component")

		assert.Error(t, st.HealthProbes["test"][0].Error, "should have error in the first result for component `test`")
		assert.NoError(t, st.HealthProbes["test2"][0].Error, "should have no errors for component `test2`")
		assert.NoError(t, st.HealthProbes["test2"][1].Error, "should have no errors for component `test2`")
		assert.NoError(t, st.HealthProbes["test2"][2].Error, "should have no errors for component `test2`")

		assert.Error(t, st.HealthProbes["test3"][2].Error, "should have error in the last result for component `test3`")
	})
	t.Run("state_has_no_more_than_requested_count_of_events_and_probes", func(t *testing.T) {
		prober := testProber(5)

		prober.Register("test", &mockComponent{
			errors: []error{nil, nil, fmt.Errorf("test1 error"), nil, nil},
		})

		prober.Register("test2", &mockComponent{
			errors: []error{nil, fmt.Errorf("test2 error"), nil, nil, nil},
		})

		prober.Register("test3", &mockComponent{
			errors: []error{nil, nil, nil, nil, fmt.Errorf("test3 error")},
		})
		prober.Run(context.Background())
		st := prober.State(1)
		assert.Len(t, prober.withHealthComponents, 3)
		assert.Len(t, st.HealthProbes, 3, "should have 3 components in the state")
		assert.Len(t, st.HealthProbes["test"], 1, "should have 1 result for test component")
		assert.Len(t, st.HealthProbes["test2"], 1, "should have 1 result for test2 component")
		assert.Len(t, st.HealthProbes["test3"], 1, "should have 1 result for test2 component")

	})
}

func testProber(iterations int) *Prober {
	p := New()
	p.interval = 1 * time.Millisecond
	p.probesTrackLength = 3
	p.eventsTrackLength = 3
	p.stopAfterIterationNum = iterations
	l, _ := test.NewNullLogger()
	p.l = l.WithField("test", "prober")
	return p
}

type mockComponent struct {
	counter int
	errors  []error
}

func (mc *mockComponent) Healthy() error {
	if mc.counter >= len(mc.errors) {
		return nil
	}
	err := mc.errors[mc.counter]
	mc.counter++
	return err
}

func TestMarshalling(t *testing.T) {
	tests := []*ProbeResult{
		{
			Component: "foo",
			At:        time.Date(2023, time.March, 17, 12, 52, 35, 123578000, time.UTC),
			Error:     nil,
		},
		{
			Component: "bar",
			At:        time.Date(2023, time.March, 18, 12, 52, 35, 123578000, time.UTC),
			Error:     errors.New("bar"),
		},
	}

	for _, source := range tests {
		data, err := json.Marshal(source)
		assert.NoError(t, err, "marshalling ProbeError shouldn't fail")

		target := &ProbeResult{}
		err = json.Unmarshal(data, target)
		assert.NoError(t, err, "unmarshalling ProbeError shouldn't fail")
		assert.Equal(t, source.Component, target.Component)
		assert.Equal(t, source.At, target.At)
		if source.Error == nil {
			assert.Equal(t, errors.New(""), target.Error)
		} else {
			assert.Equal(t, source.Error, target.Error)
		}
	}

}
