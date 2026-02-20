//go:build !windows

package main

import (
	"fmt"
	"runtime"
)

func main() {
	fmt.Printf("Claude HUD is currently only supported on Windows.\n")
	fmt.Printf("Current platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println("macOS and Linux support is planned.")
	fmt.Println("See: https://github.com/woonglife62/claude-hud/issues/19")
}
