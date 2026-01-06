package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/api/tasks/v1"

	"justdoit/internal/metadata"
)

type taskRow struct {
	ID         string
	Title      string
	Due        time.Time
	HasDue     bool
	HasTime    bool
	Index      int
	Recurrence string
}

func newListCmd() *cobra.Command {
	var (
		list    string
		section string
		all     bool
		ids     bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks grouped by section",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd)
			if err != nil {
				return err
			}
			listID, err := resolveListID(app, list, list != "")
			if err != nil {
				return err
			}
			items, err := app.Tasks.ListTasksWithOptions(listID, all, all, false, "")
			if err != nil {
				return err
			}

			sectionFilter := strings.TrimSpace(section)
			sections, order := groupTasksBySection(items, sectionFilter, app.Location)
			if len(sections) == 0 {
				fmt.Println("(no tasks)")
				return nil
			}
			for _, name := range order {
				tasks := sections[name]
				if len(tasks) == 0 {
					continue
				}
				fmt.Printf("\n%s\n", name)
				printTasks(tasks, ids)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&list, "list", "", "List name (mapped via config.json)")
	cmd.Flags().StringVar(&section, "section", "", "Filter by section name")
	cmd.Flags().BoolVar(&all, "all", false, "Include completed/hidden tasks")
	cmd.Flags().BoolVar(&ids, "ids", false, "Show task IDs")
	return cmd
}

func groupTasksBySection(items []*tasks.Task, filter string, loc *time.Location) (map[string][]taskRow, []string) {
	sections := map[string][]taskRow{}
	order := []string{}
	sectionIDs := map[string]string{}
	sectionNames := map[string]bool{}

	for _, item := range items {
		if _, ok := metadata.Extract(item.Notes, "justdoit_section"); ok {
			sectionIDs[item.Id] = item.Title
			if !sectionNames[item.Title] {
				order = append(order, item.Title)
				sectionNames[item.Title] = true
			}
		}
	}
	for i, item := range items {
		if _, ok := metadata.Extract(item.Notes, "justdoit_section"); ok {
			continue
		}
		sectionName := "General"
		if parent, ok := sectionIDs[item.Parent]; ok {
			sectionName = parent
		}
		if filter != "" && !strings.EqualFold(filter, sectionName) {
			continue
		}
		if !sectionNames[sectionName] {
			order = append(order, sectionName)
			sectionNames[sectionName] = true
		}
		row := taskRow{ID: item.Id, Title: item.Title, Index: i}
		if rule, ok := metadata.Extract(item.Notes, "justdoit_rrule"); ok {
			row.Recurrence = rule
		}
		row.Due, row.HasDue, row.HasTime = parseTaskDue(item.Due, loc)
		sections[sectionName] = append(sections[sectionName], row)
	}
	return sections, order
}

func printTasks(tasks []taskRow, showIDs bool) {
	due := make([]taskRow, 0, len(tasks))
	noDue := make([]taskRow, 0, len(tasks))
	for _, t := range tasks {
		if t.HasDue {
			due = append(due, t)
		} else {
			noDue = append(noDue, t)
		}
	}
	sort.SliceStable(due, func(i, j int) bool {
		if due[i].Due.Equal(due[j].Due) {
			return due[i].Index < due[j].Index
		}
		return due[i].Due.Before(due[j].Due)
	})

	ordered := append(due, noDue...)
	for _, t := range ordered {
		dueText := ""
		if t.HasDue {
			dueText = fmt.Sprintf(" (due %s)", t.Due.Format("2006-01-02"))
		}
		idText := ""
		if showIDs {
			idText = fmt.Sprintf(" [id: %s]", t.ID)
		}
		title := recurringTitle(t.Title, t.Recurrence)
		fmt.Printf("- %s%s%s\n", title, dueText, idText)
	}
}
