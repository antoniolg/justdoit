package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var (
		list string
		all  bool
		ids  bool
	)
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search tasks by text",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd)
			if err != nil {
				return err
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			results, err := searchTasks(newQueryContext(app), query, list, all)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Println("(no results)")
				return nil
			}
			printSearchResults(results, list == "", ids)
			return nil
		},
	}
	cmd.Flags().StringVar(&list, "list", "", "List name (mapped via config.json)")
	cmd.Flags().BoolVar(&all, "all", false, "Include completed/hidden tasks")
	cmd.Flags().BoolVar(&ids, "ids", false, "Show task IDs")
	return cmd
}

func printSearchResults(results []taskItem, showList bool, showIDs bool) {
	for _, item := range results {
		title := recurringTitle(item.TitleVal, item.Recurrence)
		contextParts := []string{}
		if showList && item.ListName != "" {
			contextParts = append(contextParts, item.ListName)
		}
		if strings.TrimSpace(item.Section) != "" {
			contextParts = append(contextParts, item.Section)
		}
		context := ""
		if len(contextParts) > 0 {
			context = " (" + strings.Join(contextParts, " / ") + ")"
		}
		idText := ""
		if showIDs {
			idText = fmt.Sprintf(" [id: %s]", item.ID)
		}
		dueText := ""
		if item.HasDue {
			dueText = fmt.Sprintf(" (due %s)", item.Due.Format("2006-01-02"))
		}
		fmt.Printf("- %s%s%s%s\n", title, context, dueText, idText)
	}
}
