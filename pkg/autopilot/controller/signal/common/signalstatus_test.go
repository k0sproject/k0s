// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	crrec "sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// The signal node that the tests in here operate on.
const testNodeName = "controller0"

// The state a successful download transitions to. The real one is the k0s
// package's Cordoning, which can't be imported here.
const testSuccessState = "Cordoning"

// newSignalData builds a k0s update request in the given status, where the empty
// string means a request that has no status yet.
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

func readSignalData(t *testing.T, client crcli.Client, key crcli.ObjectKey) apsigv2.SignalData {
	t.Helper()
	var node apv1beta2.ControlNode
	require.NoError(t, client.Get(t.Context(), key, &node))
	var data apsigv2.SignalData
	require.NoError(t, data.Unmarshal(node.GetAnnotations()))
	return data
}

// TestUpdateSignalStatus runs through a table of states a signal node can be in
// by the time a reconciler is done with its work, ensuring the resulting status
// is only recorded when that is still applicable, and that the log says what
// actually happened.
func TestUpdateSignalStatus(t *testing.T) {
	var tests = []struct {
		name         string
		stored       apsigv2.SignalData
		actedUpon    apsigv2.SignalData
		validFrom    []string
		newStatus    string
		wantPlanID   string
		wantStatus   string
		wantRecorded bool
	}{
		// The ordinary case: nothing happened to the node meanwhile.
		{
			"Unchanged",
			newSignalData("plan-1", "http://localhost/k0s", Downloading),
			newSignalData("plan-1", "http://localhost/k0s", Downloading),
			[]string{"", Downloading},
			testSuccessState,
			"plan-1", testSuccessState, true,
		},

		// A request that has no status yet, as seen by the signal handlers.
		{
			"NoStatusYet",
			newSignalData("plan-1", "http://localhost/k0s", ""),
			newSignalData("plan-1", "http://localhost/k0s", ""),
			[]string{""},
			Downloading,
			"plan-1", Downloading, true,
		},

		// Another reconciler advanced the status without invalidating the work.
		// This is what airgap updates do, where the download and signal
		// reconcilers are triggered by the same status-less event.
		{
			"StatusAdvancedMeanwhile",
			newSignalData("plan-1", "http://localhost/k0s", Downloading),
			newSignalData("plan-1", "http://localhost/k0s", ""),
			[]string{"", Downloading},
			testSuccessState,
			"plan-1", testSuccessState, true,
		},

		// A newer plan superseded the request that was acted upon. Recording
		// the status now would drop that newer request on the floor.
		{
			"SupersededByNewerPlan",
			newSignalData("plan-2", "http://localhost/new", Downloading),
			newSignalData("plan-1", "http://localhost/k0s", Downloading),
			[]string{"", Downloading},
			testSuccessState,
			"plan-2", Downloading, false,
		},

		// The same request, but already past this transition. Recording the
		// status now would take the state machine backwards.
		{
			"MovedPastTransition",
			newSignalData("plan-1", "http://localhost/k0s", Completed),
			newSignalData("plan-1", "http://localhost/k0s", Downloading),
			[]string{"", Downloading},
			testSuccessState,
			"plan-1", Completed, false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger, logged := logrus.New(), new(bytes.Buffer)
			logger.SetOutput(logged)

			node := newSignalNode(t, test.stored)
			client := newFakeClient(t, node)
			delegate := apdel.ControlNodeControllerDelegate()
			key := delegate.CreateNamespacedName(node.Name)

			recorded, err := UpdateSignalStatus(t.Context(), logrus.NewEntry(logger), client, delegate,
				key, test.actedUpon, test.validFrom, apsigv2.NewStatus(test.newStatus))
			assert.NoError(t, err, "a status that isn't applicable anymore is not an error")
			assert.Equal(t, test.wantRecorded, recorded, "should report whether it recorded the status")

			stored := readSignalData(t, client, key)
			assert.Equal(t, test.wantPlanID, stored.PlanID)
			if assert.NotNil(t, stored.Status) {
				assert.Equal(t, test.wantStatus, stored.Status.Status)
			}

			// The log has to be tied to the write, so that a status which was
			// never recorded doesn't leave a log claiming that it was.
			update := "Updating signaling response to '" + test.newStatus + "'"
			if test.wantRecorded {
				assert.Contains(t, logged.String(), update)
			} else {
				assert.NotContains(t, logged.String(), update)
				assert.Contains(t, logged.String(), "Not recording status '"+test.newStatus+"'")
			}
		})
	}
}

// staleReadClient serves the first staleReads reads from a snapshot taken up
// front, standing in for a cache that has yet to observe a write.
type staleReadClient struct {
	crcli.Client
	stale      crcli.Object
	staleReads int
}

func (c *staleReadClient) Get(ctx context.Context, key crcli.ObjectKey, obj crcli.Object, opts ...crcli.GetOption) error {
	if c.staleReads > 0 {
		c.staleReads--
		stale := c.stale.DeepCopyObject()
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(stale).Elem())
		return nil
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

// TestUpdateSignalStatusRetriesStaleReads ensures a conflict caused by a
// lagging read is retried until the read catches up. Propagating it instead
// would make the reconciler redo all of its work.
func TestUpdateSignalStatusRetriesStaleReads(t *testing.T) {
	data := newSignalData("plan-1", "http://localhost/k0s", Downloading)
	node := newSignalNode(t, data)
	base := newFakeClient(t, node)
	delegate := apdel.ControlNodeControllerDelegate()
	key := delegate.CreateNamespacedName(node.Name)

	// Snapshot the node, then bump it, so that the snapshot's resourceVersion
	// is outdated in exactly the way a lagging cache would serve it.
	var snapshot apv1beta2.ControlNode
	require.NoError(t, base.Get(t.Context(), key, &snapshot))
	bumped := snapshot.DeepCopy()
	bumped.Annotations["unrelated.k0sproject.io/churn"] = "1"
	require.NoError(t, base.Update(t.Context(), bumped))

	client := &staleReadClient{Client: base, stale: &snapshot, staleReads: 2}
	newStatus := apsigv2.NewStatus(testSuccessState)

	recorded, err := UpdateSignalStatus(t.Context(), logrus.NewEntry(logrus.StandardLogger()),
		client, delegate, key, data, []string{"", Downloading}, newStatus)
	require.NoError(t, err)
	assert.True(t, recorded, "the status is recorded once the read catches up")

	assert.Zero(t, client.staleReads, "the stale reads should have been consumed by retries")
	if stored := readSignalData(t, base, key); assert.NotNil(t, stored.Status) {
		assert.Equal(t, *newStatus, *stored.Status)
	}
}

// downloadManifestBuilder points the download reconciler at the given HTTP test
// server, mirroring the real manifest builders in the k0s and airgap packages.
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

// TestDownloadControllerAgainstAnnotationChurn is a regression test for a
// livelock seen in CI for check-ap-controllerworker, where a node stayed stuck
// reporting Downloading until the whole suite timed out.
//
// That suite writes an unrelated annotation to every ControlNode about once a
// second, on purpose, to generate reconcile churn. The download reconciler used
// to write back the signal node as it had read it before the download, which
// failed whenever one of those writes landed in between. Every failure made it
// redo the download and race again, and since a download took about as long as
// the interval between those writes, it kept losing: the CI failure showed 118
// consecutive attempts over three minutes.
//
// The churn here is much faster than the download, so that a regression fails
// immediately instead of depending on timing luck.
func TestDownloadControllerAgainstAnnotationChurn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(150 * time.Millisecond)
		_, _ = w.Write([]byte("binary-content"))
	}))
	defer srv.Close()

	node := newSignalNode(t, newSignalData("plan-1", srv.URL, Downloading))
	client := newFakeClient(t, node)
	delegate := apdel.ControlNodeControllerDelegate()
	reconciler := NewDownloadController(logrus.NewEntry(logrus.StandardLogger()), client, delegate,
		downloadManifestBuilder{url: srv.URL, dir: t.TempDir()})
	req := crrec.Request{NamespacedName: delegate.CreateNamespacedName(node.Name)}

	// Stand in for the inttest suite's load generator.
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
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		attempts++
		if _, err := reconciler.Reconcile(t.Context(), req); err != nil {
			failures++
			continue
		}
		if status := readSignalData(t, client, req.NamespacedName).Status; status != nil && status.Status == testSuccessState {
			t.Logf("reached %s after %d attempt(s), %d failure(s)", testSuccessState, attempts, failures)
			return
		}
	}

	t.Fatalf("the download reconciler never recorded %s within 5s: %d attempt(s), %d of which failed to write back the signal node",
		testSuccessState, attempts, failures)
}

// TestDownloadControllerConcurrentStatusUpdate is a regression test for
// check-ap-airgap. The airgap download and signal reconcilers are triggered by
// the same status-less event, so the download reconciler starts out from a node
// without a status while the signal reconciler moves that very node to
// Downloading. Discarding the download result in that case leaves the update
// stuck for good, because the airgap download reconciler is only ever triggered
// by status-less events and would never be asked again.
func TestDownloadControllerConcurrentStatusUpdate(t *testing.T) {
	downloadStarted, releaseDownload := make(chan struct{}), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(downloadStarted)
		<-releaseDownload
		_, _ = w.Write([]byte("bundle-content"))
	}))
	defer srv.Close()

	data := newSignalData("plan-1", srv.URL, "")
	node := newSignalNode(t, data)
	client := newFakeClient(t, node)
	delegate := apdel.ControlNodeControllerDelegate()
	reconciler := NewDownloadController(logrus.NewEntry(logrus.StandardLogger()), client, delegate,
		downloadManifestBuilder{url: srv.URL, dir: t.TempDir()})
	key := delegate.CreateNamespacedName(node.Name)

	reconciled := make(chan error, 1)
	go func() {
		_, err := reconciler.Reconcile(t.Context(), crrec.Request{NamespacedName: key})
		reconciled <- err
	}()

	// While the download is in flight, the signal reconciler moves the node to
	// Downloading, exactly as it does for airgap updates.
	<-downloadStarted
	var cn apv1beta2.ControlNode
	require.NoError(t, client.Get(t.Context(), key, &cn))
	downloading := data
	downloading.Status = apsigv2.NewStatus(Downloading)
	require.NoError(t, downloading.Marshal(cn.Annotations))
	require.NoError(t, client.Update(t.Context(), &cn))
	close(releaseDownload)

	require.NoError(t, <-reconciled)

	if stored := readSignalData(t, client, key); assert.NotNil(t, stored.Status) {
		assert.Equal(t, testSuccessState, stored.Status.Status,
			"the download result must be recorded even though the status moved to %s meanwhile", Downloading)
	}
}
