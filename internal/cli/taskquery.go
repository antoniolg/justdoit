package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/tasks/v1"

	"justdoit/internal/metadata"
)

type TaskProvider interface {
	ListTasks(listID string, showCompleted bool) ([]*tasks.Task, error)
	ListTasksWithOptions(listID string, showCompleted, showHidden, showDeleted bool, updatedMin string) ([]*tasks.Task, error)
}

type CalendarProvider interface {
	ListEvents(calendarID string, timeMin, timeMax string) ([]*calendar.Event, error)
	ListCalendars() ([]*calendar.CalendarListEntry, error)
}

type queryContext struct {
	Tasks         TaskProvider
	Calendar      CalendarProvider
	Lists         map[string]string
	ViewCalendars []string
	Location      *time.Location
	Now           func() time.Time
}

func newQueryContext(app *App) queryContext {
	now := func() time.Time {
		if app == nil || app.Location == nil {
			return time.Now()
		}
		return time.Now().In(app.Location)
	}
	if app == nil {
		return queryContext{Now: now}
	}
	return queryContext{
		Tasks:         app.Tasks,
		Calendar:      app.Calendar,
		Lists:         app.Config.Lists,
		ViewCalendars: app.Config.ViewCalendars,
		Location:      app.Location,
		Now:           now,
	}
}

func buildNextItems(ctx queryContext, showBacklog bool) ([]list.Item, error) {
	if ctx.Tasks == nil {
		return nil, fmt.Errorf("task client is not initialized")
	}
	if ctx.Location == nil {
		ctx.Location = time.Local
	}
	if ctx.Now == nil {
		ctx.Now = func() time.Time { return time.Now().In(ctx.Location) }
	}
	now := ctx.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, ctx.Location)
	todayEnd := todayStart.AddDate(0, 0, 1)
	weekStart := weekStartDate(todayStart)
	weekEnd := weekStart.AddDate(0, 0, 7)
	nextWeekEnd := weekEnd.AddDate(0, 0, 7)

	todayEvents := []calendarEventItem{}
	if ctx.Calendar != nil && len(ctx.ViewCalendars) > 0 {
		calendarNames := map[string]string{}
		if items, err := ctx.Calendar.ListCalendars(); err == nil {
			for _, cal := range items {
				calendarNames[cal.Id] = cal.Summary
			}
		}
		timeMin := todayStart.Format(time.RFC3339)
		timeMax := todayEnd.Format(time.RFC3339)
		for _, calendarID := range ctx.ViewCalendars {
			events, err := ctx.Calendar.ListEvents(calendarID, timeMin, timeMax)
			if err != nil {
				return nil, err
			}
			for _, e := range events {
				if e == nil || strings.EqualFold(e.Status, "cancelled") {
					continue
				}
				start, end, allDay := eventTimesWithAllDay(e, ctx.Location)
				if start.IsZero() || end.IsZero() {
					continue
				}
				name := calendarNames[calendarID]
				if strings.TrimSpace(name) == "" {
					name = calendarID
				}
				todayEvents = append(todayEvents, calendarEventItem{
					ID:           e.Id,
					Summary:      e.Summary,
					CalendarID:   calendarID,
					CalendarName: name,
					Start:        start,
					End:          end,
					AllDay:       allDay,
				})
			}
		}
	}

	type bucket struct {
		name  string
		tasks []taskItem
	}
	buckets := []bucket{
		{name: "Overdue"},
		{name: "Today"},
		{name: "This week"},
		{name: "Next week"},
	}
	var backlog []taskItem

	listNames := sortedListNames(ctx.Lists)
	if len(listNames) == 0 {
		return nil, fmt.Errorf("no lists configured")
	}

	for _, listName := range listNames {
		listID := ctx.Lists[listName]
		items, err := ctx.Tasks.ListTasks(listID, false)
		if err != nil {
			return nil, err
		}
		sections := buildSectionIndex(items)
		for _, item := range items {
			if item == nil || item.Status == "completed" {
				continue
			}
			if isSectionTask(item) {
				continue
			}
			section := resolveSectionName(item, sections)
			if item.Due == "" {
				if showBacklog {
					rule, _ := metadata.Extract(item.Notes, "justdoit_rrule")
					backlog = append(backlog, taskItem{
						ID:         item.Id,
						TitleVal:   item.Title,
						ListName:   listName,
						ListID:     listID,
						Section:    section,
						HasDue:     false,
						Recurrence: rule,
					})
				}
				continue
			}
			due, hasDue, hasTime := parseTaskDue(item.Due, ctx.Location)
			if !hasDue {
				continue
			}
			rule, _ := metadata.Extract(item.Notes, "justdoit_rrule")
			row := taskItem{
				ID:         item.Id,
				TitleVal:   item.Title,
				ListName:   listName,
				ListID:     listID,
				Section:    section,
				Due:        due,
				HasDue:     true,
				HasTime:    hasTime,
				Recurrence: rule,
			}

			switch {
			case due.Before(todayStart):
				buckets[0].tasks = append(buckets[0].tasks, row)
			case due.Before(todayEnd):
				buckets[1].tasks = append(buckets[1].tasks, row)
			case due.Before(weekEnd):
				buckets[2].tasks = append(buckets[2].tasks, row)
			case due.Before(nextWeekEnd):
				buckets[3].tasks = append(buckets[3].tasks, row)
			}
		}
	}

	items := []list.Item{}
	for _, b := range buckets {
		sort.SliceStable(b.tasks, func(i, j int) bool {
			if b.tasks[i].Due.Equal(b.tasks[j].Due) {
				if b.tasks[i].ListName == b.tasks[j].ListName {
					return b.tasks[i].TitleVal < b.tasks[j].TitleVal
				}
				return b.tasks[i].ListName < b.tasks[j].ListName
			}
			return b.tasks[i].Due.Before(b.tasks[j].Due)
		})
		if b.name == "Today" {
			if len(b.tasks) == 0 && len(todayEvents) == 0 {
				continue
			}
			type todayEntry struct {
				item  list.Item
				timed bool
				start time.Time
				kind  int
				label string
			}
			entries := make([]todayEntry, 0, len(todayEvents)+len(b.tasks))
			for _, e := range todayEvents {
				entries = append(entries, todayEntry{
					item:  e,
					timed: !e.AllDay,
					start: e.Start,
					kind:  0,
					label: e.Summary,
				})
			}
			for _, t := range b.tasks {
				entries = append(entries, todayEntry{
					item:  t,
					timed: t.HasTime,
					start: t.Due,
					kind:  1,
					label: t.TitleVal,
				})
			}
			sort.SliceStable(entries, func(i, j int) bool {
				if entries[i].timed != entries[j].timed {
					return !entries[i].timed
				}
				if entries[i].timed {
					if !entries[i].start.Equal(entries[j].start) {
						return entries[i].start.Before(entries[j].start)
					}
				}
				if entries[i].kind != entries[j].kind {
					return entries[i].kind < entries[j].kind
				}
				return entries[i].label < entries[j].label
			})
			items = append(items, taskItem{TitleVal: b.name, IsHeader: true})
			for _, entry := range entries {
				items = append(items, entry.item)
			}
			continue
		}
		if len(b.tasks) == 0 {
			continue
		}
		items = append(items, taskItem{TitleVal: b.name, IsHeader: true})
		for _, t := range b.tasks {
			items = append(items, t)
		}
	}
	if len(items) == 0 {
		items = append(items, taskItem{TitleVal: "(no pending tasks)", IsHeader: true})
	}
	if showBacklog && len(backlog) > 0 {
		sort.SliceStable(backlog, func(i, j int) bool {
			if backlog[i].ListName == backlog[j].ListName {
				return backlog[i].TitleVal < backlog[j].TitleVal
			}
			return backlog[i].ListName < backlog[j].ListName
		})
		items = append(items, taskItem{TitleVal: "Backlog (no date)", IsHeader: true})
		for _, t := range backlog {
			items = append(items, t)
		}
	}
	return items, nil
}

func searchTasks(ctx queryContext, query, listFilter string, includeCompleted bool) ([]taskItem, error) {
	if ctx.Tasks == nil {
		return nil, fmt.Errorf("task client is not initialized")
	}
	if ctx.Location == nil {
		ctx.Location = time.Local
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return nil, fmt.Errorf("query is required")
	}

	listMap := map[string]string{}
	listFilter = strings.TrimSpace(listFilter)
	if listFilter != "" {
		if id, ok := ctx.Lists[listFilter]; ok {
			listMap[listFilter] = id
		} else {
			listMap[listFilter] = listFilter
		}
	} else {
		for name, id := range ctx.Lists {
			listMap[name] = id
		}
	}
	if len(listMap) == 0 {
		return nil, fmt.Errorf("no lists configured")
	}

	listNames := make([]string, 0, len(listMap))
	for name := range listMap {
		listNames = append(listNames, name)
	}
	sort.Strings(listNames)

	results := []taskItem{}
	for _, listName := range listNames {
		listID := listMap[listName]
		items, err := ctx.Tasks.ListTasksWithOptions(listID, true, true, false, "")
		if err != nil {
			return nil, err
		}
		sections := buildSectionIndex(items)
		for _, item := range items {
			if item == nil || isSectionTask(item) {
				continue
			}
			if !includeCompleted && item.Status == "completed" {
				continue
			}
			section := resolveSectionName(item, sections)
			if !matchesSearch(needle, item, section) {
				continue
			}
			due, hasDue, hasTime := parseTaskDue(item.Due, ctx.Location)
			recurrence := ""
			if rule, ok := metadata.Extract(item.Notes, "justdoit_rrule"); ok {
				recurrence = rule
			}
			results = append(results, taskItem{
				ID:         item.Id,
				TitleVal:   item.Title,
				ListName:   listName,
				ListID:     listID,
				Section:    section,
				Due:        due,
				HasDue:     hasDue,
				HasTime:    hasTime,
				Recurrence: recurrence,
			})
		}
	}

	sortSearchResults(results)
	return results, nil
}

func sortedListNames(lists map[string]string) []string {
	names := make([]string, 0, len(lists))
	for name := range lists {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func buildSectionIndex(items []*tasks.Task) map[string]string {
	sections := map[string]string{}
	for _, item := range items {
		if isSectionTask(item) {
			sections[item.Id] = strings.TrimSpace(item.Title)
		}
	}
	return sections
}

func resolveSectionName(item *tasks.Task, sections map[string]string) string {
	if item == nil {
		return "General"
	}
	if section, ok := sections[item.Parent]; ok && section != "" {
		return section
	}
	return "General"
}

func isSectionTask(item *tasks.Task) bool {
	if item == nil {
		return false
	}
	_, ok := metadata.Extract(item.Notes, "justdoit_section")
	return ok
}

func matchesSearch(query string, item *tasks.Task, section string) bool {
	if item == nil {
		return false
	}
	title := strings.ToLower(item.Title)
	notes := strings.ToLower(stripMetadataNotes(item.Notes))
	section = strings.ToLower(section)
	return strings.Contains(title, query) || strings.Contains(notes, query) || strings.Contains(section, query)
}

func sortSearchResults(results []taskItem) {
	sort.SliceStable(results, func(i, j int) bool {
		a := results[i]
		b := results[j]
		if a.HasDue != b.HasDue {
			return a.HasDue
		}
		if a.HasDue && b.HasDue {
			if !a.Due.Equal(b.Due) {
				return a.Due.Before(b.Due)
			}
		}
		if a.ListName != b.ListName {
			return a.ListName < b.ListName
		}
		return strings.ToLower(a.TitleVal) < strings.ToLower(b.TitleVal)
	})
}
