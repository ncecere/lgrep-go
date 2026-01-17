// Package main is the entry point for the lgrep CLI application.
package main

import (
	"os"

	"github.com/nickcecere/lgrep/internal/cli"
)

// Version information (set at build time via ldflags)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.SetVersionInfo(version, commit, date)

	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
