# gitinfo

A [Bubbletea](https://github.com/charmbracelet/bubbletea)-based interactive TUI for exploring Git repository activity.

Run **gitinfo** from inside any Git project folder to interactively browse:

| Feature | Description |
|---|---|
| **Branch Users** | Lists every branch and the users who committed on it |
| **User Branches** | Finds all branches where a given user (name or e-mail) was active, with an optional *last N days* filter |
| **User Files** | Shows every file touched by a given user, optionally filtered by branch and/or last N days |

## Installation

```bash
go install github.com/altafino/gitinfo@latest
```

Or build from source:

```bash
git clone https://github.com/altafino/gitinfo.git
cd gitinfo
go build -o gitinfo .
```

## Usage

```bash
cd /path/to/your/git/project
gitinfo
```

### Navigation

| Key | Action |
|---|---|
| `↑` / `↓` | Move selection / scroll |
| `Enter` | Select menu item / submit form |
| `Tab` | Next input field |
| `Esc` / `q` | Go back / quit |
| `Ctrl+C` | Quit |

### Menu options

#### Branch Users

Displays every local branch alongside the list of unique committers (name + e-mail).

#### User Branches

Enter a username or e-mail (partial match, case-insensitive).  
Optionally enter a number of days to restrict to recent activity.  
Displays all branches where that user has commits.

#### User Files

Enter a username or e-mail (partial match, case-insensitive).  
Optionally enter a branch name (leave empty to search all branches).  
Optionally enter a number of days to restrict to recent activity.  
Displays all files touched by that user, sorted by commit count.

## Requirements

* Go 1.24+
* `git` must be available on `$PATH`
* Must be run from inside a Git repository

## License

[MIT](LICENSE)
