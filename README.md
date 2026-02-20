# Claude HUD

Windows desktop overlay that monitors Claude Code session status in real-time. A native HUD application built with pure Go and Win32 API — no CGo, no frameworks.

![Windows](https://img.shields.io/badge/platform-Windows%2010%2F11-blue)
![Go](https://img.shields.io/badge/Go-1.22+-00ADD8)
![License](https://img.shields.io/badge/license-Private-lightgrey)

## Features

### Monitoring
- **Real-time usage tracking** — 5-hour / weekly token usage via Anthropic OAuth API
- **Model breakdown** — Per-model token usage (Opus / Sonnet / Haiku)
- **Plan-aware limits** — Accurate token caps based on your subscription plan
- **Usage trend indicator** — Consumption velocity arrows (fast/slow/stable)
- **Session scanning** — Active JSONL transcript detection in `~/.claude/projects/`
- **Agent lifecycle** — `tool_use` / `tool_result` matching for running/completed status
- **OMC strategy detection** — Active oh-my-claudecode skill display (ralph, ultrawork, autopilot, etc.)

### UI
- **DPI-aware rendering** — Per-monitor DPI scaling (Windows 10+ `SetProcessDpiAwarenessContext`)
- **Dark / Light theme** — Toggle via system tray menu
- **Compact mode** — Minimal view showing only usage bars (~150px height)
- **Resizable window** — Drag to resize, minimum 200x300
- **Edge snapping** — Auto-snap to monitor edges
- **Desktop pinning** — Pin to desktop background (Progman child window)
- **i18n** — Korean (default) and English locale support

### System Integration
- **System tray** — Tray icon with show/hide toggle and context menu
- **Usage notifications** — Balloon alerts at configurable thresholds (80%, 95%)
- **Auto-start** — Windows registry-based auto-start on login
- **Global hotkey** — `Ctrl+Shift+H` to toggle HUD visibility
- **Single instance** — Named Mutex prevents duplicate processes
- **GDI resource caching** — Brushes/pens created once, reused across frames
- **Non-blocking refresh** — Background goroutine handles data I/O, no UI thread blocking
- **OAuth error handling** — Graceful token expiry recovery with exponential backoff

## Build

### Requirements

- Go 1.22+
- Windows 10 / 11
- `windres` (MinGW or MSYS2, for icon embedding — optional)

### Quick Build

```bash
# Build without icon
go build -ldflags "-H windowsgui" -o claude-hud.exe

# Build with icon (requires windres)
make icon
make build
```

The `-H windowsgui` flag ensures the app runs as a GUI application without a console window.

### Icon Generation

```bash
# Generate icon.ico (purple circle + white C, mathematical rendering)
go run tools/mkicon.go

# Compile Windows resource
windres resource.rc -o resource.syso
```

### Make Targets

| Target | Description |
|--------|-------------|
| `make build` | Build `claude-hud.exe` with embedded icon |
| `make icon` | Generate `icon.ico` + compile `resource.syso` |
| `make clean` | Remove build artifacts |

### Run Tests

```bash
go test ./...
```

## Run

```bash
./claude-hud.exe
```

Or double-click the executable. Enable "Start with Windows" from the tray menu for auto-launch.

## Configuration

Config file location: `%AppData%/ClaudeHUD/config.json`

Window position and size are automatically saved on exit.

| Setting | Default | Description |
|---------|---------|-------------|
| `x`, `y` | 100, 100 | Window position |
| `width`, `height` | 360, 640 | Window size |
| `opacity` | 204 (80%) | Transparency (0-255) |
| `refresh_ms` | 3000 | Data refresh interval in ms |
| `pin_desktop` | true | Pin to desktop background |
| `theme_mode` | "dark" | Color theme: `"dark"` or `"light"` |
| `auto_start` | false | Start with Windows login |
| `notify_enabled` | true | Enable usage notifications |
| `notify_threshold_1` | 0.80 | First notification threshold |
| `notify_threshold_2` | 0.95 | Warning notification threshold |
| `closed_retention_min` | 5 | Minutes to show closed sessions |
| `language` | "ko" | UI language: `"ko"` or `"en"` |
| `compact_mode` | false | Compact mode (usage bars only) |

## Project Structure

```
claude-hud/
├── main.go                          # Entry point, single instance, panic recovery
├── internal/
│   ├── config/
│   │   ├── config.go                # Config struct, load/save, defaults, validation
│   │   └── config_test.go           # Config round-trip tests
│   ├── model/
│   │   ├── types.go                 # HUDData, Session, Agent, UsageData, RateLimitWindow
│   │   └── types_test.go            # Type helper tests
│   ├── data/
│   │   ├── api.go                   # OAuth API calls, usage cache, model breakdown, trends
│   │   ├── scanner.go               # JSONL session scanning, mtime cache, incremental reads
│   │   └── scanner_test.go          # Scanner and cache tests
│   ├── ui/
│   │   ├── winapi.go                # Win32 DLL procs and constants (user32, gdi32, kernel32, shell32)
│   │   ├── window.go                # HUD window, message loop, DPI change handling, hotkey
│   │   ├── render.go                # GDI double-buffer rendering, usage bars, session cards
│   │   ├── tray.go                  # System tray icon, balloon notifications, context menu
│   │   └── colors.go                # Dark/Light color schemes
│   ├── i18n/
│   │   └── i18n.go                  # Korean/English locale strings, SetLanguage()
│   └── platform/
│       ├── autostart.go             # Windows registry auto-start (advapi32)
│       ├── desktop.go               # Desktop pinning (Progman)
│       └── debug.go                 # Diagnostic logging, message box
├── tools/
│   └── mkicon.go                    # ICO file generator (mathematical rendering)
├── resource.rc                      # Windows resource definition
├── Makefile                         # Build automation
└── go.mod
```

### Package Dependency Graph

```
main
 ├── config      (no internal deps)
 ├── model       (no internal deps)
 ├── i18n        (no internal deps)
 ├── platform    (no internal deps)
 ├── data     → config, model, platform
 └── ui       → config, model, data, platform, i18n
```

No circular dependencies. `config`, `model`, `i18n`, and `platform` are leaf packages.

## Data Sources

| Data | Source | Method |
|------|--------|--------|
| Usage | Anthropic OAuth API | `GET /api/oauth/usage` (Bearer token) |
| Usage (fallback) | OMC cache | `~/.claude/plugins/oh-my-claudecode/.usage-cache.json` |
| Plan info | Credentials | `~/.claude/.credentials.json` → `rateLimitTier` |
| Sessions | JSONL scan | `~/.claude/projects/*/*.jsonl` (mtime cache, incremental) |
| Agents | JSONL parse | Last 256KB, `tool_use`/`tool_result` lifecycle tracking |
| OMC strategy | JSONL parse | Skill `tool_use` blocks → last active skill |

## Session Display

Click a session card to expand and see details:

- **Active strategy** — `⚡ ralph`, `⚡ ultrawork`, etc.
- **Running agents** — Green dot + "Running" (awaiting `tool_result`)
- **Completed agents** — Gray dot + "Done" (`tool_result` received)
- **Agent info** — Name, model (sonnet/opus/haiku), task description
- **Stale detection** — Agents running >30min are auto-marked as completed

## Diagnostics

Log file: `%AppData%/ClaudeHUD/claude-hud.log`

The log includes Go runtime info, struct sizes, DPI state, and window lifecycle events.

## License

Private repository.
