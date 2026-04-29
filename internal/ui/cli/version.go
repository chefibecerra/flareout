package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewVersionCmd returns the "version" subcommand.
// The skip_verify annotation prevents PersistentPreRunE from calling the token
// liveness check — version is an introspection command that must run without credentials.
// Takes only the version string; the full Dependencies struct is intentionally
// not threaded here because version must work even when Build cannot.
func NewVersionCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:         "version",
		Short:       "Print the FlareOut version and exit",
		Annotations: map[string]string{"skip_verify": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "flareout", version)
			return err
		},
	}
}
