package probes

import (
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIECBytes_String(t *testing.T) {

	for _, data := range []struct {
		v   uint64
		str string
	}{
		{0, "0 B"},
		{1*Ki - 1, "1023 B"},
		{1 * Ki, "1.0 KiB"},
		{1*Mi - 1, "1024.0 KiB"},
		{1 * Mi, "1.0 MiB"},
		{math.MaxUint64, "16.0 EiB"},
	} {
		t.Run(strconv.FormatUint(data.v, 10), func(t *testing.T) {
			assert.Equal(t, data.str, iecBytes(data.v).String())
		})
	}
}
