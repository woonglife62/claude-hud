//go:build windows

package ui

// COLORREF creates a Windows color value (0x00BBGGRR)
func COLORREF(r, g, b byte) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

// ColorScheme defines the HUD color palette
type ColorScheme struct {
	BgColor       uint32
	CardColor     uint32
	CardBorder    uint32
	TextPrimary   uint32
	TextSecondary uint32
	TextMuted     uint32
	AccentPurple  uint32
	AccentGreen   uint32
	AccentOrange  uint32
	AccentRed     uint32
	AccentBlue    uint32
	AccentCyan    uint32
	ProgressBg    uint32
	ProgressFill  uint32
	Divider       uint32
}

// LightScheme is the light color scheme
var LightScheme = ColorScheme{
	BgColor:       COLORREF(245, 245, 248),
	CardColor:     COLORREF(255, 255, 255),
	CardBorder:    COLORREF(210, 210, 220),
	TextPrimary:   COLORREF(30, 30, 40),
	TextSecondary: COLORREF(80, 80, 100),
	TextMuted:     COLORREF(140, 140, 160),
	AccentPurple:  COLORREF(100, 60, 200),
	AccentGreen:   COLORREF(30, 160, 70),
	AccentOrange:  COLORREF(200, 120, 20),
	AccentRed:     COLORREF(200, 50, 50),
	AccentBlue:    COLORREF(30, 100, 200),
	AccentCyan:    COLORREF(20, 160, 160),
	ProgressBg:    COLORREF(220, 220, 230),
	ProgressFill:  COLORREF(100, 60, 200),
	Divider:       COLORREF(200, 200, 215),
}

// DarkScheme is the default dark color scheme
var DarkScheme = ColorScheme{
	BgColor:       COLORREF(24, 24, 32),
	CardColor:     COLORREF(36, 36, 50),
	CardBorder:    COLORREF(55, 55, 75),
	TextPrimary:   COLORREF(240, 240, 245),
	TextSecondary: COLORREF(180, 180, 200),
	TextMuted:     COLORREF(120, 120, 150),
	AccentPurple:  COLORREF(160, 120, 255),
	AccentGreen:   COLORREF(80, 220, 120),
	AccentOrange:  COLORREF(255, 180, 60),
	AccentRed:     COLORREF(255, 90, 90),
	AccentBlue:    COLORREF(80, 160, 255),
	AccentCyan:    COLORREF(80, 220, 220),
	ProgressBg:    COLORREF(45, 45, 65),
	ProgressFill:  COLORREF(160, 120, 255),
	Divider:       COLORREF(50, 50, 70),
}
