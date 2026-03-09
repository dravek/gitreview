package main

import (
	"fmt"
	"os"

	"gitreview/internal/app"
)

func main() {
	code, err := app.Run(os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	os.Exit(code)
}
