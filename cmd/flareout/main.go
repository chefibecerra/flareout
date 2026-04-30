package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/chefibecerra/flareout/internal/app"
	"github.com/chefibecerra/flareout/internal/infra/logging"
	"github.com/chefibecerra/flareout/internal/ui/cli"
)

// Version is the build version. Override at link time:
//
//	go build -ldflags "-X main.Version=v1.2.3"
var Version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.SetDefault(logging.Default())

	deps := cli.Dependencies{
		Version: Version,
		Build: func() (*app.Context, error) {
			return app.Build(app.Config{Version: Version})
		},
	}

	root := cli.NewRootCmd(deps)
	if err := root.ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "flareout: aborted")
			os.Exit(130)
		}
		cli.PrintError(os.Stderr, err)
		os.Exit(1)
	}
}
