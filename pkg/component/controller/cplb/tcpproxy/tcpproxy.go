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

// Package tcpproxy lets users build TCP proxies

package tcpproxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Proxy is a proxy. Its zero value is a valid proxy that does
// nothing. Call methods to add routes before calling Start or Run.
//
// The order that routes are added in matters; each is matched in the order
// registered.
type Proxy struct {
	mux     sync.RWMutex
	configs map[string]*config // ip:port => config

	lns        []net.Listener
	donec      chan struct{} // closed before err
	err        error         // any error from listening
	connNumber int           // connection number counter, used for round robin

	// ListenFunc optionally specifies an alternate listen
	// function. If nil, net.Dial is used.
	// The provided net is always "tcp". This is to match
	// the signature of net.Listen.
	ListenFunc func(net, laddr string) (net.Listener, error)
}

// Matcher reports whether hostname matches the Matcher's criteria.
type Matcher func(ctx context.Context, hostname string) bool

// config contains the proxying state for one listener.
type config struct {
	routes []Route
}

func (p *Proxy) netListen() func(net, laddr string) (net.Listener, error) {
	if p.ListenFunc != nil {
		return p.ListenFunc
	}
	return net.Listen
}

func (p *Proxy) configFor(ipPort string) *config {
	if p.configs == nil {
		p.configs = make(map[string]*config)
	}
	if p.configs[ipPort] == nil {
		p.configs[ipPort] = &config{}
	}
	return p.configs[ipPort]
}

func (p *Proxy) setRoutes(ipPort string, routes []Route) {
	cfg := p.configFor(ipPort)
	cfg.routes = routes
}

// SetRoutes replaces routes for the ipPort.
//
// It's possible that the old routes are still used once after this
// function is called. If an empty slice is passed, the routes are
// preserved in order to avoid an infinite loop.
func (p *Proxy) SetRoutes(ipPort string, targets []Route) {
	p.mux.Lock()
	defer p.mux.Unlock()
	if len(targets) == 0 {
		panic("SetRoutes with empty targets")
	}
	p.setRoutes(ipPort, targets)
}

// Run is calls Start, and then Wait.
//
// It blocks until there's an error. The return value is always
// non-nil.
func (p *Proxy) Run() error {
	if err := p.Start(); err != nil {
		return err
	}
	return p.Wait()
}

// Wait waits for the Proxy to finish running. Currently this can only
// happen if a Listener is closed, or Close is called on the proxy.
//
// It is only valid to call Wait after a successful call to Start.
func (p *Proxy) Wait() error {
	<-p.donec
	return p.err
}

// Close closes all the proxy's self-opened listeners.
func (p *Proxy) Close() error {
	for _, c := range p.lns {
		c.Close()
	}
	return nil
}

// Start creates a TCP listener for each unique ipPort from the
// previously created routes and starts the proxy. It returns any
// error from starting listeners.
//
// If it returns a non-nil error, any successfully opened listeners
// are closed.
func (p *Proxy) Start() error {
	if p.donec != nil {
		return errors.New("already started")
	}
	p.donec = make(chan struct{})
	errc := make(chan error, len(p.configs))
	p.lns = make([]net.Listener, 0, len(p.configs))
	for ipPort, config := range p.configs {
		ln, err := p.netListen()("tcp", ipPort)
		if err != nil {
			p.Close()
			return err
		}
		p.lns = append(p.lns, ln)
		go p.serveListener(errc, ln, config)
	}
	go p.awaitFirstError(errc)
	return nil
}

func (p *Proxy) awaitFirstError(errc <-chan error) {
	p.err = <-errc
	close(p.donec)
}

func (p *Proxy) serveListener(ret chan<- error, ln net.Listener, cfg *config) {
	for {
		c, err := ln.Accept()
		if err != nil {
			ret <- err
			return
		}
		go p.serveConn(c, cfg)
	}
}

// serveConn runs in its own goroutine and matches c against routes.
// It returns whether it matched purely for testing.
func (p *Proxy) serveConn(c net.Conn, cfg *config) bool {
	br := bufio.NewReader(c)

	p.mux.RLock()
	p.connNumber++
	route := cfg.routes[p.connNumber%(len(cfg.routes))]
	p.mux.RUnlock()

	if n := br.Buffered(); n > 0 {
		peeked, _ := br.Peek(br.Buffered())
		c = &Conn{
			Peeked: peeked,
			Conn:   c,
		}
	}
	route.HandleConn(c)
	return true
}

// Conn is an incoming connection that has had some bytes read from it
// to determine how to route the connection. The Read method stitches
// the peeked bytes and unread bytes back together.
type Conn struct {
	// HostName is the hostname field that was sent to the request router.
	// In the case of TLS, this is the SNI header, in the case of HTTPHost
	// route, it will be the host header.  In the case of a fixed
	// route, i.e. those created with AddRoute(), this will always be
	// empty. This can be useful in the case where further routing decisions
	// need to be made in the Target impementation.
	HostName string

	// Peeked are the bytes that have been read from Conn for the
	// purposes of route matching, but have not yet been consumed
	// by Read calls. It set to nil by Read when fully consumed.
	Peeked []byte

	// Conn is the underlying connection.
	// It can be type asserted against *net.TCPConn or other types
	// as needed. It should not be read from directly unless
	// Peeked is nil.
	net.Conn
}

func (c *Conn) Read(p []byte) (n int, err error) {
	if len(c.Peeked) > 0 {
		n = copy(p, c.Peeked)
		c.Peeked = c.Peeked[n:]
		if len(c.Peeked) == 0 {
			c.Peeked = nil
		}
		return n, nil
	}
	return c.Conn.Read(p)
}

// To is shorthand way of writing &tcpproxy.DialProxy{Addr: addr}.
func To(addr string) Route {
	return Route{Addr: addr}
}

// Route is what an incoming connection is sent to.
// It handles them by dialing a new connection to Addr
// and then proxying data back and forth.
//
// The To func is a shorthand way of creating a Route.
type Route struct {
	// Addr is the TCP address to proxy to.
	Addr string

	// KeepAlivePeriod sets the period between TCP keep alives.
	// If zero, a default is used. To disable, use a negative number.
	// The keep-alive is used for both the client connection and
	KeepAlivePeriod time.Duration

	// DialTimeout optionally specifies a dial timeout.
	// If zero, a default is used.
	// If negative, the timeout is disabled.
	DialTimeout time.Duration

	// DialContext optionally specifies an alternate dial function
	// for TCP targets. If nil, the standard
	// net.Dialer.DialContext method is used.
	DialContext func(ctx context.Context, network, address string) (net.Conn, error)

	// OnDialError optionally specifies an alternate way to handle errors dialing Addr.
	// If nil, the error is logged and src is closed.
	// If non-nil, src is not closed automatically.
	OnDialError func(src net.Conn, dstDialErr error)

	// ProxyProtocolVersion optionally specifies the version of
	// HAProxy's PROXY protocol to use. The PROXY protocol provides
	// connection metadata to the DialProxy target, via a header
	// inserted ahead of the client's traffic. The DialProxy target
	// must explicitly support and expect the PROXY header; there is
	// no graceful downgrade.
	// If zero, no PROXY header is sent. Currently, version 1 is supported.
	ProxyProtocolVersion int
}

// UnderlyingConn returns c.Conn if c of type *Conn,
// otherwise it returns c.
func UnderlyingConn(c net.Conn) net.Conn {
	if wrap, ok := c.(*Conn); ok {
		return wrap.Conn
	}
	return c
}

func tcpConn(c net.Conn) (t *net.TCPConn, ok bool) {
	if c, ok := UnderlyingConn(c).(*net.TCPConn); ok {
		return c, ok
	}
	if c, ok := c.(*net.TCPConn); ok {
		return c, ok
	}
	return nil, false
}

func goCloseConn(c net.Conn) { go c.Close() }

func closeRead(c net.Conn) {
	if c, ok := tcpConn(c); ok {
		_ = c.CloseRead()
	}
}

func closeWrite(c net.Conn) {
	if c, ok := tcpConn(c); ok {
		_ = c.CloseWrite()
	}
}

// HandleConn implements the Target interface.
func (r *Route) HandleConn(src net.Conn) {
	ctx := context.Background()
	var cancel context.CancelFunc
	if r.DialTimeout >= 0 {
		ctx, cancel = context.WithTimeout(ctx, r.dialTimeout())
	}
	dst, err := r.dialContext()(ctx, "tcp", r.Addr)
	if cancel != nil {
		cancel()
	}
	if err != nil {
		r.onDialError()(src, err)
		return
	}
	defer goCloseConn(dst)

	if err = r.sendProxyHeader(dst, src); err != nil {
		r.onDialError()(src, err)
		return
	}
	defer goCloseConn(src)

	if ka := r.keepAlivePeriod(); ka > 0 {
		for _, c := range []net.Conn{src, dst} {
			if c, ok := tcpConn(c); ok {
				_ = c.SetKeepAlive(true)
				_ = c.SetKeepAlivePeriod(ka)
			}
		}
	}

	errc := make(chan error, 2)
	go proxyCopy(errc, src, dst)
	go proxyCopy(errc, dst, src)
	<-errc
	<-errc
}

func (r *Route) sendProxyHeader(w io.Writer, src net.Conn) error {
	switch r.ProxyProtocolVersion {
	case 0:
		return nil
	case 1:
		var srcAddr, dstAddr *net.TCPAddr
		if a, ok := src.RemoteAddr().(*net.TCPAddr); ok {
			srcAddr = a
		}
		if a, ok := src.LocalAddr().(*net.TCPAddr); ok {
			dstAddr = a
		}

		if srcAddr == nil || dstAddr == nil {
			_, err := io.WriteString(w, "PROXY UNKNOWN\r\n")
			return err
		}

		family := "TCP4"
		if srcAddr.IP.To4() == nil {
			family = "TCP6"
		}
		_, err := fmt.Fprintf(w, "PROXY %s %s %s %d %d\r\n", family, srcAddr.IP, dstAddr.IP, srcAddr.Port, dstAddr.Port)
		return err
	default:
		return fmt.Errorf("PROXY protocol version %d not supported", r.ProxyProtocolVersion)
	}
}

// proxyCopy is the function that copies bytes around.
// It's a named function instead of a func literal so users get
// named goroutines in debug goroutine stack dumps.
func proxyCopy(errc chan<- error, dst, src net.Conn) {
	defer closeRead(src)
	defer closeWrite(dst)

	// Before we unwrap src and/or dst, copy any buffered data.
	if wc, ok := src.(*Conn); ok && len(wc.Peeked) > 0 {
		if _, err := dst.Write(wc.Peeked); err != nil {
			errc <- err
			return
		}
		wc.Peeked = nil
	}

	// Unwrap the src and dst from *Conn to *net.TCPConn so Go
	// 1.11's splice optimization kicks in.
	src = UnderlyingConn(src)
	dst = UnderlyingConn(dst)

	_, err := io.Copy(dst, src)
	errc <- err
}

func (r *Route) keepAlivePeriod() time.Duration {
	if r.KeepAlivePeriod != 0 {
		return r.KeepAlivePeriod
	}
	return time.Minute
}

func (r *Route) dialTimeout() time.Duration {
	if r.DialTimeout > 0 {
		return r.DialTimeout
	}
	return 10 * time.Second
}

var defaultDialer = new(net.Dialer)

func (r *Route) dialContext() func(ctx context.Context, network, address string) (net.Conn, error) {
	if r.DialContext != nil {
		return r.DialContext
	}
	return defaultDialer.DialContext
}

func (r *Route) onDialError() func(src net.Conn, dstDialErr error) {
	if r.OnDialError != nil {
		return r.OnDialError
	}
	return func(src net.Conn, dstDialErr error) {
		logrus.WithFields(logrus.Fields{"component": "tcpproxy"}).Errorf("for incoming conn %v, error dialing %q: %v", src.RemoteAddr().String(), r.Addr, dstDialErr)
		src.Close()
	}
}
