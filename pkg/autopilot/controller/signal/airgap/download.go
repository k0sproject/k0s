// Copyright 2022 k0s authors
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

package airgap

import (
	"crypto/sha256"
	"path"

	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
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
	logger.Infof("Registering 'airgap-downloading' reconciler for '%s'", delegate.Name())

	return cr.NewControllerManagedBy(mgr).
		Named("airgap-downloading").
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
			DownloadDir:  path.Join(b.k0sDataDir, apconst.K0sImagesDir),
		},
		SuccessState: apsigcomm.Completed,
	}

	return m, nil
}
