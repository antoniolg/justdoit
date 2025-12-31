package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"justdoit/internal/config"
	"justdoit/internal/paths"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage local configuration",
	}
	cmd.AddCommand(newConfigInitCmd())
	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigCalendarsCmd())
	cmd.AddCommand(newConfigCalendarSetCmd())
	cmd.AddCommand(newConfigListsCmd())
	return cmd
}

func newConfigInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a default config.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveConfigPath(cmd)
			if err != nil {
				return err
			}
			if !force {
				if _, err := os.Stat(path); err == nil {
					return fmt.Errorf("config already exists: %s", path)
				}
			}
			cfg := config.Default()
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Printf("Config written: %s\n", path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config")
	return cmd
}

func newConfigShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show current config",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveConfigPath(cmd)
			if err != nil {
				return err
			}
			cfg, err := config.LoadOrCreate(path)
			if err != nil {
				return err
			}
			data, err := json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", path)
			fmt.Printf("%s\n", string(data))
			return nil
		},
	}
	return cmd
}

func newConfigCalendarsCmd() *cobra.Command {
	var showIDs bool
	cmd := &cobra.Command{
		Use:   "calendars",
		Short: "List available calendars",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd)
			if err != nil {
				return err
			}
			items, err := app.Calendar.ListCalendars()
			if err != nil {
				return err
			}
			if len(items) == 0 {
				fmt.Println("(none)")
				return nil
			}
			for _, cal := range items {
				primary := ""
				if cal.Primary {
					primary = " (primary)"
				}
				if showIDs {
					fmt.Printf("- %s%s\n  id: %s\n", cal.Summary, primary, cal.Id)
				} else {
					fmt.Printf("- %s%s\n", cal.Summary, primary)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&showIDs, "ids", false, "Show calendar IDs")
	return cmd
}

func newConfigCalendarSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-calendar [calendarID]",
		Short: "Set calendar_id in config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveConfigPath(cmd)
			if err != nil {
				return err
			}
			cfg, err := config.LoadOrCreate(path)
			if err != nil {
				return err
			}
			cfg.CalendarID = args[0]
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Printf("calendar_id updated: %s\n", cfg.CalendarID)
			return nil
		},
	}
	return cmd
}

func newConfigListsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lists",
		Short: "Manage Google Tasks list mappings",
	}
	cmd.AddCommand(newConfigListsListCmd())
	cmd.AddCommand(newConfigListsAddCmd())
	cmd.AddCommand(newConfigListsRemoveCmd())
	cmd.AddCommand(newConfigListsRemoteCmd())
	cmd.AddCommand(newConfigListsCreateCmd())
	return cmd
}

func newConfigListsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List local list mappings",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveConfigPath(cmd)
			if err != nil {
				return err
			}
			cfg, err := config.LoadOrCreate(path)
			if err != nil {
				return err
			}
			keys := make([]string, 0, len(cfg.Lists))
			for k := range cfg.Lists {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("- %s: %s\n", k, cfg.Lists[k])
			}
			return nil
		},
	}
	return cmd
}

func newConfigListsAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [name] [listID]",
		Short: "Add a local list mapping",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveConfigPath(cmd)
			if err != nil {
				return err
			}
			cfg, err := config.LoadOrCreate(path)
			if err != nil {
				return err
			}
			cfg.Lists[args[0]] = args[1]
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Printf("Added list mapping: %s -> %s\n", args[0], args[1])
			return nil
		},
	}
	return cmd
}

func newConfigListsRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove [name]",
		Short: "Remove a local list mapping",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveConfigPath(cmd)
			if err != nil {
				return err
			}
			cfg, err := config.LoadOrCreate(path)
			if err != nil {
				return err
			}
			delete(cfg.Lists, args[0])
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Printf("Removed list mapping: %s\n", args[0])
			return nil
		},
	}
	return cmd
}

func newConfigListsRemoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "List Google Tasks lists from the API",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd)
			if err != nil {
				return err
			}
			lists, err := app.Tasks.ListTaskLists()
			if err != nil {
				return err
			}
			if len(lists) == 0 {
				fmt.Println("(none)")
				return nil
			}
			for _, l := range lists {
				fmt.Printf("- %s\n  id: %s\n", l.Title, l.Id)
			}
			return nil
		},
	}
	return cmd
}

func newConfigListsCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a Google Tasks list and add mapping",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd)
			if err != nil {
				return err
			}
			list, err := app.Tasks.CreateTaskList(args[0])
			if err != nil {
				return err
			}
			path, err := resolveConfigPath(cmd)
			if err != nil {
				return err
			}
			cfg, err := config.LoadOrCreate(path)
			if err != nil {
				return err
			}
			cfg.Lists[list.Title] = list.Id
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Printf("Created list: %s (id: %s)\n", list.Title, list.Id)
			return nil
		},
	}
	return cmd
}

func resolveConfigPath(cmd *cobra.Command) (string, error) {
	cfgPath, _ := cmd.Flags().GetString("config")
	if cfgPath == "" {
		return paths.ConfigPath()
	}
	return cfgPath, nil
}
