# gitreview

`gitreview` is a read-only terminal UI for reviewing the commits on your current branch relative to a base branch.

It is built for branch review inside the terminal:
- inspect commits ahead of `main`, `master`, or `origin/HEAD`
- review a single commit diff
- select a contiguous commit range
- narrow a diff to a single file
- switch between a root repo and initialized submodules
- stay strictly read-only

## Current Status

This repository contains a working v1 core implementation in Go.

Supported now:
- root repo plus initialized submodule browsing in one session
- commit list and diff viewer
- file list and file-filtered diffs
- contiguous range selection
- commit filter mode with `/`
- fullscreen diff and help overlay
- auto-detected base branch, with `--base` override

## Requirements

- Go 1.24.4+ to build from source
- `git` installed and available in `PATH`
- terminal size of at least `80x24`

## Build

Local build:

```bash
./rebuild.sh
```

That produces:

```bash
./gitreview
```

Print version:

```bash
./gitreview --version
```

## Install

Install to `/usr/local/bin`:

```bash
./rebuild.sh --install
```

After installation:

```bash
gitreview
git gitreview
```

## Usage

Run in the current repo:

```bash
gitreview
```

Run against an explicit repo path:

```bash
gitreview /path/to/repo
```

Override the detected base branch:

```bash
gitreview --base main
gitreview --base origin/main /path/to/repo
```

## Submodules

If you launch `gitreview` at a superproject root and initialized submodules are present, the left side will show a `REPOS` section above `COMMITS`.

Use that section to switch between:
- the root repository
- initialized submodules

Workflow:
- launch `gitreview` from the root repo
- `tab` to the `REPOS` panel
- move with `j` / `k`
- press `enter` or `space` to switch to that repo

Repos with `ahead > 0` are highlighted in green so related branch work is easier to spot.

## Keys

- `j` / `k` or arrows: move
- `tab` / `shift+tab`: switch panels
- `r`: open the repo/submodule switcher overlay
- `enter`: focus diff or apply file filter
- `space`: start or update a contiguous commit selection
- `esc`: clear selection, file filter, or active filter mode
- `/`: filter commits by subject or short SHA
- `f`: toggle fullscreen diff
- `?`: open help
- `q`: quit

## Development

Test:

```bash
GOCACHE="$(pwd)/.cache/go-build" go test ./...
```

Build:

```bash
GOCACHE="$(pwd)/.cache/go-build" go build ./cmd/gitreview
```

## Release

Create local release archives:

```bash
./release.sh
```

This writes archives to `dist/`.

`release.sh` is optional but useful in the public repo if you want a simple, repeatable local release process. If you plan to publish binaries from GitHub Actions later, keep it: the script is still useful for local testing and manual releases.
