package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:           "corekg",
		Short:         "CoreKG monolith server",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	rootCmd.AddCommand(newServerCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
