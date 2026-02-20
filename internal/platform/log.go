package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var logFile *os.File

// InitLog creates a log file next to the executable for diagnostics.
// On Windows this is critical because -H windowsgui hides all stdout/stderr output.
// TODO(cross-platform): On macOS/Linux, consider using ~/.local/share/claude-hud/
// or XDG_DATA_HOME instead of writing next to the binary.
func InitLog() {
	exePath, _ := os.Executable()
	logPath := filepath.Join(filepath.Dir(exePath), "claude-hud.log")
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
