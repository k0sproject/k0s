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

package k0s

import (
	"crypto/sha256"

	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigcomm "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common"
	apsigpred "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common/predicate"
	apdl "github.com/k0sproject/k0s/pkg/autopilot/download"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	"github.com/sirupsen/logrus"
	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crev "sigs.k8s.io/controller-runtime/pkg/event"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	crpred "sigs.k8s.io/controller-runtime/pkg/predicate"
)

// downloadEventFilter creates a controller-runtime predicate that governs which objects
// will make it into reconciliation, and which will be ignored.
func downloadEventFilter(hostname string, handler apsigpred.ErrorHandler) crpred.Predicate {
	return crpred.And(
		crpred.AnnotationChangedPredicate{},
		apsigpred.SignalNamePredicate(hostname),
		apsigpred.NewSignalDataPredicateAdapter(handler).And(
			signalDataUpdateCommandK0sPredicate(),
			apsigpred.SignalDataStatusPredicate(Downloading),
		),
		apcomm.FalseFuncs{
			CreateFunc: func(ce crev.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(ue crev.UpdateEvent) bool {
				return true
			},
		},
	)
}

type downloadManifestBuilderK0s struct {
	k0sBinaryDir string
}

var _ apsigcomm.DownloadManifestBuilder = (*downloadManifestBuilderK0s)(nil)

// registerDownloading registers the 'downloading' controller to the
// controller-runtime manager.
//
// This controller is only interested when autopilot signaling annotations have
// moved to a `Downloading` status. At this point, it will attempt to download
// the file provided in the update request.
func registerDownloading(logger *logrus.Entry, mgr crman.Manager, eventFilter crpred.Predicate, delegate apdel.ControllerDelegate, k0sBinaryDir string) error {
	logger.Infof("Registering k0s 'downloading' reconciler for '%s'", delegate.Name())

	return cr.NewControllerManagedBy(mgr).
		Named(delegate.Name() + "-downloading").
		For(delegate.CreateObject()).
		WithEventFilter(eventFilter).
		Complete(
			apsigcomm.NewDownloadController(logger, mgr.GetClient(), delegate, &downloadManifestBuilderK0s{
				k0sBinaryDir: k0sBinaryDir,
			}),
		)
}

// Build inspects the signaling information (data + node) to determine what should be downloaded, as
// well as what the next states are to be.
func (b downloadManifestBuilderK0s) Build(signalNode crcli.Object, signalData apsigv2.SignalData) (apsigcomm.DownloadManifest, error) {
	m := apsigcomm.DownloadManifest{
		Config: apdl.Config{
			URL:          signalData.Command.K0sUpdate.URL,
			ExpectedHash: signalData.Command.K0sUpdate.Sha256,
			Hasher:       sha256.New(),
			DownloadDir:  b.k0sBinaryDir,
			Filename:     apconst.K0sTempFilename,
		},
		SuccessState: Cordoning,
	}

	return m, nil
}
