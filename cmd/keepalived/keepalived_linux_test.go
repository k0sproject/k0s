// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package keepalived

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnscapeSingleQuotes(t *testing.T) {
	assert.Equal(t, `It's a "nice" 'test'`, unescapeSingleQuotes(`It\'s a "nice" \'test'`))
}
