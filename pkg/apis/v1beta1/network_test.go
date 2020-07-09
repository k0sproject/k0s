package v1beta1

import (
	"testing"
)


func TestDNSAddress(t *testing.T) {
	n := DefaultNetwork()
	dns,err := n.DNSAddress()

	if dns != "10.96.0.10" {
		t.Errorf("DNSAddress in default %s network incorrect, got: %s, want: %s", n.ServiceCIDR, dns, "10.96.0.10")
	}
	if err !=nil {
		t.Error("DNSAddress unexpectedy returned error on default network")
	}

	api, err := n.InternalAPIAddress()
	if api != "10.96.0.1" {
		t.Errorf("InternalAPIAddress in default %s network incorrect, got: %s, want: %s", n.ServiceCIDR, api, "10.96.0.1")
	}

	n.ServiceCIDR = "10.96.0.248/29"
	dns,err = n.DNSAddress()
	if dns != "10.96.0.250" {
		t.Errorf("DNSAddress in small %s network incorrect, got: %s, want: %s", n.ServiceCIDR, dns, "10.96.0.250")
	}

	api, err = n.InternalAPIAddress()
	if api != "10.96.0.249" {
		t.Errorf("InternalAPIAddress in default %s network incorrect, got: %s, want: %s", n.ServiceCIDR, api, "10.96.0.249")
	}
}
