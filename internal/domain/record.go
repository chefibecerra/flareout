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

import "context"

// Zone represents a Cloudflare DNS zone. Value type: copy-safe, comparable.
type Zone struct {
	ID     string
	Name   string
	Status string
}

// Record represents a single DNS record within a zone. Value type.
// Proxied is bool (not *bool): the infrastructure adapter normalizes
// the SDK's nullable Proxied field to false at the boundary, so
// downstream consumers see a single, definite value.
type Record struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	ZoneID   string `json:"zone_id"`
	ZoneName string `json:"zone_name"`
	Proxied  bool   `json:"proxied"`
	TTL      int64  `json:"ttl"`
}

// RecordLister is the domain port for fetching zones and DNS records from
// a Cloudflare-like provider. The infrastructure adapter implements this
// interface; the application layer depends on it.
//
// Two methods (rather than one combined) lets the application layer own the
// concurrency strategy and lets fakes stub zones and records independently
// in tests.
type RecordLister interface {
	ListZones(ctx context.Context) ([]Zone, error)
	ListRecords(ctx context.Context, zoneID string) ([]Record, error)
}
