@echo off
echo Building Claude HUD...
echo.

:: Ensure Go modules are set up
go mod tidy

:: Build for Windows (GUI mode - no console window)
set GOOS=windows
set GOARCH=amd64
go build -ldflags "-H windowsgui -s -w" -o claude-hud.exe .

if %ERRORLEVEL% EQU 0 (
    echo.
    echo Build successful! Output: claude-hud.exe
    echo.
    echo Run claude-hud.exe to start the desktop widget.
) else (
    echo.
    echo Build failed! Check errors above.
)
pause
