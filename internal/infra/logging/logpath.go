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

package logging

import (
	"fmt"
	"os"
	"path/filepath"
)

// StateLogPath returns the FlareOut debug log file path under the XDG state directory.
// Resolution order:
//  1. $XDG_STATE_HOME/flareout/debug.log    (if XDG_STATE_HOME is set and non-empty)
//  2. $HOME/.local/state/flareout/debug.log (fallback via os.UserHomeDir)
//
// Creates the parent directory with mode 0700 before returning so callers do not
// need to handle a missing directory.
//
// File permissions (0600) are set by SwapToFileJSON when the file is opened — not here.
//
// StateLogPath does not emit any slog records. Logging before the log file is
// established creates a bootstrap-ordering hazard (XL-09).
func StateLogPath() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("logpath: resolve home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "state")
	}

	dir := filepath.Join(base, "flareout")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("logpath: create state directory: %w", err)
	}
	return filepath.Join(dir, "debug.log"), nil
}
