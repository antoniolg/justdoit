package cli

import (
	"testing"
	"time"
)

func TestParseTaskDueMidnightUTCIsAllDay(t *testing.T) {
	loc := time.FixedZone("CET", 3600)
	due, hasDue, hasTime := parseTaskDue("2026-01-05T00:00:00.000Z", loc)
	if !hasDue {
		t.Fatalf("expected due to be parsed")
	}
	if hasTime {
		t.Fatalf("expected midnight UTC to be treated as all-day")
	}
	if due.In(loc).Hour() != 1 {
		t.Fatalf("expected local hour to be 1, got %d", due.In(loc).Hour())
	}
}

func TestParseTaskDueEndOfDayIsAllDay(t *testing.T) {
	loc := time.FixedZone("CET", 3600)
	_, hasDue, hasTime := parseTaskDue("2026-01-05T23:59:00+01:00", loc)
	if !hasDue {
		t.Fatalf("expected due to be parsed")
	}
	if hasTime {
		t.Fatalf("expected end-of-day to be treated as all-day")
	}
}

func TestParseTaskDueKeepsExplicitTime(t *testing.T) {
	loc := time.FixedZone("CET", 3600)
	_, hasDue, hasTime := parseTaskDue("2026-01-05T09:30:00+01:00", loc)
	if !hasDue {
		t.Fatalf("expected due to be parsed")
	}
	if !hasTime {
		t.Fatalf("expected explicit time to be kept")
	}
}
