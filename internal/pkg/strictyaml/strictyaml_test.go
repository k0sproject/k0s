/*
Copyright 2021 k0s authors

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

package strictyaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ClusterConfig cluster manifest
type testConfig struct {
	StringA string `yaml:"stringA"`
}

func TestYamlParser(t *testing.T) {
	t.Run("no_error_on_masked_fields", func(t *testing.T) {
		input := `
stringA: stringValue
stringB: shouldBeIgnoredAndGiveNoError
stringC:
  key: value
`
		tgt := testConfig{}
		err := YamlUnmarshalStrictIgnoringFields([]byte(input), &tgt, "stringC", "stringB")
		assert.NoError(t, err)
		assert.Equal(t, "stringValue", tgt.StringA)
	})

	t.Run("error_on_non_masked_fields", func(t *testing.T) {
		input := `
stringA: stringValue
stringB: shouldGiveErrorBecauseNotMasked
stringC:
  key: value
`
		tgt := testConfig{}
		err := YamlUnmarshalStrictIgnoringFields([]byte(input), &tgt, "stringC")
		assert.Error(t, err)
	})

	t.Run("error_on_invalid_yaml", func(t *testing.T) {
		input := `
	stringA: stringValue
stringB: shouldGiveErrorBecauseNotMasked
stringC:
  key: value
`
		tgt := testConfig{}
		err := YamlUnmarshalStrictIgnoringFields([]byte(input), &tgt)
		assert.Error(t, err)
	})
}
