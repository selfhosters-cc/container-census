package models

import (
	"testing"
	"time"
)

func TestNotificationSilence_IsActiveNow_OneTime(t *testing.T) {
	tests := []struct {
		name          string
		silencedUntil time.Time
		expected      bool
	}{
		{
			name:          "active one-time silence",
			silencedUntil: time.Now().Add(1 * time.Hour),
			expected:      true,
		},
		{
			name:          "expired one-time silence",
			silencedUntil: time.Now().Add(-1 * time.Hour),
			expected:      false,
		},
		{
			name:          "just expired one-time silence",
			silencedUntil: time.Now().Add(-1 * time.Second),
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NotificationSilence{
				IsRecurring:   false,
				SilencedUntil: tt.silencedUntil,
			}
			if got := s.IsActiveNow(); got != tt.expected {
				t.Errorf("IsActiveNow() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNotificationSilence_IsActiveNow_Recurring(t *testing.T) {
	// Get current time in UTC for testing
	now := time.Now().UTC()

	tests := []struct {
		name               string
		dailyStartTime     string
		dailyEndTime       string
		timezone           string
		recurringExpiresAt *time.Time
		mockTime           time.Time // We'll test with the actual time.Now()
		expected           bool
	}{
		{
			name:               "recurring without expiry - always considered for window check",
			dailyStartTime:     "00:00",
			dailyEndTime:       "23:59",
			timezone:           "UTC",
			recurringExpiresAt: nil,
			expected:           true, // Almost always active with this wide window
		},
		{
			name:               "recurring with past expiry",
			dailyStartTime:     "00:00",
			dailyEndTime:       "23:59",
			timezone:           "UTC",
			recurringExpiresAt: func() *time.Time { t := now.Add(-1 * time.Hour); return &t }(),
			expected:           false,
		},
		{
			name:               "recurring with future expiry",
			dailyStartTime:     "00:00",
			dailyEndTime:       "23:59",
			timezone:           "UTC",
			recurringExpiresAt: func() *time.Time { t := now.Add(24 * time.Hour); return &t }(),
			expected:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NotificationSilence{
				IsRecurring:        true,
				DailyStartTime:     tt.dailyStartTime,
				DailyEndTime:       tt.dailyEndTime,
				Timezone:           tt.timezone,
				RecurringExpiresAt: tt.recurringExpiresAt,
			}
			if got := s.IsActiveNow(); got != tt.expected {
				t.Errorf("IsActiveNow() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNotificationSilence_OvernightWindow(t *testing.T) {
	// Test overnight window (23:00-06:00)
	tests := []struct {
		name     string
		testTime time.Time
		expected bool
	}{
		{
			name:     "23:30 - within overnight window (after start)",
			testTime: time.Date(2024, 1, 15, 23, 30, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "02:00 - within overnight window (before end)",
			testTime: time.Date(2024, 1, 16, 2, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "12:00 - outside overnight window",
			testTime: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "06:00 - at end boundary (exclusive)",
			testTime: time.Date(2024, 1, 16, 6, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "23:00 - at start boundary (inclusive)",
			testTime: time.Date(2024, 1, 15, 23, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "05:59 - just before end",
			testTime: time.Date(2024, 1, 16, 5, 59, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "22:59 - just before start",
			testTime: time.Date(2024, 1, 15, 22, 59, 0, 0, time.UTC),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NotificationSilence{
				IsRecurring:    true,
				DailyStartTime: "23:00",
				DailyEndTime:   "06:00",
				Timezone:       "UTC",
			}
			if got := s.isWithinDailyWindow(tt.testTime); got != tt.expected {
				t.Errorf("isWithinDailyWindow(%v) = %v, want %v", tt.testTime.Format("15:04"), got, tt.expected)
			}
		})
	}
}

func TestNotificationSilence_NormalWindow(t *testing.T) {
	// Test normal daytime window (09:00-17:00)
	tests := []struct {
		name     string
		testTime time.Time
		expected bool
	}{
		{
			name:     "12:00 - within normal window",
			testTime: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "08:00 - before start",
			testTime: time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "18:00 - after end",
			testTime: time.Date(2024, 1, 15, 18, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "09:00 - at start boundary (inclusive)",
			testTime: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "17:00 - at end boundary (exclusive)",
			testTime: time.Date(2024, 1, 15, 17, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "16:59 - just before end",
			testTime: time.Date(2024, 1, 15, 16, 59, 0, 0, time.UTC),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NotificationSilence{
				IsRecurring:    true,
				DailyStartTime: "09:00",
				DailyEndTime:   "17:00",
				Timezone:       "UTC",
			}
			if got := s.isWithinDailyWindow(tt.testTime); got != tt.expected {
				t.Errorf("isWithinDailyWindow(%v) = %v, want %v", tt.testTime.Format("15:04"), got, tt.expected)
			}
		})
	}
}

func TestNotificationSilence_Timezone(t *testing.T) {
	// Test with America/New_York timezone
	// When it's 12:00 UTC, it's 07:00 or 08:00 in New York (depending on DST)
	tests := []struct {
		name           string
		testTime       time.Time
		dailyStartTime string
		dailyEndTime   string
		timezone       string
		expected       bool
	}{
		{
			name:           "12:00 UTC = 07:00 EST - outside 09:00-17:00 EST window",
			testTime:       time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC), // 07:00 EST (winter)
			dailyStartTime: "09:00",
			dailyEndTime:   "17:00",
			timezone:       "America/New_York",
			expected:       false,
		},
		{
			name:           "15:00 UTC = 10:00 EST - inside 09:00-17:00 EST window",
			testTime:       time.Date(2024, 1, 15, 15, 0, 0, 0, time.UTC), // 10:00 EST (winter)
			dailyStartTime: "09:00",
			dailyEndTime:   "17:00",
			timezone:       "America/New_York",
			expected:       true,
		},
		{
			name:           "invalid timezone falls back to UTC",
			testTime:       time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			dailyStartTime: "09:00",
			dailyEndTime:   "17:00",
			timezone:       "Invalid/Timezone",
			expected:       true, // Falls back to UTC, 12:00 is in 09:00-17:00
		},
		{
			name:           "empty timezone defaults to UTC",
			testTime:       time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			dailyStartTime: "09:00",
			dailyEndTime:   "17:00",
			timezone:       "",
			expected:       true, // UTC 12:00 is in 09:00-17:00
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NotificationSilence{
				IsRecurring:    true,
				DailyStartTime: tt.dailyStartTime,
				DailyEndTime:   tt.dailyEndTime,
				Timezone:       tt.timezone,
			}
			if got := s.isWithinDailyWindow(tt.testTime); got != tt.expected {
				t.Errorf("isWithinDailyWindow(%v) with timezone %s = %v, want %v",
					tt.testTime.Format("15:04 MST"), tt.timezone, got, tt.expected)
			}
		})
	}
}

func TestParseTimeHHMM(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantHour    int
		wantMin     int
		expectError bool
	}{
		{
			name:        "valid time 09:00",
			input:       "09:00",
			wantHour:    9,
			wantMin:     0,
			expectError: false,
		},
		{
			name:        "valid time 23:59",
			input:       "23:59",
			wantHour:    23,
			wantMin:     59,
			expectError: false,
		},
		{
			name:        "valid time 00:00",
			input:       "00:00",
			wantHour:    0,
			wantMin:     0,
			expectError: false,
		},
		{
			name:        "invalid format - too short",
			input:       "9:00",
			expectError: true,
		},
		{
			name:        "invalid format - no colon",
			input:       "09-00",
			expectError: true,
		},
		{
			name:        "invalid hour - 24",
			input:       "24:00",
			expectError: true,
		},
		{
			name:        "invalid minute - 60",
			input:       "09:60",
			expectError: true,
		},
		{
			name:        "invalid format - letters",
			input:       "ab:cd",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hour, min, err := parseTimeHHMM(tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("parseTimeHHMM(%q) expected error, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("parseTimeHHMM(%q) unexpected error: %v", tt.input, err)
				}
				if hour != tt.wantHour || min != tt.wantMin {
					t.Errorf("parseTimeHHMM(%q) = (%d, %d), want (%d, %d)",
						tt.input, hour, min, tt.wantHour, tt.wantMin)
				}
			}
		})
	}
}

func TestNotificationSilence_EmptyTimeFields(t *testing.T) {
	// Test that missing time fields return false for recurring silences
	tests := []struct {
		name           string
		dailyStartTime string
		dailyEndTime   string
		expected       bool
	}{
		{
			name:           "both times missing",
			dailyStartTime: "",
			dailyEndTime:   "",
			expected:       false,
		},
		{
			name:           "start time missing",
			dailyStartTime: "",
			dailyEndTime:   "06:00",
			expected:       false,
		},
		{
			name:           "end time missing",
			dailyStartTime: "23:00",
			dailyEndTime:   "",
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NotificationSilence{
				IsRecurring:    true,
				DailyStartTime: tt.dailyStartTime,
				DailyEndTime:   tt.dailyEndTime,
				Timezone:       "UTC",
			}
			testTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
			if got := s.isWithinDailyWindow(testTime); got != tt.expected {
				t.Errorf("isWithinDailyWindow() with start=%q end=%q = %v, want %v",
					tt.dailyStartTime, tt.dailyEndTime, got, tt.expected)
			}
		})
	}
}
