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
	EID_K0S_INF_0 = 10
	EID_K0S_ERR_0 = 0
)

type InfoEventID uint16

func (id InfoEventID) EventID() uint32 { return EID_K0S_INF_0 + uint32(id) }

type ErrorEventID uint16

func (id ErrorEventID) EventID() uint32 { return EID_K0S_ERR_0 + uint32(id) }

type eventLog struct {
	log  logrus.FieldLogger
	elog *eventlog.Log
}

var _ EventLog = (*eventLog)(nil)

func (e *eventLog) Info(id InfoEventID, args ...any) {
	e.log.WithField("eventId", id.EventID()).Info(args...)
	if e.elog != nil {
		_ = e.elog.Info(id.EventID(), fmt.Sprint(args...))
	}
}

func (e *eventLog) Error(id ErrorEventID, args ...any) {
	e.log.WithField("eventId", id.EventID()).Error(args...)
	if e.elog != nil {
		_ = e.elog.Error(id.EventID(), fmt.Sprint(args...))
	}
}
