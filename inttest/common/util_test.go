// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
	"io"
	"syscall"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
		{name: "EOF", err: io.EOF, expectedDelay: defaultRetryDelay, expectedError: nil},
		{name: "retryAfter(42)", err: retryAfterError(42), expectedDelay: 42 * time.Second, expectedError: nil},

		// The fallthrough case, don't expect retries with this.
		{name: "AnError", err: assert.AnError, expectedDelay: 0, expectedError: assert.AnError},
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

type retryAfterError int32

func (e retryAfterError) Error() string {
	return fmt.Sprintf("retryAfterError(%d)", int32(e))
}

func (e retryAfterError) Status() metav1.Status {
	return metav1.Status{
		Reason:  metav1.StatusReasonServerTimeout,
		Details: &metav1.StatusDetails{RetryAfterSeconds: int32(e)},
	}
}
