/*
Copyright 2023 k0s authors

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

package k0scontext_test

import (
	"context"
	"testing"

	"github.com/k0sproject/k0s/pkg/k0scontext"

	"github.com/stretchr/testify/assert"
)

func TestHasValue_StructPtrs(t *testing.T) {
	type Foo struct{}
	type Bar struct{}

	ctx := context.Background()
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
	type Foo interface{}
	type Bar interface{}

	ctx := context.Background()
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

	ctx := context.Background()
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

	ctx := context.Background()
	assert.Zero(t, k0scontext.Value[*Foo](ctx))
	assert.Zero(t, k0scontext.Value[*Bar](ctx))

	ctxWithFoo := k0scontext.WithValue(ctx, &Foo{})
	assert.Equal(t, k0scontext.Value[*Foo](ctxWithFoo), &Foo{})
	assert.Zero(t, k0scontext.Value[*Bar](ctx))

	ctxWithFooAndBar := k0scontext.WithValue(ctxWithFoo, &Bar{})
	assert.Equal(t, k0scontext.Value[*Foo](ctxWithFooAndBar), &Foo{})
	assert.Equal(t, k0scontext.Value[*Bar](ctxWithFooAndBar), &Bar{})
}
