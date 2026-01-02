package sync

import (
	"fmt"
	"strings"
	"time"

	cal "justdoit/internal/google/calendar"
	tasksapi "justdoit/internal/google/tasks"
	"justdoit/internal/metadata"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/tasks/v1"
)

const (
	EventTaskIDKey = "justdoit_task_id"
	TaskEventIDKey = "justdoit_event_id"
)

type Wrapper struct {
	Tasks      *tasksapi.Client
	Calendar   *cal.Client
	CalendarID string
}

type CreateInput struct {
	ListID      string
	Title       string
	Notes       string
	Due         *time.Time
	Recurrence  []string
	RepeatEvent bool
	TimeStart   *time.Time
	TimeEnd     *time.Time
	ParentID    string
}

func (w *Wrapper) Create(input CreateInput) (*tasks.Task, *calendar.Event, error) {
	if input.ListID == "" {
		return nil, nil, fmt.Errorf("listID is required")
	}
	task := &tasks.Task{
		Title: input.Title,
		Notes: input.Notes,
	}
	if input.Due != nil {
		task.Due = input.Due.Format(time.RFC3339)
	}
	if len(input.Recurrence) > 0 {
		task.Notes = metadata.Append(task.Notes, "justdoit_rrule", strings.Join(input.Recurrence, ";"))
	}
	var (
		createdTask *tasks.Task
		err         error
	)
	if input.ParentID != "" {
		createdTask, err = w.Tasks.CreateTaskWithParent(input.ListID, task, input.ParentID)
	} else {
		createdTask, err = w.Tasks.CreateTask(input.ListID, task)
	}
	if err != nil {
		return nil, nil, err
	}

	if input.TimeStart == nil || input.TimeEnd == nil {
		return createdTask, nil, nil
	}

	event := &calendar.Event{
		Summary:     input.Title,
		Description: metadata.Append("", EventTaskIDKey, createdTask.Id),
		Start: &calendar.EventDateTime{
			DateTime: input.TimeStart.Format(time.RFC3339),
		},
		End: &calendar.EventDateTime{
			DateTime: input.TimeEnd.Format(time.RFC3339),
		},
	}
	if len(input.Recurrence) > 0 && input.RepeatEvent {
		event.Recurrence = input.Recurrence
	}
	createdEvent, err := w.Calendar.CreateEvent(w.CalendarID, event)
	if err != nil {
		return createdTask, nil, err
	}

	createdTask.Notes = metadata.Append(createdTask.Notes, TaskEventIDKey, createdEvent.Id)
	if _, err := w.Tasks.UpdateTask(input.ListID, createdTask); err != nil {
		return createdTask, createdEvent, err
	}

	return createdTask, createdEvent, nil
}

func ExtractMetadata(text, key string) (string, bool) {
	return metadata.Extract(text, key)
}
