package cli

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"justdoit/internal/recurrence"
	"justdoit/internal/sync"
	"justdoit/internal/timeparse"
)

type quickCaptureMsg struct {
	err error
}

type quickCaptureInput struct {
	Title   string
	List    string
	Section string
	Date    string
	Time    string
	Every   string
}

func (m tuiModel) quickCaptureCmd(line string) tea.Cmd {
	return func() tea.Msg {
		err := createFromQuickCapture(m.app, line, m.app.Now, m.app.Location)
		return quickCaptureMsg{err: err}
	}
}

func createFromQuickCapture(app *App, line string, now time.Time, loc *time.Location) error {
	if app == nil {
		return fmt.Errorf("app is not initialized")
	}
	input, err := parseQuickCapture(line, now, loc)
	if err != nil {
		return err
	}

	title := input.Title
	recurrences := []string{}
	if strings.TrimSpace(input.Every) != "" {
		recurrences, err = recurrence.ParseEvery(input.Every)
		if err != nil {
			return err
		}
	} else {
		cleanTitle, extracted, ok := recurrence.ExtractFromText(title)
		if ok {
			title = cleanTitle
			recurrences = extracted
		}
	}

	listName := strings.TrimSpace(input.List)
	listID, err := resolveListID(app, listName, listName != "")
	if err != nil {
		return err
	}

	sectionName := strings.TrimSpace(input.Section)
	parentID := ""
	if sectionName != "" {
		sectionTask, err := ensureSectionTask(app, listID, sectionName)
		if err != nil {
			return err
		}
		parentID = sectionTask.Id
	}

	baseDate, err := timeparse.ParseDate(input.Date, now, loc)
	if err != nil {
		return err
	}

	var start *time.Time
	var end *time.Time
	var due *time.Time
	if strings.TrimSpace(input.Time) != "" {
		startTime, endTime, err := timeparse.ParseTimeRange(input.Time, baseDate, now, loc)
		if err != nil {
			return err
		}
		start = &startTime
		end = &endTime
		due = end
	} else if !baseDate.IsZero() {
		endOfDay := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 23, 59, 0, 0, loc)
		due = &endOfDay
	}
	if due == nil && len(recurrences) > 0 {
		today := now.In(loc)
		endOfDay := time.Date(today.Year(), today.Month(), today.Day(), 23, 59, 0, 0, loc)
		due = &endOfDay
	}

	createInput := sync.CreateInput{
		ListID:     listID,
		Title:      title,
		Notes:      "",
		Due:        due,
		Recurrence: recurrences,
		TimeStart:  start,
		TimeEnd:    end,
		ParentID:   parentID,
	}
	_, _, err = app.Sync.Create(createInput)
	return err
}

func parseQuickCapture(line string, now time.Time, loc *time.Location) (quickCaptureInput, error) {
	input := quickCaptureInput{}
	tokens := splitQuickCapture(line)
	titleParts := []string{}

	for _, token := range tokens {
		if token == "" {
			continue
		}
		switch {
		case strings.HasPrefix(token, "list:"):
			input.List = strings.TrimSpace(strings.TrimPrefix(token, "list:"))
			continue
		case strings.HasPrefix(token, "section:"):
			input.Section = strings.TrimSpace(strings.TrimPrefix(token, "section:"))
			continue
		case strings.HasPrefix(token, "every:"):
			input.Every = strings.TrimSpace(strings.TrimPrefix(token, "every:"))
			continue
		case strings.HasPrefix(token, "#") && len(token) > 1:
			input.List = strings.TrimSpace(strings.TrimPrefix(token, "#"))
			continue
		case strings.HasPrefix(token, "::") && len(token) > 2:
			input.Section = strings.TrimSpace(strings.TrimPrefix(token, "::"))
			continue
		case strings.HasPrefix(token, "@") && len(token) > 1:
			candidate := strings.TrimSpace(strings.TrimPrefix(token, "@"))
			if candidate == "" {
				continue
			}
			if input.Date == "" {
				if parsed, err := timeparse.ParseDate(candidate, now, loc); err == nil && !parsed.IsZero() {
					input.Date = candidate
					continue
				}
			}
			if input.Time == "" {
				if _, _, err := timeparse.ParseTimeRange(candidate, time.Time{}, now, loc); err == nil {
					input.Time = candidate
					continue
				}
			}
			titleParts = append(titleParts, token)
			continue
		}
		titleParts = append(titleParts, token)
	}

	input.Title = strings.TrimSpace(strings.Join(titleParts, " "))
	if input.Title == "" {
		return input, fmt.Errorf("title is required")
	}
	return input, nil
}

func splitQuickCapture(input string) []string {
	tokens := []string{}
	var buf strings.Builder
	var quote rune
	for _, r := range input {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
				continue
			}
			buf.WriteRune(r)
		case r == '"' || r == '\'':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			if buf.Len() > 0 {
				tokens = append(tokens, buf.String())
				buf.Reset()
			}
		default:
			buf.WriteRune(r)
		}
	}
	if buf.Len() > 0 {
		tokens = append(tokens, buf.String())
	}
	return tokens
}
