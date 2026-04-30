package cloudflare

import (
	"context"
	"fmt"

	cf "github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/dns"
	"github.com/cloudflare/cloudflare-go/v4/option"

	"github.com/chefibecerra/flareout/internal/domain"
)

// RecordToggler implements domain.RecordToggler using cloudflare-go/v4's
// DNS.Records.Edit endpoint. The Edit endpoint is PUT-shaped — it replaces
// the record with the body sent — so the caller must pass the current
// record state via SetProxied; this adapter rebuilds the full body with
// proxied swapped to the target value.
type RecordToggler struct {
	client *cf.Client
}

// NewRecordToggler constructs a RecordToggler. The Option set is shared
// with NewTokenVerifier and NewRecordLister (WithBaseURL, WithMaxRetries).
func NewRecordToggler(token string, opts ...Option) (*RecordToggler, error) {
	reqOpts := []option.RequestOption{option.WithAPIToken(token)}
	for _, o := range opts {
		o(&reqOpts)
	}
	return &RecordToggler{client: cf.NewClient(reqOpts...)}, nil
}

// SetProxied flips the proxied flag of current to proxied via Cloudflare's
// Edit endpoint. All other fields of current are preserved (Name, Type,
// Content, TTL).
func (t *RecordToggler) SetProxied(ctx context.Context, current domain.Record, proxied bool) error {
	body := dns.RecordEditParamsBody{
		Name:    cf.F(current.Name),
		Type:    cf.F(dns.RecordEditParamsBodyType(current.Type)),
		TTL:     cf.F(dns.TTL(current.TTL)),
		Content: cf.F(current.Content),
		Proxied: cf.F(proxied),
	}

	_, err := t.client.DNS.Records.Edit(ctx, current.ID, dns.RecordEditParams{
		ZoneID: cf.F(current.ZoneID),
		Body:   body,
	})
	if err != nil {
		return fmt.Errorf("cloudflare set-proxied: %w", err)
	}
	return nil
}
