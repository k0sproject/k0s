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
	"time"
)

// Eventer is an interface for components that can emit events
type Eventer interface {
	Events() chan Event
}

// Event represents a component event
type Event struct {
	At      time.Time   `json:"at"`
	Message string      `json:"message"`
	Payload interface{} `json:"payload,omitempty"`
}

// EventEmitter is a helper object to emit events with fire and forget semantics
type EventEmitter struct {
	events chan Event
}

// Emit emits an event
func (e *EventEmitter) Emit(message string) {
	e.EmitWithPayload(message, nil)
}

// EmitWithPayload emits an event with a payload
func (e *EventEmitter) EmitWithPayload(message string, payload interface{}) {
	evt := Event{
		At:      time.Now(),
		Message: message,
		Payload: payload,
	}
	// try to send the event
	// if the channel is full, drop the oldest event and send the new one
	select {
	case e.events <- evt:
	default:
		<-e.events
		e.events <- evt
	}
}

// Events returns the channel where events are emitted
func (e *EventEmitter) Events() chan Event {
	return e.events
}

// NewEventEmitter creates a new EventEmitter
func NewEventEmitter() *EventEmitter {
	return &EventEmitter{
		events: make(chan Event, 10), // TODO: make queue size configurable
	}
}
