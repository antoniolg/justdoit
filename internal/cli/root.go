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
	Config     *config.Config
	ConfigPath string
	CachePath  string
	Tasks      *tasks.Client
	Calendar   *calendar.Client
	Sync       *sync.Wrapper
	Location   *time.Location
}

// Now returns the current time in the app's configured location.
// Always use this instead of caching time at startup.
func (a *App) Now() time.Time {
	return time.Now().In(a.Location)
}

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "justdoit",
		Short: "CLI for time-blocking with Google Tasks + Calendar",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp(cmd)
			if err != nil {
				return err
			}
			return startTUI(app)
		},
	}
	cmd.PersistentFlags().String("config", "", "Path to config.json (defaults to ~/.config/justdoit/config.json)")
	cmd.PersistentFlags().String("credentials", "", "Path to OAuth credentials.json (defaults to ~/.config/justdoit/credentials.json)")

	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newDeleteCmd())
	cmd.AddCommand(newDoneCmd())
	cmd.AddCommand(newUndoCmd())
	cmd.AddCommand(newMoveCmd())
	cmd.AddCommand(newNextCmd())
	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newSearchCmd())
	cmd.AddCommand(newSectionCmd())
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
	cachePath, err := paths.CachePath()
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
		Config:     cfg,
		ConfigPath: cfgPath,
		CachePath:  cachePath,
		Tasks:      tasksClient,
		Calendar:   calendarClient,
		Sync:       syncer,
		Location:   loc,
	}, nil
}

func (a *App) SaveConfig() error {
	if a == nil || a.Config == nil || a.ConfigPath == "" {
		return fmt.Errorf("config is not initialized")
	}
	return config.Save(a.ConfigPath, a.Config)
}
