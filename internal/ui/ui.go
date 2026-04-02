// Package ui implements the bubbletea TUI for gitinfo.
package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/altafino/gitinfo/internal/git"
)

// ─── Styles ──────────────────────────────────────────────────────────────────

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	branchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("213"))

	fileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	inputLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	countStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

// ─── View identifiers ────────────────────────────────────────────────────────

type viewID int

const (
	viewMenu viewID = iota
	viewBranchUsers
	viewUserBranches
	viewUserBranchesUserSelect
	viewUserBranchesInput
	viewUserFiles
	viewUserFilesUserSelect
	viewUserFilesInput
)

// ─── Menu item ───────────────────────────────────────────────────────────────

type menuItem struct {
	title, desc string
}

func (m menuItem) Title() string       { return m.title }
func (m menuItem) Description() string { return m.desc }
func (m menuItem) FilterValue() string { return m.title }

// ─── Messages ────────────────────────────────────────────────────────────────

type branchUsersMsg []git.BranchInfo
type userBranchesMsg []string
type userFilesMsg []git.FileChange
type gitUsersMsg []git.BranchUser
type errMsg struct{ err error }

// ─── User list item ──────────────────────────────────────────────────────────

type userItem struct {
	user git.BranchUser
}

func (u userItem) Title() string       { return u.user.Name }
func (u userItem) Description() string { return u.user.Email }
func (u userItem) FilterValue() string { return u.user.Name + " " + u.user.Email }

// ─── Input form ──────────────────────────────────────────────────────────────

type inputForm struct {
	fields  []textinput.Model
	focused int
	labels  []string
}

func newInputForm(labels []string) inputForm {
	fields := make([]textinput.Model, len(labels))
	for i := range labels {
		ti := textinput.New()
		ti.Prompt = ""
		if i == 0 {
			ti.Focus()
		}
		fields[i] = ti
	}
	return inputForm{fields: fields, labels: labels}
}

func (f *inputForm) nextField() {
	f.fields[f.focused].Blur()
	f.focused = (f.focused + 1) % len(f.fields)
	f.fields[f.focused].Focus()
}

func (f inputForm) value(i int) string { return f.fields[i].Value() }

func (f inputForm) view() string {
	var b strings.Builder
	for i, lbl := range f.labels {
		cursor := "  "
		if i == f.focused {
			cursor = "> "
		}
		b.WriteString(fmt.Sprintf("%s%s: %s\n",
			cursor,
			inputLabelStyle.Render(lbl),
			f.fields[i].View(),
		))
	}
	return b.String()
}

func (f inputForm) update(msg tea.Msg) (inputForm, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	f.fields[f.focused], cmd = f.fields[f.focused].Update(msg)
	cmds = append(cmds, cmd)
	return f, tea.Batch(cmds...)
}

// ─── Model ───────────────────────────────────────────────────────────────────

// Model is the root bubbletea model.
type Model struct {
	view    viewID
	width   int
	height  int
	err     error
	loading bool
	spinner spinner.Model

	menu list.Model

	// branch-users view
	branchInfos []git.BranchInfo
	buScroll    int

	// user-branches view
	ubForm     inputForm
	ubBranches []string

	// user-files view
	ufForm   inputForm
	ufFiles  []git.FileChange
	ufScroll int

	// user selection (shared by user-branches and user-files flows)
	pendingView  viewID
	selectedUser git.BranchUser
	userList     list.Model
}

// New creates and returns the initial model.
func New() Model {
	items := []list.Item{
		menuItem{
			title: "Branch Users",
			desc:  "List users active on each branch",
		},
		menuItem{
			title: "User Branches",
			desc:  "Find branches where a user was active (optionally filter by days)",
		},
		menuItem{
			title: "User Files",
			desc:  "Show files touched by a user (filter by branch and/or days)",
		},
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = selectedStyle
	delegate.Styles.SelectedDesc = subtitleStyle

	l := list.New(items, delegate, 60, 10)
	l.Title = "gitinfo"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		view:    viewMenu,
		menu:    l,
		spinner: sp,
	}
}

// ─── Init ────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return nil
}

// ─── Update ──────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.menu.SetSize(msg.Width-4, msg.Height-6)
		return m, nil

	case tea.KeyMsg:
		// For user-selection views, most keys are delegated to the list;
		// only intercept enter/esc/q when the list is NOT in filter mode.
		if m.view == viewUserBranchesUserSelect || m.view == viewUserFilesUserSelect {
			notFiltering := m.userList.FilterState() != list.Filtering
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "q", "esc":
				if notFiltering {
					m.view = viewMenu
					return m, nil
				}
			case "enter":
				if notFiltering {
					if m.view == viewUserBranchesUserSelect {
						return m.handleUserSelectForBranches()
					}
					return m.handleUserSelectForFiles()
				}
			}
			var cmd tea.Cmd
			m.userList, cmd = m.userList.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if m.view == viewMenu {
				return m, tea.Quit
			}
			// Go back to menu from any sub-view
			m.view = viewMenu
			m.err = nil
			return m, nil

		case "esc":
			switch m.view {
			case viewBranchUsers, viewUserBranches, viewUserFiles:
				m.view = viewMenu
				m.err = nil
				return m, nil
			case viewUserBranchesInput:
				m.view = viewUserBranchesUserSelect
				return m, nil
			case viewUserFilesInput:
				m.view = viewUserFilesUserSelect
				return m, nil
			}

		case "enter":
			switch m.view {
			case viewMenu:
				return m.handleMenuEnter()
			case viewUserBranchesInput:
				return m.submitUserBranchesForm()
			case viewUserFilesInput:
				return m.submitUserFilesForm()
			}

		case "tab":
			switch m.view {
			case viewUserBranchesInput:
				m.ubForm.nextField()
				return m, nil
			case viewUserFilesInput:
				m.ufForm.nextField()
				return m, nil
			}

		case "up", "k":
			switch m.view {
			case viewBranchUsers:
				if m.buScroll > 0 {
					m.buScroll--
				}
				return m, nil
			case viewUserFiles:
				if m.ufScroll > 0 {
					m.ufScroll--
				}
				return m, nil
			}
		case "down", "j":
			switch m.view {
			case viewBranchUsers:
				m.buScroll++
				return m, nil
			case viewUserFiles:
				m.ufScroll++
				return m, nil
			}
		}

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case branchUsersMsg:
		m.loading = false
		m.branchInfos = []git.BranchInfo(msg)
		m.buScroll = 0
		m.view = viewBranchUsers
		return m, nil

	case userBranchesMsg:
		m.loading = false
		m.ubBranches = []string(msg)
		m.view = viewUserBranches
		return m, nil

	case userFilesMsg:
		m.loading = false
		m.ufFiles = []git.FileChange(msg)
		m.ufScroll = 0
		m.view = viewUserFiles
		return m, nil

	case gitUsersMsg:
		m.loading = false
		items := make([]list.Item, len(msg))
		for i, u := range msg {
			items[i] = userItem{u}
		}
		delegate := list.NewDefaultDelegate()
		delegate.Styles.SelectedTitle = selectedStyle
		delegate.Styles.SelectedDesc = subtitleStyle
		ul := list.New(items, delegate, 60, 10)
		ul.Title = "Select User"
		ul.SetShowStatusBar(false)
		ul.SetFilteringEnabled(true)
		ul.Styles.Title = titleStyle
		if m.width > 0 {
			ul.SetSize(m.width-4, m.height-6)
		}
		m.userList = ul
		m.view = m.pendingView
		return m, nil

	case errMsg:
		m.loading = false
		m.err = msg.err
		return m, nil
	}

	// Delegate to sub-model updates
	switch m.view {
	case viewMenu:
		var cmd tea.Cmd
		m.menu, cmd = m.menu.Update(msg)
		return m, cmd

	case viewUserBranchesUserSelect, viewUserFilesUserSelect:
		var cmd tea.Cmd
		m.userList, cmd = m.userList.Update(msg)
		return m, cmd

	case viewUserBranchesInput:
		var cmd tea.Cmd
		m.ubForm, cmd = m.ubForm.update(msg)
		return m, cmd

	case viewUserFilesInput:
		var cmd tea.Cmd
		m.ufForm, cmd = m.ufForm.update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleMenuEnter() (tea.Model, tea.Cmd) {
	selected, ok := m.menu.SelectedItem().(menuItem)
	if !ok {
		return m, nil
	}
	switch selected.title {
	case "Branch Users":
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, loadBranchUsers())
	case "User Branches":
		m.loading = true
		m.pendingView = viewUserBranchesUserSelect
		return m, tea.Batch(m.spinner.Tick, loadAllUsers())
	case "User Files":
		m.loading = true
		m.pendingView = viewUserFilesUserSelect
		return m, tea.Batch(m.spinner.Tick, loadAllUsers())
	}
	return m, nil
}

func (m Model) handleUserSelectForBranches() (tea.Model, tea.Cmd) {
	selected, ok := m.userList.SelectedItem().(userItem)
	if !ok {
		return m, nil
	}
	m.selectedUser = selected.user
	m.ubForm = newInputForm([]string{"Last N days (leave empty for all)"})
	m.view = viewUserBranchesInput
	return m, nil
}

func (m Model) handleUserSelectForFiles() (tea.Model, tea.Cmd) {
	selected, ok := m.userList.SelectedItem().(userItem)
	if !ok {
		return m, nil
	}
	m.selectedUser = selected.user
	m.ufForm = newInputForm([]string{"Branch (leave empty for all)", "Last N days (leave empty for all)"})
	m.view = viewUserFilesInput
	return m, nil
}

func (m Model) submitUserBranchesForm() (tea.Model, tea.Cmd) {
	user := m.selectedUser.Name
	days := parseDays(m.ubForm.value(0))
	m.loading = true
	return m, tea.Batch(m.spinner.Tick, loadUserBranches(user, days))
}

func (m Model) submitUserFilesForm() (tea.Model, tea.Cmd) {
	user := m.selectedUser.Name
	branch := strings.TrimSpace(m.ufForm.value(0))
	days := parseDays(m.ufForm.value(1))
	m.loading = true
	return m, tea.Batch(m.spinner.Tick, loadUserFiles(user, branch, days))
}

// parseDays parses a string as a positive integer; returns 0 on failure.
func parseDays(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// ─── Async commands ──────────────────────────────────────────────────────────

func loadBranchUsers() tea.Cmd {
	return func() tea.Msg {
		infos, err := git.BranchUsers()
		if err != nil {
			return errMsg{err}
		}
		return branchUsersMsg(infos)
	}
}

func loadAllUsers() tea.Cmd {
	return func() tea.Msg {
		users, err := git.AllUsers()
		if err != nil {
			return errMsg{err}
		}
		return gitUsersMsg(users)
	}
}

func loadUserBranches(user string, days int) tea.Cmd {
	return func() tea.Msg {
		branches, err := git.BranchesForUser(user, days)
		if err != nil {
			return errMsg{err}
		}
		return userBranchesMsg(branches)
	}
}

func loadUserFiles(user, branch string, days int) tea.Cmd {
	return func() tea.Msg {
		files, err := git.FilesTouchedByUser(user, branch, days)
		if err != nil {
			return errMsg{err}
		}
		return userFilesMsg(files)
	}
}

// ─── View ────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.loading {
		return "\n  " + m.spinner.View() + " Loading…\n"
	}
	if m.err != nil {
		return errorStyle.Render("\n  Error: "+m.err.Error()) +
			helpStyle.Render("\n\n  Press q or esc to go back.")
	}

	switch m.view {
	case viewMenu:
		return m.menuView()
	case viewBranchUsers:
		return m.branchUsersView()
	case viewUserBranchesUserSelect:
		return m.userSelectView("User Branches")
	case viewUserBranchesInput:
		return m.userBranchesInputView()
	case viewUserBranches:
		return m.userBranchesView()
	case viewUserFilesUserSelect:
		return m.userSelectView("User Files")
	case viewUserFilesInput:
		return m.userFilesInputView()
	case viewUserFiles:
		return m.userFilesView()
	}
	return ""
}

func (m Model) menuView() string {
	return "\n" + m.menu.View() +
		helpStyle.Render("  ↑/↓ navigate  •  enter select  •  q quit")
}

func (m Model) branchUsersView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  Branch Users"))
	b.WriteString("\n")

	if len(m.branchInfos) == 0 {
		b.WriteString(subtitleStyle.Render("  No branches found."))
		b.WriteString("\n")
	} else {
		// Calculate visible lines
		maxLines := m.height - 6
		if maxLines < 1 {
			maxLines = 20
		}

		lines := buildBranchUsersLines(m.branchInfos)
		total := len(lines)

		// Clamp scroll
		scroll := m.buScroll
		if scroll > total-maxLines {
			scroll = total - maxLines
		}
		if scroll < 0 {
			scroll = 0
		}

		end := scroll + maxLines
		if end > total {
			end = total
		}

		for _, l := range lines[scroll:end] {
			b.WriteString(l)
			b.WriteString("\n")
		}

		if total > maxLines {
			b.WriteString(countStyle.Render(fmt.Sprintf(
				"  showing %d-%d of %d lines", scroll+1, end, total,
			)))
			b.WriteString("\n")
		}
	}

	b.WriteString(helpStyle.Render("  ↑/↓ scroll  •  esc/q back"))
	return b.String()
}

func buildBranchUsersLines(infos []git.BranchInfo) []string {
	var lines []string
	for _, info := range infos {
		lines = append(lines, branchStyle.Render(fmt.Sprintf("  %-40s", info.Branch))+
			countStyle.Render(fmt.Sprintf("(%d user(s))", len(info.Users))))
		for _, u := range info.Users {
			lines = append(lines, userStyle.Render(fmt.Sprintf("    • %s", u.Name))+
				countStyle.Render(fmt.Sprintf(" <%s>", u.Email)))
		}
		if len(info.Users) == 0 {
			lines = append(lines, countStyle.Render("    (no commits)"))
		}
	}
	return lines
}

func (m Model) userSelectView(title string) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  " + title))
	b.WriteString("\n")
	b.WriteString(m.userList.View())
	b.WriteString(helpStyle.Render("  ↑/↓ navigate  •  / filter  •  enter select  •  esc/q back"))
	return b.String()
}

func (m Model) userBranchesInputView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  User Branches"))
	b.WriteString("\n")
	b.WriteString(userStyle.Render(fmt.Sprintf("  User: %s <%s>\n", m.selectedUser.Name, m.selectedUser.Email)))
	b.WriteString("\n")
	b.WriteString(m.ubForm.view())
	b.WriteString(helpStyle.Render("\n  tab next field  •  enter search  •  esc back"))
	return b.String()
}

func (m Model) userBranchesView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  User Branches"))
	b.WriteString("\n")

	if len(m.ubBranches) == 0 {
		b.WriteString(subtitleStyle.Render("  No branches found for this user."))
		b.WriteString("\n")
	} else {
		b.WriteString(countStyle.Render(fmt.Sprintf("  Found %d branch(es):\n\n", len(m.ubBranches))))
		for _, br := range m.ubBranches {
			b.WriteString(branchStyle.Render(fmt.Sprintf("  • %s", br)))
			b.WriteString("\n")
		}
	}

	b.WriteString(helpStyle.Render("\n  esc/q back"))
	return b.String()
}

func (m Model) userFilesInputView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  User Files"))
	b.WriteString("\n")
	b.WriteString(userStyle.Render(fmt.Sprintf("  User: %s <%s>\n", m.selectedUser.Name, m.selectedUser.Email)))
	b.WriteString("\n")
	b.WriteString(m.ufForm.view())
	b.WriteString(helpStyle.Render("\n  tab next field  •  enter search  •  esc back"))
	return b.String()
}

func (m Model) userFilesView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  User Files"))
	b.WriteString("\n")

	if len(m.ufFiles) == 0 {
		b.WriteString(subtitleStyle.Render("  No files found for this user."))
		b.WriteString("\n")
	} else {
		maxLines := m.height - 6
		if maxLines < 1 {
			maxLines = 20
		}

		total := len(m.ufFiles)
		scroll := m.ufScroll
		if scroll > total-maxLines {
			scroll = total - maxLines
		}
		if scroll < 0 {
			scroll = 0
		}
		end := scroll + maxLines
		if end > total {
			end = total
		}

		b.WriteString(countStyle.Render(fmt.Sprintf("  Found %d file(s):\n\n", total)))
		for _, fc := range m.ufFiles[scroll:end] {
			b.WriteString(fileStyle.Render(fmt.Sprintf("  • %-60s", fc.File)))
			b.WriteString(countStyle.Render(fmt.Sprintf(" %d commit(s)", fc.Changes)))
			b.WriteString("\n")
		}

		if total > maxLines {
			b.WriteString(countStyle.Render(fmt.Sprintf(
				"  showing %d-%d of %d files", scroll+1, end, total,
			)))
			b.WriteString("\n")
		}
	}

	b.WriteString(helpStyle.Render("\n  ↑/↓ scroll  •  esc/q back"))
	return b.String()
}
