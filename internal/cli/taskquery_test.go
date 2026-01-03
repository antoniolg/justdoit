package cli

import (
	"reflect"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
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
