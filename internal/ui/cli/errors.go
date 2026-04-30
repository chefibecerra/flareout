package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

// classification of an error into a human headline and (optional) actionable
// hint. Headlines fit on one line; hints can be longer and explain the
// remediation. Both fields are plain text — color is applied at render time
// in PrintError.
type classification struct {
	headline string
	hint     string
}

var (
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	hintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// PrintError writes a styled error block to w. Color is applied only when w
// points at a terminal — pipes and redirects get plain text so logs and
// transcripts stay clean.
//
// If err is nil this is a no-op.
func PrintError(w io.Writer, err error) {
	if err == nil {
		return
	}
	c := classify(err)
	color := writerSupportsColor(w)
	headline := "ERROR: " + c.headline
	if color {
		headline = errStyle.Render(headline)
	}
	_, _ = fmt.Fprintln(w, headline)
	if c.hint != "" {
		hint := "  " + c.hint
		if color {
			hint = hintStyle.Render(hint)
		}
		_, _ = fmt.Fprintln(w, hint)
	}
}

// FailLine writes a single per-record failure line (used in bulk operations
// like the TUI multi-apply and panic). Format:
//
//	FAIL <zone>/<name>: <human headline>
//
// "FAIL" is rendered red on a TTY, plain text otherwise. The full original
// error is suppressed in favor of the classified headline so operators see
// actionable text instead of raw JSON envelopes from the Cloudflare API.
func FailLine(w io.Writer, zone, name string, err error) {
	if err == nil {
		return
	}
	c := classify(err)
	prefix := "FAIL"
	if writerSupportsColor(w) {
		prefix = errStyle.Render(prefix)
	}
	_, _ = fmt.Fprintf(w, "  %s %s/%s: %s\n", prefix, zone, name, c.headline)
}

// classify maps a Cloudflare adapter error into a friendlier headline + hint.
// The mapping is pattern-based on the error string because the upstream SDK
// does not expose typed status codes for every shape we need; if the SDK
// later exports typed errors, swap the strings.Contains checks for errors.As.
func classify(err error) classification {
	if err == nil {
		return classification{headline: "(nil)"}
	}
	msg := err.Error()

	switch {
	case is403Auth(msg):
		return classification{
			headline: "Cloudflare rejected the write — the API token does not have permission for this operation",
			hint:     "Required scopes: Zone:Read + DNS:Read + DNS:Edit. Edit or create a token at https://dash.cloudflare.com/profile/api-tokens",
		}

	case strings.Contains(msg, "401") && strings.Contains(strings.ToLower(msg), "unauth"):
		return classification{
			headline: "Cloudflare rejected the request — the token is invalid, expired, or disabled",
			hint:     "Confirm CLOUDFLARE_API_TOKEN is correct and active at https://dash.cloudflare.com/profile/api-tokens",
		}

	case strings.Contains(msg, "404"):
		return classification{
			headline: "Resource not found on Cloudflare — the record may have been deleted or moved",
			hint:     "Run `flareout list` to see the current state",
		}

	case strings.Contains(msg, "429"):
		return classification{
			headline: "Cloudflare rate-limited the request",
			hint:     "Wait a few seconds and retry. Bulk operations may need lower concurrency.",
		}

	case strings.Contains(msg, "no such host"),
		strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "dial tcp"):
		return classification{
			headline: "Could not reach Cloudflare API",
			hint:     "Check your network connection and that api.cloudflare.com is reachable",
		}

	case strings.Contains(msg, "CLOUDFLARE_API_TOKEN"):
		return classification{
			headline: "CLOUDFLARE_API_TOKEN environment variable is not set",
			hint:     "Export the token before running flareout: export CLOUDFLARE_API_TOKEN=your-token",
		}
	}

	// Fallback: return the raw error message as the headline. No hint
	// available because we do not recognize the pattern.
	return classification{headline: msg}
}

// is403Auth reports whether the error message looks like Cloudflare's
// "code 10000 / Authentication error" pattern returned with HTTP 403. This
// is the most common failure mode users hit when they forget to include
// DNS:Edit on their token.
func is403Auth(msg string) bool {
	hasStatus := strings.Contains(msg, "403")
	hasCode := strings.Contains(msg, "10000")
	hasText := strings.Contains(msg, "Authentication error")
	// Either the numeric code or the prose message paired with the status.
	return hasStatus && (hasCode || hasText)
}

// writerSupportsColor reports whether ANSI color escapes should be sent to
// w. True only when w is an *os.File backed by a TTY; false for buffers,
// pipes, redirected stdout/stderr, etc.
func writerSupportsColor(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	if !isatty.IsTerminal(f.Fd()) {
		return false
	}
	// Respect NO_COLOR (https://no-color.org).
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}
	return true
}
