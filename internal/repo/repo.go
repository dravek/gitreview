package repo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	ggit "gitreview/internal/git"
	"gitreview/internal/state"
)

var (
	ErrPathNotDirectory = errors.New("error: path does not exist or is not a directory")
	ErrNotRepository    = errors.New("error: not a git repository")
	ErrBaseNotDetected  = errors.New("error: could not detect base branch\nhint: run with --base <branch> to specify one")
)

type Info struct {
	Root          string
	CurrentBranch string
	BaseBranch    string
	BaseRef       string
}

type Snapshot struct {
	Info      Info
	Commits   []state.Commit
	LoadError string
}

type Workspace struct {
	Repos []Snapshot
}

func LoadWorkspace(path, baseOverride string) (Workspace, error) {
	primary, err := Load(path, baseOverride)
	if err != nil {
		return Workspace{}, err
	}

	client, err := ggit.NewClient()
	if err != nil {
		return Workspace{}, err
	}

	paths, err := client.SubmodulePaths(primary.Info.Root)
	if err != nil {
		return Workspace{}, err
	}

	repos := []Snapshot{primary}
	for _, submodulePath := range paths {
		joined := filepath.Join(primary.Info.Root, submodulePath)
		snapshot, err := Load(joined, baseOverride)
		if err != nil {
			repos = append(repos, Snapshot{
				Info: Info{
					Root: joined,
				},
				LoadError: err.Error(),
			})
			continue
		}
		repos = append(repos, snapshot)
	}

	return Workspace{Repos: repos}, nil
}

func Load(path, baseOverride string) (Snapshot, error) {
	resolved, err := resolvePath(path)
	if err != nil {
		return Snapshot{}, err
	}

	client, err := ggit.NewClient()
	if err != nil {
		return Snapshot{}, err
	}

	root, err := client.RepoRoot(resolved)
	if err != nil {
		return Snapshot{}, ErrNotRepository
	}

	currentBranch, err := client.CurrentBranch(root)
	if err != nil {
		return Snapshot{}, err
	}

	baseBranch, baseRef, err := detectBaseBranch(client, root, baseOverride)
	if err != nil {
		return Snapshot{}, err
	}

	commits, err := client.LoadCommits(root, baseRef)
	if err != nil {
		return Snapshot{}, err
	}

	return Snapshot{
		Info: Info{
			Root:          root,
			CurrentBranch: currentBranch,
			BaseBranch:    baseBranch,
			BaseRef:       baseRef,
		},
		Commits: commits,
	}, nil
}

func resolvePath(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", ErrPathNotDirectory
	}

	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	return resolved, nil
}

func detectBaseBranch(client ggit.Client, root, override string) (string, string, error) {
	if override != "" {
		return override, override, nil
	}

	if ref, err := client.SymbolicOriginHEAD(root); err == nil {
		return shortRemoteRef(ref), remoteRefToRevision(ref), nil
	}

	candidates := []struct {
		display string
		ref     string
	}{
		{display: "main", ref: "refs/heads/main"},
		{display: "master", ref: "refs/heads/master"},
		{display: "main", ref: "refs/remotes/origin/main"},
		{display: "master", ref: "refs/remotes/origin/master"},
	}

	for _, candidate := range candidates {
		if client.RefExists(root, candidate.ref) {
			return candidate.display, revisionFromQualifiedRef(candidate.ref), nil
		}
	}

	return "", "", ErrBaseNotDetected
}

func shortRemoteRef(ref string) string {
	parts := strings.Split(strings.TrimSpace(ref), "/")
	return parts[len(parts)-1]
}

func remoteRefToRevision(ref string) string {
	trimmed := strings.TrimSpace(ref)
	if strings.HasPrefix(trimmed, "refs/remotes/") {
		return strings.TrimPrefix(trimmed, "refs/remotes/")
	}

	return trimmed
}

func revisionFromQualifiedRef(ref string) string {
	if strings.HasPrefix(ref, "refs/heads/") {
		return strings.TrimPrefix(ref, "refs/heads/")
	}

	if strings.HasPrefix(ref, "refs/remotes/") {
		return strings.TrimPrefix(ref, "refs/remotes/")
	}

	return ref
}
