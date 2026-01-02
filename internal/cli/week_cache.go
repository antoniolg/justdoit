package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"google.golang.org/api/calendar/v3"

	"justdoit/internal/cache"
	"justdoit/internal/metadata"
	"justdoit/internal/sync"
)

func (m *tuiModel) loadWeekDataCmd(base time.Time) tea.Cmd {
	return tea.Batch(m.loadWeekCacheCmd(base), m.refreshWeekDataCmd(base))
}

func (m *tuiModel) loadWeekCacheCmd(base time.Time) tea.Cmd {
	return func() tea.Msg {
		cachePath := m.app.CachePath
		if cachePath == "" {
			return nil
		}
		c, err := cache.Load(cachePath)
		if err != nil {
			return nil
		}
		weekStart := weekStartDate(base.In(m.app.Location))
		data, ok := buildWeekDataFromCache(m.app, c, weekStart)
		if !ok {
			return nil
		}
		return weekDataMsg{data: data, fromCache: true}
	}
}

func (m *tuiModel) refreshWeekDataCmd(base time.Time) tea.Cmd {
	return func() tea.Msg {
		cachePath := m.app.CachePath
		if cachePath == "" {
			return errMsg{err: fmt.Errorf("cache path not available")}
		}
		c, err := cache.Load(cachePath)
		if err != nil {
			return errMsg{err: err}
		}
		if err := syncCalendars(m.app, c); err != nil {
			return errMsg{err: err}
		}
		if err := syncTasks(m.app, c); err != nil {
			return errMsg{err: err}
		}
		c.SyncedAt = time.Now().Format(time.RFC3339)
		if err := cache.Save(cachePath, c); err != nil {
			return errMsg{err: err}
		}
		weekStart := weekStartDate(base.In(m.app.Location))
		data, ok := buildWeekDataFromCache(m.app, c, weekStart)
		if !ok {
			return errMsg{err: fmt.Errorf("no week data available")}
		}
		return weekDataMsg{data: data, fromCache: false}
	}
}

func buildWeekDataFromCache(app *App, c *cache.Cache, weekStart time.Time) (weekData, bool) {
	if c == nil {
		return weekData{}, false
	}
	if strings.TrimSpace(c.SyncedAt) == "" {
		return weekData{}, false
	}
	weekEnd := weekStart.AddDate(0, 0, 7)
	days := make([]time.Time, 0, 7)
	for i := 0; i < 7; i++ {
		days = append(days, weekStart.AddDate(0, 0, i))
	}

	tasks, taskByID := collectTasksFromCache(app, c, weekStart, weekEnd)

	calendarNames := map[string]string{}
	for id, meta := range c.CalendarMeta {
		calendarNames[id] = meta.Name
	}

	eventsByDay := map[int][]weekEvent{}
	allDayByDay := map[int][]weekEvent{}
	taskHasEvent := map[string]bool{}

	for _, calendarID := range app.Config.ViewCalendars {
		cal := c.Calendars[calendarID]
		if cal == nil {
			continue
		}
		for _, e := range cal.Events {
			if e == nil {
				continue
			}
			if strings.EqualFold(e.Status, "cancelled") {
				continue
			}
			start, end, allDay := eventTimesWithAllDay(e, app.Location)
			if start.IsZero() || end.IsZero() {
				continue
			}
			if end.Before(weekStart) || !start.Before(weekEnd) {
				continue
			}
			name := calendarNames[calendarID]
			taskID, _ := sync.ExtractMetadata(e.Description, sync.EventTaskIDKey)
			if taskID != "" {
				taskHasEvent[taskID] = true
			}
			event := weekEvent{
				Summary:      e.Summary,
				CalendarID:   calendarID,
				CalendarName: name,
				TaskID:       taskID,
				Start:        start,
				End:          end,
				AllDay:       allDay,
			}
			if allDay {
				addAllDayEvent(allDayByDay, days, event)
				continue
			}
			idx := dayIndex(days, start)
			if idx < 0 || idx > 6 {
				continue
			}
			event.StartSlot, event.EndSlot = slotRange(start, end)
			eventsByDay[idx] = append(eventsByDay[idx], event)
		}
	}

	for dayIdx, events := range eventsByDay {
		sort.SliceStable(events, func(i, j int) bool {
			return events[i].Start.Before(events[j].Start)
		})
		eventsByDay[dayIdx] = events
	}

	backlog := make([]taskItem, 0, len(tasks))
	for _, task := range tasks {
		if taskHasEvent[task.ID] {
			continue
		}
		backlog = append(backlog, task)
	}
	sort.SliceStable(backlog, func(i, j int) bool {
		if backlog[i].HasDue && backlog[j].HasDue {
			if backlog[i].Due.Equal(backlog[j].Due) {
				return backlog[i].TitleVal < backlog[j].TitleVal
			}
			return backlog[i].Due.Before(backlog[j].Due)
		}
		if backlog[i].HasDue {
			return true
		}
		if backlog[j].HasDue {
			return false
		}
		return backlog[i].TitleVal < backlog[j].TitleVal
	})
	return weekData{
		WeekStart: weekStart,
		Days:      days,
		Events:    eventsByDay,
		AllDay:    allDayByDay,
		Backlog:   backlog,
		TaskByID:  taskByID,
	}, true
}

func collectTasksFromCache(app *App, c *cache.Cache, start, end time.Time) ([]taskItem, map[string]taskItem) {
	var result []taskItem
	byID := map[string]taskItem{}
	if c == nil || c.Tasks == nil {
		return result, byID
	}
	for listName, listID := range app.Config.Lists {
		items := c.Tasks.Lists[listID]
		if items == nil {
			continue
		}
		sections := map[string]string{}
		for _, entry := range items {
			if _, ok := metadata.Extract(entry.Notes, "justdoit_section"); ok {
				sections[entry.ID] = entry.Title
			}
		}
		for _, entry := range items {
			section := "General"
			if parent, ok := sections[entry.Parent]; ok {
				section = parent
			}
			hasDue := false
			var due time.Time
			if entry.Due != "" {
				if parsed, err := time.Parse(time.RFC3339, entry.Due); err == nil {
					due = parsed.In(app.Location)
					hasDue = true
				}
			}
			item := taskItem{
				ID:       entry.ID,
				TitleVal: entry.Title,
				ListName: listName,
				ListID:   listID,
				Section:  section,
				Due:      due,
				HasDue:   hasDue,
			}
			byID[item.ID] = item
			if !hasDue {
				continue
			}
			if due.Before(start) || !due.Before(end) {
				continue
			}
			if strings.EqualFold(entry.Status, "completed") {
				continue
			}
			result = append(result, item)
		}
	}
	return result, byID
}

func syncCalendars(app *App, c *cache.Cache) error {
	if c == nil {
		return nil
	}
	items, err := app.Calendar.ListCalendars()
	if err != nil {
		return err
	}
	c.CalendarMeta = map[string]cache.CalendarMeta{}
	for _, cal := range items {
		c.CalendarMeta[cal.Id] = cache.CalendarMeta{Name: cal.Summary, Primary: cal.Primary}
	}
	if c.Calendars == nil {
		c.Calendars = map[string]*cache.CalendarCache{}
	}
	for _, calendarID := range app.Config.ViewCalendars {
		calCache := c.Calendars[calendarID]
		if calCache == nil {
			calCache = &cache.CalendarCache{Events: map[string]*calendar.Event{}}
			c.Calendars[calendarID] = calCache
		}
		var events []*calendar.Event
		var syncToken string
		if calCache.SyncToken == "" {
			events, syncToken, err = app.Calendar.ListAllEvents(calendarID)
		} else {
			events, syncToken, err = app.Calendar.SyncEvents(calendarID, calCache.SyncToken)
			if err != nil {
				events, syncToken, err = app.Calendar.ListAllEvents(calendarID)
			}
		}
		if err != nil {
			return err
		}
		if calCache.Events == nil {
			calCache.Events = map[string]*calendar.Event{}
		}
		for _, ev := range events {
			if ev == nil {
				continue
			}
			if strings.EqualFold(ev.Status, "cancelled") {
				delete(calCache.Events, ev.Id)
				continue
			}
			calCache.Events[ev.Id] = ev
		}
		if syncToken != "" {
			calCache.SyncToken = syncToken
		}
	}
	return nil
}

func syncTasks(app *App, c *cache.Cache) error {
	if c == nil {
		return nil
	}
	if c.Tasks == nil {
		c.Tasks = &cache.TasksCache{Lists: map[string]map[string]cache.TaskEntry{}}
	}
	updatedMin := c.Tasks.UpdatedMin
	for _, listID := range app.Config.Lists {
		items, err := app.Tasks.ListTasksWithOptions(listID, true, true, true, updatedMin)
		if err != nil {
			return err
		}
		if c.Tasks.Lists[listID] == nil {
			c.Tasks.Lists[listID] = map[string]cache.TaskEntry{}
		}
		for _, t := range items {
			if t == nil {
				continue
			}
			if t.Deleted {
				delete(c.Tasks.Lists[listID], t.Id)
				continue
			}
			c.Tasks.Lists[listID][t.Id] = cache.TaskEntry{
				ID:      t.Id,
				Title:   t.Title,
				Notes:   t.Notes,
				Parent:  t.Parent,
				Status:  t.Status,
				Due:     t.Due,
				Updated: t.Updated,
			}
		}
	}
	c.Tasks.UpdatedMin = time.Now().Add(-1 * time.Minute).Format(time.RFC3339)
	return nil
}
