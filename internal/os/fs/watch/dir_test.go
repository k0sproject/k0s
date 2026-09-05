// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/sync/value"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDir(t *testing.T) {
	t.Run("non-existent", func(t *testing.T) {
		nonExistent := filepath.Join(t.TempDir(), "non-existent")
		err := Dir(t.Context(), nonExistent, HandlerFunc(func(e Event) {
			assert.Fail(t, "Unexpected event", "%v", e)
		}))
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("populated", func(t *testing.T) {
		dir := t.TempDir()

		lorem := filepath.Join(dir, "lorem")
		ipsum := filepath.Join(dir, "ipsum")
		dolor := filepath.Join(dir, "dolor")
		sit := filepath.Join(dir, "sit")
		amet := filepath.Join(dir, "amet")

		require.NoError(t, os.WriteFile(ipsum, nil, 0644))
		require.NoError(t, os.WriteFile(dolor, nil, 0644))
		require.NoError(t, os.WriteFile(sit, nil, 0644))

		ctx, cancel := context.WithCancelCause(t.Context())
		defer cancel(nil)

		var events value.Latest[[]Event]
		handler := HandlerFunc(func(e Event) {
			if changed, ok := e.(*Changed); ok && changed.PathInfo != PathGone {
				// Remove the info part so we can use equality checks.
				e = &Changed{Name: changed.Name}
			}
			prev, _ := events.Peek()
			events.Set(slices.Concat(prev, []Event{e}))
		})

		expected := []Event{
			&Established{Path: dir},
		}

		done := make(chan error, 1)
		_, changed := events.Peek()
		go func() { done <- Dir(ctx, dir, handler) }()

		select {
		case <-changed:
			events, _ := events.Peek()
			assert.Equal(t, expected, events)
		case err := <-done:
			require.Failf(t, "Returned unexpectedly", "%v", err)
		case <-time.After(2 * time.Second):
			require.Fail(t, "Didn't establish watch in time")
		}

		_, changed = events.Peek()
		require.NoError(t, os.WriteFile(amet, nil, 0444))
		t.Cleanup(func() { _ = os.Chmod(amet, 0644) })
		expected = append(expected, &Changed{Name: "amet"})

		select {
		case <-changed:
			events, _ := events.Peek()
			require.Equal(t, expected, events, "Mismatch after creation")
		case err := <-done:
			require.Failf(t, "Returned unexpectedly", "%v", err)
		case <-time.After(2 * time.Second):
			require.Fail(t, "Didn't emit touched event in time after creation")
		}

		require.NoError(t, os.Chmod(amet, 0644))

		// https://github.com/fsnotify/fsnotify/issues/487
		if runtime.GOOS != "windows" {
			_, changed = events.Peek()
			expected = append(expected, &Changed{Name: "amet"})

			select {
			case <-changed:
				events, _ := events.Peek()
				require.Equal(t, expected, events, "Mismatch after chmod")
			case err := <-done:
				require.Failf(t, "Returned unexpectedly", "%v", err)
			case <-time.After(2 * time.Second):
				require.Fail(t, "Didn't emit touched event in time after chmod")
			}
		}

		_, changed = events.Peek()
		require.NoError(t, os.WriteFile(lorem, nil, 0644))
		require.NoError(t, os.Remove(dolor))
		require.NoError(t, os.Remove(amet))

	deletion:
		for {
			select {
			case <-changed:
				newExpected := []Event{
					&Changed{Name: "lorem"},
					&Changed{"dolor", PathGone},
					&Changed{"amet", PathGone},
				}
				var actual []Event
				actual, changed = events.Peek()
				if len(actual) < len(expected)+len(newExpected) {
					t.Logf("actual: %#v", actual)
					continue
				}
				require.ElementsMatch(t, newExpected, actual[len(expected):], "Mismatch after deletion")
				expected = actual
				break deletion
			case err := <-done:
				require.Failf(t, "Returned unexpectedly", "%v", err)
			case <-time.After(2 * time.Second):
				actual, _ := events.Peek()
				require.Failf(t, "Didn't emit gone event in time after deletion", "%#v", actual)
			}
		}

		cancel(assert.AnError)
		require.Equalf(t, assert.AnError, context.Cause(ctx), "Context has been canceled prematurely")

		select {
		case err := <-done:
			assert.NoError(t, err)
			events, _ := events.Peek()
			assert.Equal(t, expected, events)
		case <-time.After(2 * time.Second):
			assert.Fail(t, "Didn't return in time")
		}
	})

	t.Run("chmodded", func(t *testing.T) {
		dir := t.TempDir()

		var events value.Latest[[]Event]
		handler := HandlerFunc(func(e Event) {
			if changed, ok := e.(*Changed); ok && changed.PathInfo != PathGone {
				// Remove the info part so we can use equality checks.
				e = &Changed{Name: changed.Name}
			}
			prev, _ := events.Peek()
			events.Set(slices.Concat(prev, []Event{e}))
		})

		done := make(chan error, 1)
		_, changed := events.Peek()
		go func() { done <- Dir(t.Context(), dir, handler) }()

		select {
		case <-changed:
			actual, _ := events.Peek()
			require.Equal(t, []Event{&Established{Path: dir}}, actual)
		case err := <-done:
			require.Failf(t, "Returned unexpectedly", "%v", err)
		case <-time.After(2 * time.Second):
			require.Fail(t, "Didn't establish watch in time")
		}

		// Touch the watched directory itself.
		require.NoError(t, os.Chmod(dir, 0755))
		// Then create a marker file to have an event that can be waited for.
		require.NoError(t, os.WriteFile(filepath.Join(dir, "marker"), nil, 0644))

		var actual []Event
		for {
			var changed <-chan struct{}
			actual, changed = events.Peek()
			if slices.ContainsFunc(actual, func(e Event) bool {
				changed, ok := e.(*Changed)
				return ok && changed.Name == "marker"
			}) {
				break
			}

			select {
			case <-changed:
			case err := <-done:
				require.Failf(t, "Returned unexpectedly", "%v", err)
			case <-time.After(2 * time.Second):
				require.Failf(t, "Didn't emit touched event in time after marker creation", "%v", actual)
			}
		}

		assert.NotContains(t, actual, Event(&Changed{Name: "."}),
			"Events for the watched directory itself weren't suppressed")
	})

	t.Run("symlinks", func(t *testing.T) {
		dir := t.TempDir()

		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		var (
			skip    bool
			changed *Changed
		)
		require.NoError(t, Dir(ctx, dir, HandlerFunc(func(e Event) {
			switch e := e.(type) {
			case *Established:
				err := os.Symlink("nonexistent", filepath.Join(dir, "dangling"))
				if err != nil {
					cancel()
					if errors.Is(err, errors.ErrUnsupported) {
						t.Log("Symlinks unsupported:", err)
						skip = true
					} else {
						assert.Fail(t, "Symlink creation failed", "%v", err)
					}
				}

			case *Changed:
				changed = e
				cancel()
			}
		})))

		if skip {
			t.SkipNow()
		}
		if t.Failed() {
			return
		}

		require.NotNil(t, changed, "No Changed event encountered")
		assert.Equal(t, "dangling", changed.Name)
		stat, err := changed.Stat()
		if assert.NoError(t, err, "Expected EntryInfo to not follow symlinks") {
			assert.Equal(t, fs.ModeSymlink, stat.Mode().Type())
		}
	})

	t.Run("removed", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(t.Context())
		defer cancel(nil)

		dir := t.TempDir()

		var established atomic.Bool
		done := make(chan error, 1)
		go func() {
			done <- Dir(ctx, dir, HandlerFunc(func(e Event) {
				switch e := e.(type) {
				case *Established:
					if assert.False(t, established.Swap(true)) {
						if err := os.Remove(dir); !assert.NoError(t, err) {
							cancel(err)
						}
					} else {
						cancel(errors.New("established more than once"))
					}
				default:
					cancel(fmt.Errorf("unexpected event: %v", e))
				}
			}))
		}()

		select {
		case err := <-done:
			assert.True(t, established.Load())
			assert.ErrorIs(t, err, ErrWatchedDirectoryGone)
		case <-time.After(2 * time.Second):
			require.Fail(t, "Didn't establish watch in time")
		}
	})
}
