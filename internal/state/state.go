package state

type Focus string

const (
	FocusRepos   Focus = "repos"
	FocusCommits Focus = "commits"
	FocusFiles   Focus = "files"
	FocusDiff    Focus = "diff"
)

type ViewMode string

const (
	ViewModeReview     ViewMode = "review"
	ViewModeFullscreen ViewMode = "fullscreen"
)

type Commit struct {
	SHA      string
	ShortSHA string
	Date     string
	Author   string
	Subject  string
}

type AppState struct {
	ViewMode   ViewMode
	Focus      Focus
	RepoRoot   string
	CurrentRef string
	BaseBranch string
	Commits    []Commit
}

func NewAppState(repoRoot, currentRef, baseBranch string, commits []Commit) AppState {
	return AppState{
		ViewMode:   ViewModeReview,
		Focus:      FocusCommits,
		RepoRoot:   repoRoot,
		CurrentRef: currentRef,
		BaseBranch: baseBranch,
		Commits:    commits,
	}
}
