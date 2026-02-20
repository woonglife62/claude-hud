#!/bin/bash
# Cross-compile Claude HUD for Windows from Linux/Mac
echo "Building Claude HUD for Windows..."

# Ensure dependencies
go mod tidy

# Cross-compile
GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui -s -w" -o claude-hud.exe .

if [ $? -eq 0 ]; then
    echo "Build successful! Output: claude-hud.exe"
    echo "Transfer claude-hud.exe to your Windows machine and run it."
else
    echo "Build failed!"
    exit 1
fi
