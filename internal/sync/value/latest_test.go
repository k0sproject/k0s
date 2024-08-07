/*
Copyright 2024 k0s authors

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
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-expired
		got, _ = underTest.Peek()
	}()

	time.Sleep(10 * time.Millisecond) // Simulate some delay
	underTest.Set(42)
	wg.Wait()

	assert.Equal(t, 42, got)

	newValue, newExpired := underTest.Peek()
	assert.Equal(t, 42, newValue)
	assert.NotEqual(t, expired, newExpired)
}
