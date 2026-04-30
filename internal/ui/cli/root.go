package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/chefibecerra/flareout/internal/app"
)

// Dependencies holds the resolved application dependencies injected into
// every Cobra command. Build is a lazy closure: it is only invoked for
// commands that require a verified token. Commands annotated with
// skip_verify (e.g. "version") never call Build, so the binary can run
// when CLOUDFLARE_API_TOKEN is unset.
type Dependencies struct {
	Version string
	Build   func() (*app.Context, error)
}

// appCtxKey is an unexported type used to store *app.Context on cmd.Context().
// The private type prevents accidental key collision with other context values.
type appCtxKey struct{}

// WithAppCtx returns a derived context carrying the resolved *app.Context.
// The composition root (PersistentPreRunE) sets this after a successful Build
// so that RunE handlers can retrieve the same built context without invoking
// Build twice (ADR-01).
func WithAppCtx(ctx context.Context, appCtx *app.Context) context.Context {
	return context.WithValue(ctx, appCtxKey{}, appCtx)
}

// AppCtxFrom retrieves the *app.Context previously stored via WithAppCtx.
// Returns ok=false if no app context is present in the chain (e.g. the command
// bypassed PersistentPreRunE or has skip_verify=true).
func AppCtxFrom(ctx context.Context) (*app.Context, bool) {
	a, ok := ctx.Value(appCtxKey{}).(*app.Context)
	return a, ok
}

// NewRootCmd returns the Cobra root command with all subcommands registered.
// Token verification is handled in PersistentPreRunE; commands that must not
// trigger a network call set the "skip_verify" annotation to "true".
func NewRootCmd(deps Dependencies) *cobra.Command {
	root := &cobra.Command{
		Use:           "flareout",
		Short:         "Manage Cloudflare DNS proxy state with safety guardrails",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Annotations["skip_verify"] == "true" {
				return nil
			}
			appCtx, err := deps.Build()
			if err != nil {
				return err
			}
			if err := app.VerifyTokenAtStartup(cmd.Context(), appCtx, cmd.ErrOrStderr()); err != nil {
				return err
			}
			cmd.SetContext(WithAppCtx(cmd.Context(), appCtx))
			return nil
		},
	}

	root.AddCommand(NewVersionCmd(deps.Version))
	root.AddCommand(NewListCmd(deps))
	root.AddCommand(NewToggleCmd())

	return root
}
