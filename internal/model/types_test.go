package model

import (
	"testing"
	"time"
)

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{999999, "1000.0K"},
		{1000000, "1.0M"},
		{2500000, "2.5M"},
	}
	for _, tc := range tests {
		got := FormatTokens(tc.n)
		if got != tc.want {
			t.Errorf("FormatTokens(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestFormatTimeRemaining(t *testing.T) {
	// Past time should return reset string
	past := time.Now().Add(-1 * time.Minute)
	got := FormatTimeRemaining(past)
	if got != "리셋됨" {
		t.Errorf("FormatTimeRemaining(past) = %q, want %q", got, "리셋됨")
	}

	// Future: minutes only
	future := time.Now().Add(45 * time.Minute)
	got = FormatTimeRemaining(future)
	if got == "" {
		t.Errorf("FormatTimeRemaining(future 45m) returned empty string")
	}

	// Future: hours + minutes
	future2 := time.Now().Add(2*time.Hour + 30*time.Minute)
	got2 := FormatTimeRemaining(future2)
	if got2 == "" {
		t.Errorf("FormatTimeRemaining(future 2h30m) returned empty string")
	}

	// Future: days
	future3 := time.Now().Add(25 * time.Hour)
	got3 := FormatTimeRemaining(future3)
	if got3 == "" {
		t.Errorf("FormatTimeRemaining(future 25h) returned empty string")
	}
}

func TestParseISO(t *testing.T) {
	// Empty string returns zero time
	if !ParseISO("").IsZero() {
		t.Error("ParseISO(\"\") should return zero time")
	}

	// Valid RFC3339 with Z
	ts1 := ParseISO("2024-01-15T10:30:00Z")
	if ts1.IsZero() {
		t.Error("ParseISO(valid RFC3339) returned zero time")
	}
	if ts1.Year() != 2024 || ts1.Month() != 1 || ts1.Day() != 15 {
		t.Errorf("ParseISO date mismatch: got %v", ts1)
	}

	// Valid RFC3339Nano with fractional seconds
	ts2 := ParseISO("2024-06-20T08:15:30.123Z")
	if ts2.IsZero() {
		t.Error("ParseISO(RFC3339Nano) returned zero time")
	}

	// Invalid string returns zero time
	if !ParseISO("not-a-date").IsZero() {
		t.Error("ParseISO(invalid) should return zero time")
	}
}

func TestRateLimitTierToPlan(t *testing.T) {
	tests := []struct {
		tier string
		want PlanType
	}{
		{"claude_pro", PlanPro},
		{"claude_team", PlanTeam},
		{"claude_enterprise", PlanEnterprise},
		{"claude_max_5x", PlanMax5x},
		{"claude_max_20x", PlanMax20x},
		{"", PlanFree},
		{"unknown_tier", PlanFree},
		{"5x_plan", PlanMax5x},
		{"20x_plan", PlanMax20x},
	}
	for _, tc := range tests {
		got := RateLimitTierToPlan(tc.tier)
		if got != tc.want {
			t.Errorf("RateLimitTierToPlan(%q) = %q, want %q", tc.tier, got, tc.want)
		}
	}
}
