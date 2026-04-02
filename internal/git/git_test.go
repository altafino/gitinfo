package git

import (
	"testing"
	"time"
)

func TestSinceFlag(t *testing.T) {
	tests := []struct {
		days      int
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

func TestParseCommitsLogOutput(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []CommitInfo
	}{
		{
			name: "empty",
			in:   "",
			want: nil,
		},
		{
			name: "single commit no body",
			in:   "deadbeef|Ada Lovelace|2024-06-01|Parse numbers\nENC_COMMIT_END",
			want: []CommitInfo{
				{Hash: "deadbeef", Author: "Ada Lovelace", Date: "2024-06-01", Subject: "Parse numbers", Body: ""},
			},
		},
		{
			name: "single commit with body",
			in:   "c0ffee|Ada Lovelace|2024-06-02|Refine parser\n\nFirst line.\nSecond line.\nENC_COMMIT_END",
			want: []CommitInfo{
				{
					Hash: "c0ffee", Author: "Ada Lovelace", Date: "2024-06-02", Subject: "Refine parser",
					Body: "First line.\nSecond line.",
				},
			},
		},
		{
			name: "two commits",
			in:   "aaa|A|2024-01-01|S1\nENC_COMMIT_END\nbbb|B|2024-01-02|S2\nline\nENC_COMMIT_END",
			want: []CommitInfo{
				{Hash: "aaa", Author: "A", Date: "2024-01-01", Subject: "S1", Body: ""},
				{Hash: "bbb", Author: "B", Date: "2024-01-02", Subject: "S2", Body: "line"},
			},
		},
		{
			name: "malformed header skipped",
			in:   "not-enough-pipes\nENC_COMMIT_END",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCommitsLogOutput(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
