package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	CalendarID    string            `json:"calendar_id"`
	DefaultList   string            `json:"default_list"`
	ViewCalendars []string          `json:"view_calendars"`
	WorkdayStart  string            `json:"workday_start"`
	WorkdayEnd    string            `json:"workday_end"`
	Timezone      string            `json:"timezone"`
	Lists         map[string]string `json:"lists"`
}

func Load(path string) (*Config, error) {
	// #nosec G304 -- path is controlled by the app config location
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	normalize(&cfg)
	return &cfg, nil
}

func (c *Config) ListID(name string) (string, bool) {
	id, ok := c.Lists[name]
	return id, ok
}

func Save(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func Default() *Config {
	return &Config{
		CalendarID:    "primary",
		DefaultList:   "Inbox",
		ViewCalendars: nil,
		WorkdayStart:  "09:00",
		WorkdayEnd:    "18:00",
		Timezone:      "local",
		Lists:         map[string]string{},
	}
}

func LoadOrCreate(path string) (*Config, error) {
	// #nosec G304 -- path is controlled by the app config location
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := Default()
			if err := Save(path, cfg); err != nil {
				return nil, err
			}
			return cfg, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	normalize(&cfg)
	if err := Save(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func normalize(cfg *Config) {
	if cfg.CalendarID == "" {
		cfg.CalendarID = "primary"
	}
	if cfg.DefaultList == "" {
		cfg.DefaultList = "Inbox"
	}
	if cfg.WorkdayStart == "" {
		cfg.WorkdayStart = "09:00"
	}
	if cfg.WorkdayEnd == "" {
		cfg.WorkdayEnd = "18:00"
	}
	if cfg.Timezone == "" {
		cfg.Timezone = "local"
	}
	if cfg.Lists == nil {
		cfg.Lists = map[string]string{}
	}
	if len(cfg.ViewCalendars) == 0 {
		cfg.ViewCalendars = []string{cfg.CalendarID}
		return
	}
	seen := make(map[string]bool, len(cfg.ViewCalendars))
	filtered := make([]string, 0, len(cfg.ViewCalendars))
	for _, id := range cfg.ViewCalendars {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		filtered = append(filtered, id)
	}
	if len(filtered) == 0 {
		cfg.ViewCalendars = []string{cfg.CalendarID}
		return
	}
	cfg.ViewCalendars = filtered
}
