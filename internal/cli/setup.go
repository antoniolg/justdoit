package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"justdoit/internal/config"
)

type simpleList struct {
	Title string
	ID    string
}

type simpleCalendar struct {
	Title   string
	ID      string
	Primary bool
}

type choiceItem[T any] struct {
	Label string
	Item  T
}

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive setup for calendars and task lists",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd)
			if err != nil {
				return err
			}
			cfgPath, err := resolveConfigPath(cmd)
			if err != nil {
				return err
			}
			cfg, err := config.LoadOrCreate(cfgPath)
			if err != nil {
				return err
			}

			printSection("Calendar")
			if err := setupCalendars(app, cfg); err != nil {
				return err
			}

			printSection("Google Tasks")
			if err := setupLists(app, cfg); err != nil {
				return err
			}

			if err := config.Save(cfgPath, cfg); err != nil {
				return err
			}
			fmt.Printf("\nSetup complete. Config saved to %s\n", cfgPath)
			return nil
		},
	}
	return cmd
}

func setupCalendars(app *App, cfg *config.Config) error {
	items, err := app.Calendar.ListCalendars()
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Println("No calendars found.")
		return nil
	}
	calendars := make([]simpleCalendar, 0, len(items))
	for _, cal := range items {
		calendars = append(calendars, simpleCalendar{Title: cal.Summary, ID: cal.Id, Primary: cal.Primary})
	}

	choices := buildCalendarChoices(calendars)
	options := labelsFromChoices(choices)

	defaultLabel := ""
	for _, choice := range choices {
		if choice.Item.ID == cfg.CalendarID {
			defaultLabel = choice.Label
			break
		}
	}
	if defaultLabel == "" {
		for _, choice := range choices {
			if choice.Item.Primary {
				defaultLabel = choice.Label
				break
			}
		}
	}

	prompt := &survey.Select{
		Message:  "Select calendar",
		Options:  options,
		Default:  defaultLabel,
		PageSize: 12,
	}
	var selected string
	if err := survey.AskOne(prompt, &selected, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	choice, ok := findChoice(choices, selected)
	if !ok {
		return fmt.Errorf("invalid calendar selection")
	}
	cfg.CalendarID = choice.Item.ID
	return nil
}

func setupLists(app *App, cfg *config.Config) error {
	remote, err := app.Tasks.ListTaskLists()
	if err != nil {
		return err
	}
	lists := make([]simpleList, 0, len(remote))
	for _, l := range remote {
		lists = append(lists, simpleList{Title: l.Title, ID: l.Id})
	}
	if len(lists) == 0 {
		created, err := createNewList(app, "Inbox")
		if err != nil {
			return err
		}
		lists = append(lists, created)
	}

	inboxList, err := selectOrCreateList("Choose the list to use as Inbox", app, &lists)
	if err != nil {
		return err
	}
	localName := inboxList.Title
	if _, exists := cfg.Lists[localName]; exists {
		alias, err := askLabel("Local name for Inbox list", localName)
		if err != nil {
			return err
		}
		localName = alias
	}
	cfg.DefaultList = localName
	cfg.Lists[localName] = inboxList.ID

	addMore := false
	if err := survey.AskOne(&survey.Confirm{Message: "Map additional existing lists?", Default: false}, &addMore); err != nil {
		return err
	}
	if addMore {
		remaining := filterLists(lists, cfg)
		if len(remaining) == 0 {
			fmt.Println("No additional lists available.")
		} else {
			choices := buildListChoices(remaining)
			options := labelsFromChoices(choices)
			var selected []string
			prompt := &survey.MultiSelect{
				Message:  "Select lists to map",
				Options:  options,
				PageSize: 12,
			}
			if err := survey.AskOne(prompt, &selected); err != nil {
				return err
			}
			for _, label := range selected {
				choice, ok := findChoice(choices, label)
				if !ok {
					continue
				}
				localName := choice.Item.Title
				if _, exists := cfg.Lists[localName]; exists {
					alias, err := askLabel(fmt.Sprintf("Local name for %s", choice.Item.Title), localName)
					if err != nil {
						return err
					}
					localName = alias
				}
				cfg.Lists[localName] = choice.Item.ID
			}
		}
	}

	for {
		createNew := false
		if err := survey.AskOne(&survey.Confirm{Message: "Create a new Google Tasks list?", Default: false}, &createNew); err != nil {
			return err
		}
		if !createNew {
			break
		}
		name, err := askRequired("New list name")
		if err != nil {
			return err
		}
		created, err := createNewList(app, name)
		if err != nil {
			return err
		}
		lists = append(lists, created)
		localName, err := askLabel(fmt.Sprintf("Local name for %s", created.Title), created.Title)
		if err != nil {
			return err
		}
		cfg.Lists[localName] = created.ID
	}

	return nil
}

func selectOrCreateList(label string, app *App, lists *[]simpleList) (simpleList, error) {
	choices := buildListChoices(*lists)
	options := labelsFromChoices(choices)
	options = append(options, "Add new list...")

	prompt := &survey.Select{
		Message:  label,
		Options:  options,
		PageSize: 12,
	}
	var selected string
	if err := survey.AskOne(prompt, &selected, survey.WithValidator(survey.Required)); err != nil {
		return simpleList{}, err
	}
	if selected == "Add new list..." {
		name, err := askRequired("New list name")
		if err != nil {
			return simpleList{}, err
		}
		created, err := createNewList(app, name)
		if err != nil {
			return simpleList{}, err
		}
		*lists = append(*lists, created)
		return created, nil
	}
	choice, ok := findChoice(choices, selected)
	if !ok {
		return simpleList{}, fmt.Errorf("invalid list selection")
	}
	return choice.Item, nil
}

func createNewList(app *App, name string) (simpleList, error) {
	created, err := app.Tasks.CreateTaskList(name)
	if err != nil {
		return simpleList{}, err
	}
	return simpleList{Title: created.Title, ID: created.Id}, nil
}

func buildListChoices(lists []simpleList) []choiceItem[simpleList] {
	counts := map[string]int{}
	for _, l := range lists {
		counts[l.Title]++
	}
	index := map[string]int{}
	choices := make([]choiceItem[simpleList], 0, len(lists))
	for _, l := range lists {
		label := l.Title
		if counts[l.Title] > 1 {
			index[l.Title]++
			label = fmt.Sprintf("%s (%d)", l.Title, index[l.Title])
		}
		choices = append(choices, choiceItem[simpleList]{Label: label, Item: l})
	}
	sort.SliceStable(choices, func(i, j int) bool { return choices[i].Label < choices[j].Label })
	return choices
}

func buildCalendarChoices(cals []simpleCalendar) []choiceItem[simpleCalendar] {
	counts := map[string]int{}
	for _, c := range cals {
		counts[c.Title]++
	}
	index := map[string]int{}
	choices := make([]choiceItem[simpleCalendar], 0, len(cals))
	for _, c := range cals {
		label := c.Title
		if c.Primary {
			label = fmt.Sprintf("%s (primary)", label)
		}
		if counts[c.Title] > 1 {
			index[c.Title]++
			label = fmt.Sprintf("%s (%d)", label, index[c.Title])
		}
		choices = append(choices, choiceItem[simpleCalendar]{Label: label, Item: c})
	}
	sort.SliceStable(choices, func(i, j int) bool { return choices[i].Label < choices[j].Label })
	return choices
}

func labelsFromChoices[T any](choices []choiceItem[T]) []string {
	labels := make([]string, 0, len(choices))
	for _, choice := range choices {
		labels = append(labels, choice.Label)
	}
	return labels
}

func findChoice[T any](choices []choiceItem[T], label string) (choiceItem[T], bool) {
	for _, choice := range choices {
		if choice.Label == label {
			return choice, true
		}
	}
	var zero choiceItem[T]
	return zero, false
}

func askLabel(message, defaultValue string) (string, error) {
	var input string
	prompt := &survey.Input{Message: message, Default: defaultValue}
	if err := survey.AskOne(prompt, &input, survey.WithValidator(survey.Required)); err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func askRequired(message string) (string, error) {
	var input string
	prompt := &survey.Input{Message: message}
	if err := survey.AskOne(prompt, &input, survey.WithValidator(survey.Required)); err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func filterLists(lists []simpleList, cfg *config.Config) []simpleList {
	used := map[string]bool{}
	for _, id := range cfg.Lists {
		used[id] = true
	}
	var remaining []simpleList
	for _, l := range lists {
		if !used[l.ID] {
			remaining = append(remaining, l)
		}
	}
	return remaining
}

func printSection(title string) {
	fmt.Printf("\n\033[1m%s\033[0m\n", title)
}
