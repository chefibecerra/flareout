package app

import (
	"log/slog"

	"github.com/chefibecerra/flareout/internal/domain"
)

// Context bundles resolved dependencies for Cobra command handlers.
// Constructed once at startup; read-only by all consumers.
// The Token field is in-memory only — never logged or persisted.
type Context struct {
	Logger   *slog.Logger
	Verifier domain.TokenVerifier
	Token    string
	Version  string
	Lister   domain.RecordLister // set by app.Build via wiring.go; do not set elsewhere
}
