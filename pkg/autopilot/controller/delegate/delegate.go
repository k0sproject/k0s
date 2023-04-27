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

package delegate

import (
	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"

	"k8s.io/apimachinery/pkg/types"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

type K0sUpdateReadyStatus string

const (
	CanUpdate    K0sUpdateReadyStatus = "CanUpdate"
	NotReady     K0sUpdateReadyStatus = "NotReady"
	Inconsistent K0sUpdateReadyStatus = "Inconsistent"
)

type ControllerDelegateMap map[string]ControllerDelegate

// ControllerDelegate provides a means for specialized specific functionality
// for signal node controllers.
type ControllerDelegate interface {
	Name() string
	CreateObject() crcli.Object
	CreateObjectList() crcli.ObjectList
	ObjectListToPlanCommandTargetStatus(list crcli.ObjectList, status apv1beta2.PlanCommandTargetStateType) []apv1beta2.PlanCommandTargetStatus
	CreateNamespacedName(name string) types.NamespacedName
	DeepCopy(crcli.Object) crcli.Object

	// K0sUpdate features
	K0sUpdateReady(status apv1beta2.PlanCommandK0sUpdateStatus, obj crcli.Object) K0sUpdateReadyStatus
}

type createObjectFunc func() crcli.Object
type createObjectListFunc func() crcli.ObjectList
type objectListToPlanCommandTargetStatusFunc func(list crcli.ObjectList, status apv1beta2.PlanCommandTargetStateType) []apv1beta2.PlanCommandTargetStatus
type createNamespacedNameFunc func(name string) types.NamespacedName
type deepCopyFunc func(obj crcli.Object) crcli.Object
type k0sUpdateReadyFunc func(apv1beta2.PlanCommandK0sUpdateStatus, crcli.Object) K0sUpdateReadyStatus

type controllerDelegate struct {
	name                                string
	createObject                        createObjectFunc
	createObjectList                    createObjectListFunc
	k0sUpdateReady                      k0sUpdateReadyFunc
	objectListToPlanCommandTargetStatus objectListToPlanCommandTargetStatusFunc
	createNamespacedName                createNamespacedNameFunc
	deepCopy                            deepCopyFunc
}

// Name returns the name of the delegate
func (d controllerDelegate) Name() string {
	return d.name
}

// CreateObject creates a new instance of the type supported by the delegate
func (d controllerDelegate) CreateObject() crcli.Object {
	return d.createObject()
}

// CreateObjectList creates a new instance of the list type supported by the delegate
func (d controllerDelegate) CreateObjectList() crcli.ObjectList {
	return d.createObjectList()
}

// K0sUpdateReady determines if the delegate object can accept an update.
func (d controllerDelegate) K0sUpdateReady(status apv1beta2.PlanCommandK0sUpdateStatus, obj crcli.Object) K0sUpdateReadyStatus {
	return d.k0sUpdateReady(status, obj)
}

// ObjectListToPlanCommandTargetStatus converts an ObjectList to a slice of PlanCommandTargetStatus
func (d controllerDelegate) ObjectListToPlanCommandTargetStatus(list crcli.ObjectList, status apv1beta2.PlanCommandTargetStateType) []apv1beta2.PlanCommandTargetStatus {
	return d.objectListToPlanCommandTargetStatus(list, status)
}

// CreateNamespacedName creates a new namespaced-name for accessing objects.
func (d controllerDelegate) CreateNamespacedName(name string) types.NamespacedName {
	return d.createNamespacedName(name)
}

// DeepCopy creates a deep-copy of the object provided
func (d controllerDelegate) DeepCopy(obj crcli.Object) crcli.Object {
	return d.deepCopy(obj)
}
