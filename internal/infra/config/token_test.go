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

package config_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/chefibecerra/flareout/internal/domain"
	"github.com/chefibecerra/flareout/internal/infra/config"
)

func TestReadTokenFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		envVal  string
		setEnv  bool
		wantErr bool
	}{
		{
			name:    "missing token returns wrapped ErrTokenMissing",
			envVal:  "",
			setEnv:  false,
			wantErr: true,
		},
		{
			name:    "present token returns raw token with nil error",
			envVal:  "test-api-token-value",
			setEnv:  true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Setenv auto-restores the env var after the test.
			if tt.setEnv {
				t.Setenv("CLOUDFLARE_API_TOKEN", tt.envVal)
			} else {
				// Ensure the var is absent for the missing-token case.
				t.Setenv("CLOUDFLARE_API_TOKEN", "")
			}

			got, err := config.ReadTokenFromEnv()

			if tt.wantErr {
				if err == nil {
					t.Fatal("ReadTokenFromEnv() returned nil error, wanted non-nil")
				}
				if !errors.Is(err, domain.ErrTokenMissing) {
					t.Errorf("ReadTokenFromEnv() error does not wrap domain.ErrTokenMissing; got: %v", err)
				}
				msg := err.Error()
				if !strings.Contains(msg, "CLOUDFLARE_API_TOKEN") {
					t.Errorf("error message does not contain env var name %q; got: %s", "CLOUDFLARE_API_TOKEN", msg)
				}
				if !strings.Contains(msg, "https://dash.cloudflare.com/profile/api-tokens") {
					t.Errorf("error message does not contain remediation URL; got: %s", msg)
				}
				if !strings.Contains(msg, "DNS:Read") {
					t.Errorf("error message does not mention DNS:Read scope; got: %s", msg)
				}
			} else {
				if err != nil {
					t.Fatalf("ReadTokenFromEnv() returned unexpected error: %v", err)
				}
				if got != tt.envVal {
					t.Errorf("ReadTokenFromEnv() = %q, want %q", got, tt.envVal)
				}
			}
		})
	}
}
