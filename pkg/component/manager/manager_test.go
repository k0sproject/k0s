/*
Copyright 2021 k0s authors

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

package manager

import (
	"context"
	"fmt"
	"testing"
	"time"

	proberPackage "github.com/k0sproject/k0s/pkg/component/prober"
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

	ctx := context.Background()
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

	ctx := context.Background()
	f1 := &Fake{
		InitErr: fmt.Errorf("failed"),
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

	ctx := context.Background()

	f1 := &Fake{}
	m.Add(ctx, f1)

	f2 := &Fake{
		RunErr: fmt.Errorf("failed"),
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

	ctx := context.Background()

	f1 := &Fake{}
	m.Add(ctx, f1)

	f2 := &Fake{
		HealthyErr: fmt.Errorf("failed"),
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
