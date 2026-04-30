package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestClassify_Auth403(t *testing.T) {
	// Mimics the actual error string the cloudflare-go/v4 SDK surfaces
	// when DNS:Edit is missing (the case the user hit in their first
	// real toggle attempt).
	err := errors.New(`PATCH "https://api.cloudflare.com/client/v4/zones/abc/dns_records/xyz": 403 Forbidden {"success":false,"errors":[{"code":10000,"message":"Authentication error"}]}`)
	c := classify(err)
	if !strings.Contains(c.headline, "does not have permission") {
		t.Errorf("headline = %q, expected permission-related text", c.headline)
	}
	if !strings.Contains(c.hint, "DNS:Edit") {
		t.Errorf("hint = %q, expected scope guidance with DNS:Edit", c.hint)
	}
}

func TestClassify_401Unauth(t *testing.T) {
	err := errors.New("401 Unauthorized")
	c := classify(err)
	if !strings.Contains(c.headline, "invalid") && !strings.Contains(c.headline, "expired") {
		t.Errorf("headline = %q, expected invalid/expired language", c.headline)
	}
	if !strings.Contains(c.hint, "CLOUDFLARE_API_TOKEN") {
		t.Errorf("hint = %q, expected env var name", c.hint)
	}
}

func TestClassify_404(t *testing.T) {
	c := classify(errors.New("404 Not Found"))
	if !strings.Contains(c.headline, "not found") {
		t.Errorf("headline = %q", c.headline)
	}
	if !strings.Contains(c.hint, "flareout list") {
		t.Errorf("hint = %q, expected list command suggestion", c.hint)
	}
}

func TestClassify_429(t *testing.T) {
	c := classify(errors.New("429 Too Many Requests"))
	if !strings.Contains(c.headline, "rate-limited") {
		t.Errorf("headline = %q", c.headline)
	}
}

func TestClassify_NetworkError(t *testing.T) {
	cases := []string{
		"dial tcp api.cloudflare.com: no such host",
		"connection refused",
	}
	for _, msg := range cases {
		c := classify(errors.New(msg))
		if !strings.Contains(c.headline, "Could not reach") {
			t.Errorf("for %q: headline = %q", msg, c.headline)
		}
	}
}

func TestClassify_TokenMissing(t *testing.T) {
	err := errors.New("flareout: CLOUDFLARE_API_TOKEN is not set")
	c := classify(err)
	if !strings.Contains(c.headline, "CLOUDFLARE_API_TOKEN") {
		t.Errorf("headline = %q", c.headline)
	}
	if !strings.Contains(c.hint, "export") {
		t.Errorf("hint = %q, expected export instruction", c.hint)
	}
}

func TestClassify_UnknownFallsBackToRawMessage(t *testing.T) {
	err := errors.New("totally unrecognized error xyz")
	c := classify(err)
	if c.headline != "totally unrecognized error xyz" {
		t.Errorf("headline = %q, expected raw message passthrough", c.headline)
	}
	if c.hint != "" {
		t.Errorf("hint = %q, expected empty for unknown errors", c.hint)
	}
}

// PrintError must produce a deterministic, ANSI-free output when the
// writer is a non-TTY (a bytes.Buffer in this test).
func TestPrintError_NoColorOnNonTTY(t *testing.T) {
	var buf bytes.Buffer
	err := errors.New(`403 Forbidden 10000 "Authentication error"`)
	PrintError(&buf, err)

	out := buf.String()
	if strings.Contains(out, "\x1b[") {
		t.Errorf("non-TTY output contains ANSI escape: %q", out)
	}
	if !strings.Contains(out, "ERROR:") {
		t.Errorf("output missing ERROR prefix: %q", out)
	}
	if !strings.Contains(out, "DNS:Edit") {
		t.Errorf("output missing hint: %q", out)
	}
}

// FailLine produces ONE per-record line with a friendly headline; raw
// JSON envelopes from upstream errors must not appear in the output.
func TestFailLine_SuppressesRawEnvelope(t *testing.T) {
	var buf bytes.Buffer
	err := errors.New(`PATCH "https://api.cloudflare.com/x/y": 403 Forbidden {"success":false,"errors":[{"code":10000,"message":"Authentication error"}]}`)
	FailLine(&buf, "example.com", "www", err)

	out := buf.String()
	if strings.Contains(out, "https://api.cloudflare.com") {
		t.Errorf("FailLine leaked URL into output: %q", out)
	}
	if strings.Contains(out, `"success":false`) {
		t.Errorf("FailLine leaked JSON envelope: %q", out)
	}
	if !strings.Contains(out, "FAIL example.com/www:") {
		t.Errorf("output missing FAIL prefix: %q", out)
	}
}

func TestPrintError_NilIsNoOp(t *testing.T) {
	var buf bytes.Buffer
	PrintError(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("PrintError(nil) wrote %q, want empty", buf.String())
	}
}
