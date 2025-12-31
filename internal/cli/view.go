package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/api/calendar/v3"

	"justdoit/internal/agenda"
)

type taskView struct {
	ID    string
	Title string
	List  string
	Due   time.Time
}

func newViewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Show today's agenda with free slots",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd)
			if err != nil {
				return err
			}
			day := app.Now
			dayStart, dayEnd, err := agenda.DayBounds(day, app.Config.WorkdayStart, app.Config.WorkdayEnd, app.Location)
			if err != nil {
				return err
			}
			events, err := app.Calendar.ListEvents(app.Config.CalendarID, dayStart.Format(time.RFC3339), dayEnd.Format(time.RFC3339))
			if err != nil {
				return err
			}
			tasksToday, err := collectTasks(app, day)
			if err != nil {
				return err
			}
			free := agenda.FreeSlots(events, dayStart, dayEnd)

			fmt.Printf("Agenda for %s\n", day.Format("2006-01-02"))
			fmt.Println("\nCalendar events:")
			if len(events) == 0 {
				fmt.Println("- (none)")
			} else {
				for _, e := range events {
					printEvent(e, app.Location)
				}
			}

			fmt.Println("\nTasks:")
			if len(tasksToday) == 0 {
				fmt.Println("- (none)")
			} else {
				sort.Slice(tasksToday, func(i, j int) bool { return tasksToday[i].Title < tasksToday[j].Title })
				for _, t := range tasksToday {
					fmt.Printf("- [%s] %s (%s)\n", t.List, t.Title, t.ID)
				}
			}

			fmt.Println("\nFree slots:")
			if len(free) == 0 {
				fmt.Println("- (none)")
			} else {
				for _, slot := range free {
					fmt.Printf("- %s - %s\n", slot.Start.Format("15:04"), slot.End.Format("15:04"))
				}
			}
			return nil
		},
	}
	return cmd
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
				result = append(result, taskView{ID: t.Id, Title: t.Title, List: name, Due: due})
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

func printEvent(e *calendar.Event, loc *time.Location) {
	if e.Start == nil || e.End == nil {
		fmt.Printf("- %s\n", e.Summary)
		return
	}
	if e.Start.DateTime != "" {
		start, err := time.Parse(time.RFC3339, e.Start.DateTime)
		if err != nil {
			fmt.Printf("- %s\n", e.Summary)
			return
		}
		end, err := time.Parse(time.RFC3339, e.End.DateTime)
		if err != nil {
			fmt.Printf("- %s\n", e.Summary)
			return
		}
		fmt.Printf("- %s - %s %s\n", start.In(loc).Format("15:04"), end.In(loc).Format("15:04"), e.Summary)
		return
	}
	if e.Start.Date != "" {
		fmt.Printf("- All-day %s\n", e.Summary)
		return
	}
	fmt.Printf("- %s\n", e.Summary)
}
