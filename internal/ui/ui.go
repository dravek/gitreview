package ui

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	ggit "gitreview/internal/git"
	"gitreview/internal/repo"
	"gitreview/internal/state"
)

const (
	minWidth      = 80
	minHeight     = 24
	debounceDelay = 150 * time.Millisecond
)

type loadRequest struct {
	id         int
	singleSHA  string
	rangeSHAs  []string
	activeFile string
}

type debounceMsg struct {
	request loadRequest
}

type commitDetailsMsg struct {
	request loadRequest
	files   []string
	diff    string
	err     error
}

type Model struct {
	repos           []repo.Snapshot
	activeRepoIndex int
	info            repo.Info
	root            string
	git             ggit.Client

	allCommits []state.Commit
	commits    []state.Commit
	cursor     int
	anchor     int

	filterMode    bool
	filterQuery   string
	preFilterSHA  string
	filterActive  bool
	filterNoMatch bool
	helpVisible   bool
	repoVisible   bool
	repoCursor    int

	files      []string
	fileCursor int
	activeFile string

	diffLines  []string
	diffScroll int

	focus     state.Focus
	prevFocus state.Focus
	viewMode  state.ViewMode

	width  int
	height int
	ready  bool

	loading   bool
	err       error
	requestID int

	authorDots map[string]lipgloss.Style
}

func NewModel(repos []repo.Snapshot, gitClient ggit.Client) Model {
	m := Model{
		repos:      slices.Clone(repos),
		git:        gitClient,
		anchor:     -1,
		focus:      state.FocusCommits,
		viewMode:   state.ViewModeReview,
		repoCursor: 0,
	}
	m.applyRepo(0)
	return m
}

func (m Model) Init() tea.Cmd {
	_, cmd := m.scheduleDebouncedLoad()
	return cmd
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if !m.ready {
			m.width = msg.Width
			m.height = msg.Height
			m.ready = true
		}
		return m, nil
	case debounceMsg:
		if msg.request.id != m.requestID {
			return m, nil
		}
		if !m.hasActiveCommits() {
			m.loading = false
			m.files = nil
			m.diffLines = nil
			return m, nil
		}
		m.loading = true
		m.err = nil
		return m, m.loadRequest(msg.request)
	case commitDetailsMsg:
		if msg.request.id != m.requestID {
			return m, nil
		}

		m.loading = false
		m.err = msg.err
		if msg.err != nil {
			m.files = nil
			m.diffLines = []string{msg.err.Error()}
			return m, nil
		}

		m.files = msg.files
		m.fileCursor = min(m.fileCursor, max(len(m.files)-1, 0))
		m.diffLines = splitLines(msg.diff)
		if len(m.diffLines) == 0 {
			if m.activeFile != "" {
				m.diffLines = []string{"No changes for selected file."}
			} else {
				m.diffLines = []string{"No diff available."}
			}
		}
		m.diffScroll = 0
		return m, nil
	case tea.KeyMsg:
		if m.repoVisible {
			return m.updateRepoPicker(msg)
		}

		if m.helpVisible {
			switch msg.String() {
			case "q", "esc":
				m.helpVisible = false
			}
			return m, nil
		}

		if !m.ready {
			return m, nil
		}

		if m.filterMode {
			return m.updateFilterMode(msg)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "?":
			m.helpVisible = true
			return m, nil
		case "r":
			m.repoVisible = true
			m.repoCursor = m.activeRepoIndex
			return m, nil
		case "f":
			if m.viewMode == state.ViewModeReview {
				m.prevFocus = m.focus
				m.viewMode = state.ViewModeFullscreen
				m.focus = state.FocusDiff
			} else {
				m.viewMode = state.ViewModeReview
				if m.prevFocus != "" {
					m.focus = m.prevFocus
				}
			}
			return m, nil
		case "tab":
			if m.viewMode == state.ViewModeReview {
				m.focus = m.nextFocus()
			}
			return m, nil
		case "shift+tab":
			if m.viewMode == state.ViewModeReview {
				m.focus = m.previousFocus()
			}
			return m, nil
		}

		switch m.focus {
		case state.FocusRepos:
			return m.updateRepos(msg)
		case state.FocusCommits:
			return m.updateCommits(msg)
		case state.FocusFiles:
			return m.updateFiles(msg)
		case state.FocusDiff:
			return m.updateDiff(msg)
		}
	}

	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}
	if m.width < minWidth || m.height < minHeight {
		return "error: terminal too small (need at least 80x24)"
	}
	if m.helpVisible {
		return m.renderHelp()
	}
	if m.repoVisible {
		return m.renderRepoPicker()
	}
	if m.viewMode == state.ViewModeFullscreen {
		return m.renderFullscreen()
	}

	leftWidth := max(28, m.width*35/100)
	rightWidth := max(40, m.width-leftWidth-1)

	header := m.renderHeader()
	leftSections := make([]string, 0, 3)
	if m.hasRepoPanel() {
		leftSections = append(leftSections, m.renderReposPanel(leftWidth))
	}
	leftSections = append(leftSections,
		m.renderCommitsPanel(leftWidth),
		m.renderFilesPanel(leftWidth),
	)
	left := lipgloss.JoinVertical(lipgloss.Left, leftSections...)
	right := m.renderDiffPanel(rightWidth, m.diffPanelHeight())
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, m.renderStatus())
}

func (m Model) updateCommits(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "/":
		m.filterMode = true
		m.preFilterSHA = m.activeCommitSHA()
		m.anchor = -1
		return m, nil
	case "j", "down":
		if m.cursor < len(m.commits)-1 {
			m.cursor++
			m.activeFile = ""
			return m.scheduleDebouncedLoad()
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.activeFile = ""
			return m.scheduleDebouncedLoad()
		}
	case "g":
		if len(m.commits) > 0 {
			m.cursor = 0
			m.activeFile = ""
			return m.scheduleDebouncedLoad()
		}
	case "G":
		if len(m.commits) > 0 {
			m.cursor = len(m.commits) - 1
			m.activeFile = ""
			return m.scheduleDebouncedLoad()
		}
	case "pgdown":
		if len(m.commits) > 0 {
			m.cursor = min(len(m.commits)-1, m.cursor+m.visibleRows())
			m.activeFile = ""
			return m.scheduleDebouncedLoad()
		}
	case "pgup":
		if len(m.commits) > 0 {
			m.cursor = max(0, m.cursor-m.visibleRows())
			m.activeFile = ""
			return m.scheduleDebouncedLoad()
		}
	case " ":
		m.anchor = m.cursor
		m.activeFile = ""
		return m.scheduleDebouncedLoad()
	case "esc":
		if m.anchor != -1 {
			m.anchor = -1
			m.activeFile = ""
			return m.scheduleDebouncedLoad()
		}
	case "enter":
		m.focus = state.FocusDiff
		m.diffScroll = 0
		return m, nil
	}

	return m, nil
}

func (m Model) updateRepos(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.repoCursor < len(m.repos)-1 {
			m.repoCursor++
		}
	case "k", "up":
		if m.repoCursor > 0 {
			m.repoCursor--
		}
	case "g":
		m.repoCursor = 0
	case "G":
		if len(m.repos) > 0 {
			m.repoCursor = len(m.repos) - 1
		}
	case "enter", " ":
		m.applyRepo(m.repoCursor)
		return m.scheduleDebouncedLoad()
	}

	return m, nil
}

func (m Model) updateFiles(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.fileCursor < len(m.files)-1 {
			m.fileCursor++
		}
	case "k", "up":
		if m.fileCursor > 0 {
			m.fileCursor--
		}
	case "g":
		m.fileCursor = 0
	case "G":
		if len(m.files) > 0 {
			m.fileCursor = len(m.files) - 1
		}
	case "pgdown":
		if len(m.files) > 0 {
			m.fileCursor = min(len(m.files)-1, m.fileCursor+m.filesVisibleRows())
		}
	case "pgup":
		if len(m.files) > 0 {
			m.fileCursor = max(0, m.fileCursor-m.filesVisibleRows())
		}
	case "enter", " ":
		if len(m.files) > 0 {
			m.activeFile = m.files[m.fileCursor]
			return m.startImmediateLoad()
		}
	case "esc":
		if m.activeFile != "" {
			m.activeFile = ""
			return m.startImmediateLoad()
		}
	}

	return m, nil
}

func (m Model) updateDiff(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.diffScroll < max(0, len(m.diffLines)-1) {
			m.diffScroll++
		}
	case "k", "up":
		if m.diffScroll > 0 {
			m.diffScroll--
		}
	case "g":
		m.diffScroll = 0
	case "G":
		m.diffScroll = max(0, len(m.diffLines)-m.diffViewportHeight())
	case "pgdown":
		m.diffScroll = min(max(0, len(m.diffLines)-m.diffViewportHeight()), m.diffScroll+m.diffViewportHeight())
	case "pgup":
		m.diffScroll = max(0, m.diffScroll-m.diffViewportHeight())
	}

	return m, nil
}

func (m Model) updateFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.filterMode = false
		m.filterQuery = ""
		m.filterActive = false
		m.filterNoMatch = false
		m.commits = slices.Clone(m.allCommits)
		m.cursor = m.findCommitIndexBySHA(m.preFilterSHA)
		m.preFilterSHA = ""
		m.activeFile = ""
		if !m.hasActiveCommits() {
			m.files = nil
			m.diffLines = nil
			return m, nil
		}
		return m.scheduleDebouncedLoad()
	case tea.KeyEnter:
		m.filterMode = false
		m.filterActive = m.filterQuery != ""
		if !m.hasActiveCommits() {
			m.files = nil
			m.diffLines = nil
			return m, nil
		}
		return m.scheduleDebouncedLoad()
	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.filterQuery) > 0 {
			m.filterQuery = string([]rune(m.filterQuery)[:len([]rune(m.filterQuery))-1])
			return m.applyFilterAndLoad()
		}
		return m, nil
	default:
		if msg.Type == tea.KeyRunes {
			m.filterQuery += msg.String()
			return m.applyFilterAndLoad()
		}
	}

	return m, nil
}

func (m Model) applyFilterAndLoad() (tea.Model, tea.Cmd) {
	query := strings.ToLower(strings.TrimSpace(m.filterQuery))
	if query == "" {
		m.commits = slices.Clone(m.allCommits)
		m.cursor = m.findCommitIndexBySHA(m.preFilterSHA)
		m.filterNoMatch = false
		m.activeFile = ""
		return m.scheduleDebouncedLoad()
	}

	filtered := make([]state.Commit, 0, len(m.allCommits))
	for _, commit := range m.allCommits {
		if strings.Contains(strings.ToLower(commit.Subject), query) || strings.Contains(strings.ToLower(commit.ShortSHA), query) {
			filtered = append(filtered, commit)
		}
	}

	m.commits = filtered
	m.cursor = 0
	m.anchor = -1
	m.activeFile = ""
	m.filterNoMatch = len(filtered) == 0
	if len(filtered) == 0 {
		m.files = nil
		m.diffLines = nil
		m.loading = false
		return m, nil
	}

	return m.scheduleDebouncedLoad()
}

func (m Model) renderHeader() string {
	return headerStyle.Render(fmt.Sprintf(
		"repo: %s  branch: %s  base: %s  ahead: %d",
		truncateRight(m.repoLabel(m.activeRepoIndex), 22),
		truncateRight(m.info.CurrentBranch, 18),
		truncateRight(m.info.BaseBranch, 18),
		len(m.allCommits),
	))
}

func (m Model) renderCommitsPanel(width int) string {
	title := "COMMITS"
	if m.filterActive && m.filterQuery != "" {
		title += "  /" + truncateRight(m.filterQuery, max(1, width-14))
	}
	if m.filterMode {
		title += "  /" + truncateRight(m.filterQuery+"_", max(1, width-14))
	}

	if len(m.allCommits) == 0 {
		return panelStyleFor(m.focus == state.FocusCommits).
			Width(width).
			Height(m.commitsPanelHeight()).
			Render(title + "\n\nNo commits ahead of " + m.info.BaseBranch + ".")
	}
	if m.filterNoMatch {
		return panelStyleFor(m.focus == state.FocusCommits).
			Width(width).
			Height(m.commitsPanelHeight()).
			Render(title + "\n\nNo matches.")
	}

	lines := []string{title, ""}
	start := clamp(m.cursor-m.visibleRows()/3, 0, max(0, len(m.commits)-m.visibleRows()))
	end := min(len(m.commits), start+m.visibleRows())
	for idx := start; idx < end; idx++ {
		commit := m.commits[idx]
		prefix := "  "
		if idx == m.cursor {
			prefix = "▶ "
		}
		selected := " "
		if m.isSelected(idx) {
			selected = "·"
		}
		line := prefix + selected + " " + commit.ShortSHA + "  " + commit.Date + "  " + m.authorDot(commit.Author) + " " + commit.Author + "  " + commit.Subject
		lines = append(lines, truncateRight(line, width-4))
	}

	return panelStyleFor(m.focus == state.FocusCommits).
		Width(width).
		Height(m.commitsPanelHeight()).
		Render(strings.Join(lines, "\n"))
}

func (m Model) renderFilesPanel(width int) string {
	lines := []string{"FILES", ""}
	switch {
	case m.filterNoMatch:
		lines = append(lines, "No files.")
	case len(m.files) == 0:
		if m.loading {
			lines = append(lines, "Loading files...")
		} else {
			lines = append(lines, "No files.")
		}
	default:
		rows := m.filesVisibleRows()
		start := clamp(m.fileCursor-rows/3, 0, max(0, len(m.files)-rows))
		end := min(len(m.files), start+rows)
		for idx := start; idx < end; idx++ {
			prefix := "  "
			if idx == m.fileCursor {
				prefix = "▶ "
			}
			line := prefix + truncateLeft(m.files[idx], width-6)
			if m.activeFile == m.files[idx] {
				line += " *"
			}
			lines = append(lines, line)
		}
	}

	return panelStyleFor(m.focus == state.FocusFiles).
		Width(width).
		Height(m.filesPanelHeight()).
		Render(strings.Join(lines, "\n"))
}

func (m Model) renderReposPanel(width int) string {
	lines := []string{"REPOS", ""}
	rows := m.repoPanelVisibleRows()
	start := clamp(m.repoCursor-rows/3, 0, max(0, len(m.repos)-rows))
	end := min(len(m.repos), start+rows)
	for idx := start; idx < end; idx++ {
		prefix := "  "
		if idx == m.repoCursor {
			prefix = "▶ "
		}
		marker := " "
		if idx == m.activeRepoIndex {
			marker = "*"
		}

		label := m.repoLabel(idx)
		ahead := len(m.repos[idx].Commits)
		branch := m.repos[idx].Info.CurrentBranch
		if branch == "" && m.repos[idx].LoadError != "" {
			branch = "unavailable"
		}
		line := fmt.Sprintf("%s%s %s  [%s]  ahead:%d", prefix, marker, label, branch, ahead)
		line = truncateRight(line, width-4)
		if m.repos[idx].LoadError != "" {
			line = reposErrorStyle.Render(line)
		} else if ahead > 0 {
			line = reposAheadStyle.Render(line)
		}
		lines = append(lines, line)
	}

	return panelStyleFor(m.focus == state.FocusRepos).
		Width(width).
		Height(m.repoPanelHeight()).
		Render(strings.Join(lines, "\n"))
}

func (m Model) renderDiffPanel(width, height int) string {
	title := "DIFF"
	if m.activeFile != "" {
		title += "  " + truncateRight(m.activeFile, max(1, width-10))
	}

	lines := []string{title, ""}
	switch {
	case m.filterNoMatch:
		lines = append(lines, "No diff available.")
	case m.loading:
		lines = append(lines, "Loading diff...")
	case m.err != nil:
		lines = append(lines, m.err.Error())
	default:
		start := min(m.diffScroll, max(0, len(m.diffLines)-1))
		end := min(len(m.diffLines), start+m.diffViewportHeight())
		for _, line := range m.diffLines[start:end] {
			lines = append(lines, styleDiffLine(truncateRight(line, width-4)))
		}
		if len(m.diffLines) == 0 {
			lines = append(lines, "No diff available.")
		}
	}

	return panelStyleFor(m.focus == state.FocusDiff).
		Width(width).
		Height(max(5, height)).
		Render(strings.Join(lines, "\n"))
}

func (m Model) renderFullscreen() string {
	header := m.renderHeader()
	body := m.renderDiffPanel(m.width, m.height-2)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, m.renderStatus())
}

func (m Model) renderStatus() string {
	if m.filterMode {
		return statusStyle.Render("/ " + m.filterQuery + "_")
	}
	if m.err != nil {
		return statusStyle.Render(m.err.Error())
	}
	switch m.focus {
	case state.FocusRepos:
		return statusStyle.Render("[repos]    j/k move  enter switch  r overlay  tab next  ? help  q quit")
	case state.FocusCommits:
		return statusStyle.Render("[commits]  j/k move  space select  / filter  r repos  enter diff  tab next  f fullscreen  ? help  q quit")
	case state.FocusFiles:
		return statusStyle.Render("[files]  j/k move  enter filter  esc clear  r repos  tab next  ? help  q quit")
	default:
		return statusStyle.Render("[diff]  j/k scroll  g/G top/bottom  PgUp/PgDn  r repos  tab next  f fullscreen  ? help  q quit")
	}
}

func (m Model) renderHelp() string {
	lines := []string{
		"KEYBOARD HELP",
		"",
		"Global",
		"q            quit",
		"f            toggle fullscreen diff",
		"?            open keyboard help",
		"r            open repo/submodule switcher",
		"",
		"Panel Switching",
		"tab          next panel",
		"shift+tab    previous panel",
		"",
		"Repos",
		"j / k        move",
		"enter/space  switch repo or submodule",
		"",
		"Commits",
		"j / k        move",
		"PgUp/PgDn    page",
		"g / G        top / bottom",
		"space        set selection anchor",
		"esc          clear selection",
		"/            filter commits",
		"enter        focus diff",
		"",
		"Files",
		"enter/space  filter diff to file",
		"esc          clear file filter",
		"",
		"Diff",
		"j / k        scroll",
		"PgUp/PgDn    page scroll",
		"g / G        top / bottom",
		"",
		"Close help with q or esc",
	}

	return helpStyle.Width(m.width).Height(m.height).Render(strings.Join(lines, "\n"))
}

func (m Model) renderRepoPicker() string {
	lines := []string{
		"REPOSITORIES",
		"",
		"Select root repo or submodule",
		"",
	}

	for idx, snapshot := range m.repos {
		prefix := "  "
		if idx == m.repoCursor {
			prefix = "▶ "
		}
		current := ""
		if idx == m.activeRepoIndex {
			current = " *"
		}
		lines = append(lines, truncateRight(prefix+m.repoLabel(idx)+"  ["+snapshot.Info.CurrentBranch+"]  ahead:"+fmt.Sprintf("%d", len(snapshot.Commits))+current, max(20, m.width-8)))
	}

	lines = append(lines, "", "enter switch  esc close")
	return helpStyle.Width(m.width).Height(m.height).Render(strings.Join(lines, "\n"))
}

func (m Model) scheduleDebouncedLoad() (tea.Model, tea.Cmd) {
	request := m.nextRequest()
	if !m.hasActiveCommits() {
		return m, nil
	}
	return m, tea.Tick(debounceDelay, func(time.Time) tea.Msg {
		return debounceMsg{request: request}
	})
}

func (m Model) startImmediateLoad() (tea.Model, tea.Cmd) {
	if !m.hasActiveCommits() {
		m.files = nil
		m.diffLines = nil
		m.loading = false
		return m, nil
	}
	request := m.nextRequest()
	m.loading = true
	m.err = nil
	return m, m.loadRequest(request)
}

func (m *Model) nextRequest() loadRequest {
	m.requestID++
	selected := m.selectedRange()
	req := loadRequest{
		id:         m.requestID,
		activeFile: m.activeFile,
	}
	if selected.active {
		req.rangeSHAs = make([]string, 0, selected.end-selected.start+1)
		for _, commit := range m.commits[selected.start : selected.end+1] {
			req.rangeSHAs = append(req.rangeSHAs, commit.SHA)
		}
	} else if m.hasActiveCommits() {
		req.singleSHA = m.commits[m.cursor].SHA
	}
	return req
}

func (m Model) loadRequest(request loadRequest) tea.Cmd {
	root := m.root
	gitClient := m.git

	return func() tea.Msg {
		var (
			files []string
			diff  string
			err   error
		)

		if request.singleSHA != "" {
			files, err = gitClient.LoadFilesForCommit(root, request.singleSHA)
			if err == nil {
				diff, err = gitClient.LoadDiffForCommit(root, request.singleSHA, request.activeFile)
			}
		} else {
			oldest, newest, normalizeErr := gitClient.NormalizeRange(root, request.rangeSHAs)
			if normalizeErr != nil {
				err = normalizeErr
			} else {
				files, err = gitClient.LoadFilesForRange(root, oldest, newest)
			}
			if err == nil {
				diff, err = gitClient.LoadDiffForRange(root, oldest, newest, request.activeFile)
			}
		}

		return commitDetailsMsg{
			request: request,
			files:   files,
			diff:    diff,
			err:     err,
		}
	}
}

func (m Model) hasActiveCommits() bool {
	return len(m.commits) > 0
}

func (m Model) hasRepoPanel() bool {
	return len(m.repos) > 1
}

func (m Model) updateRepoPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.repoVisible = false
		return m, nil
	case "j", "down":
		if m.repoCursor < len(m.repos)-1 {
			m.repoCursor++
		}
	case "k", "up":
		if m.repoCursor > 0 {
			m.repoCursor--
		}
	case "g":
		m.repoCursor = 0
	case "G":
		if len(m.repos) > 0 {
			m.repoCursor = len(m.repos) - 1
		}
	case "enter":
		m.repoVisible = false
		m.applyRepo(m.repoCursor)
		return m.scheduleDebouncedLoad()
	}

	return m, nil
}

func (m *Model) applyRepo(index int) {
	if len(m.repos) == 0 {
		return
	}
	if index < 0 || index >= len(m.repos) {
		index = 0
	}

	snapshot := m.repos[index]
	m.activeRepoIndex = index
	m.repoCursor = index
	m.info = snapshot.Info
	m.root = snapshot.Info.Root
	m.allCommits = slices.Clone(snapshot.Commits)
	m.commits = slices.Clone(snapshot.Commits)
	m.cursor = 0
	m.anchor = -1
	m.filterMode = false
	m.filterQuery = ""
	m.preFilterSHA = ""
	m.filterActive = false
	m.filterNoMatch = false
	m.files = nil
	m.fileCursor = 0
	m.activeFile = ""
	m.diffLines = nil
	m.diffScroll = 0
	m.loading = false
	m.err = nil
	m.authorDots = buildAuthorDots(snapshot.Commits)
	if snapshot.LoadError != "" {
		m.err = errors.New(snapshot.LoadError)
	} else {
		m.err = nil
	}
	if m.hasRepoPanel() {
		m.focus = state.FocusRepos
	} else {
		m.focus = state.FocusCommits
	}
}

func (m Model) repoLabel(index int) string {
	if index < 0 || index >= len(m.repos) {
		return ""
	}
	root := m.repos[0].Info.Root
	target := m.repos[index].Info.Root
	if index == 0 || target == root {
		return lastPath(root)
	}
	if rel, err := filepath.Rel(root, target); err == nil {
		return rel
	}
	return lastPath(target)
}

func (m Model) activeCommitSHA() string {
	if !m.hasActiveCommits() {
		return ""
	}
	selected := m.selectedRange()
	if selected.active {
		return m.commits[selected.start].SHA
	}
	return m.commits[m.cursor].SHA
}

func (m Model) visibleRows() int {
	return max(5, m.commitsPanelHeight()-2)
}

func (m Model) diffViewportHeight() int {
	return max(5, m.diffPanelHeight()-2)
}

func (m Model) repoPanelHeight() int {
	budget := m.leftContentBudget()
	base := min(max(3, len(m.repos)), max(3, budget/4))
	return min(base, max(3, budget-9))
}

func (m Model) repoPanelVisibleRows() int {
	return max(1, m.repoPanelHeight()-2)
}

func (m Model) commitsPanelHeight() int {
	totalLeft := m.leftContentBudget()
	if !m.hasRepoPanel() {
		return max(5, totalLeft/2)
	}
	remaining := totalLeft - m.repoPanelHeight()
	return max(5, remaining/2)
}

func (m Model) filesPanelHeight() int {
	totalLeft := m.leftContentBudget()
	if !m.hasRepoPanel() {
		return max(4, totalLeft-m.commitsPanelHeight())
	}
	remaining := totalLeft - m.repoPanelHeight()
	return max(4, remaining-m.commitsPanelHeight())
}

func (m Model) filesVisibleRows() int {
	return max(1, m.filesPanelHeight()-2)
}

func (m Model) leftContentBudget() int {
	panelCount := 2
	if m.hasRepoPanel() {
		panelCount = 3
	}

	// Body height excludes the one-line header and one-line status bar.
	// Each bordered panel consumes two extra rows for its border.
	budget := m.bodyOuterHeight() - (panelCount * 2)
	return max(12, budget)
}

func (m Model) diffPanelHeight() int {
	return max(5, m.bodyOuterHeight()-2)
}

func (m Model) bodyOuterHeight() int {
	return max(12, m.height-2)
}

type selectionRange struct {
	active bool
	start  int
	end    int
}

func (m Model) selectedRange() selectionRange {
	if m.anchor == -1 || !m.hasActiveCommits() {
		return selectionRange{}
	}
	start := min(m.anchor, m.cursor)
	end := max(m.anchor, m.cursor)
	return selectionRange{active: true, start: start, end: end}
}

func (m Model) isSelected(index int) bool {
	selected := m.selectedRange()
	return selected.active && index >= selected.start && index <= selected.end
}

func (m Model) findCommitIndexBySHA(sha string) int {
	if sha == "" {
		return 0
	}
	for i, commit := range m.commits {
		if commit.SHA == sha {
			return i
		}
	}
	return 0
}

func (m Model) authorDot(author string) string {
	style, ok := m.authorDots[author]
	if !ok {
		return "•"
	}
	return style.Render("●")
}

func splitLines(raw string) []string {
	if raw == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(raw, "\n"), "\n")
}

func styleDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"), strings.HasPrefix(line, "diff --git"):
		return fileHeaderStyle.Render(line)
	case strings.HasPrefix(line, "@@"):
		return hunkStyle.Render(line)
	case strings.HasPrefix(line, "+"):
		return addedStyle.Render(line)
	case strings.HasPrefix(line, "-"):
		return removedStyle.Render(line)
	default:
		return line
	}
}

func (m Model) nextFocus() state.Focus {
	order := m.focusOrder()
	idx := slices.Index(order, m.focus)
	if idx == -1 {
		return order[0]
	}
	return order[(idx+1)%len(order)]
}

func (m Model) previousFocus() state.Focus {
	order := m.focusOrder()
	idx := slices.Index(order, m.focus)
	if idx <= 0 {
		return order[len(order)-1]
	}
	return order[idx-1]
}

func (m Model) focusOrder() []state.Focus {
	order := []state.Focus{state.FocusCommits, state.FocusFiles, state.FocusDiff}
	if m.hasRepoPanel() {
		return append([]state.Focus{state.FocusRepos}, order...)
	}
	return order
}

func buildAuthorDots(commits []state.Commit) map[string]lipgloss.Style {
	palette := []lipgloss.Color{"37", "179", "71", "134", "33", "167"}
	styles := make(map[string]lipgloss.Style, len(commits))
	next := 0
	for _, commit := range commits {
		if _, ok := styles[commit.Author]; ok {
			continue
		}
		styles[commit.Author] = lipgloss.NewStyle().Foreground(palette[next%len(palette)])
		next++
	}
	return styles
}

func lastPath(path string) string {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	return parts[len(parts)-1]
}

func truncateRight(value string, width int) string {
	if width <= 1 || lipgloss.Width(value) <= width {
		return value
	}
	runes := []rune(value)
	if width >= len(runes) {
		return value
	}
	return string(runes[:width-1]) + "…"
}

func truncateLeft(value string, width int) string {
	if width <= 1 || lipgloss.Width(value) <= width {
		return value
	}
	runes := []rune(value)
	if width >= len(runes) {
		return value
	}
	return "…" + string(runes[len(runes)-width+1:])
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1)
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1)
	panelBaseStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)
	fileHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("69"))
	hunkStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("44"))
	addedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))
	removedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
	reposAheadStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))
	reposErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
	helpStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("39")).
			Padding(1, 2)
)

func panelStyleFor(active bool) lipgloss.Style {
	style := panelBaseStyle
	if active {
		return style.BorderForeground(lipgloss.Color("39"))
	}
	return style.BorderForeground(lipgloss.Color("240"))
}
