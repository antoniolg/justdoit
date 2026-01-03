package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/api/tasks/v1"

	"justdoit/internal/metadata"
	"justdoit/internal/sync"
)

func newMoveCmd() *cobra.Command {
	var (
		fromList string
		toList   string
		section  string
	)
	cmd := &cobra.Command{
		Use:   "move [taskID]",
		Short: "Move a task to another list (and optional section)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd)
			if err != nil {
				return err
			}
			if strings.TrimSpace(toList) == "" {
				return fmt.Errorf("--to is required")
			}
			fromListID, err := resolveListID(app, fromList, fromList != "")
			if err != nil {
				return err
			}
			toListID, err := resolveListID(app, toList, true)
			if err != nil {
				return err
			}
			taskID := args[0]
			if fromListID == toListID {
				params := UpdateParams{
					Section:    section,
					HasSection: cmd.Flags().Changed("section"),
				}
				if _, err := updateTaskWithParams(app, fromListID, taskID, params); err != nil {
					return err
				}
				fmt.Println("✅ Task moved")
				return nil
			}
			if err := moveTaskToList(app, fromListID, toListID, taskID, section); err != nil {
				return err
			}
			fmt.Println("✅ Task moved")
			return nil
		},
	}
	cmd.Flags().StringVar(&fromList, "list", "", "Source list name (mapped via config.json)")
	cmd.Flags().StringVar(&toList, "to", "", "Target list name (mapped via config.json)")
	cmd.Flags().StringVar(&section, "section", "", "Target section name")
	return cmd
}

func moveTaskToList(app *App, fromListID, toListID, taskID, section string) error {
	task, err := app.Tasks.GetTask(fromListID, taskID)
	if err != nil {
		return err
	}
	if _, ok := metadata.Extract(task.Notes, "justdoit_section"); ok {
		return fmt.Errorf("cannot move a section task")
	}
	rule, _ := metadata.Extract(task.Notes, "justdoit_rrule")

	parentID := ""
	sectionName := strings.TrimSpace(section)
	if sectionName != "" {
		sectionTask, err := ensureSectionTask(app, toListID, sectionName)
		if err != nil {
			return err
		}
		parentID = sectionTask.Id
	}

	newTask := &tasks.Task{
		Title: recurringTitle(task.Title, rule),
		Notes: task.Notes,
		Due:   task.Due,
	}
	var created *tasks.Task
	if parentID != "" {
		created, err = app.Tasks.CreateTaskWithParent(toListID, newTask, parentID)
	} else {
		created, err = app.Tasks.CreateTask(toListID, newTask)
	}
	if err != nil {
		return err
	}

	if eventID, ok := metadata.Extract(task.Notes, sync.TaskEventIDKey); ok && eventID != "" {
		if event, err := app.Calendar.GetEvent(app.Config.CalendarID, eventID); err == nil && event != nil {
			event.Description = replaceMetadata(event.Description, sync.EventTaskIDKey, created.Id)
			if _, err := app.Calendar.UpdateEvent(app.Config.CalendarID, event); err != nil {
				return err
			}
		}
	}

	return app.Tasks.DeleteTask(fromListID, taskID)
}

func replaceMetadata(text, key, value string) string {
	prefix := key + "="
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, prefix) {
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	clean := strings.TrimSpace(strings.Join(lines, "\n"))
	return metadata.Append(clean, key, value)
}
