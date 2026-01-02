package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/api/tasks/v1"

	"justdoit/internal/metadata"
)

func newSectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "section",
		Short: "Manage task sections",
	}
	cmd.AddCommand(newSectionCreateCmd())
	cmd.AddCommand(newSectionRenameCmd())
	return cmd
}

func newSectionCreateCmd() *cobra.Command {
	var list string
	cmd := &cobra.Command{
		Use:   "create [name...]",
		Short: "Create section(s) in a task list",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd)
			if err != nil {
				return err
			}
			listID, err := resolveListID(app, list, list != "")
			if err != nil {
				return err
			}
			created := 0
			for _, name := range args {
				sectionName := strings.TrimSpace(name)
				if sectionName == "" {
					continue
				}
				_, didCreate, err := ensureSectionTaskWithStatus(app, listID, sectionName)
				if err != nil {
					return err
				}
				if didCreate {
					created++
				}
			}
			fmt.Printf("Created %d section(s)\n", created)
			return nil
		},
	}
	cmd.Flags().StringVar(&list, "list", "", "List name (mapped via config.json)")
	return cmd
}

func newSectionRenameCmd() *cobra.Command {
	var list string
	cmd := &cobra.Command{
		Use:   "rename [old] [new]",
		Short: "Rename a section in a task list",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd)
			if err != nil {
				return err
			}
			oldName := strings.TrimSpace(args[0])
			newName := strings.TrimSpace(args[1])
			if oldName == "" || newName == "" {
				return fmt.Errorf("section names cannot be empty")
			}
			if strings.EqualFold(oldName, newName) {
				return fmt.Errorf("old and new section names are the same")
			}
			listID, err := resolveListID(app, list, list != "")
			if err != nil {
				return err
			}
			renamed, err := renameSectionInList(app, listID, oldName, newName)
			if err != nil {
				return err
			}
			fmt.Printf("Renamed %d section(s)\n", renamed)
			return nil
		},
	}
	cmd.Flags().StringVar(&list, "list", "", "List name (mapped via config.json)")
	return cmd
}

func ensureSectionTaskWithStatus(app *App, listID, section string) (*tasks.Task, bool, error) {
	items, err := app.Tasks.ListTasks(listID, false)
	if err != nil {
		return nil, false, err
	}
	for _, item := range items {
		if item.Title != section {
			continue
		}
		if _, ok := metadata.Extract(item.Notes, "justdoit_section"); ok {
			return item, false, nil
		}
	}
	task := &tasks.Task{
		Title: section,
		Notes: metadata.Append("", "justdoit_section", "1"),
	}
	created, err := app.Tasks.CreateTask(listID, task)
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

func renameSectionInList(app *App, listID, oldName, newName string) (int, error) {
	items, err := app.Tasks.ListTasksWithOptions(listID, true, true, false, "")
	if err != nil {
		return 0, err
	}
	count := 0
	for _, item := range items {
		if !strings.EqualFold(item.Title, oldName) {
			continue
		}
		if _, ok := metadata.Extract(item.Notes, "justdoit_section"); !ok {
			continue
		}
		item.Title = newName
		if _, err := app.Tasks.UpdateTask(listID, item); err != nil {
			return count, err
		}
		count++
	}
	if count == 0 {
		return 0, fmt.Errorf("section not found: %s", oldName)
	}
	return count, nil
}
