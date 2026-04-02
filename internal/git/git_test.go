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

func TestParseNumstatTotals(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantIns int64
		wantDel int64
	}{
		{name: "empty", in: "", wantIns: 0, wantDel: 0},
		{
			name:    "two files",
			in:      "10\t2\ta.go\n5\t3\tb.go\n",
			wantIns: 15,
			wantDel: 5,
		},
		{
			name:    "binary skip",
			in:      "-\t-\tbin.png\n4\t1\tc.go\n",
			wantIns: 4,
			wantDel: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ins, del := parseNumstatTotals(tt.in)
			if ins != tt.wantIns || del != tt.wantDel {
				t.Errorf("parseNumstatTotals() = (%d, %d), want (%d, %d)", ins, del, tt.wantIns, tt.wantDel)
			}
		})
	}
}

func TestParseRecentCommits(t *testing.T) {
	in := "abc1234|2026-04-01T12:00:00Z|one\n|bad\n"
	got := parseRecentCommits(in)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Hash != "abc1234" || got[0].Subject != "one" {
		t.Errorf("%+v", got[0])
	}
}

func TestCommitsByYearFromLog(t *testing.T) {
	in := "2025-06-01T00:00:00Z\n2025-12-01T00:00:00Z\n2026-01-01T00:00:00Z\n"
	m := commitsByYearFromLog(in)
	if m[2025] != 2 || m[2026] != 1 {
		t.Errorf("map = %v", m)
	}
}

func TestAuthorRegexp(t *testing.T) {
	u := BranchUser{Name: "A.B", Email: "x@y.z"}
	got := authorRegexp(u)
	if got != `^A\.B <x@y\.z>$` {
		t.Errorf("authorRegexp = %q", got)
	}
}

func TestSortedYears(t *testing.T) {
	ys := SortedYears(map[int]int{2024: 1, 2026: 2, 2025: 3})
	if len(ys) != 3 || ys[0] != 2026 || ys[1] != 2025 || ys[2] != 2024 {
		t.Errorf("got %v", ys)
	}
}

func TestUserDashboardStats_integration(t *testing.T) {
	if !IsGitRepo() {
		t.Skip("not in a git repository")
	}
	users, err := AllUsers()
	if err != nil {
		t.Fatal(err)
	}
	if len(users) == 0 {
		t.Skip("no users in log")
	}
	d, err := UserDashboardStats(users[0], 0)
	if err != nil {
		t.Fatal(err)
	}
	if d == nil {
		t.Fatal("nil dashboard")
	}
	if d.User.Name != users[0].Name || d.User.Email != users[0].Email {
		t.Errorf("identity mismatch")
	}
}
