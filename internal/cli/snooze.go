package cli

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"justdoit/internal/timeparse"
)

type snoozeMsg struct {
	err error
}

var (
	dateTokenRe = regexp.MustCompile(`(?i)\b(today|tomorrow|next|this|mon|monday|tue|tuesday|wed|wednesday|thu|thursday|fri|friday|sat|saturday|sun|sunday|jan|january|feb|february|mar|march|apr|april|may|jun|june|jul|july|aug|august|sep|sept|september|oct|october|nov|november|dec|december)\b`)
	ymdTokenRe  = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`)
	slashToken  = regexp.MustCompile(`\b\d{1,2}/\d{1,2}(/\d{2,4})?\b`)
)

func (m tuiModel) snoozeCmd(task taskItem, input string) tea.Cmd {
	return func() tea.Msg {
		params, err := parseSnoozeInput(input, m.app.Now, m.app.Location)
		if err != nil {
			return snoozeMsg{err: err}
		}
		_, err = updateTaskWithParams(m.app, task.ListID, task.ID, params)
		return snoozeMsg{err: err}
	}
}

func (m *tuiModel) openSnooze(task taskItem) {
	if task.ID == "" {
		m.status = "Select a task to snooze"
		return
	}
	m.snoozeTask = task
	m.snoozeReturnState = m.state
	m.snoozeReturnListCtx = m.listCtx
	m.state = stateSnooze
	m.status = ""
	if m.snoozeInput.Placeholder == "" {
		m.snoozeInput = textinput.New()
		m.snoozeInput.Placeholder = "Snooze to (e.g. tomorrow or 15:00-16:00)"
		m.snoozeInput.CharLimit = 200
	}
	m.snoozeInput.SetValue("")
	m.snoozeInput.Focus()
}

func (m *tuiModel) restoreFromSnooze() {
	m.state = m.snoozeReturnState
	m.listCtx = m.snoozeReturnListCtx
	m.snoozeInput.Blur()
}

func (m *tuiModel) refreshAfterSnooze() (tuiModel, tea.Cmd) {
	switch m.state {
	case stateTodayTasks:
		return m.startNextLoad()
	case stateListTasks:
		return m.startListLoad(m.listName, m.showAll)
	case stateWeekView:
		m.weekLoading = true
		m.setSizes()
		return *m, m.loadWeekDataCmd(m.weekAnchor())
	case stateSearch:
		if strings.TrimSpace(m.searchQuery) == "" {
			m.tasksList = newTasksListModel([]list.Item{taskItem{TitleVal: "Type to search", IsHeader: true}}, "Results")
			return *m, nil
		}
		m.searchLoading = true
		return *m, m.searchCmd(m.searchQuery, m.searchList, m.searchIncludeCompleted)
	default:
		return *m, nil
	}
}

func parseSnoozeInput(value string, now time.Time, loc *time.Location) (UpdateParams, error) {
	input := strings.TrimSpace(value)
	if input == "" {
		return UpdateParams{}, fmt.Errorf("date or time is required")
	}

	if start, _, err := timeparse.ParseTimeRange(input, time.Time{}, now, loc); err == nil {
		params := UpdateParams{
			Time:    input,
			HasTime: true,
		}
		if hasExplicitDate(input) {
			date := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
			params.Date = date.Format("2006-01-02")
			params.HasDate = true
		}
		return params, nil
	}

	if parsed, err := timeparse.ParseDate(input, now, loc); err == nil && !parsed.IsZero() {
		return UpdateParams{
			Date:    parsed.Format("2006-01-02"),
			HasDate: true,
		}, nil
	}

	return UpdateParams{}, fmt.Errorf("invalid date or time")
}

func hasExplicitDate(input string) bool {
	return dateTokenRe.MatchString(input) || ymdTokenRe.MatchString(input) || slashToken.MatchString(input)
}
