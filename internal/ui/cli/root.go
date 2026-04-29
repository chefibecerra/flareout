package cli

import (
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
			return app.VerifyTokenAtStartup(cmd.Context(), appCtx, cmd.ErrOrStderr())
		},
	}

	root.AddCommand(NewVersionCmd(deps.Version))

	return root
}
