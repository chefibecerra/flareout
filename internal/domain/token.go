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

package domain

import (
	"context"
	"time"
)

// MaskToken returns a safe log-friendly representation of s.
// The last 4 characters are preserved so operators can correlate tokens
// without exposing the full credential.
func MaskToken(s string) string {
	if len(s) == 0 {
		return "cfx...[empty]"
	}
	if len(s) < 4 {
		return "cfx..." + s
	}
	return "cfx..." + s[len(s)-4:]
}

// TokenStatus is an immutable snapshot of a verification result.
// Always passed by value. Never mutated after construction.
type TokenStatus struct {
	Active    bool
	ExpiresAt *time.Time // nil means "provider did not supply an expiry"
}

// TokenVerifier is the port any token-verification adapter must satisfy.
// The token value is captured at adapter construction time; it does NOT
// appear in this interface's method signature.
type TokenVerifier interface {
	// Verify checks whether the configured token is active with the upstream provider.
	// Returns TokenStatus on success (err == nil).
	// Returns a non-nil error wrapping a domain sentinel on any failure.
	// Implementations MUST honor ctx cancellation.
	// The raw token MUST NOT appear in any returned error message.
	Verify(ctx context.Context) (TokenStatus, error)
}

// ScopeWarningMessage is the exact string emitted to stderr after every
// successful token verification. It is non-suppressible and non-configurable.
// Wording avoids implying enforcement capability (VerifyAPIToken returns
// only liveness, not permission scope).
const ScopeWarningMessage = "FlareOut requires only DNS:Read zone-level permissions. " +
	"Cloudflare's API does not expose token scope, so FlareOut cannot enforce " +
	"least privilege automatically. Consider using a dedicated read-only token: " +
	"https://dash.cloudflare.com/profile/api-tokens"
