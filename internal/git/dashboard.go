package git

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	dashboardMaxBranches   = 15
	dashboardMaxFiles      = 15
	dashboardRecentCommits = 10
)

// DashboardRecentCommit is a single line in the "recent activity" section.
type DashboardRecentCommit struct {
	Hash    string
	DateISO string
	Subject string
}

// UserDashboard aggregates repository statistics for one author identity.
type UserDashboard struct {
	User BranchUser
	Days int // filter window; 0 means entire history

	CommitsNonMerge int64
	CommitsMerge    int64

	FirstCommit time.Time
	LastCommit  time.Time
	HasActivity bool

	Insertions int64
	Deletions  int64

	Branches    []string
	BranchTotal int

	TopFiles []FileChange

	Recent []DashboardRecentCommit

	// CommitsByYear maps calendar year to number of non-merge commits (sorted years in UI).
	CommitsByYear map[int]int
}

// authorRegexp returns a pattern for git's --author that matches this exact
// name and e-mail as recorded in commit metadata (Author Name <email>).
func authorRegexp(u BranchUser) string {
	return fmt.Sprintf(`^%s <%s>$`,
		regexp.QuoteMeta(u.Name),
		regexp.QuoteMeta(u.Email))
}

// UserDashboardStats loads statistics for the given author and optional day window.
// days <= 0 means no time restriction.
func UserDashboardStats(u BranchUser, days int) (*UserDashboard, error) {
	pat := authorRegexp(u)
	since := sinceFlag(days)
	d := &UserDashboard{User: u, Days: days, CommitsByYear: make(map[int]int)}

	base := []string{"--all", "--author=" + pat}
	if since != "" {
		base = append(base, since)
	}

	nm, err := revListCount(append([]string{"rev-list", "--count", "--no-merges"}, base...)...)
	if err != nil {
		return nil, err
	}
	d.CommitsNonMerge = nm

	mg, err := revListCount(append([]string{"rev-list", "--count", "--merges"}, base...)...)
	if err != nil {
		return nil, err
	}
	d.CommitsMerge = mg

	if d.CommitsNonMerge > 0 || d.CommitsMerge > 0 {
		d.HasActivity = true
	}

	firstOut, err := runGit(append([]string{"log", "--no-merges", "-1", "--reverse", "--format=%aI"}, base...)...)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(firstOut) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(firstOut))
		if err == nil {
			d.FirstCommit = t
		}
	}

	lastOut, err := runGit(append([]string{"log", "--no-merges", "-1", "--format=%aI"}, base...)...)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(lastOut) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(lastOut))
		if err == nil {
			d.LastCommit = t
		}
	}

	numstatOut, err := runGit(append([]string{"log", "--no-merges", "--pretty=format:", "--numstat"}, base...)...)
	if err != nil {
		return nil, err
	}
	d.Insertions, d.Deletions = parseNumstatTotals(numstatOut)

	branches, err := BranchesForUser(u.Email, days)
	if err != nil {
		return nil, err
	}
	d.BranchTotal = len(branches)
	if len(branches) > dashboardMaxBranches {
		d.Branches = append([]string(nil), branches[:dashboardMaxBranches]...)
	} else {
		d.Branches = append([]string(nil), branches...)
	}

	files, err := FilesTouchedByUser(u.Email, "", days)
	if err != nil {
		return nil, err
	}
	if len(files) > dashboardMaxFiles {
		d.TopFiles = append([]FileChange(nil), files[:dashboardMaxFiles]...)
	} else {
		d.TopFiles = append([]FileChange(nil), files...)
	}

	recentArgs := append([]string{
		"log", "--no-merges",
		"-" + strconv.Itoa(dashboardRecentCommits),
		"--format=%h|%aI|%s",
	}, base...)
	recentOut, err := runGit(recentArgs...)
	if err != nil {
		return nil, err
	}
	d.Recent = parseRecentCommits(recentOut)

	yearOut, err := runGit(append([]string{"log", "--no-merges", "--format=%aI"}, base...)...)
	if err != nil {
		return nil, err
	}
	d.CommitsByYear = commitsByYearFromLog(yearOut)

	return d, nil
}

func revListCount(args ...string) (int64, error) {
	out, err := runGit(args...)
	if err != nil {
		return 0, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(out, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse rev-list count %q: %w", out, err)
	}
	return n, nil
}

// parseNumstatTotals sums added/deleted lines from git log --numstat output.
func parseNumstatTotals(out string) (insertions, deletions int64) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		if parts[0] == "-" && parts[1] == "-" {
			continue
		}
		a, err1 := strconv.ParseInt(parts[0], 10, 64)
		b, err2 := strconv.ParseInt(parts[1], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		insertions += a
		deletions += b
	}
	return insertions, deletions
}

func parseRecentCommits(out string) []DashboardRecentCommit {
	var res []DashboardRecentCommit
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}
		res = append(res, DashboardRecentCommit{
			Hash:    parts[0],
			DateISO: parts[1],
			Subject: parts[2],
		})
	}
	return res
}

func commitsByYearFromLog(out string) map[int]int {
	m := make(map[int]int)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, line)
		if err != nil {
			continue
		}
		m[t.Year()]++
	}
	return m
}

// SortedYears returns calendar years in descending order for dashboard display.
func SortedYears(m map[int]int) []int {
	if len(m) == 0 {
		return nil
	}
	var ys []int
	for y := range m {
		ys = append(ys, y)
	}
	sort.Slice(ys, func(i, j int) bool { return ys[i] > ys[j] })
	return ys
}
