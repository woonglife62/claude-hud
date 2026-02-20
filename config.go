package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds user-configurable settings
type Config struct {
	X           int     `json:"x"`
	Y           int     `json:"y"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	Opacity     byte    `json:"opacity"`     // 0-255, default 204 (80%)
	RefreshMs   int     `json:"refresh_ms"`  // data refresh interval
	PinDesktop  bool    `json:"pin_desktop"` // pin to desktop background
	DarkMode    bool    `json:"dark_mode"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		X:          100,
		Y:          100,
		Width:      360,
		Height:     640,
		Opacity:    204, // ~80%
		RefreshMs:  3000,
		PinDesktop: true,
		DarkMode:   true,
	}
}

// configPath returns the path to the config file
func configPath() string {
	appData, _ := os.UserConfigDir()
	dir := filepath.Join(appData, "ClaudeHUD")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "config.json")
}

// LoadConfig loads configuration from disk, or returns defaults
func LoadConfig() *Config {
	cfg := DefaultConfig()
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	json.Unmarshal(data, cfg)
	// Validate
	if cfg.Width < 200 {
		cfg.Width = 200
	}
	if cfg.Height < 300 {
		cfg.Height = 300
	}
	if cfg.Opacity < 50 {
		cfg.Opacity = 50
	}
	if cfg.RefreshMs < 1000 {
		cfg.RefreshMs = 1000
	}
	return cfg
}

// SaveConfig writes configuration to disk
func SaveConfig(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0644)
}
