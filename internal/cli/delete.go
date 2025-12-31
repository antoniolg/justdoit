package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"justdoit/internal/sync"
)

func newDeleteCmd() *cobra.Command {
	var (
		list      string
		keepEvent bool
	)
	cmd := &cobra.Command{
		Use:   "delete [taskID]",
		Short: "Delete a task (and linked calendar event)",
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

			if !keepEvent {
				if eventID, ok := sync.ExtractMetadata(task.Notes, sync.TaskEventIDKey); ok {
					_ = app.Calendar.DeleteEvent(app.Config.CalendarID, eventID)
				} else {
					if event, err := app.Calendar.FindEventByTaskID(app.Config.CalendarID, taskID); err == nil {
						_ = app.Calendar.DeleteEvent(app.Config.CalendarID, event.Id)
					}
				}
			}

			if err := app.Tasks.DeleteTask(listID, taskID); err != nil {
				return err
			}

			fmt.Println("🗑️ Task deleted")
			if !keepEvent {
				fmt.Println("📅 Event deleted (if linked)")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&list, "list", "", "List name (mapped via config.json)")
	cmd.Flags().BoolVar(&keepEvent, "keep-event", false, "Do not delete the linked calendar event")

	return cmd
}
