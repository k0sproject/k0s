/*
Copyright 2025 k0s authors

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

package token

import (
	"bytes"
	"testing"

	"github.com/k0sproject/k0s/pkg/token"
	"github.com/stretchr/testify/assert"
)

func TestPrintTokens(t *testing.T) {
	// Mock tokens
	tokens := []token.Token{
		{ID: "token1", Role: "controller", Expiry: "2025-05-12T12:00:00Z"},
		{ID: "token2", Role: "worker", Expiry: "2025-05-13T12:00:00Z"},
		{ID: "token3", Role: "worker", Expiry: "2025-05-14T12:00:00Z"},
	}

	t.Run("controller Tokens", func(t *testing.T) {
		expectedOutput := "ID       ROLE         EXPIRES AT\n" +
			"token1   controller   2025-05-12T12:00:00Z\n"
		var output bytes.Buffer
		printTokens(&output, tokens, "controller")
		assert.Equal(t, expectedOutput, output.String())
	})
	t.Run("worker Tokens", func(t *testing.T) {
		expectedOutput := "ID       ROLE     EXPIRES AT\n" +
			"token2   worker   2025-05-13T12:00:00Z\n" +
			"token3   worker   2025-05-14T12:00:00Z\n"
		var output bytes.Buffer
		printTokens(&output, tokens, "worker")
		assert.Equal(t, expectedOutput, output.String())
	})
	t.Run("No tokens", func(t *testing.T) {
		var output bytes.Buffer
		printTokens(&output, []token.Token{}, "")
		assert.Empty(t, output.String())
	})
}
