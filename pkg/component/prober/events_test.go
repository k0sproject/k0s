// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package prober

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

// default maxEvents to display, has nothing to do with actual ring size
const maxEvents = 100

func TestEvents(t *testing.T) {
	t.Run("prober_collects_emitted_events", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			prober := testProber(5)
			component := newMockWithEvents(100)
			eventsSent := []Event{
				{
					At:      time.Now(),
					Message: "Test event 1",
				},
				{
					At:      time.Now(),
					Message: "Test event 2",
				},
				{
					At:      time.Now(),
					Message: "Test event 3",
				},
				{
					At:      time.Now(),
					Message: "Test event 4",
				},
			}
			component.sendEvents(eventsSent...)
			prober.Register("component_with_events", component)
			runProberToCompletion(t, prober, 0)
			state := prober.State(maxEvents)
			assert.Len(t, prober.withEventComponents, 1)
			assert.Len(t, state.Events, 1, "should have 1 component with events")
			assert.Len(t, state.Events["component_with_events"], 3, "should have 3 events in the state due to ring buffer size")
			assert.Equal(t, eventsSent[1:], state.Events["component_with_events"], "should have 3 last events in the state due to ring buffer size")
		})
	})

	t.Run("embedded_emitter_works_as_expected", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			emitter := &EventEmitter{
				events: make(chan Event, 3),
			}
			comp := struct {
				*EventEmitter
			}{EventEmitter: emitter}
			emitter.Emit("message1")
			emitter.Emit("message2")
			emitter.Emit("message3")
			prober := testProber(10)
			prober.Register("component_with_events", comp)
			runProberToCompletion(t, prober, 0)
			state := prober.State(maxEvents)
			assert.Len(t, state.Events, 1)
			assert.Len(t, state.Events["component_with_events"], 3)
		})
	})

	t.Run("emitter_never_blocks", func(t *testing.T) {
		// this test is to ensure that the emitter never blocks even if the channel is full
		// in negative case the test will fail with timeout
		emitter := &EventEmitter{
			events: make(chan Event, 10),
		}
		for range 20 {
			emitter.Emit("Test event")
		}
	})

	t.Run("emitter_observes_events_emitted_by_components_registred_after_run_is_called", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			prober := testProber(0)
			component := newMockWithEvents(3)
			prober.Register("component_with_events", component)
			component.sendEvents(
				Event{
					At:      time.Now(),
					Message: "Test event 1",
				},
				Event{
					At:      time.Now(),
					Message: "Test event 2",
				},
				Event{
					At:      time.Now(),
					Message: "Test event 3",
				},
			)

			runProberToCompletion(t, prober, 0)

			st := prober.State(maxEvents)
			assert.Len(t, st.Events, 1)
			assert.Len(t, st.Events["component_with_events"], 3)
		})
	})
	t.Run("multiple_emitters", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			emitter := &EventEmitter{
				events: make(chan Event, 3),
			}
			comp := struct {
				*EventEmitter
			}{EventEmitter: emitter}
			emitter.Emit("message1")
			emitter.Emit("message2")
			emitter.Emit("message3")

			emitter2 := &EventEmitter{
				events: make(chan Event, 3),
			}
			comp2 := struct {
				*EventEmitter
			}{EventEmitter: emitter2}
			_ = comp2
			emitter2.Emit("message4")
			emitter2.Emit("message5")
			prober := testProber(10)
			prober.Register("component_with_events", comp)
			prober.Register("component_with_events2", comp2)
			runProberToCompletion(t, prober, 0)
			state := prober.State(maxEvents)

			assert.Len(t, state.Events, 2)
			assert.Len(t, state.Events["component_with_events"], 3)
			assert.Len(t, state.Events["component_with_events2"], 2)
		})
	})
}

type mockComponentWithEvents struct {
	ch chan Event
}

func (m mockComponentWithEvents) Events() chan Event {
	return m.ch
}

func (m mockComponentWithEvents) sendEvents(events ...Event) {
	for _, e := range events {
		m.ch <- e
	}
}

func newMockWithEvents(cap int32) mockComponentWithEvents {
	return mockComponentWithEvents{
		ch: make(chan Event, cap),
	}
}
