package handler

import "testing"

func TestShouldRecordHTTPMetrics(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/api/auth/login", want: true},
		{path: "/api/admin/settings", want: true},
		{path: "/api/admin/metrics", want: false},
		{path: "/assets/index.js", want: false},
		{path: "/", want: false},
	}

	for _, tt := range tests {
		if got := shouldRecordHTTPMetrics(tt.path); got != tt.want {
			t.Fatalf("shouldRecordHTTPMetrics(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
