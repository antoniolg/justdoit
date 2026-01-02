package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"google.golang.org/api/calendar/v3"
	"justdoit/internal/timeparse"
)

type weekFocus int

const (
	focusGrid weekFocus = iota
	focusBacklog
)

type weekEvent struct {
	Summary      string
	CalendarID   string
	CalendarName string
	TaskID       string
	Start        time.Time
	End          time.Time
	StartSlot    int
	EndSlot      int
	AllDay       bool
}

type weekData struct {
	WeekStart time.Time
	Days      []time.Time
	Events    map[int][]weekEvent
	AllDay    map[int][]weekEvent
	Backlog   []taskItem
	TaskByID  map[string]taskItem
}

type weekDataMsg struct {
	data      weekData
	fromCache bool
}

type calendarListMsg struct {
	items []simpleCalendar
}

type calendarItem struct {
	ID      string
	Name    string
	Primary bool
	Checked bool
}

func (c calendarItem) Title() string       { return c.Name }
func (c calendarItem) Description() string { return "" }
func (c calendarItem) FilterValue() string { return c.Name }

type calendarDelegate struct {
	list.DefaultDelegate
}

func (d calendarDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	cal, ok := item.(calendarItem)
	if !ok {
		return
	}
	checked := " "
	if cal.Checked {
		checked = "x"
	}
	label := fmt.Sprintf("[%s] %s", checked, cal.Name)
	if cal.Primary {
		label += " (primary)"
	}
	style := d.Styles.NormalTitle
	if index == m.Index() {
		style = d.Styles.SelectedTitle
	}
	fmt.Fprint(w, style.Render(label))
}

func newCalendarSelect(items []simpleCalendar, selected []string) list.Model {
	selectedMap := map[string]bool{}
	for _, id := range selected {
		selectedMap[id] = true
	}
	listItems := make([]list.Item, 0, len(items))
	for _, item := range items {
		listItems = append(listItems, calendarItem{
			ID:      item.ID,
			Name:    item.Title,
			Primary: item.Primary,
			Checked: selectedMap[item.ID],
		})
	}
	delegate := calendarDelegate{DefaultDelegate: list.NewDefaultDelegate()}
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(colorAccent).Bold(true)
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.Foreground(colorMuted)
	model := list.New(listItems, delegate, 0, 0)
	model.Title = "Select calendars"
	model.SetShowStatusBar(false)
	model.SetFilteringEnabled(true)
	model.SetShowHelp(true)
	model.KeyMap.Quit.SetEnabled(false)
	styles := model.Styles
	styles.Title = styles.Title.Foreground(colorAccent).Bold(true)
	styles.FilterPrompt = styles.FilterPrompt.Foreground(colorMuted)
	styles.FilterCursor = styles.FilterCursor.Foreground(colorAccent)
	styles.StatusBar = styles.StatusBar.Foreground(colorMuted)
	styles.PaginationStyle = styles.PaginationStyle.Foreground(colorMuted)
	styles.HelpStyle = styles.HelpStyle.Foreground(colorMuted)
	model.Styles = styles
	return model
}

func (m *tuiModel) loadCalendarListCmd() tea.Cmd {
	return func() tea.Msg {
		items, err := m.app.Calendar.ListCalendars()
		if err != nil {
			return errMsg{err: err}
		}
		cals := make([]simpleCalendar, 0, len(items))
		for _, cal := range items {
			cals = append(cals, simpleCalendar{Title: cal.Summary, ID: cal.Id, Primary: cal.Primary})
		}
		return calendarListMsg{items: cals}
	}
}

func (m *tuiModel) toggleCalendarSelection() {
	idx := m.calendarSelect.Index()
	if idx < 0 {
		return
	}
	items := m.calendarSelect.Items()
	if idx >= len(items) {
		return
	}
	cal, ok := items[idx].(calendarItem)
	if !ok {
		return
	}
	cal.Checked = !cal.Checked
	m.calendarSelect.SetItem(idx, cal)
}

func (m *tuiModel) saveCalendarSelectionCmd() tea.Cmd {
	return func() tea.Msg {
		items := m.calendarSelect.Items()
		selected := []string{}
		for _, item := range items {
			cal, ok := item.(calendarItem)
			if !ok {
				continue
			}
			if cal.Checked {
				selected = append(selected, cal.ID)
			}
		}
		if len(selected) == 0 {
			return errMsg{err: fmt.Errorf("select at least one calendar")}
		}
		m.app.Config.ViewCalendars = selected
		if err := m.app.SaveConfig(); err != nil {
			return errMsg{err: err}
		}
		return okMsg{msg: "✅ Calendars updated"}
	}
}

func (m *tuiModel) weekAnchor() time.Time {
	if !m.weekData.WeekStart.IsZero() {
		return m.weekData.WeekStart
	}
	return m.app.Now
}

func (m *tuiModel) shiftWeekDay(delta int) tea.Cmd {
	if m.weekDayIndex < 0 {
		m.weekDayIndex = 0
	}
	next := m.weekDayIndex + delta
	if next < 0 {
		m.weekDayIndex = 6
		m.weekLoading = true
		return m.loadWeekDataCmd(m.weekAnchor().AddDate(0, 0, -1))
	}
	if next > 6 {
		m.weekDayIndex = 0
		m.weekLoading = true
		return m.loadWeekDataCmd(m.weekAnchor().AddDate(0, 0, 7))
	}
	m.weekDayIndex = next
	m.ensureWeekSelection()
	return nil
}

func (m *tuiModel) moveWeekSelection(delta int) {
	if m.weekFocus == focusBacklog {
		count := len(m.weekData.Backlog)
		if count == 0 {
			m.weekBacklogIndex = -1
			return
		}
		next := m.weekBacklogIndex + delta
		if next < 0 {
			next = 0
		}
		if next >= count {
			next = count - 1
		}
		m.weekBacklogIndex = next
		return
	}
	events := m.weekData.Events[m.weekDayIndex]
	if len(events) == 0 {
		m.weekEventIndex = -1
		return
	}
	next := m.weekEventIndex + delta
	if next < 0 {
		next = 0
	}
	if next >= len(events) {
		next = len(events) - 1
	}
	m.weekEventIndex = next
}

func (m *tuiModel) ensureWeekSelection() {
	if len(m.weekData.Backlog) == 0 {
		m.weekBacklogIndex = -1
	} else if m.weekBacklogIndex < 0 || m.weekBacklogIndex >= len(m.weekData.Backlog) {
		m.weekBacklogIndex = 0
	}
	events := m.weekData.Events[m.weekDayIndex]
	if len(events) == 0 {
		m.weekEventIndex = -1
	} else if m.weekEventIndex < 0 || m.weekEventIndex >= len(events) {
		m.weekEventIndex = 0
	}
}

func (m *tuiModel) selectedWeekEvent() (weekEvent, bool) {
	events := m.weekData.Events[m.weekDayIndex]
	if len(events) == 0 || m.weekEventIndex < 0 || m.weekEventIndex >= len(events) {
		return weekEvent{}, false
	}
	return events[m.weekEventIndex], true
}

func (m *tuiModel) resolveTaskByID(taskID string) (taskItem, bool) {
	if taskID == "" {
		return taskItem{}, false
	}
	if task, ok := m.weekData.TaskByID[taskID]; ok {
		return task, true
	}
	if m.weekData.TaskByID == nil {
		m.weekData.TaskByID = map[string]taskItem{}
	}
	for listName, listID := range m.app.Config.Lists {
		task, err := m.app.Tasks.GetTask(listID, taskID)
		if err != nil {
			continue
		}
		section := "General"
		if task.Parent != "" {
			if parent, err := m.app.Tasks.GetTask(listID, task.Parent); err == nil && parent != nil {
				if title := strings.TrimSpace(parent.Title); title != "" {
					section = title
				}
			}
		}
		var due time.Time
		hasDue := false
		if task.Due != "" {
			if parsed, err := time.Parse(time.RFC3339, task.Due); err == nil {
				due = parsed.In(m.app.Location)
				hasDue = true
			}
		}
		item := taskItem{
			ID:       task.Id,
			TitleVal: task.Title,
			ListName: listName,
			ListID:   listID,
			Section:  section,
			Due:      due,
			HasDue:   hasDue,
		}
		m.weekData.TaskByID[taskID] = item
		return item, true
	}
	return taskItem{}, false
}

func (m *tuiModel) selectedWeekTask() (taskItem, bool) {
	if m.weekFocus == focusBacklog {
		if len(m.weekData.Backlog) == 0 || m.weekBacklogIndex < 0 || m.weekBacklogIndex >= len(m.weekData.Backlog) {
			return taskItem{}, false
		}
		return m.weekData.Backlog[m.weekBacklogIndex], true
	}
	event, ok := m.selectedWeekEvent()
	if !ok || event.TaskID == "" {
		return taskItem{}, false
	}
	return m.resolveTaskByID(event.TaskID)
}

func (m *tuiModel) completeWeekTaskCmd() tea.Cmd {
	task, ok := m.selectedWeekTask()
	if !ok {
		m.status = "Select a task to complete"
		return nil
	}
	m.listCtx = listCtxWeek
	return m.completeTaskCmd(task)
}

func (m *tuiModel) applyTaskToggle(taskID string, completed bool) {
	updateSummary := func(text string) string {
		if completed {
			if strings.HasPrefix(text, "✅ ") {
				return text
			}
			return "✅ " + strings.TrimSpace(text)
		}
		if strings.HasPrefix(text, "✅ ") {
			return strings.TrimPrefix(text, "✅ ")
		}
		if strings.HasPrefix(text, "✅") {
			return strings.TrimSpace(strings.TrimPrefix(text, "✅"))
		}
		return text
	}
	for dayIdx, events := range m.weekData.Events {
		for i := range events {
			if events[i].TaskID != taskID {
				continue
			}
			events[i].Summary = updateSummary(events[i].Summary)
		}
		m.weekData.Events[dayIdx] = events
	}
	for dayIdx, events := range m.weekData.AllDay {
		for i := range events {
			if events[i].TaskID != taskID {
				continue
			}
			events[i].Summary = updateSummary(events[i].Summary)
		}
		m.weekData.AllDay[dayIdx] = events
	}
}

func (m *tuiModel) editWeekTaskCmd() tea.Cmd {
	task, ok := m.selectedWeekTask()
	if !ok {
		m.status = "Select a task to edit"
		return nil
	}
	m.listCtx = listCtxWeek
	return m.beginEditTask(task)
}

func (m *tuiModel) prepareDeleteWeekTask() {
	task, ok := m.selectedWeekTask()
	if !ok {
		m.status = "Select a task to delete"
		return
	}
	m.listCtx = listCtxWeek
	m.prepareDeleteTask(task)
}

func (m *tuiModel) startNewTaskFromWeek() {
	m.state = stateTaskForm
	m.formMode = formNew
	m.formInputs = newTaskInputs()
	m.formStep = 1
	m.listCtx = listCtxWeek
	listName := m.app.Config.DefaultList
	if listName == "" {
		listName = m.listName
	}
	m.formInputs[0].SetValue(listName)
	m.formInputs[1].Focus()
	if len(m.weekData.Days) > 0 && m.weekDayIndex >= 0 && m.weekDayIndex < len(m.weekData.Days) {
		m.formInputs[3].SetValue(m.weekData.Days[m.weekDayIndex].Format("2006-01-02"))
	}
}

func (m *tuiModel) weekView() string {
	if m.weekLoading || len(m.weekData.Days) == 0 {
		return "Loading week..."
	}
	width := m.winW - 4
	if width < 50 {
		return "Terminal too narrow for week view"
	}
	leftWidth := width / 3
	if leftWidth < 24 {
		leftWidth = 24
	}
	if leftWidth > 40 {
		leftWidth = 40
	}
	rightWidth := width - leftWidth - 2
	bodyHeight := m.winH - 12
	if bodyHeight < 10 {
		bodyHeight = 10
	}
	left := m.renderBacklog(leftWidth, bodyHeight)
	right := m.renderWeekGrid(rightWidth, bodyHeight)
	joined := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	details := m.renderWeekDetails(width)
	return joined + "\n\n" + details
}

func (m *tuiModel) renderBacklog(width, height int) string {
	lines := []string{}
	title := fmt.Sprintf("Backlog (%d)", len(m.weekData.Backlog))
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render(title))
	lines = append(lines, "")
	available := height - len(lines)
	if available < 1 {
		available = 1
	}
	start, end := windowRange(len(m.weekData.Backlog), m.weekBacklogIndex, available)
	for i := start; i < end; i++ {
		task := m.weekData.Backlog[i]
		selected := m.weekFocus == focusBacklog && i == m.weekBacklogIndex
		line := task.TitleVal
		if task.HasDue {
			dueText := task.Due.Format("01-02")
			if selected {
				line = fmt.Sprintf("%s %s", line, dueText)
			} else {
				line = fmt.Sprintf("%s %s", line, gray(dueText))
			}
		}
		line = truncateText(line, width-2)
		if selected {
			line = lipgloss.NewStyle().Background(colorAccent).Foreground(lipgloss.Color("230")).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(colorMuted).Render(line)
		}
		lines = append(lines, line)
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func (m *tuiModel) renderWeekGrid(width, height int) string {
	if len(m.weekData.Days) == 0 {
		return "(no data)"
	}
	timeColWidth := 5
	gap := 1
	dayWidth := (width - timeColWidth - 7*gap) / 7
	if dayWidth < 8 {
		dayWidth = 8
	}
	headerLines := []string{}
	// Header
	headerCells := []string{strings.Repeat(" ", timeColWidth)}
	for i, day := range m.weekData.Days {
		label := day.Format("Mon 02")
		cell := lipgloss.NewStyle().Width(dayWidth).Render(label)
		if i == m.weekDayIndex {
			cell = lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Width(dayWidth).Render(label)
		}
		headerCells = append(headerCells, cell)
	}
	headerLines = append(headerLines, strings.Join(headerCells, strings.Repeat(" ", gap)))

	// All-day row
	if len(m.weekData.AllDay) > 0 {
		row := []string{padText("All", timeColWidth)}
		for dayIdx := range m.weekData.Days {
			items := m.weekData.AllDay[dayIdx]
			labels := []string{}
			for _, ev := range items {
				labels = append(labels, ev.Summary)
			}
			text := truncateText(strings.Join(labels, ", "), dayWidth)
			row = append(row, lipgloss.NewStyle().Width(dayWidth).Foreground(colorMuted).Render(text))
		}
		headerLines = append(headerLines, strings.Join(row, strings.Repeat(" ", gap)))
	}

	slots := 24
	selectedSlot := 9
	if m.app != nil {
		if base := m.app.Now; !base.IsZero() {
			if clock, err := timeparse.ParseClock(m.app.Config.WorkdayStart, base, m.app.Location); err == nil {
				if clock.Hour() >= 0 && clock.Hour() < slots {
					selectedSlot = clock.Hour()
				}
			}
		}
	}
	if ev, ok := m.selectedWeekEvent(); ok {
		selectedSlot = ev.StartSlot
	}
	timeLines := make([]string, 0, slots)
	for slot := 0; slot < slots; slot++ {
		hour := slot
		timeLabel := fmt.Sprintf("%02d:00", hour)
		row := []string{padText(timeLabel, timeColWidth)}
		for dayIdx := range m.weekData.Days {
			cell := m.renderWeekSlot(dayIdx, slot, dayWidth)
			row = append(row, cell)
		}
		timeLines = append(timeLines, strings.Join(row, strings.Repeat(" ", gap)))
	}
	if len(headerLines)+len(timeLines) <= height {
		return strings.Join(append(headerLines, timeLines...), "\n")
	}
	timeHeight := height - len(headerLines)
	if timeHeight < 1 {
		timeHeight = 1
	}
	start := selectedSlot - timeHeight/2
	if start < 0 {
		start = 0
	}
	if start+timeHeight > len(timeLines) {
		start = len(timeLines) - timeHeight
		if start < 0 {
			start = 0
		}
	}
	end := start + timeHeight
	if end > len(timeLines) {
		end = len(timeLines)
	}
	visible := append([]string{}, headerLines...)
	visible = append(visible, timeLines[start:end]...)
	return strings.Join(visible, "\n")
}

func (m *tuiModel) renderWeekSlot(dayIdx, slot, width int) string {
	events := m.weekData.Events[dayIdx]
	selected := m.weekFocus == focusGrid && dayIdx == m.weekDayIndex
	for idx, ev := range events {
		if slot < ev.StartSlot || slot >= ev.EndSlot {
			continue
		}
		text := "|"
		if slot == ev.StartSlot {
			text = truncateText(ev.Summary, width)
		}
		cell := lipgloss.NewStyle().Width(width).Render(text)
		if selected && idx == m.weekEventIndex {
			return lipgloss.NewStyle().Background(colorAccent).Foreground(lipgloss.Color("230")).Render(cell)
		}
		if ev.TaskID != "" {
			return lipgloss.NewStyle().Foreground(colorAccent).Render(cell)
		}
		return lipgloss.NewStyle().Foreground(colorMuted).Render(cell)
	}
	return lipgloss.NewStyle().Width(width).Render("")
}

func (m *tuiModel) renderWeekDetails(width int) string {
	lines := []string{}
	if m.weekFocus == focusGrid {
		if ev, ok := m.selectedWeekEvent(); ok {
			lines = append(lines, lipgloss.NewStyle().Bold(true).Render(ev.Summary))
			if !ev.AllDay {
				lines = append(lines, fmt.Sprintf("Time: %s - %s", ev.Start.Format("2006-01-02 15:04"), ev.End.Format("15:04")))
			} else {
				lines = append(lines, "All-day")
			}
			if ev.CalendarName != "" {
				lines = append(lines, fmt.Sprintf("Calendar: %s", ev.CalendarName))
			}
			if ev.TaskID != "" {
				if task, ok := m.resolveTaskByID(ev.TaskID); ok {
					lines = append(lines, fmt.Sprintf("List: %s", task.ListName))
					lines = append(lines, fmt.Sprintf("Section: %s", task.Section))
				}
			}
		}
	} else {
		if task, ok := m.selectedWeekTask(); ok {
			lines = append(lines, lipgloss.NewStyle().Bold(true).Render(task.TitleVal))
			lines = append(lines, fmt.Sprintf("List: %s", task.ListName))
			lines = append(lines, fmt.Sprintf("Section: %s", task.Section))
			if task.HasDue {
				lines = append(lines, fmt.Sprintf("Due: %s", task.Due.Format("2006-01-02")))
			}
		}
	}
	if len(lines) == 0 {
		lines = append(lines, gray("Select a task or event to see details"))
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func weekStartDate(day time.Time) time.Time {
	day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	weekday := int(day.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	offset := weekday - 1
	return day.AddDate(0, 0, -offset)
}

func dayIndex(days []time.Time, day time.Time) int {
	for i, d := range days {
		if sameDay(d, day) {
			return i
		}
	}
	return -1
}

func eventTimesWithAllDay(e *calendar.Event, loc *time.Location) (time.Time, time.Time, bool) {
	if e == nil || e.Start == nil || e.End == nil {
		return time.Time{}, time.Time{}, false
	}
	if e.Start.DateTime != "" && e.End.DateTime != "" {
		start, err := time.Parse(time.RFC3339, e.Start.DateTime)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		end, err := time.Parse(time.RFC3339, e.End.DateTime)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		return start.In(loc), end.In(loc), false
	}
	if e.Start.Date != "" && e.End.Date != "" {
		start, err := time.ParseInLocation("2006-01-02", e.Start.Date, loc)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		end, err := time.ParseInLocation("2006-01-02", e.End.Date, loc)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		return start, end, true
	}
	return time.Time{}, time.Time{}, false
}

func addAllDayEvent(target map[int][]weekEvent, days []time.Time, event weekEvent) {
	if event.Start.IsZero() || event.End.IsZero() {
		return
	}
	for i, day := range days {
		if !day.Before(event.Start) && day.Before(event.End) {
			target[i] = append(target[i], event)
		}
	}
}

func slotRange(start, end time.Time) (int, int) {
	startSlot := start.Hour()
	endSlot := end.Hour()
	if end.Minute() > 0 || end.Second() > 0 || end.Nanosecond() > 0 {
		endSlot++
	}
	if endSlot <= startSlot {
		endSlot = startSlot + 1
	}
	if startSlot < 0 {
		startSlot = 0
	}
	if endSlot > 24 {
		endSlot = 24
	}
	return startSlot, endSlot
}

func windowRange(total, index, height int) (int, int) {
	if height >= total {
		return 0, total
	}
	start := index - height/2
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > total {
		end = total
		start = end - height
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

func truncateText(text string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(text) <= max {
		return text
	}
	if max <= 3 {
		return text[:max]
	}
	return text[:max-3] + "..."
}

func padText(text string, width int) string {
	if len(text) >= width {
		return text[:width]
	}
	return text + strings.Repeat(" ", width-len(text))
}
