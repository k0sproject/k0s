package debounce

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/fsnotify.v1"
)

func TestDebounce(t *testing.T) {
	eventChan := make(chan fsnotify.Event)
	var debounceCalled atomic.Value
	debounceCalled.Store(0)

	var lastEvent atomic.Value
	lastEvent.Store("")
	debouncer := New(1*time.Second, eventChan, func(arg fsnotify.Event) {
		val := debounceCalled.Load().(int)
		val++
		debounceCalled.Store(val)
		lastEvent.Store(arg.Name)
	})
	go debouncer.Start()

	for i := 0; i < 5; i++ {
		eventChan <- fsnotify.Event{Name: fmt.Sprintf("event#%d", i)}
	}
	time.Sleep(2 * time.Second)
	debouncer.Stop()
	assert.Equal(t, 1, debounceCalled.Load())
	assert.Equal(t, "event#4", lastEvent.Load())
}

func TestDebounceStopWithoutActuallyDebouncing(t *testing.T) {
	eventChan := make(chan fsnotify.Event)
	var debounceCalled atomic.Value
	debounceCalled.Store(0)

	var lastEvent atomic.Value
	lastEvent.Store("")
	debouncer := New(10*time.Second, eventChan, func(arg fsnotify.Event) {
		val := debounceCalled.Load().(int)
		val++
		debounceCalled.Store(val)
		lastEvent.Store(arg.Name)

	})
	var startReturned atomic.Value
	startReturned.Store(false)
	go func() {
		debouncer.Start()
		startReturned.Store(true)
	}()
	for i := 0; i < 5; i++ {
		eventChan <- fsnotify.Event{Name: fmt.Sprintf("event#%d", i)}
	}
	debouncer.Stop()
	time.Sleep(10 * time.Millisecond)
	assert.True(t, startReturned.Load().(bool))
	assert.Equal(t, 0, debounceCalled.Load())
	assert.Equal(t, "", lastEvent.Load())
}
