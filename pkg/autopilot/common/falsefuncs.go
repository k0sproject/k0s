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
	crev "sigs.k8s.io/controller-runtime/pkg/event"
)

// FalseFuncs addresses the need of 'default false' behavior of the predicate
// implementation provided by `Funcs`.
type FalseFuncs struct {
	// Create returns true if the Create event should be processed
	CreateFunc func(crev.CreateEvent) bool

	// Delete returns true if the Delete event should be processed
	DeleteFunc func(crev.DeleteEvent) bool

	// Update returns true if the Update event should be processed
	UpdateFunc func(crev.UpdateEvent) bool

	// Generic returns true if the Generic event should be processed
	GenericFunc func(crev.GenericEvent) bool
}

// Create implements Predicate.
func (p FalseFuncs) Create(e crev.CreateEvent) bool {
	if p.CreateFunc != nil {
		return p.CreateFunc(e)
	}
	return false
}

// Delete implements Predicate.
func (p FalseFuncs) Delete(e crev.DeleteEvent) bool {
	if p.DeleteFunc != nil {
		return p.DeleteFunc(e)
	}
	return false
}

// Update implements Predicate.
func (p FalseFuncs) Update(e crev.UpdateEvent) bool {
	if p.UpdateFunc != nil {
		return p.UpdateFunc(e)
	}
	return false
}

// Generic implements Predicate.
func (p FalseFuncs) Generic(e crev.GenericEvent) bool {
	if p.GenericFunc != nil {
		return p.GenericFunc(e)
	}
	return false
}
