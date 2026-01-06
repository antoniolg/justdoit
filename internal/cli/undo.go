package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newUndoCmd() *cobra.Command {
	var (
		list      string
		markEvent bool
		title     string
		section   string
	)
	cmd := &cobra.Command{
		Use:   "undo [taskID]",
		Short: "Mark a completed task as not completed (and optionally unmark calendar event)",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return cobra.ExactArgs(1)(cmd, args)
			}
			if len(args) == 1 {
				return nil
			}
			if strings.TrimSpace(title) == "" {
				return errors.New("requires either [taskID] arg or --title")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd)
			if err != nil {
				return err
			}
			listID, err := resolveListID(app, list, list != "")
			if err != nil {
				return err
			}

			taskID := ""
			if len(args) == 1 {
				taskID = args[0]
			} else {
				resolved, err := resolveTaskIDByTitleInteractiveWithOptions(app, listID, strings.TrimSpace(title), strings.TrimSpace(section), true)
				if err != nil {
					return err
				}
				taskID = resolved
			}

			if err := markTaskUndone(app, listID, taskID, markEvent); err != nil {
				return err
			}
			fmt.Println("↩️  Task marked as not completed")
			return nil
		},
	}
	cmd.Flags().StringVar(&list, "list", "", "List name (mapped via config.json)")
	cmd.Flags().BoolVar(&markEvent, "mark-event", true, "Remove ✅ prefix from linked calendar event")
	cmd.Flags().StringVar(&title, "title", "", "Undo completion by exact title (alternative to taskID)")
	cmd.Flags().StringVar(&section, "section", "", "Only match tasks in this section when using --title")
	return cmd
}
