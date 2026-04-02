// Package ui implements the bubbletea TUI for gitinfo.
package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

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

	hashStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	dateStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("141"))

	subjectStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	bodyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
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
	viewFileCommits
	viewUserDashboardUserSelect
	viewUserDashboardInput
	viewUserDashboard
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
type fileCommitsMsg []git.CommitInfo
type userDashboardMsg struct{ data *git.UserDashboard }
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
	branchInfos     []git.BranchInfo
	buScroll        int
	buCursor        int // index into buFlatUsers
	buFlatUsers     []git.BranchUser
	buUserLineIndex []int // global line index per buFlatUsers[i]
	buLineCount     int

	// when true, Esc from dashboard day form returns to Branch Users instead of user list
	udFromBranchUsers bool

	// user-branches view
	ubForm     inputForm
	ubBranches []string

	// user-files view
	ufForm   inputForm
	ufFiles  []git.FileChange
	ufScroll int
	ufCursor int

	// file-commits view
	fcCommits    []git.CommitInfo
	fcScroll     int
	selectedFile string

	// user dashboard
	udForm   inputForm
	udData   *git.UserDashboard
	udScroll int

	// user selection (shared by user-branches, user-files, user-dashboard flows)
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
		menuItem{
			title: "User Dashboard",
			desc:  "Summary stats and activity for one author (optional day filter)",
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
		if m.view == viewUserBranchesUserSelect || m.view == viewUserFilesUserSelect || m.view == viewUserDashboardUserSelect {
			m.userList.SetSize(msg.Width-4, msg.Height-6)
		}
		if m.view == viewBranchUsers && len(m.buFlatUsers) > 0 {
			m.syncBranchUsersScroll()
		}
		return m, nil

	case tea.KeyMsg:
		// For user-selection views, most keys are delegated to the list;
		// only intercept enter/esc/q when the list is NOT in filter mode.
		if m.view == viewUserBranchesUserSelect || m.view == viewUserFilesUserSelect || m.view == viewUserDashboardUserSelect {
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
					if m.view == viewUserFilesUserSelect {
						return m.handleUserSelectForFiles()
					}
					return m.handleUserSelectForDashboard()
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
			if m.view == viewFileCommits {
				m.view = viewUserFiles
				return m, nil
			}
			if m.view == viewUserDashboard {
				m.view = viewMenu
				m.err = nil
				m.udFromBranchUsers = false
				return m, nil
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
			case viewFileCommits:
				m.view = viewUserFiles
				return m, nil
			case viewUserBranchesInput:
				m.view = viewUserBranchesUserSelect
				return m, nil
			case viewUserFilesInput:
				m.view = viewUserFilesUserSelect
				return m, nil
			case viewUserDashboard:
				m.view = viewUserDashboardInput
				return m, nil
			case viewUserDashboardInput:
				if m.udFromBranchUsers {
					m.view = viewBranchUsers
					m.udFromBranchUsers = false
					return m, nil
				}
				m.view = viewUserDashboardUserSelect
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
			case viewUserFiles:
				return m.handleFileSelect()
			case viewUserDashboardInput:
				return m.submitUserDashboardForm()
			case viewBranchUsers:
				return m.handleBranchUserOpenDashboard()
			}

		case "tab":
			switch m.view {
			case viewUserBranchesInput:
				m.ubForm.nextField()
				return m, nil
			case viewUserFilesInput:
				m.ufForm.nextField()
				return m, nil
			case viewUserDashboardInput:
				m.udForm.nextField()
				return m, nil
			}

		case "up", "k":
			switch m.view {
			case viewBranchUsers:
				if len(m.buFlatUsers) == 0 {
					return m, nil
				}
				if m.buCursor > 0 {
					m.buCursor--
					m.syncBranchUsersScroll()
				}
				return m, nil
			case viewUserFiles:
				if m.ufCursor > 0 {
					m.ufCursor--
					m.syncUserFilesScroll()
				}
				return m, nil
			case viewFileCommits:
				if m.fcScroll > 0 {
					m.fcScroll--
				}
				return m, nil
			case viewUserDashboard:
				if m.udScroll > 0 {
					m.udScroll--
				}
				return m, nil
			}
		case "down", "j":
			switch m.view {
			case viewBranchUsers:
				if len(m.buFlatUsers) == 0 {
					return m, nil
				}
				if m.buCursor < len(m.buFlatUsers)-1 {
					m.buCursor++
					m.syncBranchUsersScroll()
				}
				return m, nil
			case viewUserFiles:
				if m.ufCursor < len(m.ufFiles)-1 {
					m.ufCursor++
					m.syncUserFilesScroll()
				}
				return m, nil
			case viewFileCommits:
				m.fcScroll++
				return m, nil
			case viewUserDashboard:
				m.udScroll++
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
		m.rebuildBranchUsersSelection()
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
		m.ufCursor = 0
		m.view = viewUserFiles
		return m, nil

	case fileCommitsMsg:
		m.loading = false
		m.fcCommits = []git.CommitInfo(msg)
		m.fcScroll = 0
		m.view = viewFileCommits
		return m, nil

	case userDashboardMsg:
		m.loading = false
		m.udData = msg.data
		m.udScroll = 0
		m.view = viewUserDashboard
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

	case viewUserBranchesUserSelect, viewUserFilesUserSelect, viewUserDashboardUserSelect:
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

	case viewUserDashboardInput:
		var cmd tea.Cmd
		m.udForm, cmd = m.udForm.update(msg)
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
	case "User Dashboard":
		m.loading = true
		m.pendingView = viewUserDashboardUserSelect
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

func (m Model) handleUserSelectForDashboard() (tea.Model, tea.Cmd) {
	selected, ok := m.userList.SelectedItem().(userItem)
	if !ok {
		return m, nil
	}
	m.selectedUser = selected.user
	m.udFromBranchUsers = false
	m.udForm = newInputForm([]string{"Last N days (leave empty for all)"})
	m.view = viewUserDashboardInput
	return m, nil
}

func (m Model) handleBranchUserOpenDashboard() (tea.Model, tea.Cmd) {
	if len(m.buFlatUsers) == 0 {
		return m, nil
	}
	m.selectedUser = m.buFlatUsers[m.buCursor]
	m.udFromBranchUsers = true
	m.udForm = newInputForm([]string{"Last N days (leave empty for all)"})
	m.view = viewUserDashboardInput
	return m, nil
}

func (m Model) submitUserDashboardForm() (tea.Model, tea.Cmd) {
	days := parseDays(m.udForm.value(0))
	m.loading = true
	return m, tea.Batch(m.spinner.Tick, loadUserDashboard(m.selectedUser, days))
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

func (m Model) handleFileSelect() (tea.Model, tea.Cmd) {
	if len(m.ufFiles) == 0 {
		return m, nil
	}
	file := m.ufFiles[m.ufCursor].File
	m.selectedFile = file
	m.loading = true

	// To load commits, we need the user, branch, and days from the form
	user := m.selectedUser.Name
	branch := strings.TrimSpace(m.ufForm.value(0))
	days := parseDays(m.ufForm.value(1))

	return m, tea.Batch(m.spinner.Tick, loadFileCommits(user, file, branch, days))
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

// syncUserFilesScroll keeps the file list window scrolled so the cursor stays visible.
func (m *Model) syncUserFilesScroll() {
	maxLines := m.height - 6
	if maxLines < 1 {
		maxLines = 20
	}
	total := len(m.ufFiles)
	if total == 0 {
		return
	}
	if total <= maxLines {
		m.ufScroll = 0
		return
	}
	if m.ufCursor < m.ufScroll {
		m.ufScroll = m.ufCursor
	}
	if m.ufCursor >= m.ufScroll+maxLines {
		m.ufScroll = m.ufCursor - maxLines + 1
	}
	if m.ufScroll > total-maxLines {
		m.ufScroll = total - maxLines
	}
	if m.ufScroll < 0 {
		m.ufScroll = 0
	}
}

// buildBranchUsersSelection flattens users in display order and maps each to a global line index.
func buildBranchUsersSelection(infos []git.BranchInfo) (users []git.BranchUser, lineOfUser []int, total int) {
	lineNum := 0
	for _, info := range infos {
		lineNum++ // branch header line
		for _, u := range info.Users {
			users = append(users, u)
			lineOfUser = append(lineOfUser, lineNum)
			lineNum++
		}
		if len(info.Users) == 0 {
			lineNum++
		}
	}
	return users, lineOfUser, lineNum
}

func (m *Model) rebuildBranchUsersSelection() {
	m.buFlatUsers, m.buUserLineIndex, m.buLineCount = buildBranchUsersSelection(m.branchInfos)
	m.buCursor = 0
	m.buScroll = 0
	if len(m.buFlatUsers) > 0 {
		m.syncBranchUsersScroll()
	}
}

func (m *Model) syncBranchUsersScroll() {
	if len(m.buFlatUsers) == 0 {
		return
	}
	maxLines := m.height - 6
	if maxLines < 1 {
		maxLines = 20
	}
	total := m.buLineCount
	selLine := m.buUserLineIndex[m.buCursor]
	if total <= maxLines {
		m.buScroll = 0
		return
	}
	if selLine < m.buScroll {
		m.buScroll = selLine
	}
	if selLine >= m.buScroll+maxLines {
		m.buScroll = selLine - maxLines + 1
	}
	if m.buScroll > total-maxLines {
		m.buScroll = total - maxLines
	}
	if m.buScroll < 0 {
		m.buScroll = 0
	}
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

func loadFileCommits(user, file, branch string, days int) tea.Cmd {
	return func() tea.Msg {
		commits, err := git.CommitsForFile(user, file, branch, days)
		if err != nil {
			return errMsg{err}
		}
		return fileCommitsMsg(commits)
	}
}

func loadUserDashboard(user git.BranchUser, days int) tea.Cmd {
	return func() tea.Msg {
		d, err := git.UserDashboardStats(user, days)
		if err != nil {
			return errMsg{err}
		}
		return userDashboardMsg{data: d}
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
	case viewFileCommits:
		return m.fileCommitsView()
	case viewUserDashboardUserSelect:
		return m.userSelectView("User Dashboard")
	case viewUserDashboardInput:
		return m.userDashboardInputView()
	case viewUserDashboard:
		return m.userDashboardView()
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
		maxLines := m.height - 6
		if maxLines < 1 {
			maxLines = 20
		}

		total := m.buLineCount
		if total == 0 {
			total = len(buildBranchUsersLines(m.branchInfos))
		}

		scroll := m.buScroll
		if total > maxLines && scroll > total-maxLines {
			scroll = total - maxLines
		}
		if scroll < 0 {
			scroll = 0
		}

		end := scroll + maxLines
		if end > total {
			end = total
		}

		lineNum := 0
		for _, info := range m.branchInfos {
			if lineNum >= scroll && lineNum < end {
				b.WriteString(branchStyle.Render(fmt.Sprintf("  %-40s", info.Branch)) +
					countStyle.Render(fmt.Sprintf("(%d user(s))", len(info.Users))))
				b.WriteString("\n")
			}
			lineNum++

			for _, u := range info.Users {
				if lineNum >= scroll && lineNum < end {
					userLine := fmt.Sprintf("    • %s <%s>", u.Name, u.Email)
					if len(m.buFlatUsers) > 0 && lineNum == m.buUserLineIndex[m.buCursor] {
						b.WriteString(selectedStyle.Render(userLine))
					} else {
						b.WriteString(userStyle.Render(fmt.Sprintf("    • %s", u.Name)) +
							countStyle.Render(fmt.Sprintf(" <%s>", u.Email)))
					}
					b.WriteString("\n")
				}
				lineNum++
			}
			if len(info.Users) == 0 {
				if lineNum >= scroll && lineNum < end {
					b.WriteString(countStyle.Render("    (no commits)"))
					b.WriteString("\n")
				}
				lineNum++
			}
		}

		if total > maxLines {
			b.WriteString(countStyle.Render(fmt.Sprintf(
				"  showing %d-%d of %d lines", scroll+1, end, total,
			)))
			b.WriteString("\n")
		}
	}

	b.WriteString(helpStyle.Render("  ↑/↓ select user  •  enter dashboard  •  esc/q back"))
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
		b.WriteString(countStyle.Render(fmt.Sprintf("  Found %d branch(es):", len(m.ubBranches))))
		b.WriteString("\n")
		for _, br := range m.ubBranches {
			b.WriteString("  • ")
			b.WriteString(branchStyle.Render(br))
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

		b.WriteString(countStyle.Render(fmt.Sprintf("  Found %d file(s):", total)))
		b.WriteString("\n")

		fileColW := userFilesNameColumnWidth(m.width)
		for i, fc := range m.ufFiles {
			// Windowing the list
			if i < scroll || i >= end {
				continue
			}

			cursor := "  "
			style := fileStyle
			if i == m.ufCursor {
				cursor = "> "
				style = selectedStyle
			}

			nameCol := padFileNameColumn(fc.File, fileColW)
			b.WriteString(style.Render(cursor + nameCol))
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

	b.WriteString(helpStyle.Render("\n  ↑/↓ scroll  •  enter select  •  esc/q back"))
	return b.String()
}

func (m Model) fileCommitsView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  Commit History"))
	b.WriteString("\n")
	b.WriteString(fileStyle.Render(fmt.Sprintf("  File: %s", m.selectedFile)))
	b.WriteString("\n")
	b.WriteString(userStyle.Render(fmt.Sprintf("  User: %s <%s>", m.selectedUser.Name, m.selectedUser.Email)))
	b.WriteString("\n\n")

	if len(m.fcCommits) == 0 {
		b.WriteString(subtitleStyle.Render("  No commits found for this file/user."))
		b.WriteString("\n")
	} else {
		// Calculate visible lines
		maxLines := m.height - 8
		if maxLines < 1 {
			maxLines = 20
		}

		var lines []string
		for _, c := range m.fcCommits {
			shortHash := c.Hash
			if len(shortHash) > 7 {
				shortHash = shortHash[:7]
			}
			lines = append(lines, hashStyle.Render(fmt.Sprintf("  %s  ", shortHash))+
				dateStyle.Render(c.Date)+
				"  "+
				userStyle.Render(c.Author))
			lines = append(lines, "  "+subjectStyle.Render(c.Subject))
			if c.Body != "" {
				bodyLines := strings.Split(c.Body, "\n")
				for _, bl := range bodyLines {
					bl = strings.TrimSpace(bl)
					if bl != "" {
						lines = append(lines, bodyStyle.Render(fmt.Sprintf("    %s", bl)))
					}
				}
			}
			lines = append(lines, "") // spacer between commits
		}

		total := len(lines)
		scroll := m.fcScroll
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
				"  (scrolled %d/%d lines)", end, total,
			)))
			b.WriteString("\n")
		}
	}

	b.WriteString(helpStyle.Render("\n  ↑/↓ scroll  •  esc/q back to list"))
	return b.String()
}

func (m Model) userDashboardInputView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  User Dashboard"))
	b.WriteString("\n")
	b.WriteString(userStyle.Render(fmt.Sprintf("  User: %s <%s>\n", m.selectedUser.Name, m.selectedUser.Email)))
	b.WriteString("\n")
	b.WriteString(m.udForm.view())
	b.WriteString(helpStyle.Render("\n  tab next field  •  enter load dashboard  •  esc back"))
	return b.String()
}

func (m Model) userDashboardView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  User Dashboard"))
	b.WriteString("\n")

	if m.udData == nil {
		b.WriteString(subtitleStyle.Render("  No data loaded."))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("\n  esc back  •  q menu"))
		return b.String()
	}

	lines := buildUserDashboardLines(m.udData, m.width)
	maxLines := m.height - 6
	if maxLines < 1 {
		maxLines = 20
	}
	total := len(lines)
	scroll := m.udScroll
	if total > maxLines && scroll > total-maxLines {
		scroll = total - maxLines
	}
	if scroll < 0 {
		scroll = 0
	}
	end := scroll + maxLines
	if end > total {
		end = total
	}

	for _, line := range lines[scroll:end] {
		b.WriteString(line)
		b.WriteString("\n")
	}

	if total > maxLines {
		b.WriteString(countStyle.Render(fmt.Sprintf(
			"  lines %d-%d of %d", scroll+1, end, total,
		)))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("\n  ↑/↓ scroll  •  esc back to filter  •  q menu"))
	return b.String()
}

func buildUserDashboardLines(d *git.UserDashboard, width int) []string {
	maxW := width - 4
	if maxW < 40 {
		maxW = 40
	}
	var lines []string
	lines = append(lines, userStyle.Render(fmt.Sprintf("  %s <%s>", d.User.Name, d.User.Email)))
	if d.Days > 0 {
		lines = append(lines, countStyle.Render(fmt.Sprintf("  Time filter: last %d day(s)", d.Days)))
	} else {
		lines = append(lines, countStyle.Render("  Time filter: all history"))
	}
	lines = append(lines, "")
	lines = append(lines, subtitleStyle.Render("  Summary"))
	lines = append(lines, fmt.Sprintf("  Non-merge commits: %d", d.CommitsNonMerge))
	lines = append(lines, fmt.Sprintf("  Merge commits:     %d", d.CommitsMerge))
	if d.HasActivity && !d.FirstCommit.IsZero() {
		lines = append(lines, fmt.Sprintf("  First commit:      %s", d.FirstCommit.Format(time.RFC3339)))
		lines = append(lines, fmt.Sprintf("  Last commit:       %s", d.LastCommit.Format(time.RFC3339)))
	}
	lines = append(lines, "")
	lines = append(lines, subtitleStyle.Render("  Lines changed (non-merge diffs)"))
	lines = append(lines, fmt.Sprintf("  Insertions: %+d  Deletions: %+d  Net: %+d",
		d.Insertions, d.Deletions, d.Insertions-d.Deletions))
	lines = append(lines, "")
	lines = append(lines, subtitleStyle.Render("  Branches (local, touched)"))
	if d.BranchTotal == 0 {
		lines = append(lines, countStyle.Render("  (none)"))
	} else {
		lines = append(lines, countStyle.Render(fmt.Sprintf("  Total: %d", d.BranchTotal)))
		for _, br := range d.Branches {
			lines = append(lines, branchStyle.Render("  • "+truncateRunes(br, maxW-4)))
		}
		if d.BranchTotal > len(d.Branches) {
			lines = append(lines, countStyle.Render(fmt.Sprintf("  … +%d more", d.BranchTotal-len(d.Branches))))
		}
	}
	lines = append(lines, "")
	lines = append(lines, subtitleStyle.Render("  Top files (by commit count)"))
	if len(d.TopFiles) == 0 {
		lines = append(lines, countStyle.Render("  (none)"))
	} else {
		for _, fc := range d.TopFiles {
			line := fmt.Sprintf("  %s  (%d)", fc.File, fc.Changes)
			lines = append(lines, fileStyle.Render(truncateRunes(line, maxW)))
		}
	}
	lines = append(lines, "")
	lines = append(lines, subtitleStyle.Render("  Recent commits"))
	if len(d.Recent) == 0 {
		lines = append(lines, countStyle.Render("  (none)"))
	} else {
		for _, c := range d.Recent {
			subj := truncateRunes(c.Subject, maxW-28)
			line := hashStyle.Render("  "+c.Hash) + "  " + dateStyle.Render(shortISO(c.DateISO)) + "  " + normalStyle.Render(subj)
			lines = append(lines, line)
		}
	}
	lines = append(lines, "")
	lines = append(lines, subtitleStyle.Render("  Commits by year (non-merge)"))
	years := git.SortedYears(d.CommitsByYear)
	if len(years) == 0 {
		lines = append(lines, countStyle.Render("  (none)"))
	} else {
		for _, y := range years {
			lines = append(lines, fmt.Sprintf("  %d: %d", y, d.CommitsByYear[y]))
		}
	}
	return lines
}

func shortISO(iso string) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	return t.Format("2006-01-02 15:04")
}

// userFilesNameColumnWidth is the display width for filenames (cursor is separate).
func userFilesNameColumnWidth(termWidth int) int {
	const minW = 24
	const maxW = 72
	w := 56
	if termWidth > 40 {
		w = termWidth - 22
	}
	switch {
	case w < minW:
		return minW
	case w > maxW:
		return maxW
	default:
		return w
	}
}

// padFileNameColumn pads or truncates a path to a fixed terminal display width.
func padFileNameColumn(name string, colW int) string {
	w := runewidth.StringWidth(name)
	if w >= colW {
		return runewidth.Truncate(name, colW, "…")
	}
	return name + strings.Repeat(" ", colW-w)
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}
