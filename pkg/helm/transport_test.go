// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"

	"github.com/k0sproject/k0s/pkg/k0scontext"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControlledRESTClientGetter_InterruptsRegularRequests(t *testing.T) {
	cfg := &rest.Config{Host: "http://does-not-matter.example.com"}
	interrupted := make(chan struct{})

	underTest := transportControl{interrupted, assert.AnError}

	cfg.WrapTransport = underTest.wrap(cfg.WrapTransport)
	clients, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)

	close(interrupted)

	_, err = clients.CoreV1().Namespaces().List(t.Context(), metav1.ListOptions{})
	assert.ErrorIs(t, err, assert.AnError)
}

func TestTransportControl_InterruptsInflightDials(t *testing.T) {
	dialStarted := make(chan struct{})
	dialCtxDoneCause := make(chan error, 1)
	cfg := &rest.Config{
		Host: "http://does-not-matter.example.com",
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				close(dialStarted)
				<-ctx.Done()
				dialCtxDoneCause <- context.Cause(ctx)
				return nil, ctx.Err()
			},
		},
	}
	interrupted := make(chan struct{})

	underTest := transportControl{interrupted, assert.AnError}

	cfg.WrapTransport = underTest.wrap(cfg.WrapTransport)
	clients, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)

	var listErr error
	listDone := make(chan struct{})
	go func() {
		_, listErr = clients.CoreV1().Namespaces().List(t.Context(), metav1.ListOptions{})
		close(listDone)
	}()

	<-dialStarted

	close(interrupted)
	assert.Equal(t, <-dialCtxDoneCause, assert.AnError)

	<-listDone

	assert.ErrorIs(t, listErr, assert.AnError)
}

func TestTransportControl_RejectsUnsupportedTransports(t *testing.T) {
	underTest := transportControl{t.Context().Done(), assert.AnError}

	t.Run("OnlyHTTPTransports", func(t *testing.T) {
		cfg := &rest.Config{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				panic("unreachable")
			}),
		}
		cfg.WrapTransport = underTest.wrap(cfg.WrapTransport)
		clients, err := kubernetes.NewForConfig(cfg)
		require.NoError(t, err)

		result := clients.RESTClient().Get().Do(t.Context())
		assert.ErrorContains(t, result.Error(), "expected an *http.Transport")
	})

	t.Run("NoDeprecatedDial", func(t *testing.T) {
		cfg := &rest.Config{
			Transport: &http.Transport{
				Dial: func(_, _ string) (net.Conn, error) {
					panic("unreachable")
				},
			},
		}
		cfg.WrapTransport = underTest.wrap(cfg.WrapTransport)
		clients, err := kubernetes.NewForConfig(cfg)
		require.NoError(t, err)

		result := clients.RESTClient().Get().Do(t.Context())
		var urlErr *url.Error
		if assert.ErrorAs(t, result.Error(), &urlErr) && assert.Error(t, urlErr.Err) {
			assert.Equal(t, "cannot deal with the deprecated transport.Dial", urlErr.Err.Error())
		}
	})

	t.Run("NoDeprecatedDialTLS", func(t *testing.T) {
		cfg := &rest.Config{
			Transport: &http.Transport{
				DialTLS: func(_, _ string) (net.Conn, error) {
					panic("unreachable")
				},
			},
		}
		cfg.WrapTransport = underTest.wrap(cfg.WrapTransport)
		clients, err := kubernetes.NewForConfig(cfg)
		require.NoError(t, err)

		result := clients.RESTClient().Get().Do(t.Context())
		var urlErr *url.Error
		if assert.ErrorAs(t, result.Error(), &urlErr) && assert.Error(t, urlErr.Err) {
			assert.Equal(t, "cannot deal with the deprecated transport.DialTLS", urlErr.Err.Error())
		}
	})
}

func TestTransportControl_ResponseBodyClosePropagates(t *testing.T) {
	cfg := &rest.Config{
		Host:      "http://does-not-matter.example.com",
		Transport: startHTTPPipeServer(t),
	}

	underTest := transportControl{t.Context().Done(), assert.AnError}

	cfg.WrapTransport = underTest.wrap(cfg.WrapTransport)
	clients, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)

	responded := make(chan struct{})
	responder := &httpResponder{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(responded)
		assert.Equal(t, "/api/v1/namespaces/no/pods/matter/log", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		if _, err := io.WriteString(w, "foo"); !assert.NoError(t, err) {
			return
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		<-r.Context().Done()
	})}

	stream, err := clients.CoreV1().Pods("no").GetLogs("matter", &corev1.PodLogOptions{}).Stream(k0scontext.WithValue(t.Context(), responder))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, stream.Close()) })

	var buf strings.Builder
	_, err = io.CopyN(&buf, stream, 3)
	require.NoError(t, err)
	assert.Equal(t, "foo", buf.String())

	conn := responder.conn.Load()
	require.NotNil(t, conn)
	assert.False(t, conn.isClosed())

	require.NoError(t, stream.Close())
	<-responded
	assert.True(t, conn.isClosed())
}

func TestTransportControl_InterruptsLogsStream(t *testing.T) {
	cfg := &rest.Config{
		Host:      "http://does-not-matter.example.com",
		Transport: startHTTPPipeServer(t),
	}
	interrupted := make(chan struct{})

	underTest := transportControl{interrupted, assert.AnError}

	cfg.WrapTransport = underTest.wrap(cfg.WrapTransport)
	clients, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)

	responder := &httpResponder{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/namespaces/no/pods/matter/log", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		if _, err := io.WriteString(w, "foo"); !assert.NoError(t, err) {
			return
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		<-r.Context().Done()
	})}

	stream, err := clients.CoreV1().Pods("no").GetLogs("matter", &corev1.PodLogOptions{}).Stream(k0scontext.WithValue(t.Context(), responder))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, stream.Close()) })

	var buf [3]byte
	_, err = io.ReadFull(stream, buf[:])
	require.NoError(t, err)
	assert.Equal(t, [...]byte{'f', 'o', 'o'}, buf)

	close(interrupted)
	_, err = stream.Read(buf[:])
	assert.Error(t, err)

	conn := responder.conn.Load()
	require.NotNil(t, conn)
	assert.True(t, conn.isClosed())
}

func TestTransportControl_InterruptsWatchStream(t *testing.T) {
	cfg := &rest.Config{
		Host:      "http://does-not-matter.example.com",
		Transport: startHTTPPipeServer(t),
	}
	interrupted := make(chan struct{})

	underTest := transportControl{interrupted, errHelmOperationInterrupted}

	cfg.WrapTransport = underTest.wrap(cfg.WrapTransport)
	clients, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)

	responder := &httpResponder{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, "/api/v1/namespaces/ns/pods?watch=true", r.URL.String(), "Unexpected request URL") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]any{
			"type": string(watch.Added),
			"object": corev1.Pod{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
				ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "ns"},
			},
		}); !assert.NoError(t, err) {
			return
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		<-r.Context().Done()
	})}

	watcher, err := clients.CoreV1().Pods("ns").Watch(k0scontext.WithValue(t.Context(), responder), metav1.ListOptions{})
	require.NoError(t, err)
	t.Cleanup(watcher.Stop)

	if event, ok := <-watcher.ResultChan(); assert.True(t, ok, "Watch closed before first event") {
		assert.Equal(t, watch.Added, event.Type)
	}

	conn := responder.conn.Load()
	require.NotNil(t, conn)
	assert.False(t, conn.isClosed())

	close(interrupted)

	if event, ok := <-watcher.ResultChan(); assert.True(t, ok, "Watch closed without error event") {
		// The Kubernetes client converts all errors (i.e. errors that don't
		// stem from parsing an API server error response) into an
		// InternalServerError metav1.Status.
		if assert.Equal(t, watch.Error, event.Type) && assert.IsType(t, &metav1.Status{}, event.Object) {
			s := event.Object.(*metav1.Status)
			assert.Equal(t, metav1.StatusFailure, s.Status)
			assert.Equal(t, int32(http.StatusInternalServerError), s.Code)
			assert.Equal(t, metav1.StatusReasonInternalError, s.Reason)
			assert.Contains(t, s.Message, errHelmOperationInterrupted.Error())
			if assert.NotNil(t, s.Details) {
				if c := s.Details.Causes; assert.Len(t, c, 2) {
					expectedMsg := "unable to decode an event from the watch stream: " + errHelmOperationInterrupted.Error()
					assert.Equal(t, metav1.CauseTypeUnexpectedServerResponse, c[0].Type)
					assert.Contains(t, c[0].Message, expectedMsg)
					assert.Equal(t, "ClientWatchDecoding", string(c[1].Type))
					assert.Contains(t, c[1].Message, expectedMsg)
				}
			}
		}
	}

	if event, ok := <-watcher.ResultChan(); assert.False(t, ok, "Watch didn't close after error event") {
		assert.True(t, conn.isClosed())
	} else {
		assert.Failf(t, "Unexpected event", "%v", event)
	}

	assert.True(t, conn.isClosed())
}

var (
	errReadForwarded  = errors.New("read forwarded")
	errCloseForwarded = errors.New("close forwarded")
	errWriteForwarded = errors.New("write forwarded")
)

func TestTransportControl_RoundTrip(t *testing.T) {
	underTest := (&transportControl{t.Context().Done(), assert.AnError})

	t.Run("NilBody", func(t *testing.T) {
		var requestContext context.Context
		resp, err := underTest.roundTrip(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			requestContext = req.Context()
			return &http.Response{Body: nil}, nil
		}), &http.Request{})
		require.NoError(t, err)
		assert.Nil(t, resp.Body)
		assert.Equal(t, context.Canceled, context.Cause(requestContext))
	})

	for _, tt := range []struct {
		name         string
		body         io.ReadCloser
		flush, write bool
	}{
		{"WrapsReader", &fakeReadCloser{}, false, false},
		{"WrapsFlushableReader", &fakeFlushableReadCloser{}, true, false},
		{"WrapsReadWriter", &fakeReadWriteCloser{}, false, true},
		{"WrapsFlushableReadWriter", &fakeFlushableReadWriteCloser{}, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {

			var requestContext context.Context
			resp, err := underTest.roundTrip(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requestContext = req.Context()
				return &http.Response{Body: tt.body}, nil
			}), &http.Request{})
			require.NoError(t, err)
			assert.NotEqual(t, tt.body, resp.Body)
			require.NotNil(t, resp.Body)

			var b [1]byte
			n, err := resp.Body.Read(b[:])
			assert.Equal(t, errReadForwarded, err)
			assert.Zero(t, n)

			if tt.write {
				if w, ok := resp.Body.(io.Writer); assert.True(t, ok) {
					var b [1]byte
					n, err := w.Write(b[:])
					assert.Equal(t, 0, n)
					assert.Same(t, errWriteForwarded, err)
				}
			} else {
				var writer io.Writer
				assert.IsNotType(t, writer, tt.body)
				assert.IsNotType(t, writer, resp.Body)
			}

			if tt.flush {
				unwrapped := tt.body.(interface{ flushesSeen() uint32 })
				if f, ok := resp.Body.(http.Flusher); assert.True(t, ok) {
					assert.Zero(t, unwrapped.flushesSeen())
					f.Flush()
					assert.Equal(t, uint32(1), unwrapped.flushesSeen())
				}
			} else {
				var flusher http.Flusher
				assert.IsNotType(t, flusher, tt.body)
				assert.IsNotType(t, flusher, resp.Body)
			}

			assert.NoError(t, requestContext.Err())
			assert.Equal(t, errCloseForwarded, resp.Body.Close())
			assert.Equal(t, errHTTPBodyClosed, context.Cause(requestContext))
		})
	}

	t.Run("DoesNotDoubleWrapInterruptedError", func(t *testing.T) {
		expected := fmt.Errorf("injected: %w", assert.AnError)
		body := io.NopCloser(iotest.ErrReader(expected))
		interrupted := make(chan struct{})
		underTest := transportControl{interrupted, assert.AnError}

		resp, err := underTest.roundTrip(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{Body: body}, nil
		}), &http.Request{})
		require.NoError(t, err)
		assert.NotEqual(t, body, resp.Body)

		close(interrupted)
		var b [1]byte
		_, err = resp.Body.Read(b[:])
		assert.Equal(t, expected, err)
	})
}

func startHTTPPipeServer(t *testing.T) *http.Transport {
	server := http.Server{
		Addr: "pipe",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, ok := k0scontext.Value[net.Conn](r.Context()).(*serverConn)
			if assert.True(t, ok, "No server connection in request context") {
				conn.handler.ServeHTTP(w, r)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}),
		BaseContext: func(net.Listener) context.Context { return t.Context() },
		ConnContext: k0scontext.WithValue[net.Conn],
	}

	listener := pipeListener{
		queue: make(chan net.Conn),
		done:  make(chan struct{}),
	}

	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Serve(&listener) }()
	t.Cleanup(func() { server.Close(); assert.ErrorIs(t, http.ErrServerClosed, <-serverDone) })

	return &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			responder := k0scontext.Value[*httpResponder](ctx)
			if !assert.NotNil(t, responder, "No HTTP responder in dial context") {
				return nil, errors.New("no HTTP responder in dial context")
			}

			client, server := net.Pipe()
			cc := &clientConn{Conn: client, closed: make(chan struct{})}
			sc := &serverConn{Conn: server, handler: responder.handler}
			responder.conn.Store(cc)

			select {
			case listener.queue <- sc:
			case <-ctx.Done():
				cause := context.Cause(ctx)
				assert.Failf(t, "Dial context done", "Cause: %v", cause)
				return nil, fmt.Errorf("dial context done: %w", cause)
			}

			return cc, nil
		},
	}
}

type serverConn struct {
	net.Conn
	handler http.Handler
}

type clientConn struct {
	net.Conn
	closed chan struct{}
}

func (c *clientConn) Close() error {
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
	if c.Conn != nil {
		return c.Conn.Close()
	}
	return nil
}

func (c *clientConn) isClosed() bool {
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

type httpResponder struct {
	handler http.Handler
	conn    atomic.Pointer[clientConn]
}

type pipeListener struct {
	queue chan net.Conn
	done  chan struct{}
}

// Accept implements [net.Listener].
func (p *pipeListener) Accept() (net.Conn, error) {
	select {
	case conn, ok := <-p.queue:
		if !ok {
			<-p.done
			return nil, net.ErrClosed
		}
		return conn, nil

	case <-p.done:
		return nil, net.ErrClosed
	}
}

// Addr implements [net.Listener].
func (p *pipeListener) Addr() net.Addr {
	panic("unimplemented")
}

// Close implements [net.Listener].
func (p *pipeListener) Close() error {
	close(p.done)
	return nil
}

type fakeReadCloser struct{}

func (f *fakeReadCloser) Read(p []byte) (int, error) { return 0, errReadForwarded }
func (f *fakeReadCloser) Close() error               { return errCloseForwarded }

type fakeReadWriteCloser struct{ fakeReadCloser }

func (f *fakeReadWriteCloser) Write(p []byte) (int, error) { return 0, errWriteForwarded }

type flushCounter atomic.Uint32

func (f *flushCounter) Flush()              { (*atomic.Uint32)(f).Add(1) }
func (f *flushCounter) flushesSeen() uint32 { return (*atomic.Uint32)(f).Load() }

type fakeFlushableReadCloser struct {
	fakeReadCloser
	flushCounter
}

type fakeFlushableReadWriteCloser struct {
	fakeReadWriteCloser
	flushCounter
}
