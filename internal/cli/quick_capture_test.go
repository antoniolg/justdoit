package cli

import (
	"testing"
	"time"
)

func TestParseQuickCapture(t *testing.T) {
	now := time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC)
	input := `Call John #Work ::Current @tomorrow @15:00-16:00 every:weekly`

	parsed, err := parseQuickCapture(input, now, time.UTC)
	if err != nil {
		t.Fatalf("parseQuickCapture error: %v", err)
	}
	if parsed.Title != "Call John" {
		t.Fatalf("expected title 'Call John', got %q", parsed.Title)
	}
	if parsed.List != "Work" {
		t.Fatalf("expected list 'Work', got %q", parsed.List)
	}
	if parsed.Section != "Current" {
		t.Fatalf("expected section 'Current', got %q", parsed.Section)
	}
	if parsed.Date != "tomorrow" {
		t.Fatalf("expected date 'tomorrow', got %q", parsed.Date)
	}
	if parsed.Time != "15:00-16:00" {
		t.Fatalf("expected time '15:00-16:00', got %q", parsed.Time)
	}
	if parsed.Every != "weekly" {
		t.Fatalf("expected every 'weekly', got %q", parsed.Every)
	}
}

// Note: some "@token" values may be parsed as dates by naturaldate, so we keep
// tests focused on the explicit token formats.
