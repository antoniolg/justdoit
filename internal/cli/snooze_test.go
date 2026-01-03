package cli

import (
	"testing"
	"time"
)

func TestParseSnoozeInputDateOnly(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 1, 3, 9, 0, 0, 0, loc)
	params, err := parseSnoozeInput("tomorrow", now, loc)
	if err != nil {
		t.Fatalf("parseSnoozeInput error: %v", err)
	}
	if !params.HasDate {
		t.Fatalf("expected HasDate true")
	}
	if params.Date != "2026-01-04" {
		t.Fatalf("expected date 2026-01-04, got %q", params.Date)
	}
	if params.HasTime {
		t.Fatalf("expected HasTime false")
	}
}

func TestParseSnoozeInputTimeOnly(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 1, 3, 9, 0, 0, 0, loc)
	params, err := parseSnoozeInput("15:00-16:00", now, loc)
	if err != nil {
		t.Fatalf("parseSnoozeInput error: %v", err)
	}
	if !params.HasTime {
		t.Fatalf("expected HasTime true")
	}
	if params.Time != "15:00-16:00" {
		t.Fatalf("expected time '15:00-16:00', got %q", params.Time)
	}
	if params.HasDate {
		t.Fatalf("expected HasDate false")
	}
}
