// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervised

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/svc/eventlog"
)

type EventLog interface {
	Info(id InfoEventID, args ...any)
	Error(id ErrorEventID, args ...any)
}

// Event IDs for k0s.
// https://learn.microsoft.com/en-us/windows/win32/eventlog/event-identifiers
const (

	// (Facility is 2835)
	// EID_K0S_SUC_0 = 0x2b130000
	// EID_K0S_INF_0 = 0x6b130000
	// EID_K0S_WRN_0 = 0xab130000
	// EID_K0S_ERR_0 = 0xeb130000

	// EID_K0S_INF_0 = 0x40000000
	// EID_K0S_ERR_0 = 0xc0000000

	EID_K0S_INF_0 = 10
	EID_K0S_ERR_0 = 0
)

type InfoEventID uint16

func (id InfoEventID) EventID() uint32 { return EID_K0S_INF_0 + uint32(id) }

type ErrorEventID uint16

func (id ErrorEventID) EventID() uint32 { return EID_K0S_ERR_0 + uint32(id) }

type eventLog struct {
	serviceName string
	log         *eventlog.Log
}

var _ EventLog = (*eventLog)(nil)

func (e *eventLog) Info(id InfoEventID, args ...any) {
	logrus.WithFields(logrus.Fields{
		"component": "eventlog",
		"service":   e.serviceName,
		"eventId":   id.EventID(),
	}).Info(args...)
	e.log.Info(id.EventID(), fmt.Sprint(args...))
}

func (e *eventLog) Error(id ErrorEventID, args ...any) {
	logrus.WithFields(logrus.Fields{
		"component": "eventlog",
		"service":   e.serviceName,
		"eventId":   id.EventID(),
	}).Error(args...)
	e.log.Error(id.EventID(), fmt.Sprint(args...))
}
