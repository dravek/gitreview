package app

import (
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"

	"gitreview/internal/cli"
	ggit "gitreview/internal/git"
	"gitreview/internal/repo"
	"gitreview/internal/ui"
	"gitreview/internal/version"
)

// Run parses the CLI, performs startup validation, and launches the TUI.
func Run(args []string, stdout, stderr io.Writer) (int, error) {
	cfg, err := cli.Parse(args)
	if err != nil {
		if usageErr, ok := err.(cli.UsageError); ok {
			fmt.Fprintln(stderr, usageErr.Usage())
			return 1, nil
		}

		return 1, err
	}

	if cfg.ShowVersion {
		fmt.Fprintln(stdout, version.String())
		return 0, nil
	}

	if err := validateTerminalSize(stdout); err != nil {
		return 1, err
	}

	workspace, err := repo.LoadWorkspace(cfg.Path, cfg.BaseBranch)
	if err != nil {
		return 1, err
	}

	gitClient, err := ggit.NewClient()
	if err != nil {
		return 1, err
	}

	program := tea.NewProgram(
		ui.NewModel(workspace.Repos, gitClient),
		tea.WithOutput(stdout),
	)
	if _, err := program.Run(); err != nil {
		return 1, fmt.Errorf("error: failed to run TUI: %w", err)
	}

	return 0, nil
}

func validateTerminalSize(output io.Writer) error {
	file, ok := output.(*os.File)
	if !ok {
		return nil
	}

	width, height, err := term.GetSize(file.Fd())
	if err != nil {
		return nil
	}

	if width < 80 || height < 24 {
		return fmt.Errorf("error: terminal too small (need at least 80x24)")
	}

	return nil
}
