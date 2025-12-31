package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"justdoit/internal/sync"
)

func newDoneCmd() *cobra.Command {
	var (
		list      string
		markEvent bool
	)
	cmd := &cobra.Command{
		Use:   "done [taskID]",
		Short: "Mark a task as completed (and optionally mark calendar event)",
		Args:  cobra.ExactArgs(1),
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
			task, err := app.Tasks.GetTask(listID, taskID)
			if err != nil {
				return err
			}
			if _, err := app.Tasks.CompleteTask(listID, taskID); err != nil {
				return err
			}
			fmt.Printf("Task completed: %s\n", taskID)
			if !markEvent {
				return nil
			}

			eventID, ok := sync.ExtractMetadata(task.Notes, sync.TaskEventIDKey)
			var eventErr error
			var eventIDUsed string
			if ok {
				eventIDUsed = eventID
			} else {
				event, err := app.Calendar.FindEventByTaskID(app.Config.CalendarID, taskID)
				if err != nil {
					eventErr = err
				} else {
					eventIDUsed = event.Id
				}
			}
			if eventIDUsed == "" {
				if eventErr != nil {
					return eventErr
				}
				return nil
			}
			event, err := app.Calendar.GetEvent(app.Config.CalendarID, eventIDUsed)
			if err != nil {
				return err
			}
			if !strings.HasPrefix(event.Summary, "✅ ") {
				event.Summary = "✅ " + event.Summary
				if _, err := app.Calendar.UpdateEvent(app.Config.CalendarID, event); err != nil {
					return err
				}
				fmt.Printf("Event marked: %s\n", event.Id)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&list, "list", "", "List name (mapped via config.json)")
	cmd.Flags().BoolVar(&markEvent, "mark-event", true, "Prefix calendar event title with ✅")
	return cmd
}
