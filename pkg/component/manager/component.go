/*
Copyright 2020 k0s authors

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

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

// Component defines the lifecycle of managed components.
//
//	Created ――――――――――――――――――――――►(Stop)―――――╮
//	╰―(Init)―► Initialized ―――――――►(Stop)――――╮│
//	           ╰―(Start)―► Running ―►(Stop)―╮││
//	              ╭(Healthy)╯▲              ▼▼▼
//	              ╰――――――――――╯         ╭► Terminated╮
//	                                   ╰――――(Stop)――╯
type Component interface {
	// Init initializes the component and prepares it for execution. This should
	// include any fallible operations that can be performed without actually
	// starting the component, such as creating files and folders or validating
	// configuration settings. Init must be the first method called in the
	// component's lifecycle. Init must be called only once.
	Init(context.Context) error

	// Start starts the component. When Start returns, the component is executing in
	// the background. Run may be called only once after Init. If the component
	// is a Reconciler, a call to Reconcile may be required before Start
	// can be called. The given context is not intended to replace a call to
	// Stop when canceled. It's merely used to cancel the component's startup.
	Start(context.Context) error

	// Stop stops this component, potentially cleaning up any temporary
	// resources attached to it. Stop itself may be called in any lifecycle
	// phase. All other lifecycle methods have to return an error after Stop
	// returns. Stop may be called more than once.
	Stop() error
}

type Ready interface {
	// Ready performs a ready check and indicates that a component is ready to run.
	Ready() error
}

// Reconciler defines the component interface that is reconciled based
// on changes on the global config CR object changes.
//
//	Created ――――――――――――――――――――――――――►(Stop)―――――╮
//	╰―(Init)―► Initialized ―――――――――――►(Stop)――――╮│
//	╭(Reconcile)╯▲╰―(Start)―► Running ―►(Stop)―――╮││
//	╰――――――――――――╯╭(Reconcile)╯▲▲╰(Healthy)╮     ▼▼▼
//	              ╰――――――――――――╯╰――――――――――╯╭► Terminated╮
//	                                        ╰――――(Stop)――╯
type Reconciler interface {
	// Reconcile aligns the actual state of this component with the desired cluster
	// configuration. Reconcile may only be called after Init and before Stop.
	Reconcile(context.Context, *v1beta1.ClusterConfig) error
}
