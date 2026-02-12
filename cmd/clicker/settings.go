package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type uiSettings struct {
	MinCPS  float64 `json:"min_cps"`
	MaxCPS  float64 `json:"max_cps"`
	Trigger string  `json:"trigger"`
	Toggle  string  `json:"toggle"`
	Enabled bool    `json:"enabled"`
}

func uiSettingsPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		return filepath.Join(".", ".clicker-settings.json"), nil
	}
	return filepath.Join(configDir, "clicker", "settings.json"), nil
}

func loadUISettings() (*uiSettings, error) {
	path, err := uiSettingsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var cfg uiSettings
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse settings %s: %w", path, err)
	}
	return &cfg, nil
}

func saveUISettings(cfg uiSettings) error {
	path, err := uiSettingsPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create settings dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("failed to persist settings: %w", err)
	}

	return nil
}
