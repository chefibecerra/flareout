// wiring.go is the composition root: it imports infra adapters by design.
// The layering test in internal/domain explicitly exempts this file.
// The layering test in internal/app also exempts this file (by exact filename match).
package app

import (
	"fmt"

	"github.com/chefibecerra/flareout/internal/infra/cloudflare"
	"github.com/chefibecerra/flareout/internal/infra/config"
	"github.com/chefibecerra/flareout/internal/infra/logging"
)

// Config holds build-time configuration passed into the composition root.
type Config struct {
	Version string
}

// Build resolves dependencies and returns a *Context ready for command handlers.
// Build does NOT call Verify — verification is a use case (VerifyTokenAtStartup),
// not wiring. Keeping Build pure-construction allows commands annotated with
// skip_verify to bypass the network call entirely.
func Build(cfg Config) (*Context, error) {
	logger := logging.Default()

	token, err := config.ReadTokenFromEnv()
	if err != nil {
		return nil, fmt.Errorf("app.Build: %w", err)
	}

	verifier, err := cloudflare.NewTokenVerifier(token)
	if err != nil {
		return nil, fmt.Errorf("app.Build: %w", err)
	}

	lister, err := cloudflare.NewRecordLister(token)
	if err != nil {
		return nil, fmt.Errorf("app.Build: %w", err)
	}

	return &Context{
		Logger:   logger,
		Verifier: verifier,
		Token:    token,
		Version:  cfg.Version,
		Lister:   lister,
	}, nil
}
