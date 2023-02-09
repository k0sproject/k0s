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

package performance

import (
	"errors"
	"time"

	"github.com/sirupsen/logrus"
)

// The Timer is a performance measuring tool. You should enable bufferOutput if you want to
// programmatically decide when to output messages. If buffering is disabled, Checkpoint
// timings will be logged immediately when recorded. Buffering can be useful when you
// want to see all the recorded timings in a single place, to make comparison easy.
type Timer struct {
	log          *logrus.Entry
	bufferOutput bool
	startedAt    time.Time
	buffer       []checkpoint
}

type checkpoint struct {
	duration time.Duration
	name     string
	err      error
}

func NewTimer(name string) *Timer {
	return &Timer{
		log:          logrus.WithField("component", "performance-timer").WithField("target", name),
		bufferOutput: false,
	}
}

// Buffer will enable buffering
func (t *Timer) Buffer() *Timer {
	t.bufferOutput = true

	return t
}

// Start will start the timer. It returns itself to allow easy chaining of create + start
func (t *Timer) Start() *Timer {
	t.startedAt = time.Now()

	return t
}

// Checkpoint records the time since the timer was started
func (t *Timer) Checkpoint(name string) {
	// if the timer was never started, we'll record an errored checkpoint that Output can recognise
	if t.startedAt.IsZero() {
		t.buffer = append(t.buffer, checkpoint{
			name: name,
			err:  errors.New("failed to record checkpoint, timer not started"),
		})
		return
	}

	t.buffer = append(t.buffer, checkpoint{
		duration: time.Since(t.startedAt),
		name:     name,
	})

	if !t.bufferOutput {
		t.Output()
	}
}

// Output will loop through the message buffer and output all messages in order.
func (t *Timer) Output() {
	for {
		if len(t.buffer) == 0 {
			return
		}

		checkpoint := t.buffer[0]
		t.buffer = t.buffer[1:]

		if checkpoint.err != nil {
			// even though this is an error, we use a debug log as the performance timer
			// should only ever output debug logs.
			t.log.
				WithField("checkpoint", checkpoint.name).
				WithError(checkpoint.err).
				Debug("failed to record checkpoint")
			continue
		}

		t.log.
			WithField("checkpoint", checkpoint.name).
			WithField("duration", checkpoint.duration.String()).
			Debug("checkpoint recorded")
	}
}
