package cli

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/pflag"
)

const usageText = `usage: gitreview [path] [--base <branch>] [--version]`

type Config struct {
	Path        string
	BaseBranch  string
	ShowVersion bool
}

type UsageError struct {
	msg string
}

func (e UsageError) Error() string {
	return e.msg
}

func (e UsageError) Usage() string {
	return usageText
}

func Parse(args []string) (Config, error) {
	cfg := Config{Path: "."}
	flags := pflag.NewFlagSet("gitreview", pflag.ContinueOnError)
	flags.SetOutput(ioDiscard{})
	flags.StringVar(&cfg.BaseBranch, "base", "", "override base branch detection")
	flags.BoolVar(&cfg.ShowVersion, "version", false, "print version and exit")

	if err := flags.Parse(args); err != nil {
		return Config{}, UsageError{msg: fmt.Sprintf("error: %s", err)}
	}

	if flags.NArg() > 1 {
		return Config{}, errors.New("error: too many positional arguments")
	}
	if flags.NArg() == 1 {
		cfg.Path = flags.Arg(0)
	}

	cfg.Path = filepath.Clean(cfg.Path)
	return cfg, nil
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
