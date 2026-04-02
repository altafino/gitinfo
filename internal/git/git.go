// Package git provides helpers for querying git repository information.
package git

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// BranchUser represents a user who was active on a branch.
type BranchUser struct {
	Name  string
	Email string
}

// BranchInfo holds a branch name and the users active on it.
type BranchInfo struct {
	Branch string
	Users  []BranchUser
}

// FileChange represents a file changed by a user.
type FileChange struct {
	File    string
	Changes int
}

// CommitInfo holds information about a single git commit.
type CommitInfo struct {
	Hash    string
	Author  string
	Date    string
	Subject string
	Body    string
}

// runGit runs a git command in the current working directory and returns its output.
func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// IsGitRepo returns true when the current directory is inside a git repository.
func IsGitRepo() bool {
	_, err := runGit("rev-parse", "--git-dir")
	return err == nil
}

// ListBranches returns all local branch names.
func ListBranches() ([]string, error) {
	out, err := runGit("branch", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	branches := strings.Split(out, "\n")
	var result []string
	for _, b := range branches {
		b = strings.TrimSpace(b)
		if b != "" {
			result = append(result, b)
		}
	}
	return result, nil
}

// ListAllBranches returns both local and remote branch names (deduplicated).
func ListAllBranches() ([]string, error) {
	out, err := runGit("branch", "-a", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	seen := map[string]bool{}
	var result []string
	for _, b := range strings.Split(out, "\n") {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		// Normalise "remotes/origin/foo" -> "origin/foo"
		short := strings.TrimPrefix(b, "remotes/")
		if !seen[short] {
			seen[short] = true
			result = append(result, short)
		}
	}
	return result, nil
}

// UsersOnBranch returns the unique users who have committed on a branch.
func UsersOnBranch(branch string) ([]BranchUser, error) {
	out, err := runGit("log", branch, "--format=%an|%ae", "--no-merges")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var users []BranchUser
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0] + "|" + parts[1]
		if !seen[key] {
			seen[key] = true
			users = append(users, BranchUser{Name: parts[0], Email: parts[1]})
		}
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Name < users[j].Name })
	return users, nil
}

// BranchUsers returns all branches with their active users.
func BranchUsers() ([]BranchInfo, error) {
	branches, err := ListBranches()
	if err != nil {
		return nil, err
	}
	var result []BranchInfo
	for _, b := range branches {
		users, err := UsersOnBranch(b)
		if err != nil {
			// Skip branches that can't be read (e.g. empty)
			continue
		}
		result = append(result, BranchInfo{Branch: b, Users: users})
	}
	return result, nil
}

// sinceFlag converts a days value to a git --since flag value.
// days <= 0 means no time restriction.
func sinceFlag(days int) string {
	if days <= 0 {
		return ""
	}
	t := time.Now().AddDate(0, 0, -days)
	return fmt.Sprintf("--since=%s", t.Format("2006-01-02"))
}

// AllUsers returns all unique users who have committed to any branch.
func AllUsers() ([]BranchUser, error) {
	out, err := runGit("log", "--all", "--format=%an|%ae", "--no-merges")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var users []BranchUser
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0] + "|" + parts[1]
		if !seen[key] {
			seen[key] = true
			users = append(users, BranchUser{Name: parts[0], Email: parts[1]})
		}
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Name < users[j].Name })
	return users, nil
}

// BranchesForUser returns branches where the given user (by name or email) was active.
// days <= 0 means no time restriction.
func BranchesForUser(user string, days int) ([]string, error) {
	branches, err := ListBranches()
	if err != nil {
		return nil, err
	}
	user = strings.ToLower(user)
	since := sinceFlag(days)
	var result []string
	for _, b := range branches {
		args := []string{"log", b, "--format=%an|%ae", "--no-merges"}
		if since != "" {
			args = append(args, since)
		}
		out, err := runGit(args...)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.Contains(strings.ToLower(line), user) {
				result = append(result, b)
				break
			}
		}
	}
	return result, nil
}

// FilesTouchedByUser returns files changed by the given user, optionally filtered
// by branch and/or a number of days. days <= 0 means no time restriction.
// branch == "" means all branches (uses --all).
func FilesTouchedByUser(user, branch string, days int) ([]FileChange, error) {
	since := sinceFlag(days)
	user = strings.ToLower(user)

	// Collect matching commits first using log
	logArgs := []string{"log", "--format=%H|%an|%ae", "--no-merges"}
	if branch != "" {
		logArgs = append(logArgs, branch)
	} else {
		logArgs = append(logArgs, "--all")
	}
	if since != "" {
		logArgs = append(logArgs, since)
	}
	out, err := runGit(logArgs...)
	if err != nil {
		return nil, err
	}

	fileCounts := map[string]int{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		hash, name, email := parts[0], strings.ToLower(parts[1]), strings.ToLower(parts[2])
		if !strings.Contains(name, user) && !strings.Contains(email, user) {
			continue
		}
		// Get files changed in this commit. --root is required so that the
		// initial (root) commit is compared against an empty tree rather than
		// being silently skipped.
		filesOut, err := runGit("diff-tree", "--no-commit-id", "--root", "-r", "--name-only", hash)
		if err != nil {
			continue
		}
		for _, f := range strings.Split(filesOut, "\n") {
			f = strings.TrimSpace(f)
			if f != "" {
				fileCounts[f]++
			}
		}
	}

	result := make([]FileChange, 0, len(fileCounts))
	for f, c := range fileCounts {
		result = append(result, FileChange{File: f, Changes: c})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Changes != result[j].Changes {
			return result[i].Changes > result[j].Changes
		}
		return result[i].File < result[j].File
	})
	return result, nil
}

// CommitsForFile returns detailed commit information for a specific file and user.
func CommitsForFile(user, file, branch string, days int) ([]CommitInfo, error) {
	since := sinceFlag(days)
	// --author uses a case-sensitive regexp; keep the same casing as %an in commits.
	args := []string{"log", "--format=%H|%an|%ad|%s%n%b%nENC_COMMIT_END", "--date=short", "--author=" + user, "--no-merges"}
	if branch != "" {
		args = append(args, branch)
	} else {
		args = append(args, "--all")
	}
	if since != "" {
		args = append(args, since)
	}
	args = append(args, "--", file)

	out, err := runGit(args...)
	if err != nil {
		return nil, err
	}

	return parseCommitsLogOutput(out), nil
}

// parseCommitsLogOutput parses git log output delimited by ENC_COMMIT_END markers.
func parseCommitsLogOutput(out string) []CommitInfo {
	var commits []CommitInfo
	chunks := strings.Split(out, "ENC_COMMIT_END")
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}

		lines := strings.SplitN(chunk, "\n", 2)
		header := lines[0]
		body := ""
		if len(lines) > 1 {
			body = strings.TrimSpace(lines[1])
		}

		parts := strings.SplitN(header, "|", 4)
		if len(parts) < 4 {
			continue
		}

		commits = append(commits, CommitInfo{
			Hash:    parts[0],
			Author:  parts[1],
			Date:    parts[2],
			Subject: parts[3],
			Body:    body,
		})
	}
	return commits
}
