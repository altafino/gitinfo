# gitinfo

Terminal UI for exploring who did what in a Git repository. Run it from any clone to browse branches, users, and file-level history without leaving the shell.

Built with [Bubbletea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

---

## Features

| Feature | What it does |
| :--- | :--- |
| **Branch users** | For each local branch, lists unique committers (name and e-mail). |
| **User branches** | Pick a person (name or e-mail, partial match). Optionally limit to the last *N* days. Lists every branch where they committed. |
| **User files** | Pick a person, then optional branch and day window. Lists files they changed, sorted by touch count. **Select a file** to open **commit history** (hash, date, author, subject, body). **Esc** or **q** returns to the file list. |
| **User dashboard** | Pick a person from the same all-authors list, optionally limit to the last *N* days, then view a **scrollable summary**: commit counts (merge and non-merge), first/last activity, total insertions/deletions from non-merge diffs, local branches touched (capped list), top files, recent commits, and commits per calendar year. Large repos may take a moment while git aggregates data. |

---

## Quick start

```bash
cd /path/to/a/git/repo
gitinfo
```

If you are not inside a Git working tree, the program exits with an error.

---

## Install

**With the Go toolchain** (installs from the module at `@latest`):

```bash
go install github.com/altafino/gitinfo@latest
```

**From source:**

```bash
git clone https://github.com/altafino/gitinfo.git
cd gitinfo
go build -o gitinfo .
```

Put the binary on your `PATH` (for `go install`, `$GOPATH/bin` or `$(go env GOPATH)/bin` is typical).

---

## Controls

| Key | Action |
| :--- | :--- |
| **↑** **↓** (also **k** / **j** on some screens) | Move the highlight or scroll |
| **Enter** | Choose a menu item, confirm user, submit a form, or open commit details for the highlighted file |
| **e** | In the User Files list, open the highlighted file in your OS default editor/app |
| **/** | Filter the user list (user branches, user files, user dashboard) |
| **Tab** | Next field on forms |
| **Esc** or **q** | Go back one level; on the main menu, **q** exits the app. From the **User dashboard** screen, **Esc** returns to the day filter; **q** jumps to the main menu. |
| **Ctrl+C** | Quit |

---

## Screens in brief

### Branch users

Shows all local branches and the people who have non-merge commits on each branch. Use **↑** **↓** to move the highlight between users, then **Enter** to open the **User dashboard** (optional last *N* days, same as the menu entry). **Esc** from the day filter returns here.

### User branches

1. Choose **User Branches** from the menu.  
2. Select a user from the list (type **/** to narrow the list).  
3. Optionally enter **last N days** (empty means all time).  
4. You get a list of branches where that user appears in the history.

Matching is case-insensitive and substring-based on name or e-mail.

### User files

1. Choose **User Files**.  
2. Select a user (**/** to filter).  
3. Optionally set **branch** (empty = all branches) and **last N days** (empty = all time).  
4. Use **↑** **↓** to move the cursor over files; **Enter** loads **commit history** for that file and user (respecting the same branch and day filters); press **e** to open the file in your OS default editor/app.  
5. In the history view, **↑** **↓** scrolls long output; **Esc** or **q** returns to the file list.

### User dashboard

1. Choose **User Dashboard** from the menu.  
2. Select a user (**/** to filter).  
3. Optionally enter **last N days** (empty means all history), then **Enter** to load.  
4. Scroll the dashboard with **↑** **↓**. **Esc** goes back to the filter step; **q** returns to the main menu.

The dashboard matches the exact author identity (name and e-mail) from the user list. It does not include reflogs, stashes, or other data outside normal commit history.

---

## Requirements

- **Go** 1.24 or newer (to build)  
- **`git`** on `PATH`  
- Current working directory must be inside a Git repository when you run `gitinfo`

---

## License

[MIT](LICENSE)
