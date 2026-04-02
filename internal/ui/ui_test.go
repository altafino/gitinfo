package ui

import (
	"strings"
	"testing"

	"github.com/altafino/gitinfo/internal/git"
	"github.com/mattn/go-runewidth"
)

func TestParseDays(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"  ", 0},
		{"7", 7},
		{"30", 30},
		{"-5", 0},
		{"abc", 0},
		{"0", 0},
	}
	for _, tt := range tests {
		got := parseDays(tt.input)
		if got != tt.want {
			t.Errorf("parseDays(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestNewModel(t *testing.T) {
	m := New()
	if m.view != viewMenu {
		t.Errorf("initial view = %d, want viewMenu (%d)", m.view, viewMenu)
	}
}

func TestMenuView(t *testing.T) {
	m := New()
	v := m.View()
	if v == "" {
		t.Error("View() returned empty string for menu view")
	}
}

func TestNewInputForm(t *testing.T) {
	labels := []string{"User", "Days"}
	f := newInputForm(labels)
	if len(f.fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(f.fields))
	}
	if f.focused != 0 {
		t.Errorf("expected initial focus on field 0, got %d", f.focused)
	}
}

func TestInputFormNextField(t *testing.T) {
	f := newInputForm([]string{"A", "B", "C"})
	f.nextField()
	if f.focused != 1 {
		t.Errorf("after nextField focused = %d, want 1", f.focused)
	}
	f.nextField()
	f.nextField()
	if f.focused != 0 {
		t.Errorf("wrapping nextField focused = %d, want 0", f.focused)
	}
}

func TestInputFormView(t *testing.T) {
	f := newInputForm([]string{"Username", "Days"})
	v := f.view()
	if !strings.Contains(v, "Username") {
		t.Error("form view does not contain 'Username'")
	}
	if !strings.Contains(v, "Days") {
		t.Error("form view does not contain 'Days'")
	}
}

func TestBuildBranchUsersLines(t *testing.T) {
	infos := []interface{ getBranchName() string }{}
	_ = infos

	// Use the actual git types
	from := []struct {
		branch string
		users  []struct{ name, email string }
	}{
		{
			branch: "main",
			users:  []struct{ name, email string }{{"Alice", "alice@example.com"}},
		},
	}

	// Build mock data matching git.BranchInfo shape by going through the git package
	// directly is not necessary – we can call the helper directly.
	type user struct{ Name, Email string }
	type branchInfo struct {
		Branch string
		Users  []user
	}

	// Re-implement enough to call buildBranchUsersLines via the exported function.
	// Since buildBranchUsersLines is unexported, just test indirectly via the model.
	m := New()
	m.view = viewBranchUsers
	// branchInfos is nil → should render "No branches found."
	v := m.View()
	if !strings.Contains(v, "No branches found") {
		t.Errorf("expected 'No branches found', got: %q", v)
	}

	_ = from
}

func TestPadFileNameColumn(t *testing.T) {
	if got := padFileNameColumn("a.go", 8); runewidth.StringWidth(got) != 8 {
		t.Errorf("width = %d, want 8: %q", runewidth.StringWidth(got), got)
	}
	long := strings.Repeat("x", 100)
	if w := runewidth.StringWidth(padFileNameColumn(long, 10)); w != 10 {
		t.Errorf("truncated width = %d, want 10", w)
	}
}

func TestBuildBranchUsersSelection(t *testing.T) {
	infos := []git.BranchInfo{
		{Branch: "main", Users: []git.BranchUser{{Name: "A", Email: "a@x"}}},
		{Branch: "dev", Users: []git.BranchUser{}},
	}
	users, lines, total := buildBranchUsersSelection(infos)
	if total != 4 {
		t.Errorf("total lines = %d, want 4", total)
	}
	if len(users) != 1 || len(lines) != 1 {
		t.Fatalf("users/lines = %d/%d", len(users), len(lines))
	}
	if users[0].Name != "A" || lines[0] != 1 {
		t.Errorf("first user line = %d, want 1", lines[0])
	}
}
