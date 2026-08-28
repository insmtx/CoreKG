package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

const productName = "CoreKG CLI"

func (a *app) versionCommand() *cobra.Command {
	var short bool
	command := &cobra.Command{
		Use:   "version",
		Short: "Print CLI version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := a.format()
			if err != nil {
				return err
			}
			if short {
				_, err := fmt.Fprintln(a.out, a.info.Version)
				return err
			}
			switch format {
			case "json":
				encoder := json.NewEncoder(a.out)
				encoder.SetIndent("", "  ")
				return encoder.Encode(struct {
					Name    string `json:"name"`
					Version string `json:"version"`
				}{Name: productName, Version: a.info.Version})
			case "id":
				_, err := fmt.Fprintln(a.out, a.info.Version)
				return err
			default:
				_, err := fmt.Fprintf(a.out, "%s (%s)\n", a.info.Version, productName)
				return err
			}
		},
	}
	command.Flags().BoolVarP(&short, "short", "s", false, "Print only the version")
	return command
}
