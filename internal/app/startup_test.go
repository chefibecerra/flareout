package app_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/chefibecerra/flareout/internal/app"
	"github.com/chefibecerra/flareout/internal/domain"
)

// fakeVerifier is an inline test double for domain.TokenVerifier.
// Defined locally — not a shared global helper.
type fakeVerifier struct {
	status domain.TokenStatus
	err    error
}

func (f *fakeVerifier) Verify(_ context.Context) (domain.TokenStatus, error) {
	return f.status, f.err
}

func TestVerifyTokenAtStartup(t *testing.T) {
	const rawToken = "cfx_live_supersecrettoken1234567890"

	tests := []struct {
		name          string
		verifierSetup *fakeVerifier
		wantErr       error
		wantWarning   bool
	}{
		{
			name: "success path emits scope warning and logs masked token",
			verifierSetup: &fakeVerifier{
				status: domain.TokenStatus{Active: true},
				err:    nil,
			},
			wantErr:     nil,
			wantWarning: true,
		},
		{
			name: "failure path returns wrapped error and no warning",
			verifierSetup: &fakeVerifier{
				status: domain.TokenStatus{},
				err:    domain.ErrTokenInactive,
			},
			wantErr:     domain.ErrTokenInactive,
			wantWarning: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Capture slog output via a local logger writing to a buffer.
			// This avoids mutating global slog state.
			var slogBuf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&slogBuf, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}))

			var warningSink bytes.Buffer

			appCtx := &app.Context{
				Logger:   logger,
				Verifier: tc.verifierSetup,
				Token:    rawToken,
				Version:  "test",
			}

			err := app.VerifyTokenAtStartup(context.Background(), appCtx, &warningSink)

			// --- error assertion ---
			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error wrapping %v, got nil", tc.wantErr)
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected errors.Is(err, %v) to be true, got err=%v", tc.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
			}

			// --- warning sink assertion ---
			warnOut := warningSink.String()
			if tc.wantWarning {
				if !strings.Contains(warnOut, domain.ScopeWarningMessage) {
					t.Errorf("warningSink should contain ScopeWarningMessage %q, got %q",
						domain.ScopeWarningMessage, warnOut)
				}
				// Ensure it appears exactly once
				count := strings.Count(warnOut, domain.ScopeWarningMessage)
				if count != 1 {
					t.Errorf("ScopeWarningMessage should appear exactly once, appeared %d times", count)
				}
			} else {
				if warnOut != "" {
					t.Errorf("warningSink should be empty on error path, got %q", warnOut)
				}
			}

			// --- slog token masking assertion ---
			logOut := slogBuf.String()
			masked := domain.MaskToken(rawToken)

			// Raw token must never appear in logs
			if strings.Contains(logOut, rawToken) {
				t.Errorf("slog output MUST NOT contain raw token; got log: %s", logOut)
			}

			// Masked token must appear in logs
			if !strings.Contains(logOut, masked) {
				t.Errorf("slog output should contain masked token %q; got log: %s", masked, logOut)
			}

			// On failure path, log must be at error level
			if tc.wantErr != nil {
				if !strings.Contains(logOut, "level=ERROR") && !strings.Contains(logOut, "ERROR") {
					t.Errorf("failure path should log at ERROR level; got log: %s", logOut)
				}
			}
		})
	}
}
