package cli

import (
	"testing"
	"time"

	"google.golang.org/api/tasks/v1"
)

func TestGroupTasksBySectionBuildsSectionIndexFirst(t *testing.T) {
	sectionID := "section-1"
	items := []*tasks.Task{
		{Id: "task-1", Title: "Follow up", Parent: sectionID},
		{Id: sectionID, Title: "Recurrentes", Notes: "justdoit_section=1"},
	}

	sections, order := groupTasksBySection(items, "", time.UTC)
	if len(sections) == 0 {
		t.Fatalf("expected sections, got none")
	}
	tasks := sections["Recurrentes"]
	if len(tasks) != 1 || tasks[0].Title != "Follow up" {
		t.Fatalf("expected task under section, got %#v", sections)
	}
	if len(order) == 0 || order[0] != "Recurrentes" {
		t.Fatalf("expected section order to include Recurrentes, got %#v", order)
	}
}
