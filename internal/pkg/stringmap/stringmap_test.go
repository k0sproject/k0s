package stringmap

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToArgs(t *testing.T) {
	tests := []struct {
		name string
		args StringMap
		want []string
	}{
		{
			"basic",
			StringMap{
				"foo":   "bar",
				"bar":   "baf",
				"empty": "",
			},
			[]string{"foo=bar", "bar=baf", "empty="},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.args.ToArgs()
			sort.Strings(got)
			sort.Strings(tt.want)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMerge(t *testing.T) {
	original := StringMap{
		"foo": "bar",
	}

	original.Merge(StringMap{
		"another": "val",
		"foo":     "overridden",
	})

	assert.Equal(t, "overridden", original["foo"])
	assert.Equal(t, "val", original["another"])
}
