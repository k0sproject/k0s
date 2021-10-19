package util

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// ClusterConfig cluster manifest
type testConfig struct {
	StringA string `yaml:"stringA"`
}

func TestDoNotFailOnObsoleteFields(t *testing.T) {
	t.Run("no_error_on_masked_fields", func(t *testing.T) {
		input := `
stringA: stringValue
stringB: shouldBeIgnoredAndGiveNoError
stringC: 
  key: value
`
		tgt := testConfig{}
		err := YamlUnmarshalStrictIgnoringFields([]byte(input), &tgt, []string{"stringC", "stringB"})
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
		err := YamlUnmarshalStrictIgnoringFields([]byte(input), &tgt, []string{"stringC"})
		assert.Error(t, err)
	})
}
