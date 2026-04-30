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

// Package cloudflare provides the CloudflareTokenVerifier adapter, which
// implements domain.TokenVerifier using the official cloudflare-go/v4 SDK.
// This package is the sole location in the codebase that imports cloudflare-go.
package cloudflare

import (
	"context"
	"errors"
	"fmt"
	"net"

	cf "github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/option"
	"github.com/cloudflare/cloudflare-go/v4/user"

	"github.com/chefibecerra/flareout/internal/domain"
)

// CloudflareTokenVerifier implements domain.TokenVerifier using cloudflare-go/v4.
// The token is captured at construction time and never exposed via the interface.
type CloudflareTokenVerifier struct {
	client *cf.Client
}

// Option is a functional option for configuring CloudflareTokenVerifier construction.
type Option func(*[]option.RequestOption)

// WithBaseURL overrides the Cloudflare API base URL.
// Used in tests to point the client at a local httptest server.
func WithBaseURL(url string) Option {
	return func(opts *[]option.RequestOption) {
		*opts = append(*opts, option.WithBaseURL(url))
	}
}

// WithMaxRetries overrides the SDK retry count. Default is 2 with exponential backoff.
// Tests typically pass 0 to fail fast on network/timeout errors instead of waiting
// for backoff between retry attempts.
func WithMaxRetries(n int) Option {
	return func(opts *[]option.RequestOption) {
		*opts = append(*opts, option.WithMaxRetries(n))
	}
}

// NewTokenVerifier constructs a CloudflareTokenVerifier with the given API token.
// Additional options (e.g. WithBaseURL) are applied after the token option.
func NewTokenVerifier(token string, opts ...Option) (*CloudflareTokenVerifier, error) {
	reqOpts := []option.RequestOption{option.WithAPIToken(token)}
	for _, o := range opts {
		o(&reqOpts)
	}
	client := cf.NewClient(reqOpts...)
	return &CloudflareTokenVerifier{client: client}, nil
}

// Verify checks whether the configured token is active with the Cloudflare API.
// Maps upstream results and errors to domain sentinels per the error-mapping table.
// The raw token MUST NOT appear in any returned error message.
func (v *CloudflareTokenVerifier) Verify(ctx context.Context) (domain.TokenStatus, error) {
	result, err := v.client.User.Tokens.Verify(ctx)
	if err != nil {
		if isNetworkError(err) {
			return domain.TokenStatus{}, fmt.Errorf("cloudflare verify: %w", domain.ErrVerifyNetwork)
		}
		return domain.TokenStatus{}, fmt.Errorf("cloudflare verify: %w", domain.ErrTokenInactive)
	}

	switch result.Status {
	case user.TokenVerifyResponseStatusActive:
		s := domain.TokenStatus{Active: true}
		if !result.ExpiresOn.IsZero() {
			t := result.ExpiresOn
			s.ExpiresAt = &t
		}
		return s, nil
	case user.TokenVerifyResponseStatusExpired:
		return domain.TokenStatus{Active: false},
			fmt.Errorf("cloudflare verify: %w", domain.ErrTokenExpired)
	default:
		return domain.TokenStatus{Active: false},
			fmt.Errorf("cloudflare verify: status %q: %w", string(result.Status), domain.ErrTokenInactive)
	}
}

// isNetworkError reports whether err is a network-level transport failure.
// Catches connection refused, timeout, and similar OS-level failures that
// prevent the request from reaching the server at all.
func isNetworkError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr)
}
