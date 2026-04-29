package app

import (
	"context"
	"fmt"
	"io"

	"github.com/chefibecerra/flareout/internal/domain"
)

// VerifyTokenAtStartup calls the token verifier and handles the result.
// On success it logs at Info level and writes domain.ScopeWarningMessage to
// warningSink via fmt.Fprintln — unconditionally, regardless of log level.
// On error it logs at Error level and returns the error.
// The raw token is never logged; only the masked form is used.
func VerifyTokenAtStartup(ctx context.Context, appCtx *Context, warningSink io.Writer) error {
	// Mask once, reuse in both log paths — raw token never touches the log.
	masked := domain.MaskToken(appCtx.Token)

	status, err := appCtx.Verifier.Verify(ctx)
	if err != nil {
		appCtx.Logger.Error("token verification failed", "token", masked, "err", err)
		return err
	}

	appCtx.Logger.Info("token verified",
		"token", masked,
		"active", status.Active,
		"expires_at", status.ExpiresAt,
	)

	// Write the scope warning via fmt.Fprintln, NOT slog.
	// This makes the warning non-suppressible even if the log level is changed.
	// A stderr write failure here is essentially unrecoverable; surface it via
	// slog so it does not vanish silently.
	if _, werr := fmt.Fprintln(warningSink, domain.ScopeWarningMessage); werr != nil {
		appCtx.Logger.Warn("scope warning emission failed", "err", werr)
	}

	return nil
}
