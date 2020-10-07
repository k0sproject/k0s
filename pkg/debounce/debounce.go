// Package debounce provides functionality to "debounce" multiple events in given interval and handle all at once
// For debounce pattern, see https://drailing.net/2018/01/debounce-function-for-golang/
// As you can see, we draw some inspiration from that example. :)
package debounce

import (
	"context"
	"time"

	"gopkg.in/fsnotify.v1"
)

// Callback defines the callback function when we trigger the debounce action
type Callback func(arg fsnotify.Event)

// Debouncer defines the debouncer interface
type Debouncer interface {
	Stop()
	Start()
}

type debouncer struct {
	ctx      context.Context
	cancel   context.CancelFunc
	interval time.Duration
	callback Callback
	input    chan fsnotify.Event
}

// New creates new Debouncer with given args
func New(interval time.Duration, input chan fsnotify.Event, callback Callback) Debouncer {
	ctx, cancel := context.WithCancel(context.Background())
	db := &debouncer{
		cancel:   cancel,
		interval: interval,
		input:    input,
		ctx:      ctx,
		callback: callback,
	}

	return db
}

// Start starts the debouncer
func (d *debouncer) Start() {
	ticker := time.NewTimer(d.interval)
	var item *fsnotify.Event
	for {
		select {
		case event := <-d.input:
			item = &event
			ticker.Reset(d.interval)
		case <-ticker.C:
			if item != nil {
				d.callback(*item)
			}
		case <-d.ctx.Done():
			return
		}
	}
}

// Stop stops the debouncer, the blocking call to "Start()" wil lreturn and no more callback inivocations are done.
// Not even if there would be "pending" events in the buffer.
func (d *debouncer) Stop() {
	d.cancel()
}
