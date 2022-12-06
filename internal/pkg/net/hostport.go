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

package net

import (
	"encoding"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/asaskevich/govalidator"
)

// HostPort represents a host and port combination. The port is not optional.
type HostPort struct {
	host string
	port uint16
}

func (h *HostPort) Host() string { return h.host }
func (h *HostPort) Port() uint16 { return h.port }

func NewHostPort(host string, port uint16) (*HostPort, error) {
	if !govalidator.IsIP(host) && !govalidator.IsDNSName(host) {
		return nil, fmt.Errorf("host is neither an IP address nor a DNS name")
	}

	if port == 0 {
		return nil, errors.New("port is zero")
	}

	return &HostPort{host, port}, nil
}

func ParseHostPort(hostPort string) (*HostPort, error) {
	return ParseHostPortWithDefault(hostPort, 0)
}

func ParseHostPortWithDefault(hostPort string, defaultPort uint16) (*HostPort, error) {
	var port uint16
	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		addrErr := &net.AddrError{}
		if errors.As(err, &addrErr) {
			if defaultPort != 0 && addrErr.Err == "missing port in address" {
				host = addrErr.Addr
				port = defaultPort
			} else {
				return nil, errors.New(addrErr.Err)
			}
		} else {
			return nil, err
		}
	} else {
		parsed, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			switch {
			case errors.Is(err, strconv.ErrSyntax):
				err = fmt.Errorf("port is not a positive number: %q", portStr)
			case errors.Is(err, strconv.ErrRange):
				err = fmt.Errorf("port is out of range: %s", portStr)
			default:
				err = fmt.Errorf("invalid port: %q: %w", portStr, err)
			}

			return nil, err
		}
		port = uint16(parsed)
	}

	return NewHostPort(host, port)
}

func (h *HostPort) String() string {
	return net.JoinHostPort(h.host, strconv.FormatUint(uint64(h.port), 10))
}

var (
	_ encoding.TextMarshaler   = (*HostPort)(nil)
	_ encoding.TextUnmarshaler = (*HostPort)(nil)
)

// MarshalText implements [encoding.TextMarshaler].
func (h *HostPort) MarshalText() (text []byte, err error) {
	return []byte(h.String()), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler].
func (h *HostPort) UnmarshalText(text []byte) error {
	parsed, err := ParseHostPort(string(text))
	if err != nil {
		return err
	}
	*h = *parsed
	return err
}
