// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apdl "github.com/k0sproject/k0s/pkg/autopilot/download"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"
	apscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	crint "sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	crrec "sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// The signal node that the tests in here operate on.
const testNodeName = "controller0"

// The real one is the k0s package's Cordoning, which can't be imported here.
const testSuccessState = "Cordoning"

// newSignalData builds a k0s update request; an empty status means none yet.
func newSignalData(planID, url, status string) apsigv2.SignalData {
	commandID := 123
	data := apsigv2.SignalData{
		PlanID:  planID,
		Created: "now",
		Command: apsigv2.Command{
			ID:        &commandID,
			K0sUpdate: &apsigv2.CommandK0sUpdate{URL: url, Version: "v0.0.0"},
		},
	}
	if status != "" {
		data.Status = apsigv2.NewStatus(status)
	}
	return data
}

func newSignalNode(t *testing.T, data apsigv2.SignalData) *apv1beta2.ControlNode {
	t.Helper()
	node := &apv1beta2.ControlNode{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName, Annotations: map[string]string{}},
	}
	require.NoError(t, data.Marshal(node.Annotations))
	return node
}

func newFakeClient(t *testing.T, objects ...crcli.Object) crcli.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, apscheme.AddToScheme(scheme))
	return crfake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

// newFakeClientConflictingOnFirstUpdate stands in for a concurrent writer by
// rejecting the first update with a conflict.
func newFakeClientConflictingOnFirstUpdate(t *testing.T, objects ...crcli.Object) crcli.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, apscheme.AddToScheme(scheme))

	var conflicted atomic.Bool
	client := crfake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).
		WithInterceptorFuncs(crint.Funcs{
			Update: func(ctx context.Context, c crcli.WithWatch, obj crcli.Object, opts ...crcli.UpdateOption) error {
				if !conflicted.Swap(true) {
					return apierrors.NewConflict(
						schema.GroupResource{Group: apv1beta2.GroupName, Resource: "controlnodes"},
						obj.GetName(), errors.New("injected conflict"),
					)
				}
				return c.Update(ctx, obj, opts...)
			},
		}).Build()

	t.Cleanup(func() { assert.True(t, conflicted.Load(), "conflict hasn't been injected") })
	return client
}

func readSignalData(t *testing.T, client crcli.Client, key crcli.ObjectKey) apsigv2.SignalData {
	t.Helper()
	var node apv1beta2.ControlNode
	require.NoError(t, client.Get(t.Context(), key, &node))
	var data apsigv2.SignalData
	require.NoError(t, data.Unmarshal(node.GetAnnotations()))
	return data
}

// downloadManifestBuilder points the reconciler at the given test server.
type downloadManifestBuilder struct {
	url string
	dir string
}

func (b downloadManifestBuilder) Build(crcli.Object, apsigv2.SignalData) (DownloadManifest, error) {
	return DownloadManifest{
		Config:       apdl.Config{URL: b.url, DownloadDir: b.dir, Filename: "downloaded"},
		SuccessState: testSuccessState,
	}, nil
}

// newDownloadController wires up a reconciler and the request to hand it.
func newDownloadController(t *testing.T, client crcli.Client, url string) (*downloadController, crrec.Request) {
	t.Helper()
	delegate := apdel.ControlNodeControllerDelegate()
	reconciler := NewDownloadController(
		logrus.NewEntry(logrus.StandardLogger()), client, delegate,
		downloadManifestBuilder{url: url, dir: t.TempDir()},
	).(*downloadController)
	return reconciler, crrec.Request{NamespacedName: delegate.CreateNamespacedName(testNodeName)}
}

// TestDownloadControllerAgainstAnnotationChurn ensures a download is not repeated
// while the signal node keeps changing. Losing the write back is expected and
// still reported; refetching every time is what stalls plan execution.
func TestDownloadControllerAgainstAnnotationChurn(t *testing.T) {
	var downloads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		downloads.Add(1)
		time.Sleep(150 * time.Millisecond)
		_, _ = w.Write([]byte("binary-content"))
	}))
	defer srv.Close()

	client := newFakeClient(t, newSignalNode(t, newSignalData("plan-1", srv.URL, Downloading)))
	reconciler, req := newDownloadController(t, client, srv.URL)

	// Churn faster than the download, so a regression fails immediately.
	ctx, stopChurn := context.WithCancel(t.Context())
	churnStopped := make(chan struct{})
	go func() {
		defer close(churnStopped)
		for ctx.Err() == nil {
			var cn apv1beta2.ControlNode
			if err := client.Get(ctx, req.NamespacedName, &cn); err == nil {
				cn.Annotations["test.k0sproject.io/touch"] = time.Now().Format(time.RFC3339Nano)
				_ = client.Update(ctx, &cn)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	t.Cleanup(func() { stopChurn(); <-churnStopped })

	var attempts, failures int
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		attempts++
		if _, err := reconciler.Reconcile(t.Context(), req); err != nil {
			failures++
			continue
		}
		if status := readSignalData(t, client, req.NamespacedName).Status; status != nil && status.Status == testSuccessState {
			t.Logf("reached %s after %d attempt(s), %d failure(s)", testSuccessState, attempts, failures)
			assert.Equal(t, int32(1), downloads.Load(),
				"the download must not be repeated when only the write back failed")
			return
		}
	}

	t.Fatalf("the download reconciler never recorded %s within 10s: %d attempt(s), %d of which failed to write back the signal node",
		testSuccessState, attempts, failures)
}

// TestDownloadControllerReusesDownloadAfterConflict pins both halves: a lost
// write is reported, and the requeue reuses what was already fetched.
func TestDownloadControllerReusesDownloadAfterConflict(t *testing.T) {
	var downloads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		downloads.Add(1)
		_, _ = w.Write([]byte("binary-content"))
	}))
	defer srv.Close()

	client := newFakeClientConflictingOnFirstUpdate(t, newSignalNode(t, newSignalData("plan-1", srv.URL, Downloading)))
	reconciler, req := newDownloadController(t, client, srv.URL)

	_, err := reconciler.Reconcile(t.Context(), req)
	require.Error(t, err, "a lost write must be reported so that controller-runtime requeues")
	assert.True(t, apierrors.IsConflict(err), "expected a conflict, got %v", err)
	assert.Equal(t, int32(1), downloads.Load())

	// The requeue must not fetch anything again.
	_, err = reconciler.Reconcile(t.Context(), req)
	require.NoError(t, err)
	assert.Equal(t, int32(1), downloads.Load(), "the retry must reuse the completed download")

	if stored := readSignalData(t, client, req.NamespacedName); assert.NotNil(t, stored.Status) {
		assert.Equal(t, testSuccessState, stored.Status.Status)
	}
}

// TestDownloadControllerRefetchesForNewRequest ensures reuse is keyed on the
// request: apply installs whatever it finds, unchecked.
func TestDownloadControllerRefetchesForNewRequest(t *testing.T) {
	var downloads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		downloads.Add(1)
		_, _ = w.Write([]byte("binary-content"))
	}))
	defer srv.Close()

	client := newFakeClient(t, newSignalNode(t, newSignalData("plan-1", srv.URL, Downloading)))
	reconciler, req := newDownloadController(t, client, srv.URL)

	_, err := reconciler.Reconcile(t.Context(), req)
	require.NoError(t, err)
	require.Equal(t, int32(1), downloads.Load())

	// A new plan lands on the same node.
	var cn apv1beta2.ControlNode
	require.NoError(t, client.Get(t.Context(), req.NamespacedName, &cn))
	require.NoError(t, newSignalData("plan-2", srv.URL, Downloading).Marshal(cn.Annotations))
	require.NoError(t, client.Update(t.Context(), &cn))

	_, err = reconciler.Reconcile(t.Context(), req)
	require.NoError(t, err)
	assert.Equal(t, int32(2), downloads.Load(), "a different request must be downloaded again")
}

// TestDownloadControllerConcurrentStatusUpdate covers the airgap shape: the
// download starts from a node without a status while the signal reconciler moves
// it to Downloading. The requeue must still reuse the bundle.
func TestDownloadControllerConcurrentStatusUpdate(t *testing.T) {
	var downloads atomic.Int32
	var startOnce sync.Once
	downloadStarted, releaseDownload := make(chan struct{}), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		downloads.Add(1)
		// Tolerate a second request, so a regression fails the assertion
		// below rather than panicking on the server goroutine.
		startOnce.Do(func() { close(downloadStarted) })
		<-releaseDownload
		_, _ = w.Write([]byte("bundle-content"))
	}))
	defer srv.Close()

	data := newSignalData("plan-1", srv.URL, "")
	client := newFakeClient(t, newSignalNode(t, data))
	reconciler, req := newDownloadController(t, client, srv.URL)

	reconciled := make(chan error, 1)
	go func() {
		_, err := reconciler.Reconcile(t.Context(), req)
		reconciled <- err
	}()

	// Mid-download, the signal reconciler moves the node to Downloading.
	<-downloadStarted
	var cn apv1beta2.ControlNode
	require.NoError(t, client.Get(t.Context(), req.NamespacedName, &cn))
	downloading := data
	downloading.Status = apsigv2.NewStatus(Downloading)
	require.NoError(t, downloading.Marshal(cn.Annotations))
	require.NoError(t, client.Update(t.Context(), &cn))
	close(releaseDownload)

	require.Error(t, <-reconciled, "the write back is expected to lose")

	_, err := reconciler.Reconcile(t.Context(), req)
	require.NoError(t, err)
	assert.Equal(t, int32(1), downloads.Load(),
		"the bundle must be reused even though the status moved to %s meanwhile", Downloading)

	if stored := readSignalData(t, client, req.NamespacedName); assert.NotNil(t, stored.Status) {
		assert.Equal(t, testSuccessState, stored.Status.Status)
	}
}

// TestDownloadControllerRefetchesForForeignPath rejects a record pointing
// anywhere but where the manifest wants the file. Reporting success with the
// target empty is not harmless: apply reads a missing k0s.tmp as "already
// applied", so the node comes back on the old binary.
func TestDownloadControllerRefetchesForForeignPath(t *testing.T) {
	var downloads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		downloads.Add(1)
		_, _ = w.Write([]byte("binary-content"))
	}))
	defer srv.Close()

	client := newFakeClient(t, newSignalNode(t, newSignalData("plan-1", srv.URL, Downloading)))
	reconciler, req := newDownloadController(t, client, srv.URL)

	// A real file, correct request, wrong directory.
	elsewhere := filepath.Join(t.TempDir(), "downloaded")
	require.NoError(t, os.WriteFile(elsewhere, []byte("binary-content"), 0o600))
	reconciler.recordDownload(readSignalData(t, client, req.NamespacedName), elsewhere)

	_, err := reconciler.Reconcile(t.Context(), req)
	require.NoError(t, err)
	assert.Equal(t, int32(1), downloads.Load(),
		"a record pointing outside the manifest's download dir must not be reused")
}

// TestDownloadControllerRefetchesForIrregularFile covers a non-file at the right
// path, which os.Stat alone would accept.
func TestDownloadControllerRefetchesForIrregularFile(t *testing.T) {
	var downloads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		downloads.Add(1)
		_, _ = w.Write([]byte("binary-content"))
	}))
	defer srv.Close()

	client := newFakeClient(t, newSignalNode(t, newSignalData("plan-1", srv.URL, Downloading)))
	reconciler, req := newDownloadController(t, client, srv.URL)

	// Point the record at a directory sitting where the download would go.
	dir := reconciler.manifestBuilder.(downloadManifestBuilder).dir
	asDir := filepath.Join(dir, "downloaded")
	require.NoError(t, os.Mkdir(asDir, 0o700))
	reconciler.recordDownload(readSignalData(t, client, req.NamespacedName), asDir)

	_, err := reconciler.Reconcile(t.Context(), req)
	require.NoError(t, err)
	assert.Equal(t, int32(1), downloads.Load(), "a non-regular file must not be reused")

	// The refetch then fails, since the rename cannot replace a directory. A
	// reported failure is fine; silently reusing it and claiming success is not.
	if stored := readSignalData(t, client, req.NamespacedName); assert.NotNil(t, stored.Status) {
		assert.Equal(t, FailedDownload, stored.Status.Status)
	}
}

// TestDownloadControllerRefetchesWhenGone covers the recorded file having
// disappeared, e.g. removed by apply or by an operator.
func TestDownloadControllerRefetchesWhenGone(t *testing.T) {
	var downloads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		downloads.Add(1)
		_, _ = w.Write([]byte("binary-content"))
	}))
	defer srv.Close()

	client := newFakeClient(t, newSignalNode(t, newSignalData("plan-1", srv.URL, Downloading)))
	reconciler, req := newDownloadController(t, client, srv.URL)

	_, err := reconciler.Reconcile(t.Context(), req)
	require.NoError(t, err)
	require.Equal(t, int32(1), downloads.Load())

	// Point the record at something that isn't there.
	data := readSignalData(t, client, req.NamespacedName)
	reconciler.recordDownload(data, filepath.Join(t.TempDir(), "vanished"))

	// Put the node back into Downloading so the reconciler acts on it again.
	var cn apv1beta2.ControlNode
	require.NoError(t, client.Get(t.Context(), req.NamespacedName, &cn))
	data.Status = apsigv2.NewStatus(Downloading)
	require.NoError(t, data.Marshal(cn.Annotations))
	require.NoError(t, client.Update(t.Context(), &cn))

	_, err = reconciler.Reconcile(t.Context(), req)
	require.NoError(t, err)
	assert.Equal(t, int32(2), downloads.Load(), "a vanished download must be fetched again")
}
