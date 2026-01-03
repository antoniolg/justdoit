package cache

import (
	"encoding/json"
	"os"
	"path/filepath"

	"google.golang.org/api/calendar/v3"
)

type Cache struct {
	Version      int                       `json:"version"`
	CalendarMeta map[string]CalendarMeta   `json:"calendar_meta"`
	Calendars    map[string]*CalendarCache `json:"calendars"`
	Tasks        *TasksCache               `json:"tasks"`
	SyncedAt     string                    `json:"synced_at"`
}

type CalendarMeta struct {
	Name    string `json:"name"`
	Primary bool   `json:"primary"`
}

type CalendarCache struct {
	SyncToken string                     `json:"sync_token"`
	Events    map[string]*calendar.Event `json:"events"`
}

type TasksCache struct {
	UpdatedMin string                          `json:"updated_min"`
	Lists      map[string]map[string]TaskEntry `json:"lists"`
}

type TaskEntry struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Notes   string `json:"notes"`
	Parent  string `json:"parent"`
	Status  string `json:"status"`
	Due     string `json:"due"`
	Updated string `json:"updated"`
}

func Default() *Cache {
	return &Cache{
		Version:      1,
		CalendarMeta: map[string]CalendarMeta{},
		Calendars:    map[string]*CalendarCache{},
		Tasks: &TasksCache{
			Lists: map[string]map[string]TaskEntry{},
		},
	}
}

func Load(path string) (*Cache, error) {
	// #nosec G304 -- path is controlled by the app config/cache location
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, err
	}
	var c Cache
	if err := json.Unmarshal(data, &c); err != nil {
		return Default(), nil
	}
	ensureDefaults(&c)
	return &c, nil
}

func Save(path string, cache *Cache) error {
	if cache == nil {
		return nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func ensureDefaults(cache *Cache) {
	if cache.Version == 0 {
		cache.Version = 1
	}
	if cache.CalendarMeta == nil {
		cache.CalendarMeta = map[string]CalendarMeta{}
	}
	if cache.Calendars == nil {
		cache.Calendars = map[string]*CalendarCache{}
	}
	if cache.Tasks == nil {
		cache.Tasks = &TasksCache{}
	}
	if cache.Tasks.Lists == nil {
		cache.Tasks.Lists = map[string]map[string]TaskEntry{}
	}
}
