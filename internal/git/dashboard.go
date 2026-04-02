package git

import (
	"bufio"
	"fmt"
	"os/exec"
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

	authorBase := []string{"--author=" + pat}
	if since != "" {
		authorBase = append(authorBase, since)
	}

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

	// git log -1 --reverse still applies -1 in default (newest-first) order, so both
	// "first" and "last" queried that way were the same commit. Use rev-list traversal
	// order instead: --reverse walks oldest→newest; default walks newest→oldest.
	firstHash, err := firstRevListHash(true, authorBase)
	if err != nil {
		return nil, err
	}
	if firstHash != "" {
		if t, err := commitAuthorDateISO(firstHash); err == nil {
			d.FirstCommit = t
		}
	}

	lastHash, err := firstRevListHash(false, authorBase)
	if err != nil {
		return nil, err
	}
	if lastHash != "" {
		if t, err := commitAuthorDateISO(lastHash); err == nil {
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

// firstRevListHash returns one commit hash from git rev-list without buffering the full list.
// With reverse=true, traversal is oldest→newest, so the first line is the oldest matching commit.
// With reverse=false, traversal is newest→oldest, so the first line is the newest matching commit.
// The child process is stopped after one line so large histories are not fully scanned into memory.
func firstRevListHash(reverse bool, authorAndSince []string) (string, error) {
	args := []string{"rev-list", "--no-merges"}
	if reverse {
		args = append(args, "--reverse")
	}
	args = append(args, "--all")
	args = append(args, authorAndSince...)
	cmd := exec.Command("git", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("git %v: %w", args, err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	sc := bufio.NewScanner(stdout)
	if !sc.Scan() {
		_ = cmd.Wait()
		return "", nil
	}
	hash := strings.TrimSpace(sc.Text())
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	return hash, nil
}

func commitAuthorDateISO(hash string) (time.Time, error) {
	out, err := runGit("show", "-s", "--format=%aI", hash)
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, strings.TrimSpace(out))
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
