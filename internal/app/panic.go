// panic.go is the fourth documented exception to the app-layer-imports-only-domain
// rule. It performs a bulk un-proxy across an entire zone, calling
// ApplyToggle for every currently-proxied record. Layering test exempts
// this filename — see internal/app/layering_test.go.
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/chefibecerra/flareout/internal/domain"
)

// PanicResult is one record's outcome from a bulk un-proxy run.
type PanicResult struct {
	Record       domain.Record
	Applied      bool
	SnapshotPath string
	Err          error
}

// PanicSummary describes a panic operation before it runs (preview) or
// the aggregate outcome after it runs.
type PanicSummary struct {
	Zone           domain.Zone
	ProxiedRecords []domain.Record
	Results        []PanicResult
}

// ErrPanicZoneNotFound is returned when the requested zone name does not
// match any zone the API token can list.
var ErrPanicZoneNotFound = errors.New("panic: zone not found")

// PanicPreviewZone resolves the zone, lists every record under it, and
// returns the subset currently marked Proxied. NO mutations happen — this
// is the dry-run preview the CLI shows before asking for confirmation.
func PanicPreviewZone(ctx context.Context, appCtx *Context, zoneName string) (PanicSummary, error) {
	zones, err := appCtx.Lister.ListZones(ctx)
	if err != nil {
		return PanicSummary{}, fmt.Errorf("panic: list zones: %w", err)
	}

	var zone domain.Zone
	for _, z := range zones {
		if strings.EqualFold(z.Name, zoneName) {
			zone = z
			break
		}
	}
	if zone.ID == "" {
		return PanicSummary{}, fmt.Errorf("%w: %s", ErrPanicZoneNotFound, zoneName)
	}

	records, err := appCtx.Lister.ListRecords(ctx, zone.ID)
	if err != nil {
		return PanicSummary{}, fmt.Errorf("panic: list records: %w", err)
	}

	var proxied []domain.Record
	for _, r := range records {
		if !r.Proxied {
			continue
		}
		// Enrich (Cloudflare adapter leaves these empty).
		r.ZoneID = zone.ID
		r.ZoneName = zone.Name
		proxied = append(proxied, r)
	}

	return PanicSummary{Zone: zone, ProxiedRecords: proxied}, nil
}

// PanicApplyZone calls ApplyToggle for every Record in summary.ProxiedRecords,
// flipping each one to proxied=false. Each per-record success or failure is
// recorded in PanicResult; the function does NOT abort on first failure
// because the user already opted into a bulk operation.
//
// Returns an error only if every record failed.
func PanicApplyZone(
	ctx context.Context,
	appCtx *Context,
	summary PanicSummary,
	snapshotDir, auditPath string,
) (PanicSummary, error) {
	for _, rec := range summary.ProxiedRecords {
		res, err := ApplyToggle(ctx, appCtx, rec, false, snapshotDir, auditPath)
		summary.Results = append(summary.Results, PanicResult{
			Record:       rec,
			Applied:      res.Applied,
			SnapshotPath: res.SnapshotPath,
			Err:          err,
		})
	}

	failures := 0
	for _, r := range summary.Results {
		if r.Err != nil {
			failures++
		}
	}
	if failures > 0 && failures == len(summary.Results) {
		return summary, fmt.Errorf("panic: every record failed (%d total)", failures)
	}
	return summary, nil
}
