//go:build darwin

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const launchAgentLabel = "com.claude-hud"

func launchAgentPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

// SetAutoStart creates or removes a macOS LaunchAgent plist for auto-start at login.
func SetAutoStart(enable bool) error {
	path := launchAgentPath()
	if !enable {
		// Unload from launchctl before removing the plist
		exec.Command("launchctl", "unload", path).Run()
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

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<false/>
</dict>
</plist>
`, launchAgentLabel, exePath)

	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(plist), 0644); err != nil {
		return err
	}
	// Register with launchctl so it takes effect without logout
	exec.Command("launchctl", "load", path).Run()
	return nil
}

// IsAutoStartEnabled checks if the LaunchAgent plist exists.
func IsAutoStartEnabled() bool {
	_, err := os.Stat(launchAgentPath())
	return err == nil
}
