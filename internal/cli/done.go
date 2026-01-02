package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/api/tasks/v1"
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
			if err := markTaskDone(app, listID, taskID, markEvent); err != nil {
				return err
			}
			fmt.Println("✅ Task completed")
			if markEvent {
				if event, ok, _ := findLinkedEvent(app, &tasks.Task{Id: taskID}); ok && event != nil {
					fmt.Println("📅 Event marked")
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&list, "list", "", "List name (mapped via config.json)")
	cmd.Flags().BoolVar(&markEvent, "mark-event", true, "Prefix calendar event title with ✅")
	return cmd
}
