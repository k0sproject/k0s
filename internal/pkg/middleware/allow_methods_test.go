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
	"net/http/httptest"
	"testing"
)

func TestAllowMethods(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		allowMethods []string
		isNextCalled bool
	}{
		{name: "allow get", method: http.MethodGet, allowMethods: []string{http.MethodGet}, isNextCalled: true},
		{name: "allow post", method: http.MethodPost, allowMethods: []string{http.MethodPost}, isNextCalled: true},
		{name: "allow get and post", method: http.MethodGet, allowMethods: []string{http.MethodGet, http.MethodPost}, isNextCalled: true},
		{name: "deny all", method: http.MethodGet},
		{name: "deny post", method: http.MethodPost, allowMethods: []string{http.MethodGet}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isNextCalled := false
			fn := func(_ http.ResponseWriter, _ *http.Request) {
				isNextCalled = true
			}
			h := AllowMethods(test.allowMethods...)(http.HandlerFunc(fn))
			rec := httptest.NewRecorder()
			r, err := http.NewRequest(test.method, "/", nil)
			if err != nil {
				t.Fatal(err)
			}

			h.ServeHTTP(rec, r)
			if isNextCalled != test.isNextCalled {
				t.Fatalf("test: %s got: %v want: %v", test.name, isNextCalled, test.isNextCalled)
			}
			if isNextCalled {
				if rec.Result().StatusCode != http.StatusOK {
					t.Fatalf("test: %s got: %v want: %v", test.name, rec.Result().StatusCode, http.StatusOK)
				}
			} else {
				if rec.Result().StatusCode != http.StatusMethodNotAllowed {
					t.Fatalf("test: %s got: %v want: %v", test.name, rec.Result().StatusCode, http.StatusMethodNotAllowed)
				}
			}
		})
	}
}
