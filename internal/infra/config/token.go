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

package config

import (
	"fmt"
	"os"

	"github.com/chefibecerra/flareout/internal/domain"
)

const tokenEnvVar = "CLOUDFLARE_API_TOKEN"

// ReadTokenFromEnv reads the API token from the environment.
// Returns the raw token string on success.
// Returns a wrapped domain.ErrTokenMissing with a remediation hint on failure.
// The error message MUST NOT contain a token value (token is empty on this path).
func ReadTokenFromEnv() (string, error) {
	t := os.Getenv(tokenEnvVar)
	if t == "" {
		return "", fmt.Errorf("%w — set %s with DNS:Read scope. "+
			"Create a token at https://dash.cloudflare.com/profile/api-tokens",
			domain.ErrTokenMissing, tokenEnvVar)
	}
	return t, nil
}
