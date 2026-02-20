//go:build !windows

package ui

import "image/color"

// FyneColors defines the HUD color palette for Fyne rendering
type FyneColors struct {
	BgColor       color.NRGBA
	CardColor     color.NRGBA
	CardBorder    color.NRGBA
	TextPrimary   color.NRGBA
	TextSecondary color.NRGBA
	TextMuted     color.NRGBA
	AccentPurple  color.NRGBA
	AccentGreen   color.NRGBA
	AccentOrange  color.NRGBA
	AccentRed     color.NRGBA
	AccentBlue    color.NRGBA
	AccentCyan    color.NRGBA
	ProgressBg    color.NRGBA
	ProgressFill  color.NRGBA
	Divider       color.NRGBA
}

// FyneDarkColors is the default dark color scheme for Fyne
var FyneDarkColors = FyneColors{
	BgColor:       color.NRGBA{R: 24, G: 24, B: 32, A: 255},
	CardColor:     color.NRGBA{R: 36, G: 36, B: 50, A: 255},
	CardBorder:    color.NRGBA{R: 55, G: 55, B: 75, A: 255},
	TextPrimary:   color.NRGBA{R: 240, G: 240, B: 245, A: 255},
	TextSecondary: color.NRGBA{R: 180, G: 180, B: 200, A: 255},
	TextMuted:     color.NRGBA{R: 120, G: 120, B: 150, A: 255},
	AccentPurple:  color.NRGBA{R: 160, G: 120, B: 255, A: 255},
	AccentGreen:   color.NRGBA{R: 80, G: 220, B: 120, A: 255},
	AccentOrange:  color.NRGBA{R: 255, G: 180, B: 60, A: 255},
	AccentRed:     color.NRGBA{R: 255, G: 90, B: 90, A: 255},
	AccentBlue:    color.NRGBA{R: 80, G: 160, B: 255, A: 255},
	AccentCyan:    color.NRGBA{R: 80, G: 220, B: 220, A: 255},
	ProgressBg:    color.NRGBA{R: 45, G: 45, B: 65, A: 255},
	ProgressFill:  color.NRGBA{R: 160, G: 120, B: 255, A: 255},
	Divider:       color.NRGBA{R: 50, G: 50, B: 70, A: 255},
}

// FyneLightColors is the light color scheme for Fyne
var FyneLightColors = FyneColors{
	BgColor:       color.NRGBA{R: 245, G: 245, B: 248, A: 255},
	CardColor:     color.NRGBA{R: 255, G: 255, B: 255, A: 255},
	CardBorder:    color.NRGBA{R: 210, G: 210, B: 220, A: 255},
	TextPrimary:   color.NRGBA{R: 30, G: 30, B: 40, A: 255},
	TextSecondary: color.NRGBA{R: 80, G: 80, B: 100, A: 255},
	TextMuted:     color.NRGBA{R: 140, G: 140, B: 160, A: 255},
	AccentPurple:  color.NRGBA{R: 100, G: 60, B: 200, A: 255},
	AccentGreen:   color.NRGBA{R: 30, G: 160, B: 70, A: 255},
	AccentOrange:  color.NRGBA{R: 200, G: 120, B: 20, A: 255},
	AccentRed:     color.NRGBA{R: 200, G: 50, B: 50, A: 255},
	AccentBlue:    color.NRGBA{R: 30, G: 100, B: 200, A: 255},
	AccentCyan:    color.NRGBA{R: 20, G: 160, B: 160, A: 255},
	ProgressBg:    color.NRGBA{R: 220, G: 220, B: 230, A: 255},
	ProgressFill:  color.NRGBA{R: 100, G: 60, B: 200, A: 255},
	Divider:       color.NRGBA{R: 200, G: 200, B: 215, A: 255},
}
