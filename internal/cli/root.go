package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"justdoit/internal/auth"
	"justdoit/internal/config"
	"justdoit/internal/google/calendar"
	"justdoit/internal/google/tasks"
	"justdoit/internal/paths"
	"justdoit/internal/sync"
	"justdoit/internal/timeparse"
)

type App struct {
	Config   *config.Config
	Tasks    *tasks.Client
	Calendar *calendar.Client
	Sync     *sync.Wrapper
	Location *time.Location
	Now      time.Time
}

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "justdoit",
		Short: "CLI for time-blocking with Google Tasks + Calendar",
	}
	cmd.PersistentFlags().String("config", "", "Path to config.json (defaults to ~/.config/justdoit/config.json)")
	cmd.PersistentFlags().String("credentials", "", "Path to OAuth credentials.json (defaults to ~/.config/justdoit/credentials.json)")

	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newDeleteCmd())
	cmd.AddCommand(newDoneCmd())
	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newViewCmd())
	cmd.AddCommand(newSetupCmd())

	return cmd
}

func initApp(cmd *cobra.Command) (*App, error) {
	cfgPath, _ := cmd.Flags().GetString("config")
	if cfgPath == "" {
		var err error
		cfgPath, err = paths.ConfigPath()
		if err != nil {
			return nil, err
		}
	}
	cfg, err := config.LoadOrCreate(cfgPath)
	if err != nil {
		return nil, err
	}
	loc, err := timeparse.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, err
	}
	credPath, _ := cmd.Flags().GetString("credentials")
	if credPath == "" {
		credPath, err = paths.CredentialsPath()
		if err != nil {
			return nil, err
		}
	}
	tokenPath, err := paths.TokenPath()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	httpClient, err := auth.Client(ctx, credPath, tokenPath)
	if err != nil {
		return nil, fmt.Errorf("auth failed: %w", err)
	}
	tasksClient, err := tasks.New(ctx, httpClient)
	if err != nil {
		return nil, err
	}
	calendarClient, err := calendar.New(ctx, httpClient)
	if err != nil {
		return nil, err
	}
	syncer := &sync.Wrapper{
		Tasks:      tasksClient,
		Calendar:   calendarClient,
		CalendarID: cfg.CalendarID,
	}
	return &App{
		Config:   cfg,
		Tasks:    tasksClient,
		Calendar: calendarClient,
		Sync:     syncer,
		Location: loc,
		Now:      time.Now().In(loc),
	}, nil
}
