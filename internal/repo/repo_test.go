package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLoadDetectsLocalMainBaseAndCommitList(t *testing.T) {
	root := initRepo(t, "main")
	git(t, root, "config", "user.name", "Test User")
	git(t, root, "config", "user.email", "test@example.com")
	git(t, root, "config", "commit.gpgsign", "false")
	writeFile(t, filepath.Join(root, "README.md"), "root\n")
	git(t, root, "add", "README.md")
	git(t, root, "commit", "-m", "initial commit")
	git(t, root, "checkout", "-b", "feature/demo")
	writeFile(t, filepath.Join(root, "feature.txt"), "hello\n")
	git(t, root, "add", "feature.txt")
	git(t, root, "commit", "-m", "add feature")

	got, err := Load(root, "")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.Info.CurrentBranch != "feature/demo" {
		t.Fatalf("CurrentBranch = %q, want %q", got.Info.CurrentBranch, "feature/demo")
	}

	if got.Info.BaseBranch != "main" {
		t.Fatalf("BaseBranch = %q, want %q", got.Info.BaseBranch, "main")
	}

	if got.Info.BaseRef != "main" {
		t.Fatalf("BaseRef = %q, want %q", got.Info.BaseRef, "main")
	}

	if len(got.Commits) != 1 {
		t.Fatalf("len(Commits) = %d, want 1", len(got.Commits))
	}

	if got.Commits[0].Subject != "add feature" {
		t.Fatalf("Subject = %q, want %q", got.Commits[0].Subject, "add feature")
	}
}

func TestLoadFallsBackToRemoteTrackingMain(t *testing.T) {
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	run(t, base, "git", "init", "--bare", origin)

	seed := filepath.Join(base, "seed")
	run(t, base, "git", "clone", origin, seed)
	git(t, seed, "config", "user.name", "Test User")
	git(t, seed, "config", "user.email", "test@example.com")
	git(t, seed, "config", "commit.gpgsign", "false")
	git(t, seed, "checkout", "-b", "main")
	writeFile(t, filepath.Join(seed, "README.md"), "root\n")
	git(t, seed, "add", "README.md")
	git(t, seed, "commit", "-m", "initial commit")
	git(t, seed, "push", "-u", "origin", "main")

	work := filepath.Join(base, "work")
	run(t, base, "git", "clone", origin, work)
	git(t, work, "config", "user.name", "Test User")
	git(t, work, "config", "user.email", "test@example.com")
	git(t, work, "config", "commit.gpgsign", "false")
	git(t, work, "checkout", "-b", "feature/demo", "origin/main")
	writeFile(t, filepath.Join(work, "feature.txt"), "hello\n")
	git(t, work, "add", "feature.txt")
	git(t, work, "commit", "-m", "add feature")

	got, err := Load(work, "")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.Info.BaseBranch != "main" {
		t.Fatalf("BaseBranch = %q, want %q", got.Info.BaseBranch, "main")
	}

	if got.Info.BaseRef != "origin/main" {
		t.Fatalf("BaseRef = %q, want %q", got.Info.BaseRef, "origin/main")
	}
}

func TestLoadRejectsNonDirectoryPath(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	writeFile(t, file, "x")

	_, err := Load(file, "")
	if err != ErrPathNotDirectory {
		t.Fatalf("Load() error = %v, want %v", err, ErrPathNotDirectory)
	}
}

func TestLoadWorkspaceIncludesInitializedSubmodules(t *testing.T) {
	base := t.TempDir()
	subOrigin := filepath.Join(base, "sub-origin.git")
	run(t, base, "git", "init", "--bare", subOrigin)

	subSeed := filepath.Join(base, "sub-seed")
	run(t, base, "git", "clone", subOrigin, subSeed)
	git(t, subSeed, "config", "user.name", "Test User")
	git(t, subSeed, "config", "user.email", "test@example.com")
	git(t, subSeed, "config", "commit.gpgsign", "false")
	git(t, subSeed, "checkout", "-b", "main")
	writeFile(t, filepath.Join(subSeed, "sub.txt"), "root\n")
	git(t, subSeed, "add", "sub.txt")
	git(t, subSeed, "commit", "-m", "sub root")
	git(t, subSeed, "push", "-u", "origin", "main")

	root := initRepo(t, "main")
	git(t, root, "config", "user.name", "Test User")
	git(t, root, "config", "user.email", "test@example.com")
	git(t, root, "config", "commit.gpgsign", "false")
	writeFile(t, filepath.Join(root, "README.md"), "root\n")
	git(t, root, "add", "README.md")
	git(t, root, "commit", "-m", "root")
	run(t, root, "git", "-c", "protocol.file.allow=always", "submodule", "add", "-b", "main", subOrigin, "modules/sub")
	git(t, root, "commit", "-m", "add submodule")

	git(t, root, "-C", filepath.Join(root, "modules/sub"), "config", "user.name", "Test User")
	git(t, root, "-C", filepath.Join(root, "modules/sub"), "config", "user.email", "test@example.com")
	git(t, root, "-C", filepath.Join(root, "modules/sub"), "config", "commit.gpgsign", "false")
	git(t, root, "-C", filepath.Join(root, "modules/sub"), "checkout", "-b", "WP-1234", "origin/main")
	writeFile(t, filepath.Join(root, "modules/sub", "ticket.txt"), "ticket\n")
	git(t, filepath.Join(root, "modules/sub"), "add", "ticket.txt")
	git(t, filepath.Join(root, "modules/sub"), "commit", "-m", "ticket work")

	workspace, err := LoadWorkspace(root, "")
	if err != nil {
		t.Fatalf("LoadWorkspace() error = %v", err)
	}
	if len(workspace.Repos) != 2 {
		t.Fatalf("len(Repos) = %d, want 2", len(workspace.Repos))
	}
	if workspace.Repos[1].Info.CurrentBranch != "WP-1234" {
		t.Fatalf("submodule branch = %q, want %q", workspace.Repos[1].Info.CurrentBranch, "WP-1234")
	}
}

func TestLoadWorkspaceKeepsBrokenSubmoduleVisible(t *testing.T) {
	base := t.TempDir()
	subOrigin := filepath.Join(base, "sub-origin.git")
	run(t, base, "git", "init", "--bare", subOrigin)

	subSeed := filepath.Join(base, "sub-seed")
	run(t, base, "git", "clone", subOrigin, subSeed)
	git(t, subSeed, "config", "user.name", "Test User")
	git(t, subSeed, "config", "user.email", "test@example.com")
	git(t, subSeed, "config", "commit.gpgsign", "false")
	git(t, subSeed, "checkout", "-b", "main")
	writeFile(t, filepath.Join(subSeed, "sub.txt"), "root\n")
	git(t, subSeed, "add", "sub.txt")
	git(t, subSeed, "commit", "-m", "sub root")
	git(t, subSeed, "push", "-u", "origin", "main")

	root := initRepo(t, "main")
	git(t, root, "config", "user.name", "Test User")
	git(t, root, "config", "user.email", "test@example.com")
	git(t, root, "config", "commit.gpgsign", "false")
	writeFile(t, filepath.Join(root, "README.md"), "root\n")
	git(t, root, "add", "README.md")
	git(t, root, "commit", "-m", "root")
	run(t, root, "git", "-c", "protocol.file.allow=always", "submodule", "add", "-b", "main", subOrigin, "modules/sub")
	git(t, root, "commit", "-m", "add submodule")
	git(t, filepath.Join(root, "modules/sub"), "checkout", "-b", "feature/sub", "origin/main")
	git(t, filepath.Join(root, "modules/sub"), "branch", "-D", "main")
	git(t, filepath.Join(root, "modules/sub"), "update-ref", "-d", "refs/remotes/origin/main")

	workspace, err := LoadWorkspace(root, "")
	if err != nil {
		t.Fatalf("LoadWorkspace() error = %v", err)
	}
	if len(workspace.Repos) != 2 {
		t.Fatalf("len(Repos) = %d, want 2", len(workspace.Repos))
	}
	if workspace.Repos[1].LoadError == "" {
		t.Fatal("expected broken submodule entry to retain load error")
	}
}

func initRepo(t *testing.T, initialBranch string) string {
	t.Helper()

	root := t.TempDir()
	run(t, root, "git", "init", "-b", initialBranch)
	return root
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	run(t, dir, "git", args...)
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, output)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
