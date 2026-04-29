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

package logging_test

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chefibecerra/flareout/internal/infra/logging"
)

func TestLoggingDefault(t *testing.T) {
	logger := logging.Default()
	if logger == nil {
		t.Fatal("Default() returned nil *slog.Logger")
	}
	h := logger.Handler()
	if h == nil {
		t.Fatal("Default() returned logger with nil handler")
	}
	// The handler must be a TextHandler (not JSON) for CLI-friendly output.
	if _, ok := h.(*slog.TextHandler); !ok {
		t.Errorf("Default() handler type = %T, want *slog.TextHandler", h)
	}
}

func TestSwapToFileJSON(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	// Capture the handler type before swapping.
	prevLogger := slog.Default()
	prevHandlerType := fmt.Sprintf("%T", prevLogger.Handler())

	restore, err := logging.SwapToFileJSON(logPath)
	if err != nil {
		t.Fatalf("SwapToFileJSON(%q) returned unexpected error: %v", logPath, err)
	}

	// After swap, slog.Default() must have a JSONHandler.
	currentHandler := slog.Default().Handler()
	if _, ok := currentHandler.(*slog.JSONHandler); !ok {
		t.Errorf("after SwapToFileJSON, slog.Default() handler = %T, want *slog.JSONHandler", currentHandler)
	}

	// Write a log entry through the swapped default.
	slog.Default().Info("test-event", "key", "value")

	// Call restore to reinstate the previous logger and close the file.
	restore()

	// After restore, slog.Default() handler type must match what it was before.
	restoredHandlerType := fmt.Sprintf("%T", slog.Default().Handler())
	if restoredHandlerType != prevHandlerType {
		t.Errorf("after restore, handler type = %s, want %s", restoredHandlerType, prevHandlerType)
	}

	// Verify the log file was written and contains JSON with expected fields.
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	content := string(data)
	if len(content) == 0 {
		t.Fatal("log file is empty after writing an entry")
	}
	if !strings.Contains(content, `"test-event"`) && !strings.Contains(content, "test-event") {
		t.Errorf("log file does not contain the expected message; got: %s", content)
	}
	// JSON output must contain the "key" field we logged.
	if !strings.Contains(content, `"key"`) {
		t.Errorf("log file does not contain expected field key; got: %s", content)
	}
}
