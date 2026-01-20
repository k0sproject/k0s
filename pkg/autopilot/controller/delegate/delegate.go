// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package delegate

import (
	"context"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"

	"k8s.io/apimachinery/pkg/types"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

type K0sUpdateReadyStatus string

const (
	CanUpdate  K0sUpdateReadyStatus = "CanUpdate"
	NotReady   K0sUpdateReadyStatus = "NotReady"
	Incomplete K0sUpdateReadyStatus = "Incomplete"
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
	K0sUpdateReady(ctx context.Context, status apv1beta2.PlanCommandK0sUpdateStatus, obj crcli.Object) K0sUpdateReadyStatus
}

type createObjectFunc func() crcli.Object
type createObjectListFunc func() crcli.ObjectList
type objectListToPlanCommandTargetStatusFunc func(list crcli.ObjectList, status apv1beta2.PlanCommandTargetStateType) []apv1beta2.PlanCommandTargetStatus
type createNamespacedNameFunc func(name string) types.NamespacedName
type deepCopyFunc func(obj crcli.Object) crcli.Object
type k0sUpdateReadyFunc func(context.Context, apv1beta2.PlanCommandK0sUpdateStatus, crcli.Object) K0sUpdateReadyStatus

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
func (d controllerDelegate) K0sUpdateReady(ctx context.Context, status apv1beta2.PlanCommandK0sUpdateStatus, obj crcli.Object) K0sUpdateReadyStatus {
	return d.k0sUpdateReady(ctx, status, obj)
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
