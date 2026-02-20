//go:build !windows

package main

import (
	"fmt"
	"os"
	"runtime"

	"claude-hud/internal/config"
	"claude-hud/internal/data"
	"claude-hud/internal/i18n"
	"claude-hud/internal/platform"
	"claude-hud/internal/ui"
)

func main() {
	platform.InitLog()
	defer platform.CloseLog()

	platform.Log("=== Claude HUD starting ===")
	platform.Log("Go runtime: %s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	defer func() {
		if r := recover(); r != nil {
			platform.Log("PANIC: %v", r)
			fmt.Fprintf(os.Stderr, "Claude HUD panic: %v\n", r)
		}
	}()

	cfg := config.LoadConfig()
	platform.Log("Config loaded: %dx%d, theme=%s, lang=%s",
		cfg.Width, cfg.Height, cfg.ThemeMode, cfg.Language)

	i18n.SetLanguage(cfg.Language)

	hudData := data.LoadRealData()
	platform.Log("Data loaded: %d sessions, plan=%s", len(hudData.Sessions), hudData.Usage.Plan)

	ui.RunApp(cfg, hudData)

	platform.Log("=== Claude HUD exited normally ===")
}
