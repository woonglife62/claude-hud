//go:build !windows

package platform

import (
	"fmt"
	"os"
)

// ShowMessageBox prints an error message to stderr on non-Windows platforms.
func ShowMessageBox(title, message string) {
	fmt.Fprintf(os.Stderr, "[ERROR] %s: %s\n", title, message)
}

// ShowInfoBox prints an info message to stderr on non-Windows platforms.
func ShowInfoBox(title, message string) {
	fmt.Fprintf(os.Stderr, "[INFO] %s: %s\n", title, message)
}
