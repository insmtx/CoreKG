package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/insmtx/corekg/clients/corekg-cli/internal/api"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/buildinfo"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/clierr"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/output"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/store"
	"github.com/spf13/cobra"
)

type Options struct {
	Paths         store.Paths
	In            io.Reader
	Out           io.Writer
	ErrOut        io.Writer
	ClientFactory func(string, time.Duration) (*api.Client, error)
}

type app struct {
	info          buildinfo.Info
	paths         store.Paths
	configSource  string
	outputFormat  string
	profileName   string
	timeout       time.Duration
	in            io.Reader
	out           io.Writer
	errOut        io.Writer
	clientFactory func(string, time.Duration) (*api.Client, error)
}

func NewRoot(info buildinfo.Info) (*cobra.Command, error) {
	paths, err := store.DefaultPaths()
	if err != nil {
		return nil, err
	}
	return NewRootWithOptions(info, Options{Paths: paths, Out: os.Stdout, ErrOut: os.Stderr}), nil
}

func NewRootWithOptions(info buildinfo.Info, options Options) *cobra.Command {
	if options.Out == nil {
		options.Out = os.Stdout
	}
	if options.In == nil {
		options.In = os.Stdin
	}
	if options.ErrOut == nil {
		options.ErrOut = os.Stderr
	}

	state := &app{
		info:          info,
		paths:         options.Paths,
		in:            options.In,
		out:           options.Out,
		errOut:        options.ErrOut,
		clientFactory: options.ClientFactory,
	}
	root := &cobra.Command{
		Use:           "corekg-cli",
		Short:         "CoreKG knowledge base command-line client",
		Version:       info.Version,
		Args:          noArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	root.SetVersionTemplate(fmt.Sprintf("{{.Version}} (%s)\n", productName))
	root.SetContext(context.WithValue(context.Background(), appContextKey{}, state))
	root.SetOut(state.out)
	root.SetErr(state.errOut)
	root.SetIn(state.in)
	root.PersistentFlags().StringVar(&state.configSource, "config", "", "Load configuration from a JSON file or inline JSON")
	root.PersistentFlags().StringVarP(&state.outputFormat, "output", "o", "", "Output format: table, json, or id")
	root.PersistentFlags().StringVar(&state.profileName, "profile", "", "Override the active Profile for this command")
	root.PersistentFlags().DurationVar(&state.timeout, "timeout", 0, "HTTP request timeout override (default from config or 30s)")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return clierr.Usage("invalid_flag", err.Error())
	})
	root.AddCommand(state.versionCommand())
	root.AddCommand(state.configCommand())
	root.AddCommand(state.profileCommand())
	root.AddCommand(state.authCommand())
	root.AddCommand(state.kbCommand())
	root.AddCommand(state.fileCommand())
	root.AddCommand(state.askCommand())
	return root
}

func (a *app) resolvePaths() (store.Paths, error) {
	if a.paths.RootDir == "" {
		return store.DefaultPaths()
	}
	return a.paths, nil
}

func (a *app) effectiveConfig() (store.Config, error) {
	paths, err := a.resolvePaths()
	if err != nil {
		return store.Config{}, err
	}
	config, err := store.LoadConfig(paths)
	if err != nil {
		return store.Config{}, err
	}
	if strings.TrimSpace(a.configSource) != "" {
		override, sourceErr := store.LoadConfigSource(a.configSource)
		if sourceErr != nil {
			return store.Config{}, clierr.Usage("invalid_config", sourceErr.Error())
		}
		config = store.MergeConfig(config, override)
	}
	if err := config.Validate(); err != nil {
		return store.Config{}, clierr.Usage("invalid_config", err.Error())
	}
	return config, nil
}

func (a *app) format() (string, error) {
	value := a.outputFormat
	if value == "" {
		config, err := a.effectiveConfig()
		if err != nil {
			return "", err
		}
		value = config.Output
	}
	if value == "" {
		value = "table"
	}
	format, err := output.ParseFormat(value)
	if err != nil {
		return "", clierr.Usage("invalid_output", err.Error())
	}
	return string(format), nil
}

func (a *app) selectedProfile() string {
	if name := strings.TrimSpace(a.profileName); name != "" {
		return name
	}
	return strings.TrimSpace(os.Getenv("COREKG_PROFILE"))
}

type appContextKey struct{}

func noArgs(cmd *cobra.Command, args []string) error {
	if err := cobra.NoArgs(cmd, args); err != nil {
		return clierr.Usage("invalid_arguments", err.Error())
	}
	return nil
}

func exactArgs(count int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(count)(cmd, args); err != nil {
			return clierr.Usage("invalid_arguments", err.Error())
		}
		return nil
	}
}

func OutputFormat(command *cobra.Command) (string, error) {
	if command == nil {
		return string(output.FormatTable), nil
	}
	state, ok := command.Context().Value(appContextKey{}).(*app)
	if !ok {
		return string(output.FormatTable), nil
	}
	return state.format()
}

func (a *app) newAPIClient(serverURL string) (*api.Client, error) {
	timeout := a.timeout
	if timeout == 0 {
		config, err := a.effectiveConfig()
		if err != nil {
			return nil, err
		}
		timeout, err = config.TimeoutDuration()
		if err != nil {
			return nil, err
		}
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("HTTP request timeout must be positive")
	}
	return a.newAPIClientWithTimeout(serverURL, timeout)
}

func (a *app) newAPIClientWithTimeout(serverURL string, timeout time.Duration) (*api.Client, error) {
	if a.clientFactory != nil {
		return a.clientFactory(serverURL, timeout)
	}
	return api.NewWithTimeout(serverURL, timeout)
}
