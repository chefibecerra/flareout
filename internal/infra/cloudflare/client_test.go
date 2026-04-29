// Copyright 2026 JOSE MARIA BECERRA VAZQUEZ
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

package cloudflare_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chefibecerra/flareout/internal/domain"
	cfinfra "github.com/chefibecerra/flareout/internal/infra/cloudflare"
)

// makeServer returns a test server that responds with the given status code and body.
func makeServer(t *testing.T, statusCode int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	}))
}

func TestCloudflareTokenVerifier(t *testing.T) {
	const fakeToken = "fake-token-for-testing"

	t.Run("active token returns TokenStatus{Active: true} with nil error", func(t *testing.T) {
		server := makeServer(t, http.StatusOK,
			`{"success":true,"result":{"status":"active"},"errors":[],"messages":[]}`)
		defer server.Close()

		v, err := cfinfra.NewTokenVerifier(fakeToken, cfinfra.WithBaseURL(server.URL))
		if err != nil {
			t.Fatalf("NewTokenVerifier: %v", err)
		}

		status, err := v.Verify(context.Background())
		if err != nil {
			t.Fatalf("Verify() returned unexpected error: %v", err)
		}
		if !status.Active {
			t.Error("Verify() TokenStatus.Active = false, want true")
		}
	})

	t.Run("expired status returns ErrTokenExpired", func(t *testing.T) {
		server := makeServer(t, http.StatusOK,
			`{"success":true,"result":{"status":"expired"},"errors":[],"messages":[]}`)
		defer server.Close()

		v, err := cfinfra.NewTokenVerifier(fakeToken, cfinfra.WithBaseURL(server.URL))
		if err != nil {
			t.Fatalf("NewTokenVerifier: %v", err)
		}

		_, err = v.Verify(context.Background())
		if err == nil {
			t.Fatal("Verify() returned nil error, want ErrTokenExpired")
		}
		if !errors.Is(err, domain.ErrTokenExpired) {
			t.Errorf("Verify() error does not wrap ErrTokenExpired; got: %v", err)
		}
		// Raw token MUST NOT appear in the error message.
		if strings.Contains(err.Error(), fakeToken) {
			t.Errorf("Verify() error message contains raw token value")
		}
	})

	t.Run("network error returns ErrVerifyNetwork", func(t *testing.T) {
		server := makeServer(t, http.StatusOK, `{}`)
		// Close the server immediately to simulate a network failure.
		server.Close()

		v, err := cfinfra.NewTokenVerifier(fakeToken, cfinfra.WithBaseURL(server.URL))
		if err != nil {
			t.Fatalf("NewTokenVerifier: %v", err)
		}

		_, err = v.Verify(context.Background())
		if err == nil {
			t.Fatal("Verify() returned nil error after server closed, want ErrVerifyNetwork")
		}
		if !errors.Is(err, domain.ErrVerifyNetwork) {
			t.Errorf("Verify() error does not wrap ErrVerifyNetwork; got: %v", err)
		}
		if strings.Contains(err.Error(), fakeToken) {
			t.Errorf("Verify() error message contains raw token value")
		}
	})

	t.Run("non-200 API error returns ErrTokenInactive", func(t *testing.T) {
		server := makeServer(t, http.StatusUnauthorized,
			`{"success":false,"errors":[{"code":9109,"message":"Invalid access token"}],"messages":[],"result":null}`)
		defer server.Close()

		v, err := cfinfra.NewTokenVerifier(fakeToken, cfinfra.WithBaseURL(server.URL))
		if err != nil {
			t.Fatalf("NewTokenVerifier: %v", err)
		}

		_, err = v.Verify(context.Background())
		if err == nil {
			t.Fatal("Verify() returned nil error on 401, want ErrTokenInactive")
		}
		if !errors.Is(err, domain.ErrTokenInactive) {
			t.Errorf("Verify() error does not wrap ErrTokenInactive; got: %v", err)
		}
		if strings.Contains(err.Error(), fakeToken) {
			t.Errorf("Verify() error message contains raw token value")
		}
	})
}
