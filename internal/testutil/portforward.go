// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/validation"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// PodDialer dials ports on Kubernetes pods via the API server's port-forwarding
// endpoint. It offers a dialer implementation via [PodDialer.DialContext] that
// is compatible to [net.Dialer.DialContext] with addresses of the form
// "<podname>.<namespace>:<portnum>".
//
// This allows you to "pivot" your Go network stack into a Kubernetes cluster.
// Anything that accepts a dialer can connect to arbitrary pods as if it were
// running as a sidecar (given the provided kubeconfig has enough privileges).
//
// HTTP requests are as easy as
//
//	podDialer, err := NewPodDialer(config)
//	if err != nil {
//	    return nil, err
//	}
//
//	c := http.Client{Transport: &http.Transport{DialContext: podDialer.DialContext}}
//	resp, err := c.Get("http://my-pod.my-namespace:8080/healthz")
//	// ...
//
// Prior art: https://microcumul.us/blog/k8s-port-forwarding/
type PodDialer struct {
	rest     rest.Interface
	client   http.Client
	upgrader spdy.Upgrader
}

// NewPodDialer creates a PodDialer backed by the given REST config.
func NewPodDialer(config *rest.Config) (*PodDialer, error) {
	client, err := corev1client.NewForConfigAndClient(config, nil)
	if err != nil {
		return nil, err
	}
	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	return &PodDialer{client.RESTClient(), http.Client{Transport: transport}, upgrader}, nil
}

// DialContext connects to a port on a pod. The addr must be of the form
// "<podname>.<namespace>:<portnum>". It looks up the pod, establishes the
// port-forward streams via the portforward.k8s.io protocol, connects to the
// specified port, and wraps everything up into a single [net.Conn]. The
// connection metadata is backed by [PodPortConnInfo].
func (d *PodDialer) DialContext(ctx context.Context, network, addr string) (_ net.Conn, err error) {
	pod, port, err := parsePodAddr(addr)
	if err != nil {
		return nil, err
	}

	podConn, err := d.dialPod(ctx, pod)
	if err != nil {
		return nil, fmt.Errorf("while dialing pod %s: %w", pod, err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, podConn.Close())
		}
	}()

	portConn, requestID, err := podConn.DialPort(port)
	if err != nil {
		return nil, fmt.Errorf("while connecting port %d on %s: %w", port, pod, err)
	}

	// Close the pod connection as soon as the port connection is done.
	var podConnCloseErr error
	podConnDone := make(chan struct{})
	go func() {
		defer close(podConnDone)
		<-portConn.done
		podConnCloseErr = podConn.Close()
	}()

	return (&PodPortConnInfo{pod, port, requestID}).Wrap(portConn, func() error {
		portConnCloseErr := portConn.Close()
		<-podConnDone
		return errors.Join(portConnCloseErr, podConnCloseErr)
	}), nil
}

func parsePodAddr(addr string) (types.NamespacedName, uint16, error) {
	if host, port, ok := strings.Cut(addr, ":"); ok {
		var pod types.NamespacedName
		if pod.Name, pod.Namespace, ok = strings.Cut(host, "."); ok {
			if err := validatePod(&pod); err != nil {
				return types.NamespacedName{}, 0, err
			}

			port, err := strconv.ParseUint(port, 10, 16)
			if err != nil {
				return types.NamespacedName{}, 0, fmt.Errorf("port number is invalid: %w", err)
			}

			return pod, uint16(port), nil
		}
	}

	return types.NamespacedName{}, 0, errors.New("address needs to be <podname>.<namespace>:<portnum>")
}

func validatePod(pod *types.NamespacedName) error {
	if errs := validation.IsDNS1123Subdomain(pod.Namespace); len(errs) > 0 {
		return fmt.Errorf("namespace %q is invalid: %s", pod.Namespace, strings.Join(errs, ", "))
	}
	if errs := validation.IsDNS1123Subdomain(pod.Name); len(errs) > 0 {
		return fmt.Errorf("pod name %q is invalid: %s", pod.Name, strings.Join(errs, ", "))
	}

	return nil
}

// DialPod opens a multiplexed connection to the given pod's port-forwarding
// endpoint. Individual ports can then be dialed via [PodConnection.DialPort].
// Use [PodDialer.DialContext] instead if you only need a single [net.Conn].
// Setting deadlines on the returned connection is not supported.
func (d *PodDialer) DialPod(ctx context.Context, pod types.NamespacedName) (*PodConnection, error) {
	if err := validatePod(&pod); err != nil {
		return nil, err
	}

	return d.dialPod(ctx, pod)
}

func (d *PodDialer) dialPod(ctx context.Context, pod types.NamespacedName) (*PodConnection, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		d.rest.Post().Resource("pods").
			Name(pod.Name).Namespace(pod.Namespace).
			SubResource("portforward").URL().String(),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	conn, _, err := spdy.Negotiate(d.upgrader, &d.client, req, portforward.PortForwardProtocolV1Name)
	if err != nil {
		var statusErr *apierrors.StatusError
		if !errors.As(err, &statusErr) {
			return nil, fmt.Errorf("failed to negotiate: %w", err)
		}
		if d := statusErr.ErrStatus.Details; d != nil && d.Kind == "pods" && statusErr.ErrStatus.Reason == metav1.StatusReasonNotFound {
			return nil, (*PodNotFoundError)(statusErr)
		}
		return nil, statusErr
	}

	return &PodConnection{conn: conn}, nil
}

// PodConnection is a multiplexed connection to a pod's port-forwarding
// endpoint. Multiple logical port-forwarded connections to different ports on
// the pod can be dialed concurrently on a single connection.
type PodConnection struct {
	conn             httpstream.Connection
	requestSequencer atomic.Uint64
	connTracker      sync.WaitGroup
}

// Terminates the connection. When it returns nil, all background goroutines
// have exited. Otherwise, no such guarantee is made.
func (c *PodConnection) Close() error {
	if err := c.conn.Close(); err != nil {
		return err
	}
	c.connTracker.Wait()
	return nil
}

// The connection will be automatically closed after the given period of inactivity.
func (c *PodConnection) SetIdleTimeout(timeout time.Duration) {
	c.conn.SetIdleTimeout(timeout)
}

// Opens a new port-forwarded connection to the given port on the pod.
func (c *PodConnection) DialPort(port uint16) (_ *PodPortConnection, requestID uint64, err error) {
	requestID = c.requestSequencer.Add(1)
	headers := http.Header{}
	headers.Set(corev1.StreamType, corev1.StreamTypeError)
	headers.Set(corev1.PortHeader, strconv.FormatUint(uint64(port), 10))
	headers.Set(corev1.PortForwardRequestIDHeader, strconv.FormatUint(requestID, 10))

	errorStream, err := c.conn.CreateStream(headers)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create err stream: %w", err)
	}
	// The error stream is read-only. Closing it will shut-down the sending side only.
	if err := errorStream.Close(); err != nil {
		return nil, 0, err
	}
	defer func() {
		if err != nil {
			c.conn.RemoveStreams(errorStream)
		}
	}()

	headers.Set(corev1.StreamType, corev1.StreamTypeData)
	dataStream, err := c.conn.CreateStream(headers)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create data stream: %w", err)
	}

	portConn := PodPortConnection{
		conn:       c.conn,
		dataStream: dataStream,
		errStream:  errorStream,
		done:       make(chan struct{}),
	}
	c.connTracker.Go(portConn.watchErr)
	c.connTracker.Go(portConn.watchClose)

	return &portConn, requestID, nil
}

// Indicates that a to-be-dialed pod does not exist.
type PodNotFoundError apierrors.StatusError

func (*PodNotFoundError) Error() string   { return "pod not found" }
func (e *PodNotFoundError) Unwrap() error { return (*apierrors.StatusError)(e) }

var errPodConnectionClosed = errors.New("pod connection closed")

// A port-forwarded connection to a specific port on a pod.
type PodPortConnection struct {
	conn                  httpstream.Connection
	dataStream, errStream httpstream.Stream
	done                  chan struct{}
	doneErr               atomic.Pointer[error]
}

func (c *PodPortConnection) watchErr() {
	msg, err := io.ReadAll(c.errStream)
	if err != nil {
		err := fmt.Errorf("failed to read port-forwarding error: %w", err)
		if c.doneErr.CompareAndSwap(nil, &err) {
			c.close()
		}
	} else if len(msg) > 0 {
		err := fmt.Errorf("port-forwarding error: %s", msg)
		if c.doneErr.CompareAndSwap(nil, &err) {
			c.close()
		}
	}
}

func (c *PodPortConnection) watchClose() {
	<-c.conn.CloseChan()
	if c.doneErr.CompareAndSwap(nil, &errPodConnectionClosed) {
		c.close()
	}
}

func (c *PodPortConnection) Read(b []byte) (int, error) {
	if err := c.doneErr.Load(); err != nil {
		return 0, *err
	}

	n, err := c.dataStream.Read(b)
	if err != nil && n == 0 && errors.Is(err, io.EOF) {
		// The data stream may return EOF before the error stream has been fully
		// read. Give watchErr a chance to capture the actual error instead of
		// returning directly.
		select {
		case <-c.done:
			return 0, *c.doneErr.Load()
		case <-time.After(1 * time.Second):
		}
	}

	return n, err
}

func (c *PodPortConnection) Write(b []byte) (int, error) {
	if err := c.doneErr.Load(); err != nil {
		return 0, *err
	}
	return c.dataStream.Write(b)
}

// Closes the connection. Safe to call concurrently and more than once.
// Subsequent calls wait for the first to finish and return nil.
func (c *PodPortConnection) Close() error {
	if c.doneErr.Swap(&net.ErrClosed) == nil {
		return errors.Join(c.close()...)
	}

	<-c.done
	return nil
}

func (c *PodPortConnection) close() (errs []error) {
	defer close(c.done)
	if err := c.dataStream.Close(); err != nil {
		errs = append(errs, fmt.Errorf("while closing data stream: %w", err))
	}
	if err := c.dataStream.Reset(); err != nil {
		errs = append(errs, fmt.Errorf("while resetting data stream: %w", err))
	}
	if err := c.errStream.Reset(); err != nil {
		errs = append(errs, fmt.Errorf("while resetting error stream: %w", err))
	}
	c.conn.RemoveStreams(c.dataStream, c.errStream)
	return errs
}

// PodPortConnInfo carries the identifying metadata of a port-forward
// connection. It implements [net.Addr] and can be used as local/remote address
// in a [net.Conn]. The network reported will be kubernetes/portforward.
type PodPortConnInfo struct {
	Pod       types.NamespacedName
	Port      uint16
	RequestID uint64
}

func (i *PodPortConnInfo) String() string       { return fmt.Sprintf("%s:%d:%d", i.Pod, i.Port, i.RequestID) }
func (i *PodPortConnInfo) LocalAddr() net.Addr  { return podPortConnNetAddr("local:" + i.String()) }
func (i *PodPortConnInfo) RemoteAddr() net.Addr { return podPortConnNetAddr("remote:" + i.String()) }

// Wraps c as a [net.Conn] using i as local and remote addresses. If close is
// nil, c.Close is used.
func (i *PodPortConnInfo) Wrap(c *PodPortConnection, close func() error) net.Conn {
	if close == nil {
		close = c.Close
	}

	return &netPodPortConn{c, *i, close}
}

type netPodPortConn struct {
	*PodPortConnection
	PodPortConnInfo
	close func() error
}

func (c *netPodPortConn) Close() error                   { return c.close() }
func (*netPodPortConn) SetDeadline(time.Time) error      { return errors.ErrUnsupported }
func (*netPodPortConn) SetReadDeadline(time.Time) error  { return errors.ErrUnsupported }
func (*netPodPortConn) SetWriteDeadline(time.Time) error { return errors.ErrUnsupported }

type podPortConnNetAddr string

var _ net.Addr = (*podPortConnNetAddr)(nil)

func (a podPortConnNetAddr) Network() string { return "kubernetes/portforward" }
func (a podPortConnNetAddr) String() string  { return (string)(a) }
