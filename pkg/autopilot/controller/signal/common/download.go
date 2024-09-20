// Copyright 2021 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"context"
	"fmt"

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

type downloadController struct {
	logger   *logrus.Entry
	client   crcli.Client
	delegate apdel.ControllerDelegate

	manifestBuilder DownloadManifestBuilder
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
		return cr.Result{}, fmt.Errorf("unable to get download object for node='%s': %w", req.NamespacedName.Name, err)
	}

	var signalData apsigv2.SignalData
	if err := signalData.Unmarshal(signalNode.GetAnnotations()); err != nil {
		return cr.Result{}, fmt.Errorf("unable to unmarshal signal data for node='%s': %w", req.NamespacedName.Name, err)
	}

	signalNodeCopy := r.delegate.DeepCopy(signalNode)
	logger := r.logger.WithField("signalnode", signalNode.GetName())

	// Figure out what needs to be downloaded + where to go when completed.
	manifest, err := r.manifestBuilder.Build(signalNodeCopy, signalData)
	if err != nil {
		return cr.Result{}, fmt.Errorf("unable to build download manifest: %w", err)
	}

	logger.Infof("Starting download of '%s'", manifest.URL)

	httpdl := apdl.NewDownloader(manifest.Config)
	if err := httpdl.Download(ctx); err != nil {
		logger.Errorf("Unable to download '%s': %v", manifest.URL, err)

		// When the download is complete move the status to `FailedDownload`
		signalData.Status = apsigv2.NewStatus(FailedDownload)

	} else {
		logger.Infof("Download of '%s' successful", manifest.URL)
		// When the download is complete move the status to the success state
		signalData.Status = apsigv2.NewStatus(manifest.SuccessState)
	}

	if err := signalData.Marshal(signalNodeCopy.GetAnnotations()); err != nil {
		return cr.Result{}, fmt.Errorf("failed to marshal signal data: %w", err)
	}

	logger.Infof("Updating signaling response to '%s'", signalData.Status.Status)
	if err := r.client.Update(ctx, signalNodeCopy, &crcli.UpdateOptions{}); err != nil {
		return cr.Result{}, fmt.Errorf("failed to update signal node to status '%s': %w", signalData.Status.Status, err)
	}

	return cr.Result{}, nil
}
