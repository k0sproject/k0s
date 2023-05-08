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
