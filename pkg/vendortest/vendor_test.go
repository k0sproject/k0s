package vendortest

import (
	"testing"

	"rsc.io/qr"
)

func TestVendor(t *testing.T) {
	code := &qr.Code{}
	t.Log(code.Image())
	t.Log("build should fail because rsc.io/qr was added to go.mod, but was not commited to the vendor dif")
}
