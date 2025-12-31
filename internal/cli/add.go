package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"justdoit/internal/sync"
	"justdoit/internal/timeparse"
)

func newAddCmd() *cobra.Command {
	var (
		list    string
		dateStr string
		every   string
		timeStr string
		section string
		notes   string
	)
	cmd := &cobra.Command{
		Use:   "add [title]",
		Short: "Add a task (and optional calendar block)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd)
			if err != nil {
				return err
			}
			title := strings.Join(args, " ")
			listID, err := resolveListID(app, list, list != "")
			if err != nil {
				return err
			}
			sectionName := strings.TrimSpace(section)
			if sectionName == "" {
				sectionName = "General"
			}
			sectionTask, err := ensureSectionTask(app, listID, sectionName)
			if err != nil {
				return err
			}
			baseDate, err := timeparse.ParseDate(dateStr, app.Now, app.Location)
			if err != nil {
				return err
			}
			recurrence, err := buildRecurrence(every)
			if err != nil {
				return err
			}
			var due *time.Time
			var start *time.Time
			var end *time.Time
			if timeStr != "" {
				startTime, endTime, err := timeparse.ParseTimeRange(timeStr, baseDate, app.Now, app.Location)
				if err != nil {
					return err
				}
				start = &startTime
				end = &endTime
				due = end
			} else if !baseDate.IsZero() {
				endOfDay := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 23, 59, 0, 0, app.Location)
				due = &endOfDay
			}

			input := sync.CreateInput{
				ListID:     listID,
				Title:      title,
				Notes:      notes,
				Due:        due,
				Recurrence: recurrence,
				TimeStart:  start,
				TimeEnd:    end,
				ParentID:   sectionTask.Id,
			}
			_, event, err := app.Sync.Create(input)
			if err != nil {
				return err
			}
			fmt.Println("✅ Task created")
			if event != nil {
				fmt.Println("📅 Event created")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&list, "list", "", "List name (mapped via config.json)")
	cmd.Flags().StringVar(&dateStr, "date", "", "Due date (natural language, e.g. 'tomorrow')")
	cmd.Flags().StringVar(&every, "every", "", "Recurrence (e.g. 'daily', 'weekly')")
	cmd.Flags().StringVar(&timeStr, "time", "", "Time block (HH:MM-HH:MM or 1h)")
	cmd.Flags().StringVar(&section, "section", "General", "Section (sublist) name")
	cmd.Flags().StringVar(&notes, "notes", "", "Notes for the task")
	return cmd
}

func resolveListID(app *App, list string, explicit bool) (string, error) {
	if list == "" {
		list = app.Config.DefaultList
	}
	if id, ok := app.Config.ListID(list); ok {
		return id, nil
	}
	if explicit {
		return list, nil
	}
	return "", fmt.Errorf("list not mapped: %s (run `justdoit setup`)", list)
}

func buildRecurrence(every string) ([]string, error) {
	if every == "" {
		return nil, nil
	}
	freq := ""
	s := strings.ToLower(strings.TrimSpace(every))
	switch {
	case strings.Contains(s, "day"):
		freq = "DAILY"
	case strings.Contains(s, "week"):
		freq = "WEEKLY"
	case strings.Contains(s, "month"):
		freq = "MONTHLY"
	case strings.Contains(s, "year"):
		freq = "YEARLY"
	default:
		return nil, fmt.Errorf("unsupported recurrence: %s", every)
	}
	rrule := fmt.Sprintf("RRULE:FREQ=%s", freq)
	return []string{rrule}, nil
}
