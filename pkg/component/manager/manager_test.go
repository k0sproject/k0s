// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"testing"
	"time"

	proberPackage "github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Fake struct {
	InitErr      error
	RunErr       error
	StopErr      error
	ReconcileErr error
	HealthyErr   error

	InitCalled    bool
	RunCalled     bool
	StopCalled    bool
	HealthyCalled bool
}

func (f *Fake) Init(_ context.Context) error {
	f.InitCalled = true
	return f.InitErr
}
func (f *Fake) Start(_ context.Context) error {
	f.RunCalled = true
	return f.RunErr
}

func (f *Fake) Stop() error {
	f.StopCalled = true
	return f.StopErr
}
func (f *Fake) Ready() error {
	f.HealthyCalled = true
	return f.HealthyErr
}

func TestManagerSuccess(t *testing.T) {
	m := New(proberPackage.NopProber{})
	require.NotNil(t, m)

	ctx := t.Context()
	f1 := &Fake{}
	m.Add(ctx, f1)

	f2 := &Fake{}
	m.Add(ctx, f2)

	require.NoError(t, m.Init(ctx))
	require.True(t, f1.InitCalled)
	require.True(t, f2.InitCalled)

	require.NoError(t, m.Start(ctx))
	require.True(t, f1.RunCalled)
	require.True(t, f2.RunCalled)
	require.True(t, f1.HealthyCalled)
	require.True(t, f2.HealthyCalled)

	require.NoError(t, m.Stop())
	require.True(t, f1.StopCalled)
	require.True(t, f2.StopCalled)
}

func TestManagerInitFail(t *testing.T) {
	m := New(proberPackage.NopProber{})
	require.NotNil(t, m)

	ctx := t.Context()
	f1 := &Fake{
		InitErr: assert.AnError,
	}
	m.Add(ctx, f1)

	f2 := &Fake{}
	m.Add(ctx, f2)

	require.Error(t, m.Init(ctx))

	// all init should be called even if any fails
	require.True(t, f1.InitCalled)
	require.True(t, f2.InitCalled)
}

func TestManagerRunFail(t *testing.T) {
	m := New(proberPackage.NopProber{})
	require.NotNil(t, m)

	ctx := t.Context()

	f1 := &Fake{}
	m.Add(ctx, f1)

	f2 := &Fake{
		RunErr: assert.AnError,
	}
	m.Add(ctx, f2)

	f3 := &Fake{}
	m.Add(ctx, f3)

	require.Error(t, m.Start(ctx))
	require.True(t, f1.RunCalled)
	require.True(t, f2.RunCalled)
	require.False(t, f3.RunCalled)

	require.True(t, f1.HealthyCalled)
	require.False(t, f2.HealthyCalled)
	require.False(t, f3.HealthyCalled)

	require.True(t, f1.StopCalled)
	require.False(t, f2.StopCalled)
	require.False(t, f3.StopCalled)
}

func TestManagerHealthyFail(t *testing.T) {
	m := New(proberPackage.NopProber{})
	require.NotNil(t, m)
	m.ReadyWaitDuration = 1 * time.Millisecond

	ctx := t.Context()

	f1 := &Fake{}
	m.Add(ctx, f1)

	f2 := &Fake{
		HealthyErr: assert.AnError,
	}
	m.Add(ctx, f2)

	f3 := &Fake{}
	m.Add(ctx, f3)

	require.Error(t, m.Start(ctx))
	require.True(t, f1.RunCalled)
	require.True(t, f2.RunCalled)
	require.False(t, f3.RunCalled)

	require.True(t, f1.HealthyCalled)
	require.True(t, f2.HealthyCalled)
	require.False(t, f3.HealthyCalled)

	require.True(t, f1.StopCalled)
	require.True(t, f2.StopCalled)
	require.False(t, f3.StopCalled)
}
