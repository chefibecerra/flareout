package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/chefibecerra/flareout/internal/app"
	"github.com/chefibecerra/flareout/internal/infra/logging"
	"github.com/chefibecerra/flareout/internal/ui/cli"
)

// Version is the build version. Override at link time:
//
//	go build -ldflags "-X main.Version=v1.2.3"
var Version = "dev"

func main() {
	slog.SetDefault(logging.Default())

	deps := cli.Dependencies{
		Version: Version,
		Build: func() (*app.Context, error) {
			return app.Build(app.Config{Version: Version})
		},
	}

	root := cli.NewRootCmd(deps)
	if err := root.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "flareout:", err)
		os.Exit(1)
	}
}
