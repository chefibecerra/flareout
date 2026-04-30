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

package cloudflare

import (
	"context"
	"errors"
	"fmt"

	cf "github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/dns"
	"github.com/cloudflare/cloudflare-go/v4/option"
	"github.com/cloudflare/cloudflare-go/v4/zones"

	"github.com/chefibecerra/flareout/internal/domain"
)

// RecordLister implements domain.RecordLister using cloudflare-go/v4.
// Reuses the existing Option / WithBaseURL functional-option pattern from client.go.
type RecordLister struct {
	client *cf.Client
}

// NewRecordLister constructs a RecordLister with the given API token.
// Additional options (e.g. WithBaseURL for tests) are applied after the token option.
// Option and WithBaseURL are defined in client.go and shared within this package.
func NewRecordLister(token string, opts ...Option) (*RecordLister, error) {
	reqOpts := []option.RequestOption{option.WithAPIToken(token)}
	for _, o := range opts {
		o(&reqOpts)
	}
	return &RecordLister{client: cf.NewClient(reqOpts...)}, nil
}

// ListZones fetches all Cloudflare zones accessible with the configured token.
// Uses ListAutoPaging to transparently handle multi-page results.
// Context cancellation stops the pager and returns context.Canceled unwrapped.
func (l *RecordLister) ListZones(ctx context.Context) ([]domain.Zone, error) {
	pager := l.client.Zones.ListAutoPaging(ctx, zones.ZoneListParams{})
	var out []domain.Zone
	for pager.Next() {
		z := pager.Current()
		out = append(out, domain.Zone{
			ID:     z.ID,
			Name:   z.Name,
			Status: string(z.Status),
		})
	}
	if err := pager.Err(); err != nil {
		return nil, mapListErr("list zones", err)
	}
	return out, nil
}

// ListRecords fetches all DNS records for the given zoneID.
// cf.F(zoneID) wraps the string as a param.Field[string] — required by RecordListParams.ZoneID.
//
// Note on the v4 SDK: dns.RecordResponse does NOT carry ZoneID or ZoneName fields, so
// those are left empty on the returned slice. The application layer enriches each Record
// with zone metadata after the call, since the caller already has the Zone in hand.
//
// dns.RecordResponse.Proxied is a plain bool in v4 (not *bool); no nil normalization is
// required at this boundary. dns.RecordResponse.TTL is dns.TTL (a float64 alias) and is
// cast to int64 for the domain Record.
//
// Context cancellation returns context.Canceled unwrapped (not a domain sentinel).
func (l *RecordLister) ListRecords(ctx context.Context, zoneID string) ([]domain.Record, error) {
	pager := l.client.DNS.Records.ListAutoPaging(ctx, dns.RecordListParams{
		ZoneID: cf.F(zoneID),
	})
	var out []domain.Record
	for pager.Next() {
		r := pager.Current()
		out = append(out, domain.Record{
			ID:      r.ID,
			Type:    string(r.Type),
			Name:    r.Name,
			Content: r.Content,
			ZoneID:  zoneID,
			Proxied: r.Proxied,
			TTL:     int64(r.TTL),
		})
	}
	if err := pager.Err(); err != nil {
		return nil, mapListErr("list records", err)
	}
	return out, nil
}

// mapListErr classifies errors from the SDK pager into domain-appropriate errors.
//   - net.Error (transport failure) → wraps domain.ErrVerifyNetwork
//   - context.Canceled / context.DeadlineExceeded → returned as-is (not wrapped)
//     so errors.Is(err, context.Canceled) works at the caller.
//   - Other errors → wrapped with operation context for display clarity.
//
// isNetworkError is defined in client.go (same package).
func mapListErr(op string, err error) error {
	if isNetworkError(err) {
		return fmt.Errorf("cloudflare %s: %w", op, domain.ErrVerifyNetwork)
	}
	if isContextErr(err) {
		// Context errors propagate unwrapped so callers can use errors.Is(err, context.Canceled).
		return err
	}
	return fmt.Errorf("cloudflare %s: %w", op, err)
}

// isContextErr reports whether err is a context cancellation or deadline exceeded.
// These are expected exit signals — not infrastructure faults — and must not be
// reclassified as domain sentinels.
func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
