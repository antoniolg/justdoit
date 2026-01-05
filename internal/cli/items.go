package cli

import (
	"fmt"
	"strings"
	"time"
)

type calendarEventItem struct {
	ID           string
	Summary      string
	CalendarID   string
	CalendarName string
	Start        time.Time
	End          time.Time
	AllDay       bool
}

func (c calendarEventItem) Title() string {
	prefix := "[cal]"
	if useColor() {
		prefix = gray(prefix)
	}
	title := strings.TrimSpace(c.Summary)
	if title == "" {
		title = "(untitled)"
	}
	return fmt.Sprintf("%s %s", prefix, title)
}

func (c calendarEventItem) Description() string {
	parts := []string{}
	if !c.AllDay {
		if text := formatTimeRange(c.Start, c.End); text != "" {
			parts = append(parts, text)
		}
	} else {
		parts = append(parts, "all-day")
	}
	if c.CalendarName != "" {
		parts = append(parts, c.CalendarName)
	}
	return strings.Join(parts, " | ")
}

func (c calendarEventItem) FilterValue() string {
	return fmt.Sprintf("%s %s", c.Summary, c.CalendarName)
}

func formatTimeRange(start, end time.Time) string {
	if start.IsZero() {
		return ""
	}
	startText := start.Format("15:04")
	if end.IsZero() || end.Equal(start) {
		return startText
	}
	return fmt.Sprintf("%s-%s", startText, end.Format("15:04"))
}
