package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFilesForRangeIncludesRootCommitChanges(t *testing.T) {
	root := initRepo(t, "main")
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	writeFile(t, filepath.Join(root, "one.txt"), "one\n")
	gitCmd(t, root, "add", "one.txt")
	gitCmd(t, root, "commit", "-m", "root")
	rootSHA := gitOutput(t, root, "rev-parse", "HEAD")

	writeFile(t, filepath.Join(root, "two.txt"), "two\n")
	gitCmd(t, root, "add", "two.txt")
	gitCmd(t, root, "commit", "-m", "second")
	headSHA := gitOutput(t, root, "rev-parse", "HEAD")

	files, err := client.LoadFilesForRange(root, rootSHA, headSHA)
	if err != nil {
		t.Fatalf("LoadFilesForRange() error = %v", err)
	}

	if len(files) != 2 || files[0] != "one.txt" || files[1] != "two.txt" {
		t.Fatalf("LoadFilesForRange() = %#v, want [one.txt two.txt]", files)
	}
}

func TestLoadDiffForRangeCanFilterFile(t *testing.T) {
	root := initRepo(t, "main")
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	writeFile(t, filepath.Join(root, "one.txt"), "one\n")
	gitCmd(t, root, "add", "one.txt")
	gitCmd(t, root, "commit", "-m", "root")
	rootSHA := gitOutput(t, root, "rev-parse", "HEAD")

	writeFile(t, filepath.Join(root, "one.txt"), "one updated\n")
	writeFile(t, filepath.Join(root, "two.txt"), "two\n")
	gitCmd(t, root, "add", "one.txt", "two.txt")
	gitCmd(t, root, "commit", "-m", "second")
	headSHA := gitOutput(t, root, "rev-parse", "HEAD")

	diff, err := client.LoadDiffForRange(root, rootSHA, headSHA, "two.txt")
	if err != nil {
		t.Fatalf("LoadDiffForRange() error = %v", err)
	}

	if !strings.Contains(diff, "two.txt") || strings.Contains(diff, "one updated") {
		t.Fatalf("LoadDiffForRange() diff did not match file filter:\n%s", diff)
	}
}

func TestLoadDiffForCommitRejectsOversizedDiff(t *testing.T) {
	root := initRepo(t, "main")
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	writeFile(t, filepath.Join(root, "big.txt"), "small\n")
	gitCmd(t, root, "add", "big.txt")
	gitCmd(t, root, "commit", "-m", "root")

	large := strings.Repeat("x", maxDiffBytes+1024)
	writeFile(t, filepath.Join(root, "big.txt"), large)
	gitCmd(t, root, "add", "big.txt")
	gitCmd(t, root, "commit", "-m", "big change")
	headSHA := gitOutput(t, root, "rev-parse", "HEAD")

	_, err = client.LoadDiffForCommit(root, headSHA, "")
	if !errors.Is(err, ErrDiffTooLarge) {
		t.Fatalf("LoadDiffForCommit() error = %v, want %v", err, ErrDiffTooLarge)
	}
}

func TestNormalizeRangeOrdersOldestToNewest(t *testing.T) {
	root := initRepo(t, "main")
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	writeFile(t, filepath.Join(root, "one.txt"), "one\n")
	gitCmd(t, root, "add", "one.txt")
	gitCmd(t, root, "commit", "-m", "one")
	sha1 := gitOutput(t, root, "rev-parse", "HEAD")

	writeFile(t, filepath.Join(root, "two.txt"), "two\n")
	gitCmd(t, root, "add", "two.txt")
	gitCmd(t, root, "commit", "-m", "two")
	sha2 := gitOutput(t, root, "rev-parse", "HEAD")

	writeFile(t, filepath.Join(root, "three.txt"), "three\n")
	gitCmd(t, root, "add", "three.txt")
	gitCmd(t, root, "commit", "-m", "three")
	sha3 := gitOutput(t, root, "rev-parse", "HEAD")

	oldest, newest, err := client.NormalizeRange(root, []string{sha3, sha2, sha1})
	if err != nil {
		t.Fatalf("NormalizeRange() error = %v", err)
	}

	if oldest != sha1 || newest != sha3 {
		t.Fatalf("NormalizeRange() = (%q, %q), want (%q, %q)", oldest, newest, sha1, sha3)
	}
}

func TestNormalizeRangeRejectsAmbiguousMergeSet(t *testing.T) {
	root := initRepo(t, "main")
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	writeFile(t, filepath.Join(root, "base.txt"), "base\n")
	gitCmd(t, root, "add", "base.txt")
	gitCmd(t, root, "commit", "-m", "base")

	gitCmd(t, root, "checkout", "-b", "left")
	writeFile(t, filepath.Join(root, "left.txt"), "left\n")
	gitCmd(t, root, "add", "left.txt")
	gitCmd(t, root, "commit", "-m", "left")
	leftSHA := gitOutput(t, root, "rev-parse", "HEAD")

	gitCmd(t, root, "checkout", "main")
	gitCmd(t, root, "checkout", "-b", "right")
	writeFile(t, filepath.Join(root, "right.txt"), "right\n")
	gitCmd(t, root, "add", "right.txt")
	gitCmd(t, root, "commit", "-m", "right")
	rightSHA := gitOutput(t, root, "rev-parse", "HEAD")

	_, _, err = client.NormalizeRange(root, []string{leftSHA, rightSHA})
	if !errors.Is(err, ErrAmbiguousRange) {
		t.Fatalf("NormalizeRange() error = %v, want %v", err, ErrAmbiguousRange)
	}
}

func TestLoadDiffForCommitShowsMergeCommitPatch(t *testing.T) {
	root := initRepo(t, "main")
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	writeFile(t, filepath.Join(root, "base.txt"), "base\n")
	gitCmd(t, root, "add", "base.txt")
	gitCmd(t, root, "commit", "-m", "base")

	gitCmd(t, root, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(root, "feature.txt"), "feature\n")
	gitCmd(t, root, "add", "feature.txt")
	gitCmd(t, root, "commit", "-m", "feature")

	gitCmd(t, root, "checkout", "main")
	writeFile(t, filepath.Join(root, "main.txt"), "main\n")
	gitCmd(t, root, "add", "main.txt")
	gitCmd(t, root, "commit", "-m", "main")

	gitCmd(t, root, "checkout", "feature")
	gitCmd(t, root, "merge", "main", "-m", "merge main")
	mergeSHA := gitOutput(t, root, "rev-parse", "HEAD")

	diff, err := client.LoadDiffForCommit(root, mergeSHA, "")
	if err != nil {
		t.Fatalf("LoadDiffForCommit() error = %v", err)
	}
	if !strings.Contains(diff, "diff --git") {
		t.Fatalf("LoadDiffForCommit() did not return a patch for merge commit:\n%s", diff)
	}

	files, err := client.LoadFilesForCommit(root, mergeSHA)
	if err != nil {
		t.Fatalf("LoadFilesForCommit() error = %v", err)
	}
	if len(files) == 0 {
		t.Fatal("LoadFilesForCommit() returned no files for merge commit")
	}
}

func initRepo(t *testing.T, branch string) string {
	t.Helper()

	root := t.TempDir()
	run(t, root, "git", "init", "-b", branch)
	run(t, root, "git", "config", "user.name", "Test User")
	run(t, root, "git", "config", "user.email", "test@example.com")
	run(t, root, "git", "config", "commit.gpgsign", "false")
	return root
}

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	run(t, dir, "git", args...)
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v output error = %v", args, err)
	}

	return strings.TrimSpace(string(out))
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
