package points

import (
	"testing"
	"time"
)

// withFixedCheckInTime overrides package-level time vars for testing.
// Do NOT use t.Parallel() — mutates shared state.
func withFixedCheckInTime(t *testing.T, utcTime time.Time) {
	t.Helper()
	oldNow := nowForCheckIn
	oldLoc := checkInLocation
	t.Cleanup(func() {
		nowForCheckIn = oldNow
		checkInLocation = oldLoc
	})
	checkInLocation = time.FixedZone("CST", 8*60*60)
	nowForCheckIn = func() time.Time { return utcTime }
}

func TestTodayDateStr_UsesCheckInTimezone(t *testing.T) {
	tests := []struct {
		name    string
		utcTime time.Time
		want    string
	}{
		{
			name:    "UTC 16:30 → CST next day",
			utcTime: time.Date(2026, 3, 1, 16, 30, 0, 0, time.UTC),
			want:    "2026-03-02",
		},
		{
			name:    "UTC 15:59 → CST same day 23:59",
			utcTime: time.Date(2026, 3, 1, 15, 59, 0, 0, time.UTC),
			want:    "2026-03-01",
		},
		{
			name:    "UTC 16:00 → CST exactly midnight next day",
			utcTime: time.Date(2026, 3, 1, 16, 0, 0, 0, time.UTC),
			want:    "2026-03-02",
		},
		{
			name:    "year boundary: UTC 2026-12-31 16:00 → CST 2027-01-01",
			utcTime: time.Date(2026, 12, 31, 16, 0, 0, 0, time.UTC),
			want:    "2027-01-01",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFixedCheckInTime(t, tt.utcTime)
			if got := todayDateStr(); got != tt.want {
				t.Fatalf("todayDateStr() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestYesterdayDateStr_UsesCheckInTimezone(t *testing.T) {
	tests := []struct {
		name    string
		utcTime time.Time
		want    string
	}{
		{
			name:    "UTC 16:30 → CST next day, yesterday is today UTC",
			utcTime: time.Date(2026, 3, 1, 16, 30, 0, 0, time.UTC),
			want:    "2026-03-01",
		},
		{
			name:    "UTC 15:59 → CST same day, yesterday is prev day",
			utcTime: time.Date(2026, 3, 1, 15, 59, 0, 0, time.UTC),
			want:    "2026-02-28",
		},
		{
			name:    "year boundary: UTC 2027-01-01 00:00 → CST 2027-01-01 08:00, yesterday is 2026-12-31",
			utcTime: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			want:    "2026-12-31",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFixedCheckInTime(t, tt.utcTime)
			if got := yesterdayDateStr(); got != tt.want {
				t.Fatalf("yesterdayDateStr() = %s, want %s", got, tt.want)
			}
		})
	}
}
