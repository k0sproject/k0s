// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package prober

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestHealthChecks(t *testing.T) {

	t.Run("prober_stores_not_more_than_n_last_results_for_each_component", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {

			prober := testProber(9)
			prober.Register("test", &mockComponent{})
			prober.Register("test2", &mockComponent{})
			runProberToCompletion(t, prober, 9*time.Millisecond)
			st := prober.State(maxEvents)
			assert.Len(t, prober.withHealthComponents, 2)
			assert.Len(t, st.HealthProbes, 2, "should have 2 components in the state")
			assert.Len(t, st.HealthProbes["test"], 3, "should have 3 results for test component even after 9 iterations")
			assert.Len(t, st.HealthProbes["test2"], 3, "should have 3 results for test2 component even after 9 iterations")
		})
	})

	t.Run("prober_stores_and_overrides_error_results", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			prober := testProber(5)

			prober.Register("test", &mockComponent{
				errors: []error{nil, nil, errors.New("test1 error"), nil, nil},
			})

			prober.Register("test2", &mockComponent{
				errors: []error{nil, errors.New("test2 error"), nil, nil, nil},
			})

			prober.Register("test3", &mockComponent{
				errors: []error{nil, nil, nil, nil, errors.New("test3 error")},
			})
			runProberToCompletion(t, prober, 5*time.Millisecond)
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
	})
	t.Run("state_has_no_more_than_requested_count_of_events_and_probes", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			prober := testProber(5)

			prober.Register("test", &mockComponent{
				errors: []error{nil, nil, errors.New("test1 error"), nil, nil},
			})

			prober.Register("test2", &mockComponent{
				errors: []error{nil, errors.New("test2 error"), nil, nil, nil},
			})

			prober.Register("test3", &mockComponent{
				errors: []error{nil, nil, nil, nil, errors.New("test3 error")},
			})
			runProberToCompletion(t, prober, 5*time.Millisecond)
			st := prober.State(1)
			assert.Len(t, prober.withHealthComponents, 3)
			assert.Len(t, st.HealthProbes, 3, "should have 3 components in the state")
			assert.Len(t, st.HealthProbes["test"], 1, "should have 1 result for test component")
			assert.Len(t, st.HealthProbes["test2"], 1, "should have 1 result for test2 component")
			assert.Len(t, st.HealthProbes["test3"], 1, "should have 1 result for test2 component")

		})
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

func runProberToCompletion(t *testing.T, prober *Prober, duration time.Duration) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var done atomic.Bool
	go func() { defer done.Store(true); prober.Run(ctx) }()

	synctest.Wait() // Wait until all events have been processed.
	assert.False(t, done.Load(), "Prober exited unexpectedly")

	if duration > 0 {
		time.Sleep(duration)
	} else {
		cancel()
	}

	synctest.Wait() // Wait until the prober exited.
	assert.True(t, done.Load(), "Prober didn't exit")
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
		assert.NoError(t, err, "marshaling ProbeError shouldn't fail")

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
