package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var logFile *os.File

// logDir returns the OS-appropriate directory for log files.
func logDir() string {
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Logs", "claude-hud")
	case "linux":
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "claude-hud")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", "claude-hud")
	default:
		// Windows: write next to executable (critical for -H windowsgui)
		exePath, _ := os.Executable()
		return filepath.Dir(exePath)
	}
}

// InitLog creates a log file in the OS-appropriate directory for diagnostics.
// Windows: next to the executable (critical because -H windowsgui hides stdout/stderr).
// macOS: ~/Library/Logs/claude-hud.log
// Linux: $XDG_DATA_HOME/claude-hud/ or ~/.local/share/claude-hud/
func InitLog() {
	dir := logDir()
	os.MkdirAll(dir, 0755)
	logPath := filepath.Join(dir, "claude-hud.log")
	var err error
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		// Last resort: try temp directory
		logPath = filepath.Join(os.TempDir(), "claude-hud.log")
		logFile, _ = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	}
}

// Log writes a timestamped message to the log file
func Log(format string, args ...interface{}) {
	if logFile == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("15:04:05.000")
	fmt.Fprintf(logFile, "[%s] %s\n", timestamp, msg)
	logFile.Sync() // Flush immediately so crash doesn't lose logs
}

// CloseLog closes the log file
func CloseLog() {
	if logFile != nil {
		logFile.Close()
	}
}
