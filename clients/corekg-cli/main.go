package main

import (
	"fmt"
	"os"

	"github.com/insmtx/corekg/clients/corekg-cli/internal/buildinfo"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/clierr"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/commands"
)

func main() {
	root, err := commands.NewRoot(buildinfo.Info{
		Name:      "corekg-cli",
		Version:   buildinfo.Version,
		GitCommit: buildinfo.GitCommit,
		BuiltAt:   buildinfo.BuiltAt,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := root.Execute(); err != nil {
		format, formatErr := commands.OutputFormat(root)
		if formatErr == nil && format == "json" {
			if renderErr := clierr.WriteJSON(os.Stderr, err); renderErr != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(clierr.ExitCode(err))
	}
}
