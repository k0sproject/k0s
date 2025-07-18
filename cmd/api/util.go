// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

func sendError(err error, resp http.ResponseWriter, status ...int) {
	code := http.StatusInternalServerError
	if len(status) == 1 {
		code = status[0]
	}
	logrus.Error(err)
	resp.Header().Set("Content-Type", "text/plain")
	resp.WriteHeader(code)
	if _, err := resp.Write([]byte(err.Error())); err != nil {
		sendError(err, resp)
		return
	}
}
