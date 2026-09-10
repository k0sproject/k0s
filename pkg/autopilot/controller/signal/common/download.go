// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"

	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apdl "github.com/k0sproject/k0s/pkg/autopilot/download"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	"github.com/sirupsen/logrus"
	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crrec "sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type DownloadManifest struct {
	apdl.Config

	SuccessState string
}

type DownloadManifestBuilder interface {
	Build(signalNode crcli.Object, signalData apsigv2.SignalData) (DownloadManifest, error)
}

type downloadRecord struct {
	// The request it was made for, with its status stripped.
	request apsigv2.SignalData
	path    string
}

type downloadController struct {
	logger   *logrus.Entry
	client   crcli.Client
	delegate apdel.ControllerDelegate

	manifestBuilder DownloadManifestBuilder

	// The download this reconciler completed most recently, so that a failure
	// after the download doesn't cost another fetch. Kept per reconciler, not
	// per process: it only has to survive a requeue, so losing it on a manager
	// rebuild costs one repeated download, not correctness.
	completed atomic.Pointer[downloadRecord]
}

// NewDownloadController builds a download reconciler that delegates to a manifest builder to
// determine what to actually download.
func NewDownloadController(logger *logrus.Entry, client crcli.Client, delegate apdel.ControllerDelegate, manifestBuilder DownloadManifestBuilder) crrec.Reconciler {
	return &downloadController{
		logger:          logger.WithFields(logrus.Fields{"reconciler": "downloading", "object": delegate.Name()}),
		client:          client,
		delegate:        delegate,
		manifestBuilder: manifestBuilder,
	}
}

// Reconcile collects the signaling information from the request, and invokes the configured manifest builder to
// determine what to download + what to transition to when completed.
func (r *downloadController) Reconcile(ctx context.Context, req cr.Request) (cr.Result, error) {
	signalNode := r.delegate.CreateObject()
	if err := r.client.Get(ctx, req.NamespacedName, signalNode); err != nil {
		return cr.Result{}, fmt.Errorf("unable to get download object for node='%s': %w", req.Name, err)
	}

	var signalData apsigv2.SignalData
	if err := signalData.Unmarshal(signalNode.GetAnnotations()); err != nil {
		return cr.Result{}, fmt.Errorf("unable to unmarshal signal data for node='%s': %w", req.Name, err)
	}

	logger := r.logger.WithField("signalnode", signalNode.GetName())

	if signalData.Status != nil && signalData.Status.Status != Downloading {
		logger.Debug("Ignoring signal status ", signalData.Status.Status)
		return cr.Result{}, nil
	}

	signalNodeCopy := r.delegate.DeepCopy(signalNode)

	// Figure out what needs to be downloaded + where to go when completed.
	manifest, err := r.manifestBuilder.Build(signalNodeCopy, signalData)
	if err != nil {
		return cr.Result{}, fmt.Errorf("unable to build download manifest: %w", err)
	}

	var status string
	if path, ok := r.reusableDownload(logger, signalData, manifest); ok {
		logger.Infof("Reusing '%s', already downloaded from '%s'", path, manifest.URL)
		status = manifest.SuccessState
	} else {
		logger.Infof("Starting download of '%s'", manifest.URL)

		httpdl := apdl.NewDownloader(manifest.Config)
		path, err := httpdl.Download(ctx)
		if err != nil {
			logger.Errorf("Unable to download '%s': %v", manifest.URL, err)

			// When the download failed move the status to `FailedDownload`
			status = FailedDownload
		} else {
			logger.Infof("Download of '%s' successful", manifest.URL)

			// Record it first: a failure below must not cost another fetch.
			r.recordDownload(signalData, path)

			// When the download is complete move the status to the success state
			status = manifest.SuccessState
		}
	}

	signalData.Status = apsigv2.NewStatus(status)

	if err := signalData.Marshal(signalNodeCopy.GetAnnotations()); err != nil {
		return cr.Result{}, fmt.Errorf("failed to marshal signal data: %w", err)
	}

	logger.Infof("Updating signaling response to '%s'", signalData.Status.Status)
	if err := r.client.Update(ctx, signalNodeCopy, &crcli.UpdateOptions{}); err != nil {
		return cr.Result{}, fmt.Errorf("failed to update signal node to status '%s': %w", signalData.Status.Status, err)
	}

	return cr.Result{}, nil
}

// recordDownload remembers a completed download, so a requeue needn't refetch.
func (r *downloadController) recordDownload(signalData apsigv2.SignalData, path string) {
	// The status is not part of a request's identity: the airgap signal and
	// download reconcilers share a status-less filter and disagree about it.
	signalData.Status = nil

	r.completed.Store(&downloadRecord{request: signalData, path: path})
}

// reusableDownload reports a still-present file already downloaded for this request.
func (r *downloadController) reusableDownload(logger *logrus.Entry, signalData apsigv2.SignalData, manifest DownloadManifest) (string, bool) {
	record := r.completed.Load()
	if record == nil {
		return "", false
	}

	signalData.Status = nil
	if !reflect.DeepEqual(record.request, signalData) {
		return "", false
	}

	// The manifest, not the signal data alone, decides where a download belongs.
	// Reusing one from elsewhere would report success with the target path
	// empty, which apply reads as "already applied". Recorded paths are
	// absolute, so resolve the configured directory likewise.
	wantDir, err := filepath.Abs(manifest.DownloadDir)
	if err != nil {
		logger.Infof("Not reusing '%s': %v", record.path, err)
		return "", false
	}
	if filepath.Dir(record.path) != wantDir {
		logger.Infof("Not reusing '%s': expected a download in '%s'", record.path, wantDir)
		return "", false
	}
	if manifest.Filename != "" && filepath.Base(record.path) != manifest.Filename {
		logger.Infof("Not reusing '%s': expected a download named '%s'", record.path, manifest.Filename)
		return "", false
	}

	// Existence is enough: the file only appears through an atomic rename after
	// the download verified its hash. Re-hashing would undo the saving.
	stat, err := os.Stat(record.path)
	if err != nil {
		logger.Infof("Not reusing '%s': %v", record.path, err)
		return "", false
	}
	if !stat.Mode().IsRegular() {
		logger.Infof("Not reusing '%s': not a regular file", record.path)
		return "", false
	}

	return record.path, true
}
