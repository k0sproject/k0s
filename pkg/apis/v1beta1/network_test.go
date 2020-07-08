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

	n.ServiceCIDR = "10.96.0.248/29"
	dns,err = n.DNSAddress()
	if dns != "10.96.0.250" {
		t.Errorf("DNSAddress in small %s network incorrect, got: %s, want: %s", n.ServiceCIDR, dns, "10.96.0.250")
	}
}
