package cli

import (
	"strings"
	"time"

	"justdoit/internal/recurrence"
)

func recurringTitle(title, rule string) string {
	if strings.TrimSpace(rule) == "" {
		return title
	}
	trimmed := strings.TrimSpace(title)
	if strings.HasPrefix(trimmed, "🔁") {
		return title
	}
	return "🔁 " + title
}

func recurrenceText(rule string, loc *time.Location) string {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return ""
	}
	if text, ok := recurrence.Describe(rule, loc); ok {
		return text
	}
	return rule
}
