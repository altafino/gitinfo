package git

import (
	"testing"
	"time"
)

func TestSinceFlag(t *testing.T) {
	tests := []struct {
		days     int
		wantEmpty bool
	}{
		{0, true},
		{-1, true},
		{7, false},
		{30, false},
	}
	for _, tt := range tests {
		got := sinceFlag(tt.days)
		if tt.wantEmpty && got != "" {
			t.Errorf("sinceFlag(%d) = %q, want empty", tt.days, got)
		}
		if !tt.wantEmpty && got == "" {
			t.Errorf("sinceFlag(%d) = empty, want non-empty", tt.days)
		}
	}
}

func TestSinceFlagDateFormat(t *testing.T) {
	// sinceFlag(7) should produce --since=YYYY-MM-DD
	flag := sinceFlag(7)
	expected := "--since="
	if len(flag) < len(expected)+10 {
		t.Fatalf("sinceFlag(7) = %q, too short", flag)
	}
	prefix := flag[:len(expected)]
	if prefix != expected {
		t.Errorf("sinceFlag(7) prefix = %q, want %q", prefix, expected)
	}
	dateStr := flag[len(expected):]
	_, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		t.Errorf("sinceFlag(7) date %q not parseable: %v", dateStr, err)
	}
}

func TestParseDaysEquivalent(t *testing.T) {
	// parseDays is in ui package, test sinceFlag boundary behaviour
	if sinceFlag(0) != "" {
		t.Error("sinceFlag(0) should be empty")
	}
}

func TestIsGitRepo(t *testing.T) {
	// Running inside the gitinfo repo – this should return true.
	if !IsGitRepo() {
		t.Skip("not running inside a git repository; skipping")
	}
}
