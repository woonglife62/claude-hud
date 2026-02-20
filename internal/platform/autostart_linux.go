//go:build linux

package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

func desktopFilePath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "autostart", "claude-hud.desktop")
}

// SetAutoStart creates or removes an XDG .desktop file for auto-start at login.
func SetAutoStart(enable bool) error {
	path := desktopFilePath()
	if !enable {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	desktop := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Claude HUD
Comment=Claude Code Desktop Monitor
Exec=%s
Terminal=false
X-GNOME-Autostart-enabled=true
`, exePath)

	os.MkdirAll(filepath.Dir(path), 0755)
	return os.WriteFile(path, []byte(desktop), 0644)
}

// IsAutoStartEnabled checks if the .desktop autostart file exists.
func IsAutoStartEnabled() bool {
	_, err := os.Stat(desktopFilePath())
	return err == nil
}
