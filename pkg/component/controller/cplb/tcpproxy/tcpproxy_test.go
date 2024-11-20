// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Modifications made by Mirantis Inc., 2024.
// Copyright 2017 Google Inc.
//
// Copyright 2024 Mirantis, Inc.

package tcpproxy

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"testing"
)

func TestProxyStartNone(t *testing.T) {
	var p Proxy
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
}

func newLocalListener(t *testing.T) net.Listener {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		ln, err = net.Listen("tcp", "[::1]:0")
		if err != nil {
			t.Fatal(err)
		}
	}
	return ln
}

const testFrontAddr = "1.2.3.4:567"

func testListenFunc(t *testing.T, ln net.Listener) func(network, laddr string) (net.Listener, error) {
	return func(network, laddr string) (net.Listener, error) {
		if network != "tcp" {
			t.Errorf("got Listen call with network %q, not tcp", network)
			return nil, errors.New("invalid network")
		}
		if laddr != testFrontAddr {
			t.Fatalf("got Listen call with laddr %q, want %q", laddr, testFrontAddr)
			panic("bogus address")
		}
		return ln, nil
	}
}

func testProxy(t *testing.T, front net.Listener) *Proxy {
	return &Proxy{
		ListenFunc: testListenFunc(t, front),
	}
}

func TestBufferedClose(t *testing.T) {
	front := newLocalListener(t)
	defer front.Close()
	back := newLocalListener(t)
	defer back.Close()

	p := testProxy(t, front)
	p.AddRoute(testFrontAddr, To(back.Addr().String()))
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}

	toFront, err := net.Dial("tcp", front.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer toFront.Close()

	fromProxy, err := back.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer fromProxy.Close()
	const msg = "message"
	if _, err := io.WriteString(toFront, msg); err != nil {
		t.Fatal(err)
	}
	// actively close toFront, the write should still make to the back.
	toFront.Close()

	buf := make([]byte, len(msg))
	if _, err := io.ReadFull(fromProxy, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != msg {
		t.Fatalf("got %q; want %q", buf, msg)
	}
}

func TestProxyAlwaysMatch(t *testing.T) {
	front := newLocalListener(t)
	defer front.Close()
	back := newLocalListener(t)
	defer back.Close()

	p := testProxy(t, front)
	p.AddRoute(testFrontAddr, To(back.Addr().String()))
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}

	toFront, err := net.Dial("tcp", front.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer toFront.Close()

	fromProxy, err := back.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer fromProxy.Close()
	const msg = "message"
	_, _ = io.WriteString(toFront, msg)

	buf := make([]byte, len(msg))
	if _, err := io.ReadFull(fromProxy, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != msg {
		t.Fatalf("got %q; want %q", buf, msg)
	}
}

func TestProxyPROXYOut(t *testing.T) {
	front := newLocalListener(t)
	defer front.Close()
	back := newLocalListener(t)
	defer back.Close()

	p := testProxy(t, front)
	p.AddRoute(testFrontAddr, &DialProxy{
		Addr:                 back.Addr().String(),
		ProxyProtocolVersion: 1,
	})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}

	toFront, err := net.Dial("tcp", front.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	_, _ = io.WriteString(toFront, "foo")
	toFront.Close()

	fromProxy, err := back.Accept()
	if err != nil {
		t.Fatal(err)
	}

	bs, err := ioutil.ReadAll(fromProxy)
	if err != nil {
		t.Fatal(err)
	}

	want := fmt.Sprintf("PROXY TCP4 %s %s %d %d\r\nfoo", toFront.LocalAddr().(*net.TCPAddr).IP, toFront.RemoteAddr().(*net.TCPAddr).IP, toFront.LocalAddr().(*net.TCPAddr).Port, toFront.RemoteAddr().(*net.TCPAddr).Port)
	if string(bs) != want {
		t.Fatalf("got %q; want %q", bs, want)
	}
}

func TestSetRoutes(t *testing.T) {

	var p Proxy
	ipPort := ":8080"
	p.AddRoute(ipPort, To("127.0.0.2:8080"))
	cfg := p.configFor(ipPort)

	expectedAddrsList := [][]string{
		{"127.0.0.1:80"},
		{"127.0.0.1:80", "127.0.0.1:443"},
		{},
		{"127.0.0.1:80"},
	}

	for _, expectedAddrs := range expectedAddrsList {
		p.setRoutes(ipPort, stringsToTargets(expectedAddrs))
		if !equalRoutes(cfg.routes, expectedAddrs) {
			t.Fatalf("got %v; want %v", cfg.routes, expectedAddrs)
		}
	}
}

func stringsToTargets(s []string) []Target {
	targets := make([]Target, len(s))
	for i, v := range s {
		targets[i] = To(v)
	}

	return targets
}
func equalRoutes(routes []route, expectedAddrs []string) bool {
	if len(routes) != len(expectedAddrs) {
		return false
	}

	for i := range routes {
		addr := routes[i].(fixedTarget).t.(*DialProxy).Addr
		if addr != expectedAddrs[i] {
			return false
		}
	}
	return true
}
