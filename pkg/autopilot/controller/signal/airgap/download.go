// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package airgap

import (
	"crypto/sha256"
	"path"
	"strings"

	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigcomm "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common"
	apdl "github.com/k0sproject/k0s/pkg/autopilot/download"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	"github.com/sirupsen/logrus"
	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	crpred "sigs.k8s.io/controller-runtime/pkg/predicate"
)

type downloadManfiestBuilderAirgap struct {
	k0sDataDir string
}

var _ apsigcomm.DownloadManifestBuilder = (*downloadManfiestBuilderAirgap)(nil)

// registerDownloading registers the 'airgap-downloading' controller to the
// controller-runtime manager.
//
// This controller is only interested when autopilot signaling annotations have
// moved to a `Downloading` status. At this point, it will attempt to download
// the file provided in the update request.
func registerDownloading(logger *logrus.Entry, mgr crman.Manager, eventFilter crpred.Predicate, delegate apdel.ControllerDelegate, k0sDataDir string) error {
	name := strings.ToLower(delegate.Name()) + "_airgap_downloading"
	logger.Info("Registering reconciler: ", name)

	return cr.NewControllerManagedBy(mgr).
		Named(name).
		For(delegate.CreateObject()).
		WithEventFilter(eventFilter).
		Complete(
			apsigcomm.NewDownloadController(logger, mgr.GetClient(), delegate, &downloadManfiestBuilderAirgap{k0sDataDir: k0sDataDir}),
		)
}

// Build inspects the signaling information (data + node) to determine what should be downloaded, as
// well as what the next states are to be.
func (b downloadManfiestBuilderAirgap) Build(signalNode crcli.Object, signalData apsigv2.SignalData) (apsigcomm.DownloadManifest, error) {
	m := apsigcomm.DownloadManifest{
		Config: apdl.Config{
			URL:          signalData.Command.AirgapUpdate.URL,
			ExpectedHash: signalData.Command.AirgapUpdate.Sha256,
			Hasher:       sha256.New(),
			DownloadDir:  path.Join(b.k0sDataDir, "images"),
		},
		SuccessState: apsigcomm.Completed,
	}

	return m, nil
}
