package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"google.golang.org/api/tasks/v1"

	"justdoit/internal/metadata"
)

type taskTitleMatch struct {
	ID      string
	Title   string
	Section string
}

func newDoneCmd() *cobra.Command {
	var (
		list      string
		markEvent bool
		title     string
		section   string
	)
	cmd := &cobra.Command{
		Use:   "done [taskID]",
		Short: "Mark a task as completed (and optionally mark calendar event)",
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
				resolved, err := resolveTaskIDByTitleInteractiveWithOptions(app, listID, strings.TrimSpace(title), strings.TrimSpace(section), false)
				if err != nil {
					return err
				}
				taskID = resolved
			}

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
	cmd.Flags().StringVar(&title, "title", "", "Complete a task by exact title (alternative to taskID)")
	cmd.Flags().StringVar(&section, "section", "", "Only match tasks in this section when using --title")
	return cmd
}

func resolveTaskIDByTitleInteractiveWithOptions(app *App, listID, title, section string, includeCompleted bool) (string, error) {
	matches, err := findTasksByExactTitle(app, listID, title, section, includeCompleted)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		if section == "" {
			return "", fmt.Errorf("task not found with exact title: %q", title)
		}
		return "", fmt.Errorf("task not found with exact title %q in section %q", title, section)
	}
	if len(matches) == 1 {
		return matches[0].ID, nil
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		ids := make([]string, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, m.ID)
		}
		return "", fmt.Errorf("multiple tasks match exact title %q; use taskID instead (matches: %s)", title, strings.Join(ids, ", "))
	}

	fmt.Fprintf(os.Stderr, "Multiple tasks match title %q. Select one:\n", title)
	for i, m := range matches {
		ctx := m.Section
		if strings.TrimSpace(ctx) == "" {
			ctx = "General"
		}
		fmt.Fprintf(os.Stderr, "%d) %s (%s) [id: %s]\n", i+1, m.Title, ctx, m.ID)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprintf(os.Stderr, "Select task number (1-%d) or 0 to cancel: ", len(matches))
		line, readErr := reader.ReadString('\n')
		if readErr != nil && line == "" {
			return "", readErr
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		choice, convErr := strconv.Atoi(line)
		if convErr != nil {
			fmt.Fprintln(os.Stderr, "Invalid input. Enter a number.")
			continue
		}
		if choice == 0 {
			return "", errors.New("canceled")
		}
		if choice < 1 || choice > len(matches) {
			fmt.Fprintln(os.Stderr, "Out of range.")
			continue
		}
		return matches[choice-1].ID, nil
	}
}

func findTasksByExactTitle(app *App, listID, title, section string, includeCompleted bool) ([]taskTitleMatch, error) {
	items, err := app.Tasks.ListTasksWithOptions(listID, includeCompleted, false, false, "")
	if err != nil {
		return nil, err
	}

	sectionIDs := map[string]string{}
	for _, item := range items {
		if _, ok := metadata.Extract(item.Notes, "justdoit_section"); ok {
			sectionIDs[item.Id] = item.Title
		}
	}

	matches := []taskTitleMatch{}
	for _, item := range items {
		if _, ok := metadata.Extract(item.Notes, "justdoit_section"); ok {
			continue
		}
		if item.Title != title {
			continue
		}
		sectionName := "General"
		if parent, ok := sectionIDs[item.Parent]; ok {
			sectionName = parent
		}
		if section != "" && !strings.EqualFold(sectionName, section) {
			continue
		}
		matches = append(matches, taskTitleMatch{ID: item.Id, Title: item.Title, Section: sectionName})
	}

	return matches, nil
}
