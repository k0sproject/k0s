package util

import (
	"net"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func AllAddresses() ([]string, error) {
	addresses := make([]string, 0, 5)

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list network interfaces")
	}

	for _, a := range addrs {
		// check the address type and skip if loopback
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				addresses = append(addresses, ipnet.IP.String())
			}
		}
	}

	logrus.Debugf("found local addresses: %s", addresses)

	return addresses, nil
}
