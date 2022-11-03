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
package common

import (
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRetryWatchErrors ensures that the ErrorCallback returned by RetryWatchErrors can
// properly identify known syscall errors.
func TestRetryWatchErrors_syscalls(t *testing.T) {
	defaultRetryDelay := 1 * time.Second

	tests := []struct {
		name          string
		err           error
		expectedDelay time.Duration
		expectedError error
	}{
		{name: "ECONNRESET", err: syscall.ECONNRESET, expectedDelay: defaultRetryDelay, expectedError: nil},
		{name: "ECONNREFUSED", err: syscall.ECONNREFUSED, expectedDelay: defaultRetryDelay, expectedError: nil},

		// The fallthrough case, don't expect retries with this.
		{name: "EBADFD", err: syscall.EBADFD, expectedDelay: 0, expectedError: syscall.EBADFD},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ecb := RetryWatchErrors(func(format string, args ...any) {})
			retryDelay, err := ecb(test.err)

			assert.Equal(t, test.expectedDelay, retryDelay)
			assert.Equal(t, test.expectedError, err)
		})
	}
}
