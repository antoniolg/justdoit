package cli

import (
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/tasks/v1"

	"justdoit/internal/metadata"
	"justdoit/internal/recurrence"
	"justdoit/internal/sync"
	"justdoit/internal/timeparse"
)

type UpdateParams struct {
	Title      string
	HasTitle   bool
	Notes      string
	HasNotes   bool
	Section    string
	HasSection bool
	Date       string
	HasDate    bool
	Time       string
	HasTime    bool
}

type UpdateResult struct {
	SectionChanged bool
	EventUpdated   bool
	EventRenamed   bool
}

func updateTaskWithParams(app *App, listID, taskID string, params UpdateParams) (UpdateResult, error) {
	var result UpdateResult
	if listID == "" || taskID == "" {
		return result, fmt.Errorf("listID and taskID are required")
	}

	task, err := app.Tasks.GetTask(listID, taskID)
	if err != nil {
		return result, err
	}

	if params.HasTitle && params.Title != "" {
		task.Title = params.Title
	}

	if params.HasNotes {
		task.Notes = mergeNotes(params.Notes, task.Notes)
	}

	var (
		event       *calendar.Event
		eventExists bool
		newStart    *time.Time
		newEnd      *time.Time
		newDue      *time.Time
	)

	if params.HasSection {
		sectionName := strings.TrimSpace(params.Section)
		if sectionName == "" {
			if _, err := app.Tasks.MoveTask(listID, taskID, ""); err != nil {
				return result, err
			}
			result.SectionChanged = true
		} else {
			sectionTask, err := ensureSectionTask(app, listID, sectionName)
			if err != nil {
				return result, err
			}
			if _, err := app.Tasks.MoveTask(listID, taskID, sectionTask.Id); err != nil {
				return result, err
			}
			result.SectionChanged = true
		}
	}

	if params.HasTime || params.HasDate || params.HasTitle {
		event, eventExists, _ = findLinkedEvent(app, task)
	}

	if params.HasTime || params.HasDate {
		baseDate := resolveBaseDate(app, task, event, params.Date)
		if params.HasTime {
			start, end, err := timeparse.ParseTimeRange(params.Time, baseDate, app.Now, app.Location)
			if err != nil {
				return result, err
			}
			newStart = &start
			newEnd = &end
			newDue = &end
		} else if params.HasDate {
			if baseDate.IsZero() {
				return result, fmt.Errorf("invalid date")
			}
			endOfDay := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 23, 59, 0, 0, app.Location)
			newDue = &endOfDay
			if eventExists {
				start, end := eventTimes(event, app.Location)
				if !start.IsZero() && !end.IsZero() {
					start = time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), start.Hour(), start.Minute(), 0, 0, app.Location)
					end = time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), end.Hour(), end.Minute(), 0, 0, app.Location)
					newStart = &start
					newEnd = &end
				}
			}
		}
	}

	if newDue != nil {
		task.Due = newDue.Format(time.RFC3339)
	}

	if _, err := app.Tasks.UpdateTask(listID, task); err != nil {
		return result, err
	}

	if params.HasTitle && params.Title != "" && eventExists && event != nil {
		event.Summary = params.Title
		result.EventRenamed = true
	}

	if newStart != nil && newEnd != nil {
		if eventExists && event != nil {
			event.Start.DateTime = newStart.Format(time.RFC3339)
			event.End.DateTime = newEnd.Format(time.RFC3339)
			if _, err := app.Calendar.UpdateEvent(app.Config.CalendarID, event); err != nil {
				return result, err
			}
			result.EventUpdated = true
		} else {
			created, err := createLinkedEvent(app, task, newStart, newEnd)
			if err != nil {
				return result, err
			}
			updatedNotes := metadata.Append(task.Notes, sync.TaskEventIDKey, created.Id)
			task.Notes = updatedNotes
			if _, err := app.Tasks.UpdateTask(listID, task); err != nil {
				return result, err
			}
			result.EventUpdated = true
		}
	} else if result.EventRenamed && eventExists && event != nil {
		if _, err := app.Calendar.UpdateEvent(app.Config.CalendarID, event); err != nil {
			return result, err
		}
	}

	return result, nil
}

func markTaskDone(app *App, listID, taskID string, markEvent bool) error {
	task, err := app.Tasks.GetTask(listID, taskID)
	if err != nil {
		return err
	}
	event, _, _ := findLinkedEvent(app, task)
	if _, err := app.Tasks.CompleteTask(listID, taskID); err != nil {
		return err
	}
	if !markEvent {
		return createNextRecurringTask(app, listID, task, event)
	}
	if err := updateLinkedEventPrefix(app, task, true); err != nil {
		return err
	}
	return createNextRecurringTask(app, listID, task, event)
}

func deleteTask(app *App, listID, taskID string, deleteEvent bool) error {
	task, err := app.Tasks.GetTask(listID, taskID)
	if err != nil {
		return err
	}
	if deleteEvent {
		if eventID, ok := sync.ExtractMetadata(task.Notes, sync.TaskEventIDKey); ok {
			_ = app.Calendar.DeleteEvent(app.Config.CalendarID, eventID)
		} else if event, err := app.Calendar.FindEventByTaskID(app.Config.CalendarID, taskID); err == nil {
			_ = app.Calendar.DeleteEvent(app.Config.CalendarID, event.Id)
		}
	}
	return app.Tasks.DeleteTask(listID, taskID)
}

func toggleTaskDone(app *App, listID, taskID string, markEvent bool) (bool, error) {
	task, err := app.Tasks.GetTask(listID, taskID)
	if err != nil {
		return false, err
	}
	event, _, _ := findLinkedEvent(app, task)
	completed := strings.EqualFold(task.Status, "completed")
	if completed {
		if _, err := app.Tasks.UncompleteTask(listID, taskID); err != nil {
			return false, err
		}
		if markEvent {
			_ = updateLinkedEventPrefix(app, task, false)
		}
		return false, nil
	}
	if _, err := app.Tasks.CompleteTask(listID, taskID); err != nil {
		return false, err
	}
	if markEvent {
		_ = updateLinkedEventPrefix(app, task, true)
	}
	if err := createNextRecurringTask(app, listID, task, event); err != nil {
		return true, err
	}
	return true, nil
}

func updateLinkedEventPrefix(app *App, task *tasks.Task, completed bool) error {
	event, ok, _ := findLinkedEvent(app, task)
	if !ok || event == nil {
		return nil
	}
	if completed {
		if !strings.HasPrefix(event.Summary, "✅ ") {
			event.Summary = "✅ " + event.Summary
			_, _ = app.Calendar.UpdateEvent(app.Config.CalendarID, event)
		}
		return nil
	}
	if strings.HasPrefix(event.Summary, "✅ ") {
		event.Summary = strings.TrimPrefix(event.Summary, "✅ ")
		_, _ = app.Calendar.UpdateEvent(app.Config.CalendarID, event)
		return nil
	}
	if strings.HasPrefix(event.Summary, "✅") {
		event.Summary = strings.TrimPrefix(event.Summary, "✅")
		event.Summary = strings.TrimSpace(event.Summary)
		_, _ = app.Calendar.UpdateEvent(app.Config.CalendarID, event)
	}
	return nil
}

func stripMetadataNotes(notes string) string {
	lines := strings.Split(notes, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, sync.TaskEventIDKey+"=") {
			continue
		}
		if strings.HasPrefix(trim, sync.EventTaskIDKey+"=") {
			continue
		}
		if strings.HasPrefix(trim, "justdoit_rrule=") {
			continue
		}
		if strings.HasPrefix(trim, "justdoit_section=") {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.TrimSpace(strings.Join(filtered, "\n"))
}

func createNextRecurringTask(app *App, listID string, task *tasks.Task, event *calendar.Event) error {
	if task == nil || app == nil {
		return nil
	}
	rule, ok := metadata.Extract(task.Notes, "justdoit_rrule")
	if !ok || strings.TrimSpace(rule) == "" {
		return nil
	}

	baseStart, duration := taskTiming(app, task, event)
	nextStart, ok, err := recurrence.NextOccurrence(rule, baseStart, app.Now, app.Location)
	if err != nil || !ok || nextStart.IsZero() {
		return err
	}

	var (
		start *time.Time
		end   *time.Time
		due   *time.Time
	)
	if duration > 0 {
		startVal := nextStart
		endVal := nextStart.Add(duration)
		start = &startVal
		end = &endVal
		due = &endVal
	} else {
		endOfDay := time.Date(nextStart.Year(), nextStart.Month(), nextStart.Day(), 23, 59, 0, 0, app.Location)
		due = &endOfDay
	}

	notes := stripMetadataNotes(task.Notes)
	input := sync.CreateInput{
		ListID:     listID,
		Title:      task.Title,
		Notes:      notes,
		Due:        due,
		Recurrence: []string{rule},
		TimeStart:  start,
		TimeEnd:    end,
		ParentID:   task.Parent,
	}
	if event != nil && len(event.Recurrence) > 0 {
		input.TimeStart = nil
		input.TimeEnd = nil
	}
	_, _, err = app.Sync.Create(input)
	return err
}

func taskTiming(app *App, task *tasks.Task, event *calendar.Event) (time.Time, time.Duration) {
	if event != nil {
		start, end := eventTimes(event, app.Location)
		if !start.IsZero() && !end.IsZero() && end.After(start) {
			return start, end.Sub(start)
		}
	}
	if task != nil && task.Due != "" {
		if due, err := time.Parse(time.RFC3339, task.Due); err == nil {
			return due.In(app.Location), 0
		}
	}
	return time.Time{}, 0
}

func ensureSectionTask(app *App, listID, section string) (*tasks.Task, error) {
	items, err := app.Tasks.ListTasks(listID, false)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Title != section {
			continue
		}
		if _, ok := metadata.Extract(item.Notes, "justdoit_section"); ok {
			return item, nil
		}
	}
	return nil, fmt.Errorf("section not found: %s (create it with `justdoit section create \"%s\"`)", section, section)
}
