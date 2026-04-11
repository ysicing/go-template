package model

import "testing"

func TestRandomCheckInPoints_Range(t *testing.T) {
	for i := 0; i < 10000; i++ {
		pts := RandomCheckInPoints()
		if pts < 1 || pts > 5 {
			t.Fatalf("RandomCheckInPoints() = %d, want [1, 5]", pts)
		}
	}
}

func TestRandomInviteRewardPoints_Range(t *testing.T) {
	for i := 0; i < 10000; i++ {
		pts := RandomInviteRewardPoints()
		if pts < 1 || pts > 5 {
			t.Fatalf("RandomInviteRewardPoints() = %d, want [1, 5]", pts)
		}
	}
}

func TestRandomCheckInPoints_Distribution(t *testing.T) {
	counts := make(map[int64]int)
	const n = 100000
	var sum int64
	for i := 0; i < n; i++ {
		pts := RandomCheckInPoints()
		counts[pts]++
		sum += pts
	}
	mean := float64(sum) / float64(n)
	// Mean should be close to 3.0 (within 0.15 tolerance).
	if mean < 2.85 || mean > 3.15 {
		t.Errorf("mean = %.2f, want ~3.0", mean)
	}
	// Every value 1-5 should appear.
	for v := int64(1); v <= 5; v++ {
		if counts[v] == 0 {
			t.Errorf("value %d never appeared in %d samples", v, n)
		}
	}
	t.Logf("distribution over %d samples: %v, mean=%.2f", n, counts, mean)
}

func TestCalcLevel(t *testing.T) {
	tests := []struct {
		earned int64
		want   int
	}{
		{0, 1}, {199, 1}, {200, 2}, {799, 2}, {800, 3},
		{2400, 4}, {6000, 5}, {15000, 6}, {99999, 6},
	}
	for _, tt := range tests {
		if got := CalcLevel(tt.earned); got != tt.want {
			t.Errorf("CalcLevel(%d) = %d, want %d", tt.earned, got, tt.want)
		}
	}
}

func TestCountsTowardEXP(t *testing.T) {
	if !CountsTowardEXP(PointKindCheckIn) {
		t.Fatal("check-in should count toward EXP")
	}
	if !CountsTowardEXP(PointKindInviteReward) {
		t.Fatal("invite reward should count toward EXP")
	}
	if CountsTowardEXP(PointKindAdminAdjust) {
		t.Fatal("admin adjust must not count toward EXP")
	}
	if CountsTowardEXP(PointKindPurchase) {
		t.Fatal("purchase must not count toward EXP")
	}
}
