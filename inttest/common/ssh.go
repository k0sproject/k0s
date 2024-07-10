/*
Copyright 2020 k0s authors

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

package common

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
)

// SSHConnection describes an SSH connection
type SSHConnection struct {
	Address string
	User    string
	Port    int
	KeyPath string

	client *ssh.Client
}

// Disconnect closes the SSH connection
func (c *SSHConnection) Disconnect() {
	c.client.Close()
}

// Connect opens the SSH connection
func (c *SSHConnection) Connect(ctx context.Context) error {
	key, err := loadExternalFile(c.KeyPath)
	if err != nil {
		return err
	}

	config := &ssh.ClientConfig{
		User:            c.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	address := fmt.Sprintf("%s:%d", c.Address, c.Port)

	sshAgentSock := os.Getenv("SSH_AUTH_SOCK")
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil && sshAgentSock == "" {
		return err
	}
	if err == nil {
		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	}

	// https://github.com/golang/go/issues/20288#issuecomment-707078634
	d := net.Dialer{Timeout: config.Timeout}
	netConn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	conn, chans, reqs, err := ssh.NewClientConn(netConn, address, config)
	if err != nil {
		return err
	}
	c.client = ssh.NewClient(conn, chans, reqs)

	return nil
}

type SSHStreams struct {
	In       io.Reader
	Out, Err io.Writer
}

// SSHExecErrorWithStderr is the error returned by the various Exec* methods of
// [SSHConnection] in case a separate stderr was captured.
type SSHExecErrorWithStderr struct {
	ExecErr error
	Stderr  []byte
}

func (e *SSHExecErrorWithStderr) Error() string {
	var buf strings.Builder
	buf.WriteString(e.ExecErr.Error())
	if e.Stderr != nil {
		buf.WriteString(": ")
		buf.Write(strconv.AppendQuote(nil, trimOutput(e.Stderr)))
	}
	return buf.String()
}

func (e *SSHExecErrorWithStderr) Unwrap() error {
	return e.ExecErr
}

// ExecWithOutput execs a command on the host and returns its output.
func (c *SSHConnection) ExecWithOutput(ctx context.Context, cmd string) (string, error) {
	// This method doesn't distinguish between stdout and stderr, but it's
	// helpful to have a separate stderr for error reporting. Hence have two
	// buffers: a combined one with stdout/stderr that's returned as string, and
	// another one that only contains stderr. The latter one is only used if an
	// error occurs.

	outputWriter, getOutputBuffer := newWriterBuffer()
	defer outputWriter.Close()

	errOnlyWriter, getErrOnlyBuffer := newWriterBuffer()
	defer errOnlyWriter.Close()

	combinedWriter := writerFunc(func(p []byte) (int, error) {
		if n, err := errOnlyWriter.Write(p); err != nil {
			return n, err
		}
		return outputWriter.Write(p)
	})

	err := c.Exec(ctx, cmd, SSHStreams{Out: outputWriter, Err: combinedWriter})

	out := trimOutput(getOutputBuffer())
	if err != nil {
		return out, &SSHExecErrorWithStderr{err, getErrOnlyBuffer()}
	}

	return out, nil
}

// Exec execs a command on the host and connects the given streams to it.
func (c *SSHConnection) Exec(ctx context.Context, cmd string, streams SSHStreams) error {
	session, err := c.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdin = streams.In
	session.Stdout = streams.Out
	session.Stderr = streams.Err

	// Capture stderr in case the caller doesn't. This is useful for error reporting.
	// If this is undesired by the caller, it can still specify Err as io.Discard.
	var getErr func() []byte
	if session.Stderr == nil {
		var e io.WriteCloser
		e, getErr = newWriterBuffer()
		defer e.Close()
		session.Stderr = e
	}

	err = session.Start(cmd)
	if err != nil {
		return err
	}

	errChan := make(chan error)
	go func() {
		defer close(errChan)
		errChan <- session.Wait()
	}()

	select {
	case err, ok := <-errChan:
		if !ok {
			return errors.New("channel closed unexpectedly")
		}
		if getErr != nil && err != nil {
			return &SSHExecErrorWithStderr{err, getErr()}
		}
		return err

	case <-ctx.Done():
		return ctx.Err()
	}
}

// Returns SSH streams that log lines to the test log.
func TestLogStreams(t *testing.T, prefix string) (_ SSHStreams, flush func()) {
	out := LineWriter{WriteLine: func(line []byte) { t.Logf("%s stdout: %s", prefix, string(line)) }}
	err := LineWriter{WriteLine: func(line []byte) { t.Logf("%s stderr: %s", prefix, string(line)) }}
	return SSHStreams{Out: &out, Err: &err}, func() {
		out.Flush()
		err.Flush()
	}
}

func newWriterBuffer() (io.WriteCloser, func() []byte) {
	var mu sync.Mutex
	var buf bytes.Buffer
	writer := lockWriter{&mu, &buf}

	return &writer, func() []byte {
		writer.Lock()
		defer writer.Unlock()
		writer.Writer = io.Discard
		return buf.Bytes()
	}
}

type lockWriter struct {
	sync.Locker
	io.Writer
}

func (w *lockWriter) Write(p []byte) (int, error) {
	w.Lock()
	defer w.Unlock()
	return w.Writer.Write(p)
}

func (w *lockWriter) Close() error {
	w.Lock()
	defer w.Unlock()
	writer := w.Writer
	w.Writer = io.Discard
	if c, ok := writer.(io.Closer); ok {
		return c.Close()
	}

	return nil
}

type writerFunc func(p []byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func trimOutput(output []byte) string {
	if len(output) == 0 {
		return ""
	}

	return strings.TrimSpace(string(output))
}

func loadExternalFile(path string) ([]byte, error) {
	realpath, err := homedir.Expand(path)
	if err != nil {
		return []byte{}, err
	}

	filedata, err := os.ReadFile(realpath)
	if err != nil {
		return []byte{}, err
	}
	return filedata, nil
}
