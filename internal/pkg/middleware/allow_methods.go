// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"net/http"
	"strings"
)

// AllowMethods is a middleware that accepts list of given HTTP methods.
// Responds with HTTP status code 405 "Method not allowed" if no match is found.
func AllowMethods(methods ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			for _, m := range methods {
				if strings.EqualFold(m, r.Method) {
					next.ServeHTTP(w, r)
					return
				}
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return http.HandlerFunc(fn)
	}
}
