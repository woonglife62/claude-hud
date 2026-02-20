//go:build !windows

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"claude-hud/internal/config"
	"claude-hud/internal/i18n"
	"claude-hud/internal/platform"
)

// SetupTray configures the system tray icon and menu for macOS/Linux.
func SetupTray(h *HUDApp) {
	desk, ok := h.fyneApp.(desktop.App)
	if !ok {
		platform.Log("System tray not supported on this platform")
		return
	}

	menu := buildTrayMenu(h)
	desk.SetSystemTrayMenu(menu)
	platform.Log("System tray configured")
}

func buildTrayMenu(h *HUDApp) *fyne.Menu {
	showLabel := "Show HUD"
	if h.visible {
		showLabel = "Hide HUD"
	}
	showHide := fyne.NewMenuItem(showLabel, func() {
		if h.visible {
			h.window.Hide()
			h.visible = false
		} else {
			h.window.Show()
			h.visible = true
		}
		refreshTrayMenu(h)
	})

	autoStart := fyne.NewMenuItem("Auto Start", func() {
		h.cfg.AutoStart = !h.cfg.AutoStart
		platform.SetAutoStart(h.cfg.AutoStart)
		config.SaveConfig(h.cfg)
		refreshTrayMenu(h)
	})
	autoStart.Checked = platform.IsAutoStartEnabled()

	notifications := fyne.NewMenuItem(i18n.T.Notifications, func() {
		h.cfg.NotifyEnabled = !h.cfg.NotifyEnabled
		config.SaveConfig(h.cfg)
		refreshTrayMenu(h)
	})
	notifications.Checked = h.cfg.NotifyEnabled

	compactMode := fyne.NewMenuItem("Compact Mode", func() {
		h.cfg.CompactMode = !h.cfg.CompactMode
		config.SaveConfig(h.cfg)
		h.rebuildUI()
		refreshTrayMenu(h)
	})
	compactMode.Checked = h.cfg.CompactMode

	themeLabel := "Light Theme"
	if h.cfg.ThemeMode == "light" {
		themeLabel = "Dark Theme"
	}
	themeToggle := fyne.NewMenuItem(themeLabel, func() {
		if h.cfg.ThemeMode == "light" {
			h.cfg.ThemeMode = "dark"
			h.colors = FyneDarkColors
		} else {
			h.cfg.ThemeMode = "light"
			h.colors = FyneLightColors
		}
		h.fyneApp.Settings().SetTheme(&hudTheme{
			dark:   h.cfg.ThemeMode != "light",
			colors: h.colors,
		})
		config.SaveConfig(h.cfg)
		h.rebuildUI()
		refreshTrayMenu(h)
	})

	langToggle := fyne.NewMenuItem(i18n.T.LanguageToggle, func() {
		if h.cfg.Language == "en" {
			h.cfg.Language = "ko"
		} else {
			h.cfg.Language = "en"
		}
		i18n.SetLanguage(h.cfg.Language)
		config.SaveConfig(h.cfg)
		h.rebuildUI()
		refreshTrayMenu(h)
	})

	exit := fyne.NewMenuItem(i18n.T.Exit, func() {
		config.SaveConfig(h.cfg)
		h.fyneApp.Quit()
	})

	return fyne.NewMenu("Claude HUD",
		showHide,
		fyne.NewMenuItemSeparator(),
		autoStart,
		notifications,
		fyne.NewMenuItemSeparator(),
		compactMode,
		themeToggle,
		langToggle,
		fyne.NewMenuItemSeparator(),
		exit,
	)
}

func refreshTrayMenu(h *HUDApp) {
	desk, ok := h.fyneApp.(desktop.App)
	if !ok {
		return
	}
	desk.SetSystemTrayMenu(buildTrayMenu(h))
}
