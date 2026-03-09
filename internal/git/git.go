package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"sort"
	"strings"

	"gitreview/internal/state"
)

var ErrGitNotFound = errors.New("error: git executable not found in PATH")
var ErrDiffTooLarge = errors.New("Diff too large to display.")
var ErrAmbiguousRange = errors.New("error: selected commits do not form a single ancestry path")

const maxDiffBytes = 5 * 1024 * 1024

type Client struct {
	binary string
}

func FindBinary() (string, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return "", ErrGitNotFound
	}

	return path, nil
}

func NewClient() (Client, error) {
	path, err := FindBinary()
	if err != nil {
		return Client{}, err
	}

	return Client{binary: path}, nil
}

func (c Client) RepoRoot(dir string) (string, error) {
	return c.runTrimmed(dir, "rev-parse", "--show-toplevel")
}

func (c Client) CurrentBranch(dir string) (string, error) {
	return c.runTrimmed(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

func (c Client) SymbolicOriginHEAD(dir string) (string, error) {
	return c.runTrimmed(dir, "symbolic-ref", "refs/remotes/origin/HEAD")
}

func (c Client) RefExists(dir, ref string) bool {
	_, err := c.runTrimmed(dir, "rev-parse", "--verify", ref)
	return err == nil
}

func (c Client) SubmodulePaths(dir string) ([]string, error) {
	out, err := c.run(dir, "submodule", "status", "--recursive")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}

	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		paths = append(paths, fields[1])
	}

	return slices.Compact(paths), nil
}

func (c Client) LoadCommits(dir, baseRef string) ([]state.Commit, error) {
	out, err := c.run(dir, "log", baseRef+"..HEAD", "--date=short", "--pretty=format:%H%x09%h%x09%ad%x09%an%x09%s")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}

	commits := make([]state.Commit, 0, len(lines))
	for _, line := range lines {
		fields := strings.SplitN(line, "\t", 5)
		if len(fields) != 5 {
			return nil, fmt.Errorf("error: unexpected git log output")
		}

		commits = append(commits, state.Commit{
			SHA:      fields[0],
			ShortSHA: fields[1],
			Date:     fields[2],
			Author:   fields[3],
			Subject:  fields[4],
		})
	}

	return commits, nil
}

func (c Client) LoadFilesForCommit(dir, sha string) ([]string, error) {
	out, err := c.run(dir, "show", "-m", "--name-only", "--pretty=format:", sha)
	if err != nil {
		return nil, err
	}

	lines := splitNonEmptyLines(out)
	sortFiles(lines)
	return lines, nil
}

func (c Client) LoadFilesForRange(dir, oldest, newest string) ([]string, error) {
	args, err := c.rangeArgs(dir, oldest, newest)
	if err != nil {
		return nil, err
	}

	out, err := c.run(dir, append([]string{"diff", "--name-only"}, args...)...)
	if err != nil {
		return nil, err
	}

	lines := splitNonEmptyLines(out)
	sortFiles(lines)
	return lines, nil
}

func (c Client) NormalizeRange(dir string, shas []string) (string, string, error) {
	if len(shas) == 0 {
		return "", "", fmt.Errorf("error: empty range")
	}
	if len(shas) == 1 {
		return shas[0], shas[0], nil
	}

	oldest := ""
	for _, candidate := range shas {
		allAncestors := true
		for _, other := range shas {
			if candidate == other {
				continue
			}
			ancestor, err := c.IsAncestor(dir, candidate, other)
			if err != nil {
				return "", "", err
			}
			if !ancestor {
				allAncestors = false
				break
			}
		}
		if allAncestors {
			oldest = candidate
			break
		}
	}

	newest := ""
	for _, candidate := range shas {
		allDescendants := true
		for _, other := range shas {
			if candidate == other {
				continue
			}
			ancestor, err := c.IsAncestor(dir, other, candidate)
			if err != nil {
				return "", "", err
			}
			if !ancestor {
				allDescendants = false
				break
			}
		}
		if allDescendants {
			newest = candidate
			break
		}
	}

	if oldest == "" || newest == "" {
		return "", "", ErrAmbiguousRange
	}

	return oldest, newest, nil
}

func (c Client) LoadDiffForCommit(dir, sha, path string) (string, error) {
	args := []string{"show", "-m", "--patch", "--find-renames", sha}
	if path != "" {
		args = append(args, "--", path)
	}

	out, err := c.run(dir, args...)
	if err != nil {
		return "", err
	}

	if len(out) > maxDiffBytes {
		return "", ErrDiffTooLarge
	}

	return out, nil
}

func (c Client) LoadDiffForRange(dir, oldest, newest, path string) (string, error) {
	rangeArgs, err := c.rangeArgs(dir, oldest, newest)
	if err != nil {
		return "", err
	}

	args := append([]string{"diff"}, rangeArgs...)
	if path != "" {
		args = append(args, "--", path)
	}

	out, err := c.run(dir, args...)
	if err != nil {
		return "", err
	}

	if len(out) > maxDiffBytes {
		return "", ErrDiffTooLarge
	}

	return out, nil
}

func (c Client) IsAncestor(dir, maybeAncestor, descendant string) (bool, error) {
	cmd := exec.Command(c.binary, "merge-base", "--is-ancestor", maybeAncestor, descendant)
	cmd.Dir = dir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}

	return false, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
}

func (c Client) runTrimmed(dir string, args ...string) (string, error) {
	out, err := c.run(dir, args...)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(out), nil
}

func (c Client) run(dir string, args ...string) (string, error) {
	cmd := exec.Command(c.binary, args...)
	cmd.Dir = dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}

func (c Client) rangeArgs(dir, oldest, newest string) ([]string, error) {
	root, err := c.isRootCommit(dir, oldest)
	if err != nil {
		return nil, err
	}

	if root {
		return []string{"4b825dc642cb6eb9a060e54bf8d69288fbee4904", newest}, nil
	}

	return []string{oldest + "^.." + newest}, nil
}

func (c Client) isRootCommit(dir, sha string) (bool, error) {
	out, err := c.runTrimmed(dir, "rev-list", "--parents", "-n", "1", sha)
	if err != nil {
		return false, err
	}

	return len(strings.Fields(out)) == 1, nil
}

func splitNonEmptyLines(raw string) []string {
	lines := strings.Split(raw, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		filtered = append(filtered, line)
	}
	return slices.Compact(filtered)
}

func sortFiles(paths []string) {
	sort.SliceStable(paths, func(i, j int) bool {
		return strings.ToLower(paths[i]) < strings.ToLower(paths[j])
	})
}
