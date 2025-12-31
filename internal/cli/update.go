package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/tasks/v1"

	"justdoit/internal/metadata"
	"justdoit/internal/sync"
	"justdoit/internal/timeparse"
)

func newUpdateCmd() *cobra.Command {
	var (
		list    string
		dateStr string
		timeStr string
		section string
		notes   string
		title   string
	)
	cmd := &cobra.Command{
		Use:   "update [taskID] [new title]",
		Short: "Update a task (title/date/time/section)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd)
			if err != nil {
				return err
			}

			listID, err := resolveListID(app, list, list != "")
			if err != nil {
				return err
			}

			taskID := args[0]
			newTitle := title
			if len(args) > 1 {
				newTitle = strings.Join(args[1:], " ")
			}

			task, err := app.Tasks.GetTask(listID, taskID)
			if err != nil {
				return err
			}

			if newTitle != "" {
				task.Title = newTitle
			}

			if cmd.Flags().Changed("notes") {
				task.Notes = mergeNotes(notes, task.Notes)
			}

			var (
				event          *calendar.Event
				eventExists    bool
				newStart       *time.Time
				newEnd         *time.Time
				newDue         *time.Time
				sectionChanged bool
			)

			if cmd.Flags().Changed("section") {
				sectionName := strings.TrimSpace(section)
				if sectionName == "" {
					sectionName = "General"
				}
				sectionTask, err := ensureSectionTask(app, listID, sectionName)
				if err != nil {
					return err
				}
				if _, err := app.Tasks.MoveTask(listID, taskID, sectionTask.Id); err != nil {
					return err
				}
				sectionChanged = true
			}

			if cmd.Flags().Changed("time") || cmd.Flags().Changed("date") || newTitle != "" {
				event, eventExists, _ = findLinkedEvent(app, task)
			}

			if cmd.Flags().Changed("time") || cmd.Flags().Changed("date") {
				baseDate := resolveBaseDate(app, task, event, dateStr)
				if cmd.Flags().Changed("time") {
					start, end, err := timeparse.ParseTimeRange(timeStr, baseDate, app.Now, app.Location)
					if err != nil {
						return err
					}
					newStart = &start
					newEnd = &end
					newDue = &end
				} else if cmd.Flags().Changed("date") {
					if baseDate.IsZero() {
						return fmt.Errorf("invalid date")
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
				return err
			}

			if newTitle != "" && eventExists && event != nil {
				event.Summary = newTitle
			}

			if newStart != nil && newEnd != nil {
				if eventExists && event != nil {
					event.Start.DateTime = newStart.Format(time.RFC3339)
					event.End.DateTime = newEnd.Format(time.RFC3339)
					if _, err := app.Calendar.UpdateEvent(app.Config.CalendarID, event); err != nil {
						return err
					}
				} else {
					created, err := createLinkedEvent(app, task, newStart, newEnd)
					if err != nil {
						return err
					}
					updatedNotes := metadata.Append(task.Notes, sync.TaskEventIDKey, created.Id)
					task.Notes = updatedNotes
					if _, err := app.Tasks.UpdateTask(listID, task); err != nil {
						return err
					}
				}
			} else if eventExists && event != nil && newTitle != "" {
				if _, err := app.Calendar.UpdateEvent(app.Config.CalendarID, event); err != nil {
					return err
				}
			}

			fmt.Println("✅ Task updated")
			if sectionChanged {
				fmt.Println("📌 Section updated")
			}
			if newStart != nil {
				fmt.Println("📅 Event updated")
			} else if newTitle != "" && eventExists {
				fmt.Println("📅 Event renamed")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&list, "list", "", "List name (mapped via config.json)")
	cmd.Flags().StringVar(&title, "title", "", "New title")
	cmd.Flags().StringVar(&dateStr, "date", "", "Due date (natural language, e.g. 'tomorrow')")
	cmd.Flags().StringVar(&timeStr, "time", "", "Time block (HH:MM-HH:MM or 1h)")
	cmd.Flags().StringVar(&section, "section", "", "Move task to section (sublist)")
	cmd.Flags().StringVar(&notes, "notes", "", "Replace task notes")

	return cmd
}

func mergeNotes(userNotes, existing string) string {
	result := strings.TrimSpace(userNotes)
	for _, key := range []string{sync.TaskEventIDKey, "justdoit_rrule", "justdoit_section"} {
		if value, ok := metadata.Extract(existing, key); ok {
			result = metadata.Append(result, key, value)
		}
	}
	return result
}

func findLinkedEvent(app *App, task *tasks.Task) (*calendar.Event, bool, error) {
	if eventID, ok := sync.ExtractMetadata(task.Notes, sync.TaskEventIDKey); ok {
		event, err := app.Calendar.GetEvent(app.Config.CalendarID, eventID)
		if err == nil {
			return event, true, nil
		}
	}
	if event, err := app.Calendar.FindEventByTaskID(app.Config.CalendarID, task.Id); err == nil {
		return event, true, nil
	}
	return nil, false, nil
}

func createLinkedEvent(app *App, task *tasks.Task, start, end *time.Time) (*calendar.Event, error) {
	event := &calendar.Event{
		Summary:     task.Title,
		Description: metadata.Append("", sync.EventTaskIDKey, task.Id),
		Start: &calendar.EventDateTime{
			DateTime: start.Format(time.RFC3339),
		},
		End: &calendar.EventDateTime{
			DateTime: end.Format(time.RFC3339),
		},
	}
	return app.Calendar.CreateEvent(app.Config.CalendarID, event)
}

func resolveBaseDate(app *App, task *tasks.Task, event *calendar.Event, dateStr string) time.Time {
	if dateStr != "" {
		baseDate, err := timeparse.ParseDate(dateStr, app.Now, app.Location)
		if err == nil && !baseDate.IsZero() {
			return baseDate
		}
	}
	if event != nil {
		start, _ := eventTimes(event, app.Location)
		if !start.IsZero() {
			return time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, app.Location)
		}
	}
	if task.Due != "" {
		if due, err := time.Parse(time.RFC3339, task.Due); err == nil {
			return time.Date(due.In(app.Location).Year(), due.In(app.Location).Month(), due.In(app.Location).Day(), 0, 0, 0, 0, app.Location)
		}
	}
	return time.Date(app.Now.Year(), app.Now.Month(), app.Now.Day(), 0, 0, 0, 0, app.Location)
}

func eventTimes(event *calendar.Event, loc *time.Location) (time.Time, time.Time) {
	if event == nil || event.Start == nil || event.End == nil {
		return time.Time{}, time.Time{}
	}
	if event.Start.DateTime != "" {
		start, err := time.Parse(time.RFC3339, event.Start.DateTime)
		if err != nil {
			return time.Time{}, time.Time{}
		}
		end, err := time.Parse(time.RFC3339, event.End.DateTime)
		if err != nil {
			return time.Time{}, time.Time{}
		}
		return start.In(loc), end.In(loc)
	}
	return time.Time{}, time.Time{}
}
