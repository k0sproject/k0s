package debounce

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/fsnotify.v1"
)

func TestDebounce(t *testing.T) {
	eventChan := make(chan fsnotify.Event)
	var debounceCalled = 0
	var lastEvent string
	debouncer := New(1*time.Second, eventChan, func(arg fsnotify.Event) {
		debounceCalled++
		lastEvent = arg.Name
	})
	go debouncer.Start()

	for i := 0; i < 5; i++ {
		eventChan <- fsnotify.Event{Name: fmt.Sprintf("event#%d", i)}
	}
	time.Sleep(2 * time.Second)
	debouncer.Stop()
	assert.Equal(t, 1, debounceCalled)
	assert.Equal(t, "event#4", lastEvent)
}

func TestDebounceStopWithoutActuallyDebouncing(t *testing.T) {
	eventChan := make(chan fsnotify.Event)
	var debounceCalled = 0
	var lastEvent string
	debouncer := New(10*time.Second, eventChan, func(arg fsnotify.Event) {
		debounceCalled++
		lastEvent = arg.Name

	})
	startReturned := false
	go func() {
		debouncer.Start()
		startReturned = true
	}()
	for i := 0; i < 5; i++ {
		eventChan <- fsnotify.Event{Name: fmt.Sprintf("event#%d", i)}
	}
	debouncer.Stop()
	time.Sleep(10 * time.Millisecond)
	assert.True(t, startReturned)
	assert.Equal(t, 0, debounceCalled)
	assert.Equal(t, "", lastEvent)
}
