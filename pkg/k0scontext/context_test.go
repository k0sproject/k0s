// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0scontext_test

import (
	"testing"

	"github.com/k0sproject/k0s/pkg/k0scontext"

	"github.com/stretchr/testify/assert"
)

func TestHasValue_StructPtrs(t *testing.T) {
	type Foo struct{}
	type Bar struct{}

	ctx := t.Context()
	assert.False(t, k0scontext.HasValue[*Foo](ctx))
	assert.False(t, k0scontext.HasValue[*Bar](ctx))

	ctxWithFoo := k0scontext.WithValue(ctx, (*Foo)(nil))
	assert.True(t, k0scontext.HasValue[*Foo](ctxWithFoo))
	assert.False(t, k0scontext.HasValue[*Bar](ctxWithFoo))

	ctxWithFooAndBar := k0scontext.WithValue(ctxWithFoo, (*Bar)(nil))
	assert.True(t, k0scontext.HasValue[*Foo](ctxWithFooAndBar))
	assert.True(t, k0scontext.HasValue[*Bar](ctxWithFooAndBar))
}

func TestHasValue_Ifaces(t *testing.T) {
	type Foo any
	type Bar any

	ctx := t.Context()
	assert.False(t, k0scontext.HasValue[Foo](ctx))
	assert.False(t, k0scontext.HasValue[Bar](ctx))

	ctxWithFoo := k0scontext.WithValue(ctx, (Foo)(nil))
	assert.True(t, k0scontext.HasValue[Foo](ctxWithFoo))
	assert.False(t, k0scontext.HasValue[Bar](ctxWithFoo))

	ctxWithFooAndBar := k0scontext.WithValue(ctxWithFoo, (Bar)(nil))
	assert.True(t, k0scontext.HasValue[Foo](ctxWithFooAndBar))
	assert.True(t, k0scontext.HasValue[Bar](ctxWithFooAndBar))
}

func TestHasValue_Aliases(t *testing.T) {
	type Foo string
	type Bar string

	ctx := t.Context()
	assert.False(t, k0scontext.HasValue[Foo](ctx))
	assert.False(t, k0scontext.HasValue[Bar](ctx))

	ctxWithFoo := k0scontext.WithValue(ctx, Foo(""))
	assert.True(t, k0scontext.HasValue[Foo](ctxWithFoo))
	assert.False(t, k0scontext.HasValue[Bar](ctxWithFoo))

	ctxWithFooAndBar := k0scontext.WithValue(ctxWithFoo, Bar(""))
	assert.True(t, k0scontext.HasValue[Foo](ctxWithFooAndBar))
	assert.True(t, k0scontext.HasValue[Bar](ctxWithFooAndBar))
}

func TestValue_StructPtrs(t *testing.T) {
	type Foo struct{}
	type Bar struct{}

	ctx := t.Context()
	assert.Zero(t, k0scontext.Value[*Foo](ctx))
	assert.Zero(t, k0scontext.Value[*Bar](ctx))

	ctxWithFoo := k0scontext.WithValue(ctx, &Foo{})
	assert.Equal(t, &Foo{}, k0scontext.Value[*Foo](ctxWithFoo))
	assert.Zero(t, k0scontext.Value[*Bar](ctx))

	ctxWithFooAndBar := k0scontext.WithValue(ctxWithFoo, &Bar{})
	assert.Equal(t, &Foo{}, k0scontext.Value[*Foo](ctxWithFooAndBar))
	assert.Equal(t, &Bar{}, k0scontext.Value[*Bar](ctxWithFooAndBar))
}
