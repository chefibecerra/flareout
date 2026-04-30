// toggle.go is the second documented exception to the app-layer-imports-only-domain
// rule (alongside wiring.go). It orchestrates snapshot + audit log writes around a
// Cloudflare API mutation; promoting these adapters to ports would force four new
// domain interfaces for a use case used in exactly one place. The layering test
// exempts BOTH files by filename — see internal/app/layering_test.go.
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chefibecerra/flareout/internal/domain"
	"github.com/chefibecerra/flareout/internal/infra/audit"
	"github.com/chefibecerra/flareout/internal/infra/snapshot"
)

// ToggleResult bundles the outcome of a ToggleProxy invocation. Diff is
// always populated; SnapshotPath and Applied are only set on the apply path.
type ToggleResult struct {
	Diff         domain.Diff
	Applied      bool
	SnapshotPath string
}

// ErrZoneNotFound — selector zone name had no match in ListZones.
var ErrZoneNotFound = errors.New("toggle: zone not found")

// ErrRecordNotFound — selector zone+name (and optional type) had no record match.
var ErrRecordNotFound = errors.New("toggle: record not found")

// ErrAmbiguous — selector matched more than one record. Use --type to disambiguate.
var ErrAmbiguous = errors.New("toggle: multiple records matched; use --type to disambiguate")

// ToggleProxy resolves a single DNS record by zone, name, and optional type;
// computes the proxy-state diff; and (when apply is true) writes a snapshot,
// invokes the toggler, and appends an audit entry. dry-run (apply=false) has
// zero side effects and never calls Cloudflare.
//
// snapshotDir and auditPath are passed in (not derived) so tests can isolate
// I/O via t.TempDir() without env var manipulation.
func ToggleProxy(
	ctx context.Context,
	appCtx *Context,
	zoneName, recordName, recordType string,
	proxied bool,
	apply bool,
	snapshotDir, auditPath string,
) (ToggleResult, error) {
	target, err := resolveTarget(ctx, appCtx, zoneName, recordName, recordType)
	if err != nil {
		return ToggleResult{}, err
	}

	diff := domain.Diff{
		Record:      target,
		FromProxied: target.Proxied,
		ToProxied:   proxied,
	}
	result := ToggleResult{Diff: diff}

	if !apply || diff.IsNoOp() {
		return result, nil
	}

	return ApplyToggle(ctx, appCtx, target, proxied, snapshotDir, auditPath)
}

// ApplyToggle performs the snapshot + API call + audit log sequence for a
// pre-resolved record. It is the shared apply path used by both the CLI
// `flareout toggle` subcommand (after selector resolution) and the TUI
// multi-select flow (where the records are already in hand from the list).
//
// Caller is responsible for ensuring target.ZoneID and target.ZoneName are
// populated; the adapter needs both.
func ApplyToggle(
	ctx context.Context,
	appCtx *Context,
	target domain.Record,
	proxied bool,
	snapshotDir, auditPath string,
) (ToggleResult, error) {
	diff := domain.Diff{
		Record:      target,
		FromProxied: target.Proxied,
		ToProxied:   proxied,
	}
	result := ToggleResult{Diff: diff}

	if diff.IsNoOp() {
		return result, nil
	}

	// Snapshot BEFORE the mutation. If snapshot fails we abort without touching
	// Cloudflare so the caller's recoverable state is intact.
	snapPath, err := snapshot.Write(target, snapshotDir)
	if err != nil {
		return result, fmt.Errorf("toggle: snapshot: %w", err)
	}
	result.SnapshotPath = snapPath

	apiErr := appCtx.Toggler.SetProxied(ctx, target, proxied)

	// Append the audit entry on EVERY apply attempt (success or failure).
	entry := audit.Entry{
		Timestamp:     time.Now().UTC(),
		Zone:          target.ZoneName,
		Name:          target.Name,
		Type:          target.Type,
		BeforeProxied: target.Proxied,
		AfterProxied:  proxied,
		Applied:       apiErr == nil,
		SnapshotPath:  snapPath,
	}
	if apiErr != nil {
		entry.Error = apiErr.Error()
	}
	if auditErr := audit.Append(entry, auditPath); auditErr != nil {
		// Audit failure does not override the primary error path; surface
		// it via slog so operators see it but the user still gets the real
		// outcome of the API call.
		appCtx.Logger.Warn("audit append failed",
			"err", auditErr, "snapshot", snapPath, "zone", target.ZoneName, "name", target.Name)
	}

	if apiErr != nil {
		return result, fmt.Errorf("toggle: %w", apiErr)
	}
	result.Applied = true
	return result, nil
}

func resolveTarget(ctx context.Context, appCtx *Context, zoneName, recordName, recordType string) (domain.Record, error) {
	zones, err := appCtx.Lister.ListZones(ctx)
	if err != nil {
		return domain.Record{}, fmt.Errorf("toggle: list zones: %w", err)
	}

	var zone domain.Zone
	for _, z := range zones {
		if strings.EqualFold(z.Name, zoneName) {
			zone = z
			break
		}
	}
	if zone.ID == "" {
		return domain.Record{}, fmt.Errorf("%w: %s", ErrZoneNotFound, zoneName)
	}

	records, err := appCtx.Lister.ListRecords(ctx, zone.ID)
	if err != nil {
		return domain.Record{}, fmt.Errorf("toggle: list records: %w", err)
	}

	var matched []domain.Record
	for _, r := range records {
		if !strings.EqualFold(r.Name, recordName) {
			continue
		}
		if recordType != "" && !strings.EqualFold(r.Type, recordType) {
			continue
		}
		// Enrich with zone metadata (the Cloudflare adapter leaves these empty
		// because dns.RecordResponse omits ZoneID/ZoneName).
		r.ZoneID = zone.ID
		r.ZoneName = zone.Name
		matched = append(matched, r)
	}

	switch len(matched) {
	case 0:
		return domain.Record{}, fmt.Errorf("%w: %s/%s", ErrRecordNotFound, zoneName, recordName)
	case 1:
		return matched[0], nil
	default:
		return domain.Record{}, fmt.Errorf("%w: %s/%s (matched %d)", ErrAmbiguous, zoneName, recordName, len(matched))
	}
}
