package i18n

// Strings holds all localizable UI strings
type Strings struct {
	// Usage section
	Usage       string
	Cache       string
	Offline     string
	AuthExpired string // shown when OAuth token is expired (401/403)

	// Session section
	Sessions string
	Active   string
	Idle     string

	// Usage card
	Reset      string
	UsagePct   string // e.g. "%.0f%% 사용" — caller must fmt.Sprintf with pct*100
	FiveHour   string // label for 5-hour window detection (contains this substring)
	WeeklyUsed string // "주간" for weekly window

	// Agent status
	AgentRunning string
	AgentDone    string
	DirectWork   string // "직접 작업 중 (에이전트 위임 없음)"

	// Notifications
	NotifyWarning     string // title for >= threshold2
	NotifyInfo        string // title for >= threshold1
	FiveHourWarnMsg   string // "5시간 사용량 %.0f%% 도달 (리셋: %s)"
	FiveHourInfoMsg   string // "5시간 사용량 %.0f%% (리셋: %s)"
	WeeklyWarnMsg     string // "주간 사용량 %.0f%% 도달 (리셋: %s)"
	WeeklyInfoMsg     string // "주간 사용량 %.0f%% (리셋: %s)"

	// Tray menu
	StartWithWindows string
	PinToDesktop     string
	Notifications    string
	Exit             string
	Language         string
	LanguageToggle   string // label for switching to the other language

	// Window label suffix (e.g. "사용" after percentage)
	Used string
}

// KoreanStrings is the Korean locale
var KoreanStrings = Strings{
	Usage:       "\uC0AC\uC6A9\uB7C9",     // 사용량
	Cache:       "\uCE90\uC2DC",           // 캐시
	Offline:     "\uC624\uD504\uB77C\uC778", // 오프라인
	AuthExpired: "\uC778\uC99D \uB9CC\uB8CC", // 인증 만료

	Sessions: "\uC138\uC158",          // 세션
	Active:   "\uD65C\uC131",          // 활성
	Idle:     "\uC720\uD734",          // 유휴

	Reset:      "\uB9AC\uC14B:",        // 리셋:
	UsagePct:   "%.0f%% \uC0AC\uC6A9", // %.0f%% 사용
	FiveHour:   "\uC2DC\uAC04",        // 시간
	WeeklyUsed: "\uC8FC\uAC04",        // 주간

	AgentRunning: "\uC2E4\uD589 \uC911", // 실행 중
	AgentDone:    "\uC644\uB8CC",        // 완료
	DirectWork:   "\uC9C1\uC811 \uC791\uC5C5 \uC911 (\uC5D0\uC774\uC804\uD2B8 \uC704\uC784 \uC5C6\uC74C)", // 직접 작업 중 (에이전트 위임 없음)

	NotifyWarning:   "Claude HUD - \uC0AC\uC6A9\uB7C9 \uACBD\uACE0",  // 사용량 경고
	NotifyInfo:      "Claude HUD - \uC0AC\uC6A9\uB7C9 \uC54C\uB9BC",  // 사용량 알림
	FiveHourWarnMsg: "5\uC2DC\uAC04 \uC0AC\uC6A9\uB7C9 %.0f%% \uB3C4\uB2EC (\uB9AC\uC14B: %s)",
	FiveHourInfoMsg: "5\uC2DC\uAC04 \uC0AC\uC6A9\uB7C9 %.0f%% (\uB9AC\uC14B: %s)",
	WeeklyWarnMsg:   "\uC8FC\uAC04 \uC0AC\uC6A9\uB7C9 %.0f%% \uB3C4\uB2EC (\uB9AC\uC14B: %s)",
	WeeklyInfoMsg:   "\uC8FC\uAC04 \uC0AC\uC6A9\uB7C9 %.0f%% (\uB9AC\uC14B: %s)",

	StartWithWindows: "Start with Windows",
	PinToDesktop:     "Pin to Desktop",
	Notifications:    "\uC54C\uB9BC", // 알림
	Exit:             "Exit",
	Language:         "\uC5B8\uC5B4", // 언어
	LanguageToggle:   "English",

	Used: "\uC0AC\uC6A9", // 사용
}

// EnglishStrings is the English locale
var EnglishStrings = Strings{
	Usage:       "Usage",
	Cache:       "Cache",
	Offline:     "Offline",
	AuthExpired: "Auth Expired",

	Sessions: "Sessions",
	Active:   "Active",
	Idle:     "Idle",

	Reset:      "Reset:",
	UsagePct:   "%.0f%% used",
	FiveHour:   "hour",       // used for window label detection
	WeeklyUsed: "weekly",

	AgentRunning: "Running",
	AgentDone:    "Done",
	DirectWork:   "Working directly (no agent delegation)",

	NotifyWarning:   "Claude HUD - Usage Warning",
	NotifyInfo:      "Claude HUD - Usage Notice",
	FiveHourWarnMsg: "5-hour usage %.0f%% reached (reset: %s)",
	FiveHourInfoMsg: "5-hour usage %.0f%% (reset: %s)",
	WeeklyWarnMsg:   "Weekly usage %.0f%% reached (reset: %s)",
	WeeklyInfoMsg:   "Weekly usage %.0f%% (reset: %s)",

	StartWithWindows: "Start with Windows",
	PinToDesktop:     "Pin to Desktop",
	Notifications:    "Notifications",
	Exit:             "Exit",
	Language:         "Language",
	LanguageToggle:   "\uD55C\uAD6D\uC5B4", // 한국어

	Used: "used",
}

// T is the active locale; defaults to Korean
var T = KoreanStrings

// SetLanguage switches the active locale
func SetLanguage(lang string) {
	switch lang {
	case "en":
		T = EnglishStrings
	default:
		T = KoreanStrings
	}
}
