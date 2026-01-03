package cli

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type searchFocus int

const (
	focusSearchInput searchFocus = iota
	focusSearchList
)

type searchMsg struct {
	results []taskItem
	err     error
}

type searchItem struct {
	Task taskItem
}

func (s searchItem) Title() string {
	return recurringTitle(s.Task.TitleVal, s.Task.Recurrence)
}

func (s searchItem) Description() string {
	parts := []string{}
	if s.Task.ListName != "" {
		parts = append(parts, s.Task.ListName)
	}
	if strings.TrimSpace(s.Task.Section) != "" {
		parts = append(parts, s.Task.Section)
	}
	if s.Task.HasDue {
		parts = append(parts, "due "+s.Task.Due.Format("2006-01-02"))
	}
	return strings.Join(parts, " • ")
}

func (s searchItem) FilterValue() string {
	return strings.TrimSpace(s.Task.TitleVal + " " + s.Task.Section + " " + s.Task.ListName)
}

func (m tuiModel) searchCmd(query, list string, includeCompleted bool) tea.Cmd {
	return func() tea.Msg {
		results, err := searchTasks(newQueryContext(m.app), query, list, includeCompleted)
		return searchMsg{results: results, err: err}
	}
}

func buildSearchItems(results []taskItem) []list.Item {
	if len(results) == 0 {
		return []list.Item{taskItem{TitleVal: "No results", IsHeader: true}}
	}
	items := make([]list.Item, 0, len(results))
	for _, item := range results {
		items = append(items, searchItem{Task: item})
	}
	return items
}

func (m *tuiModel) openSearch() {
	m.searchReturnState = m.state
	m.searchReturnListCtx = m.listCtx
	m.state = stateSearch
	m.listCtx = listCtxSearch
	m.status = ""
	if m.searchInput.Placeholder == "" {
		m.searchInput = textinput.New()
		m.searchInput.Placeholder = "Search tasks"
		m.searchInput.CharLimit = 200
	}
	m.searchInput.SetValue("")
	m.searchInput.Focus()
	m.searchFocus = focusSearchInput
	m.searchQuery = ""
	m.searchList = ""
	m.searchIncludeCompleted = false
	m.searchLoading = false
	m.tasksList = newTasksListModel([]list.Item{taskItem{TitleVal: "Type to search", IsHeader: true}}, "Results")
	m.setSizes()
}

func (m *tuiModel) restoreFromSearch() {
	m.state = m.searchReturnState
	m.listCtx = m.searchReturnListCtx
	m.searchInput.Blur()
}

func (m *tuiModel) refreshAfterSearch() (tuiModel, tea.Cmd) {
	switch m.state {
	case stateTodayTasks:
		return m.startNextLoad()
	case stateListTasks:
		return m.startListLoad(m.listName, m.showAll)
	case stateWeekView:
		m.weekLoading = true
		m.setSizes()
		return *m, m.loadWeekDataCmd(m.weekAnchor())
	default:
		return *m, nil
	}
}

func (m *tuiModel) cycleSearchList() {
	options := m.searchListOptions()
	if len(options) == 0 {
		m.searchList = ""
		return
	}
	current := m.searchList
	idx := 0
	for i, option := range options {
		if option == current {
			idx = i
			break
		}
	}
	idx = (idx + 1) % len(options)
	m.searchList = options[idx]
}

func (m *tuiModel) searchListOptions() []string {
	names := make([]string, 0, len(m.app.Config.Lists))
	for name := range m.app.Config.Lists {
		names = append(names, name)
	}
	sort.Strings(names)
	return append([]string{""}, names...)
}
