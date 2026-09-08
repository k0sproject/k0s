// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/rest"
)

// Holds an HTTP transport with the same defaults that client-go applies to
// the transports it builds.
var defaultTransport = sync.OnceValue(func() *http.Transport {
	return utilnet.SetTransportDefaults(new(http.Transport))
})

// Injects an external interruption signal into HTTP transports.
//
// It propagates interruption across:
//   - network connections, by wrapping the dialer,
//   - request execution, by injecting a wrapping RoundTripper that mangles request contexts,
//   - response-body I/O, by wrapping request bodies returned by the underlying RoundTrippers.
type transportControl struct {
	interrupted    <-chan struct{}
	interruptedErr error
}

// Injects the transport control into config.
//
// Note that this only supports configs that don't have a Transport set.
func (c *transportControl) injectInto(config *rest.Config) error {
	// The interruptible dialer is hooked in via config.Dial, which allows
	// client-go to build its transport around it while keeping all the
	// transport layers that client-go adds (TLS cache tracking, CA rotation,
	// and so on) intact. However, custom transports are not supported because
	// client-go ignores the dialer for those.
	if config.Transport != nil {
		// Nothing in k0s can currently produce a REST config with a custom
		// transport. In particular, REST configs constructed from kubeconfigs
		// will never have a transport set.
		return errors.New("custom transports are not supported")
	}
	dial := config.Dial
	if dial == nil {
		dial = defaultTransport().DialContext
	}
	config.Dial = c.wrapDial(dial)

	// The transport control is short-lived, and so is its dialer. Setting a
	// dialer makes client-go's TLS transport cache key unique to this config,
	// so that every client created from it would leave behind an entry in the
	// cache. Setting the proxy func explicitly to the upstream default makes the
	// config uncacheable altogether, as client-go can't compare proxy funcs.
	if config.Proxy == nil {
		config.Proxy = defaultTransport().Proxy
	}

	// Injects externally cancellable request contexts.
	config.Wrap(c.roundTripper)

	return nil
}

type dialFunc = func(ctx context.Context, net, addr string) (net.Conn, error)

// Makes dials interruption-aware. If interrupted, both in-flight and future
// dials fail with interruptedErr. Established connections are wrapped so they
// can be force-closed upon interruption.
func (c *transportControl) wrapDial(dial dialFunc) dialFunc {
	return func(ctx context.Context, net, addr string) (net.Conn, error) {
		select {
		case <-c.interrupted:
			return nil, c.interruptedErr
		default:
		}

		ctx, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)

		go func() {
			select {
			case <-c.interrupted:
				cancel(c.interruptedErr)
			case <-ctx.Done():
			}
		}()

		conn, err := dial(ctx, net, addr)
		if err != nil {
			return nil, err
		}

		closing := make(chan struct{})
		close := sync.OnceValue(func() error {
			close(closing)
			return conn.Close()
		})

		go func() {
			select {
			case <-c.interrupted:
				_ = close()
			case <-closing:
			}
		}()

		return &closeWrappingConn{conn, close}, nil
	}
}

func (c *transportControl) roundTripper(rt http.RoundTripper) http.RoundTripper {
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return c.roundTrip(rt, req)
	})
}

// Clones req and injects a request context that gets additionally canceled when
// interrupted. Response bodies returned by rt are wrapped so that they can be
// force-closed upon interruption.
func (c *transportControl) roundTrip(rt http.RoundTripper, req *http.Request) (*http.Response, error) {
	select {
	case <-c.interrupted:
		return nil, c.interruptedErr
	default:
	}

	ctx, cancel := context.WithCancelCause(req.Context())
	go func() {
		select {
		case <-c.interrupted:
			cancel(c.interruptedErr)
		case <-ctx.Done():
		}
	}()

	resp, err := rt.RoundTrip(req.Clone(ctx))
	if err != nil {
		cancel(err)
		select {
		case <-c.interrupted:
			if errors.Is(err, c.interruptedErr) {
				return nil, err
			}
			return nil, fmt.Errorf("%w (%w)", c.interruptedErr, err)
		default:
		}

		return nil, err
	}

	resp.Body = c.wrapBody(resp.Body, cancel)

	return resp, nil
}

var errHTTPBodyClosed = errors.New("HTTP body closed")

// Extends interruption handling to response-body I/O, ensuring that
// stream-based operations terminate promptly, too.
func (c *transportControl) wrapBody(body io.ReadCloser, cancel context.CancelCauseFunc) io.ReadCloser {
	if body == nil {
		cancel(nil)
		return nil
	}

	close := sync.OnceValue(func() error {
		err := body.Close()
		cancel(errHTTPBodyClosed)
		return err
	})

	switch body := body.(type) {
	case flushableWritableBody:
		return &flushableWritableBodyWrapper{
			writableBodyWrapper[flushableWritableBody]{
				makeBodyWrapper(c, body, close),
			},
		}

	case io.ReadWriter:
		return &writableBodyWrapper[io.ReadWriter]{
			makeBodyWrapper(c, body, close),
		}

	case flushableBody:
		return &flushableBodyWrapper{
			makeBodyWrapper(c, body, close),
		}

	default:
		return new(makeBodyWrapper(c, body, close))
	}
}

func makeBodyWrapper[T io.Reader](c *transportControl, body T, close func() error) bodyWrapper[T] {
	return bodyWrapper[T]{body, c.interrupted, c.interruptedErr, close}
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type closeWrappingConn struct {
	net.Conn
	close func() error
}

func (c *closeWrappingConn) Close() error { return c.close() }

type flushableBody interface {
	io.Reader
	http.Flusher
}

type flushableWritableBody interface {
	io.ReadWriter
	http.Flusher
}

type bodyWrapper[T io.Reader] struct {
	inner          T
	interrupted    <-chan struct{}
	interruptedErr error
	close          func() error
}

// Read implements [io.ReadCloser].
func (w *bodyWrapper[T]) Read(p []byte) (int, error) {
	n, err := w.inner.Read(p)
	return n, w.wrapErr(err)
}

// Close implements [io.ReadCloser].
func (w *bodyWrapper[T]) Close() error {
	return w.close()
}

func (w *bodyWrapper[T]) wrapErr(err error) error {
	if err != nil {
		select {
		case <-w.interrupted:
			if !errors.Is(err, w.interruptedErr) {
				return fmt.Errorf("%w (%w)", w.interruptedErr, err)
			}
		default:
		}
	}

	return err
}

type flushableBodyWrapper struct {
	bodyWrapper[flushableBody]
}

// Flush implements [http.Flusher].
func (w *flushableBodyWrapper) Flush() {
	w.inner.Flush()
}

type writableBodyWrapper[T io.ReadWriter] struct {
	bodyWrapper[T]
}

// Write implements [io.ReadWriteCloser].
func (w *writableBodyWrapper[T]) Write(p []byte) (int, error) {
	n, err := w.inner.Write(p)
	return n, w.wrapErr(err)
}

type flushableWritableBodyWrapper struct {
	writableBodyWrapper[flushableWritableBody]
}

// Flush implements [http.Flusher].
func (w *flushableWritableBodyWrapper) Flush() {
	w.inner.Flush()
}
