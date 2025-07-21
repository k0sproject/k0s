// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"

	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	"github.com/sirupsen/logrus"
	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crrec "sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SignalControllerContext provides all of the required data to operate, in addition provides
// signal information passed as 'context' to handler implementations.
type SignalControllerContext struct {
	Log      *logrus.Entry
	Client   crcli.Client
	Delegate apdel.ControllerDelegate

	SignalNode crcli.Object
	SignalData *apsigv2.SignalData
}

// WithSignalData produces a copy of the SignalContext with the provided SignalNode
// and SignalData fields populated.
func (sc SignalControllerContext) WithSignalData(logger *logrus.Entry, node crcli.Object, data *apsigv2.SignalData) SignalControllerContext {
	sc.Log = logger
	sc.SignalNode = node
	sc.SignalData = data

	return sc
}

// SignalControllerHandler allows implementations to handle the initial signaling data.
type SignalControllerHandler interface {
	Handle(ctx context.Context, sctx SignalControllerContext) (cr.Result, error)
}

type signalController struct {
	ctx     SignalControllerContext
	handler SignalControllerHandler
}

// NewSignalController builds up a Reconciler that specializes in handling the initial signaling
// 'signal' that is received by an autopilot controller.
func NewSignalController(logger *logrus.Entry, client crcli.Client, delegate apdel.ControllerDelegate, handler SignalControllerHandler) crrec.Reconciler {
	ctx := SignalControllerContext{
		Log:      logger.WithFields(logrus.Fields{"controller": "signal", "object": delegate.Name()}),
		Client:   client,
		Delegate: delegate,
	}

	return &signalController{ctx: ctx, handler: handler}
}

// Reconcile will obtain the required object from the cache and determine if it should
// be handled by implementations. Basic filtering on completeness and unmarshaling issues
// are addressed.
func (r *signalController) Reconcile(ctx context.Context, req cr.Request) (cr.Result, error) {
	signalNode := r.ctx.Delegate.CreateObject()
	if err := r.ctx.Client.Get(ctx, req.NamespacedName, signalNode); err != nil {
		return cr.Result{}, fmt.Errorf("unable to get signal for node='%s': %w", req.Name, err)
	}

	// Extract an update request from the signal node annotations, and determine at what
	// part of the lifecycle the signaling request is in (if available). If internal states
	// indicate that the signaling request is being processed, no update request will be
	// returned here.

	logger := r.ctx.Log.WithField("signalnode", signalNode.GetName())

	return r.handler.Handle(
		ctx,
		r.ctx.WithSignalData(
			logger,
			signalNode,
			extractSignalData(logger, signalNode.GetAnnotations()),
		),
	)
}

// extractSignalData ensures that a `SignalData` instance can be obtained from the provided
// map, relying on special annotation keys. This returns the `SignalData` instance if
// unmarshaling and non-completeness checks have passed.
func extractSignalData(logger *logrus.Entry, data map[string]string) *apsigv2.SignalData {
	signalData := &apsigv2.SignalData{}

	if err := signalData.Unmarshal(data); err != nil {
		logger.Warnf("Unable to unmarshal signal data: %v", err)
		return nil
	}

	// If the response is valid, but has already been completed, ignore this.
	if signalData.Status != nil && signalData.Status.Status == Completed {
		return nil
	}

	return signalData
}
