# gitreview

`gitreview` is a read-only terminal UI for reviewing the commits on your current branch relative to a base branch.

It is built for branch review inside the terminal:
- inspect commits ahead of `main`, `master`, or `origin/HEAD`
- toggle into recent `HEAD` history for merged/older commit review
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
- toggleable capped history mode for large repos

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

Set the app version in one place:

- edit `DefaultVersion` in `internal/version/version.go`
- `./rebuild.sh` and `./release.sh` always build from that value
- release archive filenames are sanitized if needed, but the app version itself is embedded exactly as written
- the same version is used for `--version` output and the footer in the UI

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

History review mode:
- press `h` to toggle between ahead-only review and recent `HEAD` history
- the header shows the active scope and either `ahead: N` or `loaded: N`
- history mode loads the most recent 300 commits first
- press `]` to load 200 more history commits

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

Global:
- `tab` / `shift+tab`: switch panels
- `r`: open the repo/submodule switcher overlay
- `h`: toggle ahead-only review vs recent `HEAD` history
- `f`: toggle fullscreen diff
- `?`: open help
- `q`: quit

Repos panel:
- `j` / `k` or arrows: move
- `g` / `G`: jump to top / bottom
- `enter` or `space`: switch repo or submodule

Commits panel:
- `j` / `k` or arrows: move
- `PgUp` / `PgDn`: page
- `g` / `G`: jump to top / bottom
- `space`: start or update a contiguous commit selection
- `enter`: focus diff
- `/`: filter commits by subject or short SHA
- `]`: load 200 more commits in history mode
- `esc`: clear selection or exit active filter mode

Files panel:
- `j` / `k` or arrows: move
- `PgUp` / `PgDn`: page
- `g` / `G`: jump to top / bottom
- `enter` or `space`: filter the diff to the selected file
- `esc`: clear the active file filter

Diff panel:
- `j` / `k` or arrows: scroll
- `PgUp` / `PgDn`: page scroll
- `g` / `G`: jump to top / bottom

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
