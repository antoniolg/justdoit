package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/spf13/cobra"
)

func newNextCmd() *cobra.Command {
	var (
		includeBacklog bool
		ids            bool
	)
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Show your next tasks (overdue/today/this week/next week)",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd)
			if err != nil {
				return err
			}
			items, err := buildNextItems(newQueryContext(app), includeBacklog)
			if err != nil {
				return err
			}
			if len(items) == 0 {
				fmt.Println("(no tasks)")
				return nil
			}
			printNextItems(items, ids)
			return nil
		},
	}
	cmd.Flags().BoolVar(&includeBacklog, "backlog", true, "Include backlog tasks without due date")
	cmd.Flags().BoolVar(&ids, "ids", false, "Show task IDs")
	return cmd
}

func printNextItems(items []list.Item, showIDs bool) {
	currentHeader := ""
	for _, it := range items {
		switch v := it.(type) {
		case taskItem:
			if v.IsHeader {
				header := strings.TrimSpace(v.TitleVal)
				if header != "" && header != currentHeader {
					fmt.Printf("\n%s\n", header)
					currentHeader = header
				}
				continue
			}
			contextParts := []string{}
			if strings.TrimSpace(v.Section) != "" && v.Section != "General" {
				contextParts = append(contextParts, v.Section)
			}
			if strings.TrimSpace(v.ListName) != "" {
				contextParts = append(contextParts, v.ListName)
			}
			context := ""
			if len(contextParts) > 0 {
				context = " (" + strings.Join(contextParts, " / ") + ")"
			}
			due := ""
			if v.HasDue {
				formatted := formatDueText(v.Due, v.HasTime)
				if formatted != "" {
					due = " (due " + formatted + ")"
				}
			}
			idText := ""
			if showIDs {
				idText = " [id: " + v.ID + "]"
			}
			title := recurringTitle(v.TitleVal, v.Recurrence)
			fmt.Printf("- %s%s%s%s\n", title, context, due, idText)
		case calendarEventItem:
			// If TUI included events but somehow no Today header made it through,
			// render a Today header to keep the output readable.
			if currentHeader != "Today" {
				fmt.Printf("\nToday\n")
				currentHeader = "Today"
			}
			desc := strings.TrimSpace(v.Description())
			ctx := ""
			if desc != "" {
				ctx = " (" + desc + ")"
			}
			summary := strings.TrimSpace(v.Summary)
			if summary == "" {
				summary = "(untitled)"
			}
			fmt.Printf("- [cal] %s%s\n", summary, ctx)
		}
	}
}
