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

func newSearchCmd() *cobra.Command {
	var (
		list string
		all  bool
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
			results, err := searchTasks(app, query, list, all)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Println("(no results)")
				return nil
			}
			printSearchResults(results, list == "")
			return nil
		},
	}
	cmd.Flags().StringVar(&list, "list", "", "List name (mapped via config.json)")
	cmd.Flags().BoolVar(&all, "all", false, "Include completed/hidden tasks")
	return cmd
}

func searchTasks(app *App, query, listFilter string, includeCompleted bool) ([]taskItem, error) {
	if app == nil {
		return nil, fmt.Errorf("app is not initialized")
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return nil, fmt.Errorf("query is required")
	}

	listMap := map[string]string{}
	if strings.TrimSpace(listFilter) != "" {
		listID, err := resolveListID(app, listFilter, listFilter != "")
		if err != nil {
			return nil, err
		}
		listName := resolveListName(app, listFilter, listID)
		listMap[listName] = listID
	} else {
		for name, id := range app.Config.Lists {
			listMap[name] = id
		}
	}
	if len(listMap) == 0 {
		return nil, fmt.Errorf("no lists configured")
	}

	listNames := make([]string, 0, len(listMap))
	for name := range listMap {
		listNames = append(listNames, name)
	}
	sort.Strings(listNames)

	results := []taskItem{}
	for _, listName := range listNames {
		listID := listMap[listName]
		items, err := app.Tasks.ListTasksWithOptions(listID, true, true, false, "")
		if err != nil {
			return nil, err
		}
		sections := buildSectionIndex(items)
		for _, item := range items {
			if isSectionTask(item) {
				continue
			}
			if !includeCompleted && item.Status == "completed" {
				continue
			}
			section := resolveSectionName(item, sections)
			if !matchesSearch(needle, item, section) {
				continue
			}
			var due time.Time
			hasDue := false
			if item.Due != "" {
				if parsed, err := time.Parse(time.RFC3339, item.Due); err == nil {
					due = parsed.In(app.Location)
					hasDue = true
				}
			}
			recurrence := ""
			if rule, ok := metadata.Extract(item.Notes, "justdoit_rrule"); ok {
				recurrence = rule
			}
			results = append(results, taskItem{
				ID:         item.Id,
				TitleVal:   item.Title,
				ListName:   listName,
				ListID:     listID,
				Section:    section,
				Due:        due,
				HasDue:     hasDue,
				Recurrence: recurrence,
			})
		}
	}

	sortSearchResults(results)
	return results, nil
}

func printSearchResults(results []taskItem, showList bool) {
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
		dueText := ""
		if item.HasDue {
			dueText = fmt.Sprintf(" (due %s)", item.Due.Format("2006-01-02"))
		}
		fmt.Printf("- %s%s%s\n", title, context, dueText)
	}
}

func resolveListName(app *App, listName, listID string) string {
	if listName == "" {
		return listID
	}
	if _, ok := app.Config.ListID(listName); ok {
		return listName
	}
	for name, id := range app.Config.Lists {
		if id == listID {
			return name
		}
	}
	return listName
}

func buildSectionIndex(items []*tasks.Task) map[string]string {
	sections := map[string]string{}
	for _, item := range items {
		if isSectionTask(item) {
			sections[item.Id] = strings.TrimSpace(item.Title)
		}
	}
	return sections
}

func resolveSectionName(item *tasks.Task, sections map[string]string) string {
	if item == nil {
		return "General"
	}
	if section, ok := sections[item.Parent]; ok && section != "" {
		return section
	}
	return "General"
}

func isSectionTask(item *tasks.Task) bool {
	if item == nil {
		return false
	}
	_, ok := metadata.Extract(item.Notes, "justdoit_section")
	return ok
}

func matchesSearch(query string, item *tasks.Task, section string) bool {
	if item == nil {
		return false
	}
	title := strings.ToLower(item.Title)
	notes := strings.ToLower(stripMetadataNotes(item.Notes))
	section = strings.ToLower(section)
	return strings.Contains(title, query) || strings.Contains(notes, query) || strings.Contains(section, query)
}

func sortSearchResults(results []taskItem) {
	sort.SliceStable(results, func(i, j int) bool {
		a := results[i]
		b := results[j]
		if a.HasDue != b.HasDue {
			return a.HasDue
		}
		if a.HasDue && b.HasDue {
			if !a.Due.Equal(b.Due) {
				return a.Due.Before(b.Due)
			}
		}
		if a.ListName != b.ListName {
			return a.ListName < b.ListName
		}
		return strings.ToLower(a.TitleVal) < strings.ToLower(b.TitleVal)
	})
}
