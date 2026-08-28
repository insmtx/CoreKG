package commands

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/insmtx/corekg/clients/corekg-cli/internal/clierr"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/output"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/store"
	"github.com/spf13/cobra"
)

type configPathsOutput struct {
	RootDir    string `json:"root_dir"`
	ConfigFile string `json:"config_file"`
	StateFile  string `json:"state_file"`
	AuthFile   string `json:"auth_file"`
}

func (a *app) configCommand() *cobra.Command {
	config := &cobra.Command{
		Use:   "config",
		Short: "Manage local CLI configuration",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	config.AddCommand(a.configInitCommand())
	config.AddCommand(a.configPathCommand())
	return config
}

func (a *app) configPathCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print configuration file locations",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := a.resolvePaths()
			if err != nil {
				return err
			}
			value := configPathsOutput{RootDir: paths.RootDir, ConfigFile: paths.ConfigFile, StateFile: paths.StateFile, AuthFile: paths.AuthFile}
			format, err := a.format()
			if err != nil {
				return err
			}
			switch format {
			case "json":
				return output.WriteJSON(a.out, value)
			case "id":
				_, err := fmt.Fprintln(a.out, value.RootDir)
				return err
			default:
				return output.WriteTable(a.out, []string{"NAME", "PATH"}, [][]string{
					{"root", value.RootDir},
					{"config", value.ConfigFile},
					{"state", value.StateFile},
					{"auth", value.AuthFile},
				})
			}
		},
	}
}

func (a *app) configInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Interactively initialize the default configuration",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(a.configSource) != "" {
				return clierr.Usage("config_init_override", "config init always writes the default configuration; remove --config")
			}
			paths, err := a.resolvePaths()
			if err != nil {
				return err
			}
			defaults := store.DefaultConfig()
			current, loadErr := store.LoadConfig(paths)
			if loadErr != nil {
				return loadErr
			}
			defaults = store.MergeConfig(defaults, current)
			reader := bufio.NewReader(a.in)
			server, err := promptConfigValue(reader, a.errOut, "CoreKG server", defaults.Server)
			if err != nil {
				return err
			}
			frontend, err := promptConfigValue(reader, a.errOut, "CoreKG frontend", defaults.Frontend)
			if err != nil {
				return err
			}
			format, err := promptConfigValue(reader, a.errOut, "Default output", defaults.Output)
			if err != nil {
				return err
			}
			timeout, err := promptConfigValue(reader, a.errOut, "HTTP timeout", defaults.Timeout)
			if err != nil {
				return err
			}
			save, err := promptConfigConfirmation(reader, a.errOut)
			if err != nil {
				return err
			}
			switch strings.ToLower(strings.TrimSpace(save)) {
			case "", "y", "yes":
			case "n", "no":
				return clierr.Confirm("config_init_cancelled", "configuration was not saved")
			default:
				return clierr.Usage("config_init_confirmation", "answer yes or no to save configuration")
			}
			config := store.Config{Version: 1, Server: server, Frontend: frontend, Output: format, Timeout: timeout}
			config = config.Normalize()
			if err := config.Validate(); err != nil {
				return clierr.Usage("invalid_config", err.Error())
			}
			if err := store.WithLock(paths, func() error {
				return store.SaveConfig(paths, config)
			}); err != nil {
				return err
			}
			return a.writeOutput(map[string]string{"config_file": paths.ConfigFile, "status": "initialized"})
		},
	}
}

func promptConfigValue(reader *bufio.Reader, writer io.Writer, label, defaultValue string) (string, error) {
	if _, err := fmt.Fprintf(writer, "%s [%s]: ", label, defaultValue); err != nil {
		return "", err
	}
	value, err := reader.ReadString('\n')
	if err != nil && len(value) == 0 {
		if err == io.EOF {
			return "", clierr.Usage("config_init_input_required", "config init requires interactive input")
		}
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}

func promptConfigConfirmation(reader *bufio.Reader, writer io.Writer) (string, error) {
	if _, err := fmt.Fprint(writer, "Save configuration? [Y/n]: "); err != nil {
		return "", err
	}
	value, err := reader.ReadString('\n')
	if err != nil && len(value) == 0 {
		if err == io.EOF {
			return "", clierr.Usage("config_init_input_required", "config init requires interactive input")
		}
		return "", err
	}
	return strings.TrimSpace(value), nil
}
