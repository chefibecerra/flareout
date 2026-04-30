package app

import (
	"context"
	"sort"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/chefibecerra/flareout/internal/domain"
)

// concurrencyCap is the maximum number of in-flight per-zone record fetches.
// 5 is comfortably below Cloudflare's rate limit (1200 req / 5min) for typical
// account sizes.
const concurrencyCap = 5

// ListAllRecords fetches every DNS record across all zones the API token
// can access, sorted by zone name (case-insensitive) then record name.
//
// Concurrency: zones are fetched serially; per-zone records fan out via
// errgroup with a buffered-channel semaphore of size 5. errgroup returns the
// FIRST error; subsequent errors from other goroutines are silently dropped
// (standard errgroup behavior — callers must not assume which zone's error
// is returned when multiple zones fail).
func ListAllRecords(ctx context.Context, appCtx *Context) ([]domain.Record, error) {
	zones, err := appCtx.Lister.ListZones(ctx)
	if err != nil {
		return nil, err
	}

	g, gctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, concurrencyCap)
	var (
		mu  sync.Mutex
		all []domain.Record
	)

	for _, z := range zones {
		z := z // capture range variable
		g.Go(func() error {
			// Acquire semaphore or abort if context is done.
			select {
			case sem <- struct{}{}:
			case <-gctx.Done():
				return gctx.Err()
			}
			defer func() { <-sem }()

			recs, err := appCtx.Lister.ListRecords(gctx, z.ID)
			if err != nil {
				return err
			}
			// Enrich each record with zone metadata. The cloudflare-go/v4
			// dns.RecordResponse does not carry ZoneID/ZoneName, so the adapter
			// returns records without them; the application layer fills them in
			// here using the Zone we already have.
			for i := range recs {
				recs[i].ZoneID = z.ID
				recs[i].ZoneName = z.Name
			}
			mu.Lock()
			all = append(all, recs...)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	sort.SliceStable(all, func(i, j int) bool {
		zi, zj := strings.ToLower(all[i].ZoneName), strings.ToLower(all[j].ZoneName)
		if zi != zj {
			return zi < zj
		}
		return strings.ToLower(all[i].Name) < strings.ToLower(all[j].Name)
	})
	return all, nil
}
