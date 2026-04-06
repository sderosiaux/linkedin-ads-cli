package main

import (
	"fmt"
	"os"

	"github.com/sderosiaux/linkedin-ads-cli/internal/cmd"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	rootCmd := cmd.NewRootCmd()
	rootCmd.Version = fmt.Sprintf("%s (commit %s, built %s)", version, commit, date)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
