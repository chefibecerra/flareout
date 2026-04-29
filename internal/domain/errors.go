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

import "errors"

var (
	// ErrTokenMissing is returned when no token is available in the environment.
	ErrTokenMissing = errors.New("CLOUDFLARE_API_TOKEN is not set")

	// ErrTokenExpired is returned when the upstream confirms the token is expired.
	ErrTokenExpired = errors.New("token is expired")

	// ErrTokenInactive is returned when the token is disabled, revoked, or rejected.
	ErrTokenInactive = errors.New("token is inactive or revoked")

	// ErrVerifyNetwork is returned when the liveness check cannot be completed
	// due to a network or transport failure.
	ErrVerifyNetwork = errors.New("token verification failed: network error")
)
