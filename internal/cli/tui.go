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

type formMode string

type listContext string

const (
	stateMenu tuiState = iota
	stateWeekView
	stateTodayTasks
	stateAgendaDetails
	stateListSelect
	stateListTasks
	stateNewTaskList
	stateTaskForm
	stateConfirmDelete
	stateCalendarSelect
)

const (
	formNew  formMode = "new"
	formEdit formMode = "edit"
)

const (
	listCtxToday listContext = "today"
	listCtxList  listContext = "list"
	listCtxWeek  listContext = "week"
)

var (
	colorAccent = lipgloss.Color("69")
	colorMuted  = lipgloss.Color("241")
)

type menuItem string

func (m menuItem) Title() string       { return string(m) }
func (m menuItem) Description() string { return "" }
func (m menuItem) FilterValue() string { return string(m) }

type listItem string

func (l listItem) Title() string       { return string(l) }
func (l listItem) Description() string { return "" }
func (l listItem) FilterValue() string { return string(l) }

type taskItem struct {
	ID         string
	TitleVal   string
	ListName   string
	ListID     string
	Section    string
	Due        time.Time
	HasDue     bool
	IsHeader   bool
	Recurrence string
}

func (t taskItem) Title() string {
	if t.IsHeader {
		return lipgloss.NewStyle().Bold(true).Foreground(colorMuted).Render(t.TitleVal)
	}
	return recurringTitle(t.TitleVal, t.Recurrence)
}

func (t taskItem) Description() string {
	if t.IsHeader {
		return ""
	}
	dueText := ""
	if t.HasDue {
		dueText = fmt.Sprintf("due %s", t.Due.Format("2006-01-02"))
	}
	idText := gray("[" + t.ID + "]")
	if dueText == "" {
		return idText
	}
	return fmt.Sprintf("%s · %s", dueText, idText)
}

func (t taskItem) FilterValue() string { return t.TitleVal }

type tuiModel struct {
	app *App

	state          tuiState
	menu           list.Model
	listSelect     list.Model
	tasksList      list.Model
	viewport       viewport.Model
	calendarSelect list.Model

	listName string
	listCtx  listContext
	showAll  bool
	status   string

	weekData         weekData
	weekFocus        weekFocus
	weekDayIndex     int
	weekBacklogIndex int
	weekEventIndex   int
	weekAllDayIndex  int
	weekLoading      bool
	calendarLoading  bool
	weekRefreshing   bool

	confirmMsg  string
	confirmTask taskItem

	formInputs []textinput.Model
	formStep   int
	formMode   formMode
	editTask   taskItem

	winW int
	winH int
}

func startTUI(app *App) error {
	menu := list.New([]list.Item{
		menuItem("Today"),
		menuItem("Week"),
		menuItem("Lists"),
		menuItem("New Task"),
		menuItem("Quit"),
	}, list.NewDefaultDelegate(), 0, 0)
	menu.Title = "justdoit"
	menu.SetShowStatusBar(false)
	menu.SetFilteringEnabled(false)
	menu.SetShowHelp(true)
	menu.KeyMap.Quit.SetEnabled(false)
	menu = styleList(menu)

	model := tuiModel{
		app:            app,
		state:          stateMenu,
		menu:           menu,
		listSelect:     styleList(list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)),
		tasksList:      styleList(list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)),
		calendarSelect: list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
		listCtx:        listCtxList,
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
	m.tasksList.SetSize(m.winW-4, m.winH-8)
	m.viewport.Width = m.winW - 4
	m.viewport.Height = m.winH - 8
	m.calendarSelect.SetSize(m.winW-4, m.winH-6)
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
		case "esc":
			switch m.state {
			case stateMenu:
				return m, tea.Quit
			case stateWeekView:
				m.state = stateMenu
				return m, nil
			case stateListTasks:
				m.state = stateListSelect
				return m, nil
			case stateTodayTasks:
				m.state = stateMenu
				return m, nil
			case stateAgendaDetails:
				m.state = stateTodayTasks
				return m, nil
			case stateCalendarSelect:
				m.state = stateWeekView
				return m, nil
			case stateNewTaskList:
				m.state = stateMenu
				return m, nil
			case stateTaskForm, stateConfirmDelete:
				switch m.listCtx {
				case listCtxToday:
					m.state = stateTodayTasks
				case listCtxList:
					if m.listName == "" {
						m.state = stateMenu
					} else {
						m.state = stateListTasks
					}
				case listCtxWeek:
					m.state = stateWeekView
				default:
					m.state = stateMenu
				}
				return m, nil
			default:
				m.state = stateMenu
				return m, nil
			}
		}
	}

	switch msg.(type) {
	case okMsg, errMsg, weekDataMsg, calendarListMsg, taskToggleMsg:
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
				m.state = stateTodayTasks
				m.listCtx = listCtxToday
				m.showAll = false
				m.tasksList = newTasksListModel(buildTodayItems(m.app), "Today")
				m.setSizes()
			case "Week":
				m.state = stateWeekView
				m.listCtx = listCtxWeek
				m.weekFocus = focusGrid
				m.weekDayIndex = -1
				m.weekEventIndex = -1
				m.weekAllDayIndex = -1
				m.weekBacklogIndex = -1
				m.weekLoading = true
				return m, m.loadWeekDataCmd(m.app.Now)
			case "Lists":
				m.state = stateListSelect
				m.listSelect = newListSelect(m.app)
				m.setSizes()
			case "New Task":
				m.state = stateNewTaskList
				m.listCtx = listCtxList
				m.listSelect = newListSelect(m.app)
				m.setSizes()
			case "Quit":
				return m, tea.Quit
			}
		}
		return m, cmd
	case stateWeekView:
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "tab":
				if m.weekFocus == focusBacklog {
					m.weekFocus = focusGrid
				} else {
					m.weekFocus = focusBacklog
				}
				return m, nil
			case "left", "h":
				return m, m.shiftWeekDay(-1)
			case "right", "l":
				return m, m.shiftWeekDay(1)
			case "[":
				return m, m.shiftWeek(-1)
			case "]":
				return m, m.shiftWeek(1)
			case "up", "k":
				m.moveWeekSelection(-1)
				return m, nil
			case "down", "j":
				m.moveWeekSelection(1)
				return m, nil
			case " ":
				return m, m.completeWeekTaskCmd()
			case "e", "enter":
				return m, m.editWeekTaskCmd()
			case "d":
				m.prepareDeleteWeekTask()
				return m, nil
			case "n":
				date := time.Time{}
				if len(m.weekData.Days) > 0 && m.weekDayIndex >= 0 && m.weekDayIndex < len(m.weekData.Days) {
					date = m.weekData.Days[m.weekDayIndex]
				}
				m.openTaskForm(m.app.Config.DefaultList, date)
				return m, nil
			case "c":
				m.state = stateCalendarSelect
				m.calendarLoading = true
				return m, m.loadCalendarListCmd()
			case "r":
				m.weekRefreshing = true
				return m, m.refreshWeekDataCmd(m.weekAnchor())
			case "t":
				m.weekDayIndex = -1
				m.weekLoading = true
				return m, m.loadWeekDataCmd(m.app.Now)
			}
		}
		return m, nil
	case stateTodayTasks:
		var cmd tea.Cmd
		m.tasksList, cmd = m.tasksList.Update(msg)
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "a":
				m.state = stateAgendaDetails
				m.viewport = viewport.New(m.viewport.Width, m.viewport.Height)
				m.viewport.SetContent(buildDayText(m.app, time.Now().In(m.app.Location)))
			case " ":
				return m, m.completeSelectedTaskCmd()
			case "e", "enter":
				return m, m.editSelectedTaskCmd()
			case "d":
				m.prepareDelete()
				return m, nil
			case "n":
				m.openTaskForm(m.app.Config.DefaultList, time.Now().In(m.app.Location))
			}
		}
		return m, cmd
	case stateAgendaDetails:
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
			m.listCtx = listCtxList
			m.showAll = false
			items, err := buildListItems(m.app, m.listName, m.showAll)
			if err != nil {
				m.status = err.Error()
				m.state = stateMenu
				return m, nil
			}
			m.tasksList = newTasksListModel(items, m.listName)
			m.setSizes()
		}
		return m, cmd
	case stateListTasks:
		var cmd tea.Cmd
		m.tasksList, cmd = m.tasksList.Update(msg)
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "a":
				m.showAll = !m.showAll
				items, err := buildListItems(m.app, m.listName, m.showAll)
				if err != nil {
					m.status = err.Error()
					m.state = stateMenu
					return m, nil
				}
				m.tasksList = newTasksListModel(items, m.listName)
				m.setSizes()
			case " ":
				return m, m.completeSelectedTaskCmd()
			case "e", "enter":
				return m, m.editSelectedTaskCmd()
			case "d":
				m.prepareDelete()
				return m, nil
			case "n":
				m.openTaskForm(m.listName, time.Time{})
			}
		}
		return m, cmd
	case stateNewTaskList:
		var cmd tea.Cmd
		m.listSelect, cmd = m.listSelect.Update(msg)
		if key, ok := msg.(tea.KeyMsg); ok && (key.String() == "enter" || key.String() == " ") {
			selected := m.listSelect.SelectedItem().(listItem)
			m.listName = string(selected)
			m.state = stateTaskForm
			m.formMode = formNew
			m.formInputs = newTaskInputs()
			m.formStep = 1
			m.formInputs[0].SetValue(m.listName)
			m.formInputs[1].Focus()
		}
		return m, cmd
	case stateTaskForm:
		var cmd tea.Cmd
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "tab":
				m.formInputs[m.formStep].Blur()
				m.formStep = (m.formStep + 1) % len(m.formInputs)
				m.formInputs[m.formStep].Focus()
				return m, nil
			case "shift+tab", "backtab":
				m.formInputs[m.formStep].Blur()
				m.formStep = (m.formStep - 1 + len(m.formInputs)) % len(m.formInputs)
				m.formInputs[m.formStep].Focus()
				return m, nil
			case "ctrl+u":
				m.formInputs[m.formStep].SetValue("")
				m.formInputs[m.formStep].SetCursor(0)
				return m, nil
			case "ctrl+k":
				pos := m.formInputs[m.formStep].Position()
				if pos < 0 {
					pos = 0
				}
				val := m.formInputs[m.formStep].Value()
				if pos > len(val) {
					pos = len(val)
				}
				m.formInputs[m.formStep].SetValue(val[:pos])
				return m, nil
			case "enter":
				if m.formStep < len(m.formInputs)-1 {
					m.formInputs[m.formStep].Blur()
					m.formStep++
					m.formInputs[m.formStep].Focus()
					return m, nil
				}
				if m.formMode == formNew {
					return m, m.createTaskCmd()
				}
				return m, m.updateTaskCmd()
			}
		}
		m.formInputs[m.formStep], cmd = m.formInputs[m.formStep].Update(msg)
		return m, cmd
	case stateConfirmDelete:
		if key, ok := msg.(tea.KeyMsg); ok {
			switch strings.ToLower(key.String()) {
			case "y":
				return m, m.deleteTaskCmd()
			case "n":
				switch m.listCtx {
				case listCtxToday:
					m.state = stateTodayTasks
				case listCtxWeek:
					m.state = stateWeekView
				default:
					m.state = stateListTasks
				}
				return m, nil
			}
		}
		return m, nil
	case stateCalendarSelect:
		var cmd tea.Cmd
		m.calendarSelect, cmd = m.calendarSelect.Update(msg)
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case " ":
				m.toggleCalendarSelection()
				return m, nil
			case "enter":
				return m, m.saveCalendarSelectionCmd()
			}
		}
		return m, cmd
	default:
		return m, nil
	}
}

func (m tuiModel) View() string {
	padding := lipgloss.NewStyle().Padding(1, 2)
	status := ""
	if m.status != "" {
		status = "\n\n" + gray(m.status)
	}

	switch m.state {
	case stateMenu:
		return padding.Render(renderHeader("Home") + "\n\n" + m.menu.View() + status)
	case stateWeekView:
		hint := "tab: switch • ←/→ day • [ ]: week • t: today • ↑/↓ item • space: done • e: edit • d: delete • n: new task • c: calendars • r: refresh • esc: back"
		if m.weekRefreshing {
			hint += " • refreshing…"
		}
		return padding.Render(renderHeader("Week") + "\n\n" + m.weekView() + "\n\n" + gray(hint) + status)
	case stateTodayTasks:
		return padding.Render(renderHeader("Today") + "\n\n" + m.splitPane(m.tasksList.View(), m.detailsView()) + "\n\n" + gray("space: done • e: edit • d: delete • n: new task • a: agenda • esc: back") + status)
	case stateAgendaDetails:
		return padding.Render(renderHeader("Agenda") + "\n\n" + m.viewport.View() + "\n\n" + gray("esc: back") + status)
	case stateListSelect:
		return padding.Render(renderHeader("Select a list") + "\n\n" + m.listSelect.View() + status)
	case stateListTasks:
		return padding.Render(renderHeader(m.listName) + "\n\n" + m.splitPane(m.tasksList.View(), m.detailsView()) + "\n\n" + gray("space: done • e: edit • d: delete • n: new task • a: all • esc: back") + status)
	case stateNewTaskList:
		return padding.Render(renderHeader("Choose list") + "\n\n" + m.listSelect.View() + status)
	case stateTaskForm:
		return padding.Render(renderForm(m.formInputs, m.formStep, m.formMode) + status)
	case stateConfirmDelete:
		return padding.Render(renderHeader("Confirm delete") + "\n\n" + m.confirmMsg + "\n\n" + gray("y: delete • n: cancel"))
	case stateCalendarSelect:
		if m.calendarLoading {
			return padding.Render(renderHeader("Calendars") + "\n\n" + "Loading calendars..." + status)
		}
		return padding.Render(renderHeader("View calendars") + "\n\n" + m.calendarSelect.View() + "\n\n" + gray("space: toggle • enter: save • esc: back") + status)
	default:
		return ""
	}
}

func renderHeader(title string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Render("justdoit") + " · " + lipgloss.NewStyle().Bold(true).Render(title)
}

func (m *tuiModel) splitPane(left, right string) string {
	width := m.winW - 4
	if width < 60 {
		return left
	}
	leftWidth := width / 2
	if leftWidth < 32 {
		leftWidth = 32
	}
	rightWidth := width - leftWidth - 2

	m.tasksList.SetSize(leftWidth, m.winH-8)
	leftPane := lipgloss.NewStyle().Width(leftWidth).Render(left)
	rightPane := lipgloss.NewStyle().Width(rightWidth).Render(right)
	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

func (m *tuiModel) detailsView() string {
	task, ok := m.selectedTask()
	if !ok {
		return gray("Select a task to see details")
	}
	dueText := "None"
	if task.HasDue {
		dueText = task.Due.Format("2006-01-02")
	}
	lines := []string{
		lipgloss.NewStyle().Bold(true).Render(task.TitleVal),
		"",
		fmt.Sprintf("List: %s", task.ListName),
		fmt.Sprintf("Section: %s", task.Section),
		fmt.Sprintf("Due: %s", dueText),
		fmt.Sprintf("ID: %s", task.ID),
	}
	return strings.Join(lines, "\n")
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
	return styleList(model)
}

func newTasksListModel(items []list.Item, title string) list.Model {
	model := list.New(items, list.NewDefaultDelegate(), 0, 0)
	model.Title = title
	model.SetShowStatusBar(false)
	model.SetFilteringEnabled(true)
	model.SetShowHelp(false)
	model.KeyMap.Quit.SetEnabled(false)
	return styleList(model)
}

func styleList(model list.Model) list.Model {
	styles := model.Styles
	styles.Title = styles.Title.Foreground(colorAccent).Bold(true)
	styles.FilterPrompt = styles.FilterPrompt.Foreground(colorMuted)
	styles.FilterCursor = styles.FilterCursor.Foreground(colorAccent)
	styles.StatusBar = styles.StatusBar.Foreground(colorMuted)
	styles.PaginationStyle = styles.PaginationStyle.Foreground(colorMuted)
	styles.HelpStyle = styles.HelpStyle.Foreground(colorMuted)
	model.Styles = styles

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(colorAccent).Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(colorMuted)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.Foreground(colorMuted)
	model.SetDelegate(delegate)

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

	notesInput := textinput.New()
	notesInput.Placeholder = "Notes"

	return []textinput.Model{listInput, titleInput, sectionInput, dateInput, timeInput, notesInput}
}

func renderForm(inputs []textinput.Model, step int, mode formMode) string {
	labels := []string{"List", "Title", "Section", "Date", "Time", "Notes"}
	var b strings.Builder
	title := "New Task"
	if mode == formEdit {
		title = "Edit Task"
	}
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(title) + "\n\n")
	for i, input := range inputs {
		cursor := " "
		if i == step {
			cursor = "▶"
		}
		b.WriteString(fmt.Sprintf("%s %s: %s\n", cursor, labels[i], input.View()))
	}
	b.WriteString("\n")
	b.WriteString(gray("tab/shift+tab: navigate • enter: save • esc: cancel • ctrl+u: clear"))
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
			Notes:     strings.TrimSpace(m.formInputs[5].Value()),
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

func (m tuiModel) updateTaskCmd() tea.Cmd {
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
		section := strings.TrimSpace(m.formInputs[2].Value())
		dateStr := strings.TrimSpace(m.formInputs[3].Value())
		timeStr := strings.TrimSpace(m.formInputs[4].Value())
		notes := strings.TrimSpace(m.formInputs[5].Value())

		params := UpdateParams{
			Title:      title,
			HasTitle:   true,
			Notes:      notes,
			HasNotes:   true,
			Section:    section,
			HasSection: true,
			Date:       dateStr,
			HasDate:    dateStr != "",
			Time:       timeStr,
			HasTime:    timeStr != "",
		}
		_, err := updateTaskWithParams(m.app, listID, m.editTask.ID, params)
		if err != nil {
			return errMsg{err: err}
		}
		return okMsg{msg: "✅ Task updated"}
	}
}

type okMsg struct{ msg string }

type errMsg struct{ err error }

type taskToggleMsg struct {
	TaskID    string
	Completed bool
}

func (m tuiModel) handleMessage(msg tea.Msg) (tuiModel, tea.Cmd) {
	switch msg := msg.(type) {
	case okMsg:
		m.status = msg.msg
		switch m.state {
		case stateTaskForm:
			switch m.listCtx {
			case listCtxToday:
				m.state = stateTodayTasks
				m.tasksList = newTasksListModel(buildTodayItems(m.app), "Today")
			case listCtxWeek:
				m.state = stateWeekView
				m.weekLoading = true
				m.setSizes()
				return m, m.loadWeekDataCmd(m.weekAnchor())
			default:
				if m.listName != "" {
					items, _ := buildListItems(m.app, m.listName, m.showAll)
					m.state = stateListTasks
					m.tasksList = newTasksListModel(items, m.listName)
				} else {
					m.state = stateMenu
				}
			}
		case stateConfirmDelete:
			switch m.listCtx {
			case listCtxToday:
				m.state = stateTodayTasks
				m.tasksList = newTasksListModel(buildTodayItems(m.app), "Today")
			case listCtxWeek:
				m.state = stateWeekView
				m.weekLoading = true
				m.setSizes()
				return m, m.loadWeekDataCmd(m.weekAnchor())
			default:
				items, _ := buildListItems(m.app, m.listName, m.showAll)
				m.state = stateListTasks
				m.tasksList = newTasksListModel(items, m.listName)
			}
		case stateListTasks:
			items, _ := buildListItems(m.app, m.listName, m.showAll)
			m.tasksList = newTasksListModel(items, m.listName)
		case stateTodayTasks:
			m.tasksList = newTasksListModel(buildTodayItems(m.app), "Today")
		case stateCalendarSelect:
			m.state = stateWeekView
			m.weekLoading = true
			m.setSizes()
			return m, m.loadWeekDataCmd(m.weekAnchor())
		default:
			m.state = stateMenu
		}
	case errMsg:
		m.status = msg.err.Error()
		if m.state == stateWeekView {
			m.weekLoading = false
			m.weekRefreshing = false
		}
		if m.state == stateCalendarSelect {
			m.calendarLoading = false
		}
		// keep state
	case weekDataMsg:
		m.weekData = msg.data
		m.weekLoading = false
		m.weekRefreshing = msg.fromCache
		if m.weekDayIndex < 0 || m.weekDayIndex > 6 {
			idx := dayIndex(msg.data.Days, m.app.Now)
			if idx < 0 {
				idx = 0
			}
			m.weekDayIndex = idx
		}
		m.ensureWeekSelection()
	case calendarListMsg:
		m.calendarLoading = false
		m.calendarSelect = newCalendarSelect(msg.items, m.app.Config.ViewCalendars)
	case taskToggleMsg:
		if msg.TaskID != "" {
			m.applyTaskToggle(msg.TaskID, msg.Completed)
			if msg.Completed {
				m.status = "✅ Task completed"
			} else {
				m.status = "↩️ Task reopened"
			}
		}
	}
	m.setSizes()
	return m, nil
}

func buildListItems(app *App, listName string, all bool) ([]list.Item, error) {
	listID, ok := app.Config.ListID(listName)
	if !ok {
		return nil, fmt.Errorf("unknown list: %s", listName)
	}
	items, err := app.Tasks.ListTasksWithOptions(listID, all, all, false, "")
	if err != nil {
		return nil, err
	}
	sections, order := groupTasksBySection(items, "")
	result := []list.Item{}
	for _, section := range order {
		rows := sections[section]
		if len(rows) == 0 {
			continue
		}
		result = append(result, taskItem{TitleVal: section, IsHeader: true})
		for _, row := range orderTaskRows(rows) {
			result = append(result, taskItem{
				ID:         row.ID,
				TitleVal:   row.Title,
				ListName:   listName,
				ListID:     listID,
				Section:    section,
				Due:        row.Due,
				HasDue:     row.HasDue,
				Recurrence: row.Recurrence,
			})
		}
	}
	return result, nil
}

func buildTodayItems(app *App) []list.Item {
	items := []list.Item{}
	tasksToday, err := collectTasks(app, time.Now().In(app.Location))
	if err != nil {
		return []list.Item{taskItem{TitleVal: err.Error(), IsHeader: true}}
	}
	byList := map[string][]taskView{}
	order := []string{}
	for _, t := range tasksToday {
		if _, ok := byList[t.List]; !ok {
			order = append(order, t.List)
		}
		byList[t.List] = append(byList[t.List], t)
	}
	for _, listName := range order {
		listID, _ := app.Config.ListID(listName)
		items = append(items, taskItem{TitleVal: listName, IsHeader: true})
		rows := byList[listName]
		for _, t := range rows {
			items = append(items, taskItem{
				ID:         t.ID,
				TitleVal:   t.Title,
				ListName:   listName,
				ListID:     listID,
				Due:        t.Due,
				HasDue:     true,
				Recurrence: t.Recurrence,
			})
		}
	}
	if len(items) == 0 {
		items = append(items, taskItem{TitleVal: "(no tasks due today)", IsHeader: true})
	}
	return items
}

func orderTaskRows(rows []taskRow) []taskRow {
	due := make([]taskRow, 0, len(rows))
	noDue := make([]taskRow, 0, len(rows))
	for _, t := range rows {
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
	return append(due, noDue...)
}

func (m *tuiModel) selectedTask() (taskItem, bool) {
	item := m.tasksList.SelectedItem()
	if item == nil {
		return taskItem{}, false
	}
	task, ok := item.(taskItem)
	if !ok || task.IsHeader {
		return taskItem{}, false
	}
	return task, true
}

func (m *tuiModel) completeSelectedTaskCmd() tea.Cmd {
	task, ok := m.selectedTask()
	if !ok {
		return nil
	}
	return m.completeTaskCmd(task)
}

func (m *tuiModel) completeTaskCmd(task taskItem) tea.Cmd {
	return func() tea.Msg {
		completed, err := toggleTaskDone(m.app, task.ListID, task.ID, true)
		if err != nil {
			return errMsg{err: err}
		}
		if m.listCtx == listCtxWeek {
			return taskToggleMsg{TaskID: task.ID, Completed: completed}
		}
		if completed {
			return okMsg{msg: "✅ Task completed"}
		}
		return okMsg{msg: "↩️ Task reopened"}
	}
}

func (m *tuiModel) editSelectedTaskCmd() tea.Cmd {
	task, ok := m.selectedTask()
	if !ok {
		return nil
	}
	return m.beginEditTask(task)
}

func (m *tuiModel) prepareDelete() {
	task, ok := m.selectedTask()
	if !ok {
		return
	}
	m.prepareDeleteTask(task)
}

func (m *tuiModel) prepareDeleteTask(task taskItem) {
	m.confirmTask = task
	m.confirmMsg = fmt.Sprintf("Delete '%s'? (event will be removed)", task.TitleVal)
	m.state = stateConfirmDelete
}

func (m *tuiModel) beginEditTask(task taskItem) tea.Cmd {
	m.editTask = task
	m.formMode = formEdit
	m.state = stateTaskForm
	m.formInputs = newTaskInputs()
	m.formStep = 1
	m.formInputs[0].SetValue(task.ListName)
	m.formInputs[1].SetValue(task.TitleVal)
	m.formInputs[2].SetValue(task.Section)
	if task.HasDue {
		m.formInputs[3].SetValue(task.Due.Format("2006-01-02"))
	}
	if t, err := m.app.Tasks.GetTask(task.ListID, task.ID); err == nil {
		m.formInputs[5].SetValue(stripMetadataNotes(t.Notes))
		if event, ok, _ := findLinkedEvent(m.app, t); ok && event != nil {
			if start, end := eventTimes(event, m.app.Location); !start.IsZero() && !end.IsZero() {
				m.formInputs[4].SetValue(fmt.Sprintf("%s-%s", start.Format("15:04"), end.Format("15:04")))
			}
		}
	}
	m.formInputs[1].Focus()
	return nil
}

func (m *tuiModel) openTaskForm(listName string, dateHint time.Time) {
	m.state = stateTaskForm
	m.formMode = formNew
	m.formInputs = newTaskInputs()
	m.formStep = 1
	if listName == "" {
		listName = m.app.Config.DefaultList
	}
	m.formInputs[0].SetValue(listName)
	if !dateHint.IsZero() {
		m.formInputs[3].SetValue(dateHint.Format("2006-01-02"))
	}
	m.formInputs[1].Focus()
}

func (m *tuiModel) deleteTaskCmd() tea.Cmd {
	task := m.confirmTask
	return func() tea.Msg {
		err := deleteTask(m.app, task.ListID, task.ID, true)
		if err != nil {
			return errMsg{err: err}
		}
		return okMsg{msg: "🗑️ Task deleted"}
	}
}
