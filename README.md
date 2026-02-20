# Claude HUD

Windows 데스크톱 오버레이로 Claude Code 세션 상태를 실시간 모니터링하는 네이티브 HUD 애플리케이션.

## 기능

- **실시간 사용량 모니터링** - Anthropic OAuth API를 통한 5시간/주간 토큰 사용량 추적
- **세션 스캔** - `~/.claude/projects/` 디렉토리의 활성 JSONL 트랜스크립트 감지
- **에이전트 라이프사이클 추적** - `tool_use` → `tool_result` 매칭으로 실행 중/완료 상태 표시
- **OMC 전략 감지** - ralph, ultrawork, autopilot 등 활성 oh-my-claudecode 스킬 표시
- **백그라운드 에이전트** - 비동기 에이전트 실행/완료 추적
- **리사이즈 가능** - 드래그로 창 크기 조절, 최소 280x300
- **화면 가장자리 스냅** - 모니터 경계에 자동 맞춤
- **시스템 트레이** - 트레이 아이콘으로 표시/숨김 토글
- **데스크톱 고정** - 바탕화면 배경에 고정 (Progman 자식 윈도우)
- **비차단 갱신** - 백그라운드 고루틴에서 데이터 I/O, UI 스레드 차단 없음
- **단일 인스턴스** - Named Mutex로 중복 실행 방지

## 빌드

### 요구사항

- Go 1.21+
- Windows 10/11
- `windres` (MinGW 또는 MSYS2, 아이콘 임베딩용)

### 빌드 방법

```bash
# 아이콘 생성 + 리소스 컴파일 + 빌드
make icon
make build

# 또는 아이콘 없이 빌드
go build -ldflags "-H windowsgui" -o claude-hud.exe
```

`-H windowsgui` 플래그는 콘솔 창 없이 GUI 앱으로 실행되게 합니다.

### 아이콘 생성

```bash
# 수학적 렌더링으로 icon.ico 생성 (보라 원 + 흰색 C)
go run tools/mkicon.go

# 리소스 컴파일
windres resource.rc -o resource.syso
```

## 실행

```bash
./claude-hud.exe
```

더블클릭으로 실행하거나, 시작 프로그램에 등록하여 자동 실행할 수 있습니다.

## 설정

설정 파일: `%AppData%/ClaudeHUD/config.json`

| 항목 | 기본값 | 설명 |
|------|--------|------|
| `x`, `y` | 100, 100 | 창 위치 |
| `width`, `height` | 360, 640 | 창 크기 |
| `opacity` | 204 (80%) | 투명도 (0-255) |
| `refresh_ms` | 3000 | 데이터 갱신 주기 (ms) |
| `pin_desktop` | true | 바탕화면에 고정 |
| `dark_mode` | true | 다크 테마 |

창 위치와 크기는 종료 시 자동 저장됩니다.

## 프로젝트 구조

```
claude-hud/
├── main.go          # 진입점, 단일 인스턴스, 패닉 복구
├── window.go        # Win32 윈도우 관리, 메시지 루프, 비동기 갱신
├── render.go        # GDI 더블버퍼 렌더링, 사용량 바, 세션 카드
├── data.go          # 데이터 모델, API 호출, JSONL 파싱, 에이전트 추적
├── config.go        # 설정 로드/저장
├── tray.go          # 시스템 트레이 아이콘
├── desktop.go       # 데스크톱 고정 (Progman)
├── debug.go         # 진단 로깅
├── tools/
│   └── mkicon.go    # ICO 파일 생성기 (수학적 렌더링)
├── resource.rc      # Windows 리소스 정의
├── Makefile         # 빌드 자동화
└── build.bat        # Windows 배치 빌드
```

## 데이터 소스

| 데이터 | 소스 | 방법 |
|--------|------|------|
| 사용량 | Anthropic OAuth API | `GET /api/oauth/usage` (Bearer 토큰) |
| 사용량 (폴백) | OMC 캐시 | `~/.claude/plugins/oh-my-claudecode/.usage-cache.json` |
| 플랜 정보 | 자격증명 | `~/.claude/.credentials.json` → `rateLimitTier` |
| 세션 | JSONL 스캔 | `~/.claude/projects/*/\*.jsonl` (최근 30분 이내) |
| 에이전트 | JSONL 파싱 | 최근 256KB 파싱, tool_use/tool_result 라이프사이클 추적 |
| OMC 전략 | JSONL 파싱 | Skill tool_use 블록에서 마지막 활성 스킬 추출 |

## 에이전트 표시

세션 카드를 클릭하면 확장되어 상세 정보를 표시합니다:

- **활성 전략** - `⚡ ralph`, `⚡ ultrawork` 등 현재 OMC 모드
- **실행 중 에이전트** - 초록 ● + "실행 중" (tool_result 미수신)
- **완료 에이전트** - 회색 ○ + "완료" (tool_result 수신)
- **에이전트 정보** - 이름, 모델(sonnet/opus/haiku), 작업 설명
- **Stale 감지** - 30분 초과 실행 에이전트는 자동으로 완료 처리

## 라이선스

Private repository.
