package paths

import (
	"os"
	"path/filepath"
)

const (
	appDirName = "justdoit"
	configFile = "config.json"
	tokenFile  = "token.json"
	credsFile  = "credentials.json"
)

func ConfigDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, appDirName), nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		legacy := filepath.Join(home, ".config", appDirName)
		if _, err := os.Stat(legacy); err == nil {
			return legacy, nil
		}
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, appDirName), nil
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFile), nil
}

func TokenPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, tokenFile), nil
}

func CredentialsPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, credsFile), nil
}
