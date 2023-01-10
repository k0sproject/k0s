/*
Copyright 2022 k0s authors

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

package net_test

import (
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/net"
	"github.com/stretchr/testify/assert"
)

func TestNewHostPort(t *testing.T) {
	for _, test := range []struct {
		name                         string
		host                         string
		port                         uint16
		expectedHost, expectedString string
	}{
		{"ipv4", "127.0.0.1", 4711, "127.0.0.1", "127.0.0.1:4711"},
		{"ipv6", "::1", 4711, "::1", "[::1]:4711"},
		{"dns", "example.com", 4711, "example.com", "example.com:4711"},
	} {
		t.Run(test.name, func(t *testing.T) {
			hostPort, err := net.NewHostPort(test.host, test.port)
			if assert.NoError(t, err) && assert.NotNil(t, hostPort) {
				assert.Equal(t, test.expectedHost, hostPort.Host())
				assert.Equal(t, test.port, hostPort.Port())
				assert.Equal(t, test.expectedString, hostPort.String())
			}
		})
	}

	for _, test := range []struct {
		name   string
		host   string
		port   uint16
		errMsg string
	}{
		{"spaces", "f o o", 4711, "host is neither an IP address nor a DNS name"},
		{"zero_port", "foo", 0, "port is zero"},
	} {
		t.Run(test.name, func(t *testing.T) {
			hostPort, err := net.NewHostPort(test.host, test.port)
			assert.Nil(t, hostPort)
			if assert.Error(t, err) {
				assert.Equal(t, test.errMsg, err.Error())
			}
		})
	}
}

func TestParseHostPort(t *testing.T) {
	for _, test := range []struct {
		name, hostPort string
		host           string
		port           uint16
		str            string
	}{
		{"ipv4", "127.0.0.1:4711", "127.0.0.1", 4711, "127.0.0.1:4711"},
		{"ipv6", "[::1]:4711", "::1", 4711, "[::1]:4711"},
		{"dns", "example.com:4711", "example.com", 4711, "example.com:4711"},
	} {
		t.Run(test.name, func(t *testing.T) {
			hostPort, err := net.ParseHostPort(test.hostPort)
			if assert.NoError(t, err) && assert.NotNil(t, hostPort) {
				assert.Equal(t, test.host, hostPort.Host())
				assert.Equal(t, test.port, hostPort.Port())
				assert.Equal(t, test.str, hostPort.String())
			}
		})
	}

	for _, test := range []struct{ name, hostPort, errMsg string }{
		{"spaces", "f o o:4711", "host is neither an IP address nor a DNS name"},
		{"missing_port", "foo", "missing port in address"},
		{"empty_port", "foo:", `port is not a positive number: ""`},
		{"zero_port", "foo:0", "port is zero"},
		{"negative_port", "foo:-1", `port is not a positive number: "-1"`},
		{"big_port", "foo:65536", "port is out of range: 65536"},
	} {
		t.Run(test.name, func(t *testing.T) {
			hostPort, err := net.ParseHostPort(test.hostPort)
			assert.Nil(t, hostPort)
			if assert.Error(t, err) {
				assert.Equal(t, test.errMsg, err.Error())
			}
		})
	}
}

func TestParseHostPortWithDefault(t *testing.T) {
	hostPort, err := net.ParseHostPortWithDefault("yep", 4711)
	if assert.NoError(t, err) && assert.NotNil(t, hostPort) {
		assert.Equal(t, "yep", hostPort.Host())
		assert.Equal(t, uint16(4711), hostPort.Port())
		assert.Equal(t, "yep:4711", hostPort.String())
	}

	hostPort, err = net.ParseHostPortWithDefault("yep", 0)
	assert.Nil(t, hostPort)
	if assert.Error(t, err) {
		assert.Equal(t, "missing port in address", err.Error())
	}
}
