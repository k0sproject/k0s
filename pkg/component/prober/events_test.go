package prober

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEvents(t *testing.T) {
	t.Run("prober_collects_emitted_events", func(t *testing.T) {
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
		prober.Run(context.Background())
		state := prober.State()
		assert.Len(t, prober.withEventComponents, 1)
		assert.Len(t, state.Events, 1, "should have 1 component with events")
		assert.Len(t, state.Events["component_with_events"], 3, "should have 3 events in the state due to ring buffer size")
		assert.Equal(t, eventsSent[1:], state.Events["component_with_events"], "should have 3 last events in the state due to ring buffer size")
	})

	t.Run("embedded_emitter_works_as_expected", func(t *testing.T) {
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
		prober.Run(context.Background())
		state := prober.State()
		assert.Len(t, state.Events, 1)
		assert.Len(t, state.Events["component_with_events"], 3)
	})

	t.Run("emitter_never_blocks", func(t *testing.T) {
		// this test is to ensure that the emitter never blocks even if the channel is full
		// in negative case the test will fail with timeout
		emitter := &EventEmitter{
			events: make(chan Event, 10),
		}
		for i := 0; i < 20; i++ {
			emitter.Emit("Test event")
		}
	})

	t.Run("emitter_observes_events_emited_by_components_registred_after_run_is_called", func(t *testing.T) {
		prober := testProber(0)
		ctx, cancel := context.WithCancel(context.Background())
		go prober.Run(ctx)
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
		time.Sleep(time.Second)
		st := prober.State()
		assert.Len(t, st.Events, 1)
		assert.Len(t, st.Events["component_with_events"], 3)
		cancel()
	})
}

type mockComponentWithEvents struct {
	ch chan Event
}

func (m mockComponentWithEvents) Events() chan Event {
	return m.ch
}

func (m mockComponentWithEvents) sendEvents(events ...Event) error {
	for _, e := range events {
		m.ch <- e
	}
	return nil
}

func newMockWithEvents(cap int32) mockComponentWithEvents {
	return mockComponentWithEvents{
		ch: make(chan Event, cap),
	}
}
