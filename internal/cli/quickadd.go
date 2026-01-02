package cli

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"justdoit/internal/timeparse"
)

type quickAddResult struct {
	Title    string
	ListName string
	ListID   string
	Section  string
	Notes    string
	Due      *time.Time
	Start    *time.Time
	End      *time.Time
}

func parseQuickAdd(input string, app *App, listHint string, dateHint time.Time) (quickAddResult, error) {
	text := strings.TrimSpace(input)
	if text == "" {
		return quickAddResult{}, fmt.Errorf("empty input")
	}
	listName := pickDefaultList(app, listHint)
	listNames := listKeys(app)
	text, picked := extractListSuffix(text, listNames)
	if picked != "" {
		listName = picked
	}
	listID, ok := app.Config.ListID(listName)
	if !ok {
		return quickAddResult{}, fmt.Errorf("unknown list: %s", listName)
	}

	text, timeStr := extractTime(text)
	text, dateStr := extractDate(text)
	text = normalizeSpaces(text)

	if text == "" {
		return quickAddResult{}, fmt.Errorf("missing task title")
	}

	var baseDate time.Time
	if dateStr != "" {
		parsed, err := timeparse.ParseDate(dateStr, app.Now, app.Location)
		if err != nil {
			return quickAddResult{}, err
		}
		baseDate = parsed
	} else if !dateHint.IsZero() {
		baseDate = time.Date(dateHint.Year(), dateHint.Month(), dateHint.Day(), 0, 0, 0, 0, app.Location)
	}

	var start *time.Time
	var end *time.Time
	var due *time.Time
	if timeStr != "" {
		startVal, endVal, err := timeparse.ParseTimeRange(timeStr, baseDate, app.Now, app.Location)
		if err != nil {
			return quickAddResult{}, err
		}
		start = &startVal
		end = &endVal
		due = &endVal
	} else if !baseDate.IsZero() {
		endOfDay := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 23, 59, 0, 0, app.Location)
		due = &endOfDay
	}

	return quickAddResult{
		Title:    text,
		ListName: listName,
		ListID:   listID,
		Due:      due,
		Start:    start,
		End:      end,
	}, nil
}

func listKeys(app *App) []string {
	keys := make([]string, 0, len(app.Config.Lists))
	for k := range app.Config.Lists {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	return keys
}

func pickDefaultList(app *App, hint string) string {
	if hint != "" {
		return hint
	}
	if app.Config.DefaultList != "" {
		return app.Config.DefaultList
	}
	for name := range app.Config.Lists {
		return name
	}
	return ""
}

func extractListSuffix(text string, listNames []string) (string, string) {
	lower := strings.ToLower(text)
	for _, name := range listNames {
		lname := strings.ToLower(name)
		for _, prefix := range []string{" en ", " in ", " @"} {
			suffix := prefix + lname
			if strings.HasSuffix(lower, suffix) {
				trimmed := strings.TrimSpace(text[:len(text)-len(suffix)])
				return trimmed, name
			}
		}
		if strings.HasSuffix(lower, "@"+lname) {
			trimmed := strings.TrimSpace(text[:len(text)-len(lname)-1])
			return trimmed, name
		}
	}
	return text, ""
}

func extractTime(text string) (string, string) {
	work := text
	if match := reTimeRange.FindStringSubmatch(work); len(match) > 0 {
		start := strings.TrimSpace(match[1])
		end := strings.TrimSpace(match[5])
		work = strings.Replace(work, match[0], "", 1)
		return normalizeSpaces(work), fmt.Sprintf("%s-%s", start, end)
	}
	if match := reTimeRangeDash.FindStringSubmatch(work); len(match) > 0 {
		start := strings.TrimSpace(match[1])
		end := strings.TrimSpace(match[3])
		work = strings.Replace(work, match[0], "", 1)
		return normalizeSpaces(work), fmt.Sprintf("%s-%s", start, end)
	}
	if match := reDuration.FindStringSubmatch(work); len(match) > 0 {
		dur := strings.ReplaceAll(strings.TrimSpace(match[1]), " ", "")
		work = strings.Replace(work, match[0], "", 1)
		return normalizeSpaces(work), dur
	}
	return text, ""
}

func extractDate(text string) (string, string) {
	work := text
	lower := strings.ToLower(work)
	if strings.Contains(lower, "pasado mañana") {
		work = replaceOnceCaseInsensitive(work, "pasado mañana", "")
		return normalizeSpaces(work), "day after tomorrow"
	}
	if strings.Contains(lower, "pasadomanana") {
		work = replaceOnceCaseInsensitive(work, "pasadomanana", "")
		return normalizeSpaces(work), "day after tomorrow"
	}
	if strings.Contains(lower, "mañana") {
		work = replaceOnceCaseInsensitive(work, "mañana", "")
		return normalizeSpaces(work), "tomorrow"
	}
	if strings.Contains(lower, "manana") {
		work = replaceOnceCaseInsensitive(work, "manana", "")
		return normalizeSpaces(work), "tomorrow"
	}
	if strings.Contains(lower, "hoy") {
		work = replaceOnceCaseInsensitive(work, "hoy", "")
		return normalizeSpaces(work), "today"
	}
	if strings.Contains(lower, "tomorrow") {
		work = replaceOnceCaseInsensitive(work, "tomorrow", "")
		return normalizeSpaces(work), "tomorrow"
	}
	if strings.Contains(lower, "today") {
		work = replaceOnceCaseInsensitive(work, "today", "")
		return normalizeSpaces(work), "today"
	}
	if iso := reISODate.FindString(work); iso != "" {
		work = strings.Replace(work, iso, "", 1)
		return normalizeSpaces(work), iso
	}
	return text, ""
}

func replaceOnceCaseInsensitive(text, target, repl string) string {
	index := strings.Index(strings.ToLower(text), strings.ToLower(target))
	if index == -1 {
		return text
	}
	return text[:index] + repl + text[index+len(target):]
}

func normalizeSpaces(text string) string {
	fields := strings.Fields(text)
	return strings.TrimSpace(strings.Join(fields, " "))
}

var (
	reTimeRange     = regexp.MustCompile(`(?i)\bde\s+([0-9]{1,2}(:[0-9]{2})?\s*(am|pm)?)\s+(a|hasta)\s+([0-9]{1,2}(:[0-9]{2})?\s*(am|pm)?)\b`)
	reTimeRangeDash = regexp.MustCompile(`(?i)\b([0-9]{1,2}(:[0-9]{2})?)\s*-\s*([0-9]{1,2}(:[0-9]{2})?)\b`)
	reDuration      = regexp.MustCompile(`(?i)\b(\d+\s*(h|m))\b`)
	reISODate       = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`)
)
