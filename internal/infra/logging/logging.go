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

// Package logging provides a centralized handler-construction strategy for
// FlareOut's structured logging via log/slog. All handler setup is isolated
// here so callers (app, CLI) inject *slog.Logger instances rather than
// constructing handlers themselves, preventing drift in handler configuration.
package logging

import (
	"fmt"
	"log/slog"
	"os"
)

// Default returns the default CLI logger: TextHandler writing to os.Stderr
// at LevelInfo. If FLAREOUT_DEBUG=1, level is lowered to LevelDebug.
// Call once at application startup in cmd/flareout/main.go before anything else runs.
func Default() *slog.Logger {
	level := slog.LevelInfo
	if os.Getenv("FLAREOUT_DEBUG") == "1" {
		level = slog.LevelDebug
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	return slog.New(h)
}

// SwapToFileJSON opens path for append-write (mode 0600), swaps slog.Default()
// to a JSONHandler writing to that file, and returns a restore callback.
//
// The restore callback reinstates the previous slog.Default() AND closes the file.
// It is safe to call restore from any goroutine (slog.SetDefault is atomic).
//
// MUST be called BEFORE tea.NewProgram(...).Run() in any TUI command.
// Failing to do so causes slog writes to reach stderr while Bubbletea owns
// the terminal, corrupting rendering.
func SwapToFileJSON(path string) (restore func(), err error) {
	// Security: 0600 ensures only the file owner can read the log file.
	// Log files may contain operational metadata (masked tokens, timing, errors)
	// that should not be world-readable on shared systems.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("logging: open log file %q: %w", path, err)
	}
	prev := slog.Default()
	h := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))
	return func() {
		slog.SetDefault(prev)
		_ = f.Close()
	}, nil
}
