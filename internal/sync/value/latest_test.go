// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package value_test

import (
	"sync"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/sync/value"

	"github.com/stretchr/testify/assert"
)

func TestLatest(t *testing.T) {
	var underTest value.Latest[int]
	value, expired := underTest.Peek()
	assert.Zero(t, value, "Zero latest should return zero value")
	assert.NotNil(t, expired)

	var got int
	var wg sync.WaitGroup
	wg.Go(func() {
		<-expired
		got, _ = underTest.Peek()
	})

	time.Sleep(10 * time.Millisecond) // Simulate some delay
	underTest.Set(42)
	wg.Wait()

	assert.Equal(t, 42, got)

	newValue, newExpired := underTest.Peek()
	assert.Equal(t, 42, newValue)
	assert.NotEqual(t, expired, newExpired)
}
