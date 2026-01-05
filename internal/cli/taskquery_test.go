package cli

import (
	"reflect"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/tasks/v1"
)

type fakeTaskProvider struct {
	lists map[string][]*tasks.Task
}

func (f fakeTaskProvider) ListTasks(listID string, showCompleted bool) ([]*tasks.Task, error) {
	return f.lists[listID], nil
}

func (f fakeTaskProvider) ListTasksWithOptions(listID string, showCompleted, showHidden, showDeleted bool, updatedMin string) ([]*tasks.Task, error) {
	return f.lists[listID], nil
}

type fakeCalendarProvider struct {
	calendars []*calendar.CalendarListEntry
	events    map[string][]*calendar.Event
}

func (f fakeCalendarProvider) ListEvents(calendarID string, timeMin, timeMax string) ([]*calendar.Event, error) {
	return f.events[calendarID], nil
}

func (f fakeCalendarProvider) ListCalendars() ([]*calendar.CalendarListEntry, error) {
	return f.calendars, nil
}

func TestBuildNextItemsBuckets(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 1, 3, 10, 0, 0, 0, loc)
	listID := "list-1"

	items := []*tasks.Task{
		{Id: "1", Title: "Overdue", Status: "needsAction", Due: time.Date(2026, 1, 2, 12, 0, 0, 0, loc).Format(time.RFC3339)},
		{Id: "2", Title: "Today", Status: "needsAction", Due: time.Date(2026, 1, 3, 12, 0, 0, 0, loc).Format(time.RFC3339)},
		{Id: "3", Title: "This Week", Status: "needsAction", Due: time.Date(2026, 1, 4, 12, 0, 0, 0, loc).Format(time.RFC3339)},
		{Id: "4", Title: "Next Week", Status: "needsAction", Due: time.Date(2026, 1, 10, 12, 0, 0, 0, loc).Format(time.RFC3339)},
		{Id: "5", Title: "No Due", Status: "needsAction"},
	}

	ctx := queryContext{
		Tasks:    fakeTaskProvider{lists: map[string][]*tasks.Task{listID: items}},
		Lists:    map[string]string{"Work": listID},
		Location: loc,
		Now:      func() time.Time { return now },
	}

	itemsWithBacklog, err := buildNextItems(ctx, true)
	if err != nil {
		t.Fatalf("buildNextItems error: %v", err)
	}
	headers := extractHeaders(itemsWithBacklog)
	expected := []string{"Overdue", "Today", "This week", "Next week", "Backlog (no date)"}
	if !reflect.DeepEqual(headers, expected) {
		t.Fatalf("unexpected headers: %v", headers)
	}

	itemsNoBacklog, err := buildNextItems(ctx, false)
	if err != nil {
		t.Fatalf("buildNextItems error: %v", err)
	}
	headersNoBacklog := extractHeaders(itemsNoBacklog)
	for _, header := range headersNoBacklog {
		if header == "Backlog (no date)" {
			t.Fatalf("unexpected backlog header when showBacklog=false")
		}
	}
}

func TestSearchTasks(t *testing.T) {
	loc := time.UTC
	listID := "list-1"
	section := &tasks.Task{Id: "section-1", Title: "Projects", Notes: "justdoit_section=1"}
	task1 := &tasks.Task{Id: "1", Title: "Ship", Status: "needsAction", Parent: "section-1"}
	task2 := &tasks.Task{Id: "2", Title: "Misc", Status: "needsAction", Notes: "remember this"}
	task3 := &tasks.Task{Id: "3", Title: "Done", Status: "completed"}

	ctx := queryContext{
		Tasks:    fakeTaskProvider{lists: map[string][]*tasks.Task{listID: {section, task1, task2, task3}}},
		Lists:    map[string]string{"Work": listID},
		Location: loc,
		Now:      func() time.Time { return time.Date(2026, 1, 3, 10, 0, 0, 0, loc) },
	}

	results, err := searchTasks(ctx, "projects", "", false)
	if err != nil {
		t.Fatalf("searchTasks error: %v", err)
	}
	if len(results) != 1 || results[0].TitleVal != "Ship" {
		t.Fatalf("expected section match to return Ship, got %#v", results)
	}

	results, err = searchTasks(ctx, "remember", "", false)
	if err != nil {
		t.Fatalf("searchTasks error: %v", err)
	}
	if len(results) != 1 || results[0].TitleVal != "Misc" {
		t.Fatalf("expected notes match to return Misc, got %#v", results)
	}

	results, err = searchTasks(ctx, "done", "", false)
	if err != nil {
		t.Fatalf("searchTasks error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected completed task to be excluded, got %#v", results)
	}

	results, err = searchTasks(ctx, "done", "", true)
	if err != nil {
		t.Fatalf("searchTasks error: %v", err)
	}
	if len(results) != 1 || results[0].TitleVal != "Done" {
		t.Fatalf("expected completed task to be included, got %#v", results)
	}
}

func TestBuildNextItemsIncludesTodayEvents(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 1, 3, 10, 0, 0, 0, loc)
	listID := "list-1"
	calID := "cal-1"

	items := []*tasks.Task{
		{Id: "1", Title: "All Day Task", Status: "needsAction", Due: time.Date(2026, 1, 3, 0, 0, 0, 0, loc).Format(time.RFC3339)},
		{Id: "2", Title: "Timed Task", Status: "needsAction", Due: time.Date(2026, 1, 3, 15, 0, 0, 0, loc).Format(time.RFC3339)},
	}

	calendars := []*calendar.CalendarListEntry{
		{Id: calID, Summary: "Work"},
	}

	events := map[string][]*calendar.Event{
		calID: {
			{Id: "evt-2", Summary: "Standup", Start: &calendar.EventDateTime{DateTime: time.Date(2026, 1, 3, 9, 0, 0, 0, loc).Format(time.RFC3339)}, End: &calendar.EventDateTime{DateTime: time.Date(2026, 1, 3, 9, 30, 0, 0, loc).Format(time.RFC3339)}},
			{Id: "evt-1", Summary: "All Day", Start: &calendar.EventDateTime{Date: "2026-01-03"}, End: &calendar.EventDateTime{Date: "2026-01-04"}},
		},
	}

	ctx := queryContext{
		Tasks:         fakeTaskProvider{lists: map[string][]*tasks.Task{listID: items}},
		Calendar:      fakeCalendarProvider{calendars: calendars, events: events},
		Lists:         map[string]string{"Work": listID},
		ViewCalendars: []string{calID},
		Location:      loc,
		Now:           func() time.Time { return now },
	}

	got, err := buildNextItems(ctx, false)
	if err != nil {
		t.Fatalf("buildNextItems error: %v", err)
	}

	todayIdx := -1
	for i, item := range got {
		task, ok := item.(taskItem)
		if ok && task.IsHeader && task.TitleVal == "Today" {
			todayIdx = i
			break
		}
	}
	if todayIdx == -1 {
		t.Fatalf("missing Today header")
	}
	if len(got) <= todayIdx+4 {
		t.Fatalf("expected Today bucket to include events and tasks")
	}

	firstEvent, ok := got[todayIdx+1].(calendarEventItem)
	if !ok || !firstEvent.AllDay || firstEvent.Summary != "All Day" {
		t.Fatalf("expected all-day event first, got %#v", got[todayIdx+1])
	}
	secondTask, ok := got[todayIdx+2].(taskItem)
	if !ok || secondTask.TitleVal != "All Day Task" {
		t.Fatalf("expected all-day task second, got %#v", got[todayIdx+2])
	}
	thirdEvent, ok := got[todayIdx+3].(calendarEventItem)
	if !ok || thirdEvent.Summary != "Standup" {
		t.Fatalf("expected timed event third, got %#v", got[todayIdx+3])
	}
	if task, ok := got[todayIdx+4].(taskItem); !ok || task.TitleVal != "Timed Task" {
		t.Fatalf("expected timed task fourth, got %#v", got[todayIdx+4])
	}
}

func extractHeaders(items []list.Item) []string {
	headers := []string{}
	for _, item := range items {
		task, ok := item.(taskItem)
		if !ok || !task.IsHeader {
			continue
		}
		headers = append(headers, task.TitleVal)
	}
	return headers
}
