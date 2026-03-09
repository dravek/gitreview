package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	ggit "gitreview/internal/git"
	"gitreview/internal/repo"
	"gitreview/internal/state"
	"gitreview/internal/version"
)

func TestWindowResizeIgnoredAfterInitialSize(t *testing.T) {
	model := NewModel(nil, ggit.Client{})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	got := updated.(Model)
	if got.width != 100 || got.height != 40 || !got.ready {
		t.Fatalf("initial size not applied: %+v", got)
	}

	updated, _ = got.Update(tea.WindowSizeMsg{Width: 10, Height: 5})
	got = updated.(Model)
	if got.width != 100 || got.height != 40 {
		t.Fatalf("resize should be ignored after ready, got width=%d height=%d", got.width, got.height)
	}
}

func TestDebouncedLoadAdvancesRequestID(t *testing.T) {
	commits := []state.Commit{
		{SHA: "1", ShortSHA: "1111111", Subject: "alpha"},
	}
	model := NewModel([]repo.Snapshot{{Info: repo.Info{Root: "/tmp/root"}, Commits: commits}}, ggit.Client{})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	got := updated.(Model)

	updatedModel, cmd := got.scheduleDebouncedLoad()
	got = updatedModel.(Model)
	if cmd == nil {
		t.Fatal("expected debounce command")
	}
	if got.requestID == 0 {
		t.Fatal("expected requestID to increment for debounced load")
	}
}

func TestFilterModeEscRestoresPreFilterCursor(t *testing.T) {
	commits := []state.Commit{
		{SHA: "1", ShortSHA: "1111111", Subject: "alpha"},
		{SHA: "2", ShortSHA: "2222222", Subject: "beta"},
		{SHA: "3", ShortSHA: "3333333", Subject: "gamma"},
	}
	model := NewModel([]repo.Snapshot{{Info: repo.Info{Root: "/tmp/root"}, Commits: commits}}, ggit.Client{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	got := updated.(Model)
	got.cursor = 1
	got.anchor = 1

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	got = updated.(Model)
	if !got.filterMode {
		t.Fatal("expected filter mode to be enabled")
	}
	if got.anchor != -1 {
		t.Fatalf("expected selection to clear on filter start, got anchor=%d", got.anchor)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	got = updated.(Model)
	if got.filterNoMatch != false || len(got.commits) != 1 || got.commits[0].SHA != "3" {
		t.Fatalf("unexpected filtered state: noMatch=%v commits=%v", got.filterNoMatch, got.commits)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = updated.(Model)
	if got.filterMode {
		t.Fatal("expected filter mode to exit on esc")
	}
	if got.cursor != 1 {
		t.Fatalf("expected cursor to restore to pre-filter commit, got %d", got.cursor)
	}
}

func TestFilterModeNoMatchesClearsPanelsState(t *testing.T) {
	commits := []state.Commit{
		{SHA: "1", ShortSHA: "1111111", Subject: "alpha"},
	}
	model := NewModel([]repo.Snapshot{{Info: repo.Info{Root: "/tmp/root"}, Commits: commits}}, ggit.Client{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	got := updated.(Model)
	got.files = []string{"x.go"}
	got.diffLines = []string{"diff"}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	got = updated.(Model)
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	got = updated.(Model)

	if !got.filterNoMatch {
		t.Fatal("expected no-match state")
	}
	if len(got.files) != 0 || len(got.diffLines) != 0 {
		t.Fatalf("expected panels to clear on no-match, got files=%v diff=%v", got.files, got.diffLines)
	}
}

func TestRepoPickerSwitchesActiveRepo(t *testing.T) {
	repos := []repo.Snapshot{
		{Info: repo.Info{Root: "/tmp/root", CurrentBranch: "main", BaseBranch: "main"}, Commits: []state.Commit{{SHA: "1", ShortSHA: "1", Subject: "root"}}},
		{Info: repo.Info{Root: "/tmp/root/sub", CurrentBranch: "WP-1234", BaseBranch: "main"}, Commits: []state.Commit{{SHA: "2", ShortSHA: "2", Subject: "sub"}}},
	}

	model := NewModel(repos, ggit.Client{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	got := updated.(Model)

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	got = updated.(Model)
	if !got.repoVisible {
		t.Fatal("expected repo picker to open")
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got = updated.(Model)
	if got.repoCursor != 1 {
		t.Fatalf("repoCursor = %d, want 1", got.repoCursor)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(Model)
	if got.activeRepoIndex != 1 {
		t.Fatalf("activeRepoIndex = %d, want 1", got.activeRepoIndex)
	}
	if got.info.CurrentBranch != "WP-1234" {
		t.Fatalf("CurrentBranch = %q, want %q", got.info.CurrentBranch, "WP-1234")
	}
}

func TestRepoPanelSwitchesActiveRepo(t *testing.T) {
	repos := []repo.Snapshot{
		{Info: repo.Info{Root: "/tmp/root", CurrentBranch: "main", BaseBranch: "main"}, Commits: []state.Commit{{SHA: "1", ShortSHA: "1", Subject: "root"}}},
		{Info: repo.Info{Root: "/tmp/root/sub", CurrentBranch: "WP-1234", BaseBranch: "main"}, Commits: []state.Commit{{SHA: "2", ShortSHA: "2", Subject: "sub"}}},
	}

	model := NewModel(repos, ggit.Client{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	got := updated.(Model)
	got.focus = state.FocusRepos

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyDown})
	got = updated.(Model)
	if got.repoCursor != 1 {
		t.Fatalf("repoCursor = %d, want 1", got.repoCursor)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(Model)
	if got.activeRepoIndex != 1 {
		t.Fatalf("activeRepoIndex = %d, want 1", got.activeRepoIndex)
	}
}

func TestRenderStatusIncludesVersionOnRight(t *testing.T) {
	model := NewModel([]repo.Snapshot{{Info: repo.Info{Root: "/tmp/root"}, Commits: []state.Commit{{SHA: "1", ShortSHA: "1", Subject: "root"}}}}, ggit.Client{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	got := updated.(Model)

	status := got.renderStatus()
	lines := strings.Split(strings.TrimRight(status, "\n"), "\n")
	lastLine := lines[len(lines)-1]
	if !strings.HasSuffix(lastLine, "v"+version.String()+" ") {
		t.Fatalf("status line = %q, want suffix %q", lastLine, "v"+version.String()+" ")
	}
}

func TestLeftPanelHeightsFitBudgetWithRepos(t *testing.T) {
	repos := []repo.Snapshot{
		{Info: repo.Info{Root: "/tmp/root", CurrentBranch: "main", BaseBranch: "main"}},
		{Info: repo.Info{Root: "/tmp/root/sub", CurrentBranch: "WP-1234", BaseBranch: "main"}},
	}

	model := NewModel(repos, ggit.Client{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	got := updated.(Model)

	totalContent := got.repoPanelHeight() + got.commitsPanelHeight() + got.filesPanelHeight()
	if totalContent > got.leftContentBudget() {
		t.Fatalf("left panel heights overflow budget: content=%d budget=%d", totalContent, got.leftContentBudget())
	}
	if got.repoPanelVisibleRows()+2 > got.repoPanelHeight() {
		t.Fatalf("repo panel rows overflow height: rows=%d height=%d", got.repoPanelVisibleRows()+2, got.repoPanelHeight())
	}
	if got.filesVisibleRows()+2 > got.filesPanelHeight() {
		t.Fatalf("files panel rows overflow height: rows=%d height=%d", got.filesVisibleRows()+2, got.filesPanelHeight())
	}
	if got.diffPanelHeight()+2 != got.bodyOuterHeight() {
		t.Fatalf("diff panel outer height mismatch: diff=%d body=%d", got.diffPanelHeight()+2, got.bodyOuterHeight())
	}
}

func TestFilePagingUsesFilePanelRows(t *testing.T) {
	repos := []repo.Snapshot{
		{
			Info:    repo.Info{Root: "/tmp/root", CurrentBranch: "main", BaseBranch: "main"},
			Commits: []state.Commit{{SHA: "1", ShortSHA: "1", Subject: "root"}},
		},
	}

	model := NewModel(repos, ggit.Client{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	got := updated.(Model)
	got.focus = state.FocusFiles
	got.files = []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}

	rows := got.filesVisibleRows()
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	got = updated.(Model)
	if got.fileCursor != rows {
		t.Fatalf("fileCursor = %d, want %d", got.fileCursor, rows)
	}
}

func TestToggleHistoryScopeResetsReviewState(t *testing.T) {
	commits := []state.Commit{
		{SHA: "1", ShortSHA: "1111111", Subject: "alpha"},
		{SHA: "2", ShortSHA: "2222222", Subject: "beta"},
	}
	model := NewModel([]repo.Snapshot{{Info: repo.Info{Root: "/tmp/root", BaseBranch: "main"}, Commits: commits}}, ggit.Client{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	got := updated.(Model)
	got.cursor = 1
	got.anchor = 0
	got.filterMode = false
	got.filterQuery = "beta"
	got.filterActive = true
	got.activeFile = "one.txt"
	got.files = []string{"one.txt"}
	got.diffLines = []string{"diff"}

	updated, cmd := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	got = updated.(Model)
	if cmd == nil {
		t.Fatal("expected history load command")
	}
	if got.scope != commitScopeHistory {
		t.Fatalf("scope = %q, want %q", got.scope, commitScopeHistory)
	}
	if got.cursor != 0 || got.anchor != -1 {
		t.Fatalf("expected cursor reset and anchor cleared, got cursor=%d anchor=%d", got.cursor, got.anchor)
	}
	if got.filterMode || got.filterQuery != "" || got.filterActive || got.filterNoMatch {
		t.Fatalf("expected filter state reset, got mode=%v query=%q active=%v noMatch=%v", got.filterMode, got.filterQuery, got.filterActive, got.filterNoMatch)
	}
	if got.activeFile != "" || len(got.files) != 0 || len(got.diffLines) != 0 {
		t.Fatalf("expected panels reset, got activeFile=%q files=%v diff=%v", got.activeFile, got.files, got.diffLines)
	}
	if !got.loading {
		t.Fatal("expected history mode to start loading")
	}
	if got.historyLimit != defaultHistoryLimit {
		t.Fatalf("historyLimit = %d, want %d", got.historyLimit, defaultHistoryLimit)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	got = updated.(Model)
	if got.scope != commitScopeAhead {
		t.Fatalf("scope = %q, want %q", got.scope, commitScopeAhead)
	}
	if len(got.commits) != len(commits) {
		t.Fatalf("commits len = %d, want %d", len(got.commits), len(commits))
	}
}

func TestLoadMoreHistoryIncreasesLimitOnlyInHistoryMode(t *testing.T) {
	commits := []state.Commit{{SHA: "1", ShortSHA: "1111111", Subject: "alpha"}}
	model := NewModel([]repo.Snapshot{{Info: repo.Info{Root: "/tmp/root", BaseBranch: "main"}, Commits: commits}}, ggit.Client{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	got := updated.(Model)

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	got = updated.(Model)
	if got.historyLimit != defaultHistoryLimit {
		t.Fatalf("historyLimit changed in ahead mode: %d", got.historyLimit)
	}

	got.scope = commitScopeHistory
	got.listRequestID = 1
	updated, cmd := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	got = updated.(Model)
	if cmd == nil {
		t.Fatal("expected load-more command in history mode")
	}
	if got.historyLimit != defaultHistoryLimit+historyIncrement {
		t.Fatalf("historyLimit = %d, want %d", got.historyLimit, defaultHistoryLimit+historyIncrement)
	}
}

func TestTruncateRightPreservesStyledContentWidth(t *testing.T) {
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("●")
	input := "prefix " + styled + " long subject"
	got := truncateRight(input, 18)
	if lipgloss.Width(got) > 18 {
		t.Fatalf("truncateRight width = %d, want <= 18", lipgloss.Width(got))
	}
	if !strings.Contains(got, "long") && !strings.Contains(got, "subject") {
		t.Fatalf("truncateRight truncated too aggressively: %q", got)
	}
}

func TestHistoryReloadPreservesActiveCommitBySHA(t *testing.T) {
	model := NewModel([]repo.Snapshot{{Info: repo.Info{Root: "/tmp/root", BaseBranch: "main"}, Commits: []state.Commit{{SHA: "a1", ShortSHA: "a1", Subject: "ahead"}}}}, ggit.Client{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	got := updated.(Model)
	got.scope = commitScopeHistory
	got.listRequestID = 1
	got.setCommitList([]state.Commit{
		{SHA: "c3", ShortSHA: "c3", Subject: "third"},
		{SHA: "b2", ShortSHA: "b2", Subject: "second"},
		{SHA: "a1", ShortSHA: "a1", Subject: "first"},
	}, "")
	got.cursor = 1

	updated, _ = got.Update(commitListMsg{
		request: commitListRequest{id: 1, scope: commitScopeHistory, limit: defaultHistoryLimit + historyIncrement},
		commits: []state.Commit{
			{SHA: "d4", ShortSHA: "d4", Subject: "fourth"},
			{SHA: "c3", ShortSHA: "c3", Subject: "third"},
			{SHA: "b2", ShortSHA: "b2", Subject: "second"},
			{SHA: "a1", ShortSHA: "a1", Subject: "first"},
		},
	})
	got = updated.(Model)
	if got.cursor != 2 {
		t.Fatalf("cursor = %d, want 2 to preserve SHA b2", got.cursor)
	}
	if got.commits[got.cursor].SHA != "b2" {
		t.Fatalf("active SHA = %q, want b2", got.commits[got.cursor].SHA)
	}
}

func TestToggleHistoryInvalidatesStaleDiffRequests(t *testing.T) {
	commits := []state.Commit{{SHA: "1", ShortSHA: "1111111", Subject: "alpha"}}
	model := NewModel([]repo.Snapshot{{Info: repo.Info{Root: "/tmp/root", BaseBranch: "main"}, Commits: commits}}, ggit.Client{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	got := updated.(Model)
	got.requestID = 7

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	got = updated.(Model)
	if got.requestID != 8 {
		t.Fatalf("requestID = %d, want 8", got.requestID)
	}

	updated, _ = got.Update(commitDetailsMsg{
		request: loadRequest{id: 7, singleSHA: "1"},
		files:   []string{"stale.txt"},
		diff:    "stale diff",
	})
	got = updated.(Model)
	if len(got.files) != 0 || len(got.diffLines) != 0 {
		t.Fatalf("stale diff request should be ignored, got files=%v diff=%v", got.files, got.diffLines)
	}
}

func TestRepoPickerSpaceSwitchesRepo(t *testing.T) {
	repos := []repo.Snapshot{
		{Info: repo.Info{Root: "/tmp/root", CurrentBranch: "main", BaseBranch: "main"}, Commits: []state.Commit{{SHA: "1", ShortSHA: "1", Subject: "root"}}},
		{Info: repo.Info{Root: "/tmp/root/sub", CurrentBranch: "WP-1234", BaseBranch: "main"}, Commits: []state.Commit{{SHA: "2", ShortSHA: "2", Subject: "sub"}}},
	}

	model := NewModel(repos, ggit.Client{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	got := updated.(Model)
	got.repoVisible = true
	got.repoCursor = 1

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	got = updated.(Model)
	if got.activeRepoIndex != 1 {
		t.Fatalf("activeRepoIndex = %d, want 1", got.activeRepoIndex)
	}
	if got.repoVisible {
		t.Fatal("expected repo picker to close after switching with space")
	}
}
