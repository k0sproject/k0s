// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
