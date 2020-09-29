package sonobuoy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var testOutput = `
Plugin: e2e
Status: passed
Total: 5232
Passed: 32
Failed: 0
Skipped: 5200
`

func TestParsing(t *testing.T) {
	a := assert.New(t)

	r, err := ResultFromString(testOutput)

	a.NoError(err)

	a.Equal("e2e", r.Plugin)
	a.Equal("passed", r.Status)
	a.Equal(5232, r.Total)
	a.Equal(0, r.Failed)
	a.Equal(5200, r.Skipped)

}
