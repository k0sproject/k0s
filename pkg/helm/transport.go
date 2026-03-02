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

	"k8s.io/client-go/transport"
	"k8s.io/utils/ptr"
)

// Injects an external interruption signal into HTTP transports.
//
// It propagates interruption across:
//   - network connections, by wrapping DialContext and DialTLSContext,
//   - request execution, by injecting a wrapping RoundTripper that mangles request contexts,
//   - response-body I/O, by wrapping request bodies returned by the underlying RoundTrippers.
type transportControl struct {
	interrupted    <-chan struct{}
	interruptedErr error
}

// Inject the transport control into wrapTransport, returning the combined
// wrapper function.
//
// Note that this only supports RoundTrippers of type [*http.Transport].
func (c *transportControl) wrap(wrapTransport transport.WrapperFunc) transport.WrapperFunc {
	wrappers := make([]transport.WrapperFunc, 0, 3)
	wrappers = append(wrappers, c.transport) // Needs to come first to wrap *http.Transport.
	if wrapTransport != nil {
		wrappers = append(wrappers, wrapTransport) // This is the original wrapper.
	}
	wrappers = append(wrappers, c.roundTripper) // Injects externally cancellable request contexts.

	return transport.Wrappers(wrappers...)
}

func (c *transportControl) transport(rt http.RoundTripper) http.RoundTripper {
	transport, ok := rt.(*http.Transport)
	if !ok {
		err := fmt.Errorf("expected an *http.Transport, got %T", rt)
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return nil, err
		})
	}

	transport = transport.Clone()

	if dial := transport.DialContext; dial == nil && transport.Dial != nil {
		err := errors.New("cannot deal with the deprecated transport.Dial")
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return nil, err
		})
	} else {
		if dial == nil {
			dial = (&net.Dialer{}).DialContext
		}
		transport.DialContext = c.wrapDial(dial)
	}

	if dial := transport.DialTLSContext; dial == nil {
		if transport.DialTLS != nil {
			err := errors.New("cannot deal with the deprecated transport.DialTLS")
			return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return nil, err
			})
		}
	} else {
		transport.DialTLSContext = c.wrapDial(dial)
	}

	return transport
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
		return ptr.To(makeBodyWrapper(c, body, close))
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
