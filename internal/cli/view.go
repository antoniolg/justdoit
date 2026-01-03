package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/api/calendar/v3"

	"justdoit/internal/agenda"
	"justdoit/internal/metadata"
	"justdoit/internal/timeparse"
)

type taskView struct {
	ID         string
	Title      string
	List       string
	Due        time.Time
	Recurrence string
}

func newViewCmd() *cobra.Command {
	var dateStr string
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Show schedule with free slots",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd)
			if err != nil {
				return err
			}
			start, end, err := parseDateRange(dateStr, app)
			if err != nil {
				return err
			}
			for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
				if err := viewDay(app, day); err != nil {
					return err
				}
				if day.Before(end) {
					fmt.Println()
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dateStr, "date", "", "Date or range (e.g. 'today', 'tomorrow', '2026-01-02', '2026-01-01..2026-01-07')")
	return cmd
}

func viewDay(app *App, day time.Time) error {
	text, err := buildDayTextWithError(app, day)
	if err != nil {
		return err
	}
	fmt.Println(text)
	return nil
}

func buildDayText(app *App, day time.Time) string {
	text, err := buildDayTextWithError(app, day)
	if err != nil {
		return err.Error()
	}
	return text
}

func buildDayTextWithError(app *App, day time.Time) (string, error) {
	dayStart, dayEnd, err := agenda.DayBounds(day, app.Config.WorkdayStart, app.Config.WorkdayEnd, app.Location)
	if err != nil {
		return "", err
	}
	events, err := app.Calendar.ListEvents(app.Config.CalendarID, dayStart.Format(time.RFC3339), dayEnd.Format(time.RFC3339))
	if err != nil {
		return "", err
	}
	tasksToday, err := collectTasks(app, day)
	if err != nil {
		return "", err
	}
	free := agenda.FreeSlots(events, dayStart, dayEnd)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Schedule for %s\n", day.Format("2006-01-02")))
	b.WriteString("\nCalendar events:\n")
	if len(events) == 0 {
		b.WriteString("- (none)\n")
	} else {
		for _, e := range events {
			b.WriteString(renderEvent(e, app.Location))
		}
	}

	b.WriteString("\nTasks:\n")
	if len(tasksToday) == 0 {
		b.WriteString("- (none)\n")
	} else {
		sort.Slice(tasksToday, func(i, j int) bool { return tasksToday[i].Title < tasksToday[j].Title })
		for _, t := range tasksToday {
			b.WriteString(fmt.Sprintf("- [%s] %s (%s)\n", t.List, t.Title, t.ID))
		}
	}

	b.WriteString("\nFree slots:\n")
	if len(free) == 0 {
		b.WriteString("- (none)\n")
	} else {
		for _, slot := range free {
			b.WriteString(fmt.Sprintf("- %s - %s\n", slot.Start.Format("15:04"), slot.End.Format("15:04")))
		}
	}

	return strings.TrimSpace(b.String()), nil
}

func parseDateRange(dateStr string, app *App) (time.Time, time.Time, error) {
	if strings.TrimSpace(dateStr) == "" {
		day := time.Date(app.Now.Year(), app.Now.Month(), app.Now.Day(), 0, 0, 0, 0, app.Location)
		return day, day, nil
	}
	parts := strings.Split(dateStr, "..")
	if len(parts) == 2 {
		start, err := timeparse.ParseDate(strings.TrimSpace(parts[0]), app.Now, app.Location)
		if err != nil || start.IsZero() {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid start date")
		}
		end, err := timeparse.ParseDate(strings.TrimSpace(parts[1]), app.Now, app.Location)
		if err != nil || end.IsZero() {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end date")
		}
		if end.Before(start) {
			return time.Time{}, time.Time{}, fmt.Errorf("end date before start date")
		}
		return start, end, nil
	}
	day, err := timeparse.ParseDate(dateStr, app.Now, app.Location)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if day.IsZero() {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date")
	}
	return day, day, nil
}

func collectTasks(app *App, day time.Time) ([]taskView, error) {
	var result []taskView
	for name, id := range app.Config.Lists {
		items, err := app.Tasks.ListTasks(id, false)
		if err != nil {
			return nil, err
		}
		for _, t := range items {
			if t.Due == "" {
				continue
			}
			due, err := time.Parse(time.RFC3339, t.Due)
			if err != nil {
				continue
			}
			due = due.In(app.Location)
			if sameDay(due, day) {
				task := taskView{ID: t.Id, Title: t.Title, List: name, Due: due}
				if rule, ok := metadata.Extract(t.Notes, "justdoit_rrule"); ok {
					task.Recurrence = rule
				}
				result = append(result, task)
			}
		}
	}
	return result, nil
}

func sameDay(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func renderEvent(e *calendar.Event, loc *time.Location) string {
	if e.Start == nil || e.End == nil {
		return fmt.Sprintf("- %s\n", e.Summary)
	}
	if e.Start.DateTime != "" {
		start, err := time.Parse(time.RFC3339, e.Start.DateTime)
		if err != nil {
			return fmt.Sprintf("- %s\n", e.Summary)
		}
		end, err := time.Parse(time.RFC3339, e.End.DateTime)
		if err != nil {
			return fmt.Sprintf("- %s\n", e.Summary)
		}
		return fmt.Sprintf("- %s - %s %s\n", start.In(loc).Format("15:04"), end.In(loc).Format("15:04"), e.Summary)
	}
	if e.Start.Date != "" {
		return fmt.Sprintf("- All-day %s\n", e.Summary)
	}
	return fmt.Sprintf("- %s\n", e.Summary)
}

func printEvent(e *calendar.Event, loc *time.Location) {
	fmt.Print(renderEvent(e, loc))
}
