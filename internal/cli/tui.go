package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"justdoit/internal/sync"
	"justdoit/internal/timeparse"
)

type tuiState int

const (
	stateMenu tuiState = iota
	stateToday
	stateListSelect
	stateListTasks
	stateNewTaskList
	stateNewTaskForm
)

type menuItem string

func (m menuItem) Title() string       { return string(m) }
func (m menuItem) Description() string { return "" }
func (m menuItem) FilterValue() string { return string(m) }

type listItem string

func (l listItem) Title() string       { return string(l) }
func (l listItem) Description() string { return "" }
func (l listItem) FilterValue() string { return string(l) }

type tuiModel struct {
	app        *App
	state      tuiState
	menu       list.Model
	listSelect list.Model
	viewport   viewport.Model

	listName string
	status   string

	formInputs []textinput.Model
	formStep   int
	winW       int
	winH       int
}

func startTUI(app *App) error {
	menu := list.New([]list.Item{
		menuItem("Today"),
		menuItem("Lists"),
		menuItem("New Task"),
		menuItem("Quit"),
	}, list.NewDefaultDelegate(), 0, 0)
	menu.Title = "justdoit"
	menu.SetShowStatusBar(false)
	menu.SetFilteringEnabled(false)
	menu.SetShowHelp(true)
	menu.KeyMap.Quit.SetEnabled(false)

	model := tuiModel{
		app:        app,
		state:      stateMenu,
		menu:       menu,
		listSelect: list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m tuiModel) Init() tea.Cmd {
	return nil
}

func (m *tuiModel) setSizes() {
	if m.winW == 0 || m.winH == 0 {
		return
	}
	m.menu.SetSize(m.winW-4, m.winH-6)
	m.listSelect.SetSize(m.winW-4, m.winH-6)
	m.viewport.Width = m.winW - 4
	m.viewport.Height = m.winH - 8
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.winW = msg.Width
		m.winH = msg.Height
		m.setSizes()
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.state == stateMenu {
				return m, tea.Quit
			}
		case "esc", "backspace":
			switch m.state {
			case stateMenu:
				return m, tea.Quit
			case stateListTasks:
				m.state = stateListSelect
				return m, nil
			default:
				m.state = stateMenu
				return m, nil
			}
		}
	}

	switch msg.(type) {
	case okMsg, errMsg:
		return m.handleMessage(msg)
	}

	switch m.state {
	case stateMenu:
		var cmd tea.Cmd
		m.menu, cmd = m.menu.Update(msg)
		if key, ok := msg.(tea.KeyMsg); ok && (key.String() == "enter" || key.String() == " ") {
			selected := m.menu.SelectedItem().(menuItem)
			switch string(selected) {
			case "Today":
				m.state = stateToday
				m.viewport = viewport.New(m.viewport.Width, m.viewport.Height)
				m.viewport.SetContent(buildDayText(m.app, time.Now().In(m.app.Location)))
			case "Lists":
				m.state = stateListSelect
				m.listSelect = newListSelect(m.app)
				m.setSizes()
			case "New Task":
				m.state = stateNewTaskList
				m.listSelect = newListSelect(m.app)
				m.setSizes()
			case "Quit":
				return m, tea.Quit
			}
		}
		return m, cmd
	case stateToday:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	case stateListSelect:
		var cmd tea.Cmd
		m.listSelect, cmd = m.listSelect.Update(msg)
		if key, ok := msg.(tea.KeyMsg); ok && (key.String() == "enter" || key.String() == " ") {
			selected := m.listSelect.SelectedItem().(listItem)
			m.listName = string(selected)
			m.state = stateListTasks
			m.viewport = viewport.New(m.viewport.Width, m.viewport.Height)
			m.viewport.SetContent(buildListText(m.app, m.listName, "", false))
		}
		return m, cmd
	case stateListTasks:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "n":
				m.state = stateNewTaskForm
				m.formInputs = newTaskInputs()
				m.formStep = 1
				m.formInputs[0].SetValue(m.listName)
				m.formInputs[1].Focus()
			}
		}
		return m, cmd
	case stateNewTaskList:
		var cmd tea.Cmd
		m.listSelect, cmd = m.listSelect.Update(msg)
		if key, ok := msg.(tea.KeyMsg); ok && (key.String() == "enter" || key.String() == " ") {
			selected := m.listSelect.SelectedItem().(listItem)
			m.listName = string(selected)
			m.state = stateNewTaskForm
			m.formInputs = newTaskInputs()
			m.formStep = 1
			m.formInputs[0].SetValue(m.listName)
			m.formInputs[1].Focus()
		}
		return m, cmd
	case stateNewTaskForm:
		var cmd tea.Cmd
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "enter":
				if m.formStep < len(m.formInputs)-1 {
					m.formInputs[m.formStep].Blur()
					m.formStep++
					m.formInputs[m.formStep].Focus()
					return m, nil
				}
				return m, m.createTaskCmd()
			}
		}
		m.formInputs[m.formStep], cmd = m.formInputs[m.formStep].Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m tuiModel) View() string {
	padding := lipgloss.NewStyle().Padding(1, 2)
	header := lipgloss.NewStyle().Bold(true).Render("justdoit")
	status := ""
	if m.status != "" {
		status = "\n\n" + gray(m.status)
	}

	switch m.state {
	case stateMenu:
		return padding.Render(header + "\n\n" + m.menu.View() + status)
	case stateToday:
		return padding.Render(lipgloss.NewStyle().Bold(true).Render("Today") + "\n\n" + m.viewport.View() + status)
	case stateListSelect:
		return padding.Render(lipgloss.NewStyle().Bold(true).Render("Select a list") + "\n\n" + m.listSelect.View() + status)
	case stateListTasks:
		return padding.Render(lipgloss.NewStyle().Bold(true).Render(m.listName) + "\n\n" + m.viewport.View() + "\n\n" + gray("n: new task • esc: back") + status)
	case stateNewTaskList:
		return padding.Render(lipgloss.NewStyle().Bold(true).Render("Choose list") + "\n\n" + m.listSelect.View() + status)
	case stateNewTaskForm:
		return padding.Render(renderForm(m.formInputs, m.formStep) + status)
	default:
		return ""
	}
}

func newListSelect(app *App) list.Model {
	items := make([]list.Item, 0, len(app.Config.Lists))
	keys := make([]string, 0, len(app.Config.Lists))
	for k := range app.Config.Lists {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		items = append(items, listItem(k))
	}
	if len(items) == 0 {
		items = append(items, listItem("(no lists configured)"))
	}
	model := list.New(items, list.NewDefaultDelegate(), 0, 0)
	model.SetShowStatusBar(false)
	model.SetFilteringEnabled(true)
	model.SetShowHelp(true)
	model.KeyMap.Quit.SetEnabled(false)
	return model
}

func newTaskInputs() []textinput.Model {
	listInput := textinput.New()
	listInput.Placeholder = "List"
	listInput.CharLimit = 64

	titleInput := textinput.New()
	titleInput.Placeholder = "Title"
	titleInput.CharLimit = 200

	sectionInput := textinput.New()
	sectionInput.Placeholder = "Section (default: General)"

	dateInput := textinput.New()
	dateInput.Placeholder = "Date (e.g. tomorrow)"

	timeInput := textinput.New()
	timeInput.Placeholder = "Time (e.g. 15:00-16:00 or 1h)"

	return []textinput.Model{listInput, titleInput, sectionInput, dateInput, timeInput}
}

func renderForm(inputs []textinput.Model, step int) string {
	labels := []string{"List", "Title", "Section", "Date", "Time"}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("New Task") + "\n\n")
	for i, input := range inputs {
		cursor := " "
		if i == step {
			cursor = "▶"
		}
		b.WriteString(fmt.Sprintf("%s %s: %s\n", cursor, labels[i], input.View()))
	}
	b.WriteString("\n")
	b.WriteString(gray("enter: next • esc: cancel"))
	return b.String()
}

func (m tuiModel) createTaskCmd() tea.Cmd {
	return func() tea.Msg {
		listName := strings.TrimSpace(m.formInputs[0].Value())
		if listName == "" {
			listName = m.listName
		}
		listID, ok := m.app.Config.ListID(listName)
		if !ok {
			return errMsg{err: fmt.Errorf("unknown list: %s", listName)}
		}
		title := strings.TrimSpace(m.formInputs[1].Value())
		if title == "" {
			return errMsg{err: fmt.Errorf("title is required")}
		}
		section := strings.TrimSpace(m.formInputs[2].Value())
		if section == "" {
			section = "General"
		}

		baseDate, err := timeparse.ParseDate(strings.TrimSpace(m.formInputs[3].Value()), m.app.Now, m.app.Location)
		if err != nil {
			return errMsg{err: err}
		}
		var start *time.Time
		var end *time.Time
		var due *time.Time
		if strings.TrimSpace(m.formInputs[4].Value()) != "" {
			startTime, endTime, err := timeparse.ParseTimeRange(strings.TrimSpace(m.formInputs[4].Value()), baseDate, m.app.Now, m.app.Location)
			if err != nil {
				return errMsg{err: err}
			}
			start = &startTime
			end = &endTime
			due = end
		} else if !baseDate.IsZero() {
			endOfDay := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 23, 59, 0, 0, m.app.Location)
			due = &endOfDay
		}

		sectionTask, err := ensureSectionTask(m.app, listID, section)
		if err != nil {
			return errMsg{err: err}
		}

		input := sync.CreateInput{
			ListID:    listID,
			Title:     title,
			Notes:     "",
			Due:       due,
			TimeStart: start,
			TimeEnd:   end,
			ParentID:  sectionTask.Id,
		}
		_, _, err = m.app.Sync.Create(input)
		if err != nil {
			return errMsg{err: err}
		}
		return okMsg{msg: "✅ Task created"}
	}
}

type okMsg struct{ msg string }

type errMsg struct{ err error }

func (m tuiModel) handleMessage(msg tea.Msg) (tuiModel, tea.Cmd) {
	switch msg := msg.(type) {
	case okMsg:
		m.status = msg.msg
		m.state = stateMenu
	case errMsg:
		m.status = msg.err.Error()
		m.state = stateMenu
	}
	return m, nil
}

func buildListText(app *App, listName, section string, all bool) string {
	listID, ok := app.Config.ListID(listName)
	if !ok {
		return "unknown list"
	}
	items, err := app.Tasks.ListTasksWithOptions(listID, all, all)
	if err != nil {
		return err.Error()
	}
	sections, order := groupTasksBySection(items, strings.TrimSpace(section))
	if len(sections) == 0 {
		return "(no tasks)"
	}
	var b strings.Builder
	for _, name := range order {
		tasks := sections[name]
		if len(tasks) == 0 {
			continue
		}
		b.WriteString(name + "\n")
		for _, t := range formatTasks(tasks) {
			b.WriteString("- " + t + "\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func formatTasks(tasks []taskRow) []string {
	due := make([]taskRow, 0, len(tasks))
	noDue := make([]taskRow, 0, len(tasks))
	for _, t := range tasks {
		if t.HasDue {
			due = append(due, t)
		} else {
			noDue = append(noDue, t)
		}
	}
	sort.SliceStable(due, func(i, j int) bool {
		if due[i].Due.Equal(due[j].Due) {
			return due[i].Index < due[j].Index
		}
		return due[i].Due.Before(due[j].Due)
	})
	ordered := append(due, noDue...)
	lines := make([]string, 0, len(ordered))
	for _, t := range ordered {
		dueText := ""
		if t.HasDue {
			dueText = fmt.Sprintf(" (due %s)", t.Due.Format("2006-01-02"))
		}
		lines = append(lines, fmt.Sprintf("%s [%s]%s", t.Title, t.ID, dueText))
	}
	return lines
}
