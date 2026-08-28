package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/insmtx/corekg/clients/corekg-cli/internal/api"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/clierr"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/output"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/store"
	"github.com/spf13/cobra"
)

const defaultKBForestType = "file"

type kbSelectionOutput struct {
	Profile           string `json:"profile"`
	KnowledgeBaseID   string `json:"knowledge_base_id"`
	KnowledgeBaseName string `json:"knowledge_base_name"`
}

type kbCreateOutput struct {
	Profile     string `json:"profile"`
	ForestID    uint   `json:"forest_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ForestType  string `json:"forest_type"`
	AvatarURL   string `json:"avatar_url"`
}

func (a *app) kbCommand() *cobra.Command {
	kb := &cobra.Command{Use: "kb", Short: "Manage knowledge bases"}
	kb.AddCommand(a.kbCreateCommand())
	kb.AddCommand(a.kbListCommand())
	kb.AddCommand(a.kbUseCommand())
	return kb
}

func (a *app) kbCreateCommand() *cobra.Command {
	var description, avatarURL string
	var use, yes bool
	command := &cobra.Command{
		Use:   "create [NAME]",
		Short: "Create a knowledge base",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return clierr.Usage("invalid_arguments", "create accepts at most one knowledge base name")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := a.format(); err != nil {
				return err
			}
			active, err := a.loadActiveProfile(a.profileName)
			if err != nil {
				return err
			}
			var reader *bufio.Reader
			if !yes {
				reader = bufio.NewReader(a.in)
			}
			values, err := a.readKBCreateValues(cmd, args, reader, description, avatarURL, yes)
			if err != nil {
				return err
			}
			if !yes {
				if err := a.writeKBCreateConfirmation(values, use); err != nil {
					return err
				}
				confirmed, confirmErr := promptConfirmation(reader, a.errOut, "Create knowledge base? [Y/n]: ", "kb_create_input_required")
				if confirmErr != nil {
					return confirmErr
				}
				switch strings.ToLower(strings.TrimSpace(confirmed)) {
				case "", "y", "yes":
				case "n", "no":
					return clierr.Confirm("kb_create_cancelled", "knowledge base was not created")
				default:
					return clierr.Usage("kb_create_confirmation", "answer yes or no to create the knowledge base")
				}
			}

			var created api.CreateForestResult
			if err := active.Client.DoJSON(cmd.Context(), active.Credential.APIKey, "keapi.CreateForest", map[string]any{
				"name":        values.Name,
				"avatar_url":  values.AvatarURL,
				"description": values.Description,
				"forest_type": values.ForestType,
			}, &created); err != nil {
				return clierr.Wrap("kb_create_failed", err)
			}
			if created.ForestID == 0 {
				return clierr.WithDetails("kb_create_invalid_response", "CreateForest returned an invalid forest ID", map[string]any{
					"name":        values.Name,
					"forest_type": values.ForestType,
				})
			}
			if use {
				if err := saveKnowledgeBaseSelection(active, created.ForestID, values.Name); err != nil {
					return clierr.WithDetails("kb_create_selection_failed", fmt.Sprintf("knowledge base %d was created but could not be selected: %v", created.ForestID, err), map[string]any{
						"profile":     active.Name,
						"forest_id":   created.ForestID,
						"name":        values.Name,
						"forest_type": values.ForestType,
					})
				}
			}
			return a.writeKBCreateOutput(kbCreateOutput{
				Profile:     active.Name,
				ForestID:    created.ForestID,
				Name:        values.Name,
				Description: values.Description,
				ForestType:  values.ForestType,
				AvatarURL:   values.AvatarURL,
			})
		},
	}
	command.Flags().StringVar(&description, "description", "", "Knowledge base description")
	command.Flags().StringVar(&avatarURL, "avatar-url", "", "Optional knowledge base avatar URL")
	command.Flags().BoolVar(&use, "use", false, "Set the created knowledge base as the current default")
	command.Flags().BoolVarP(&yes, "yes", "y", false, "Skip interactive prompts and confirmation")
	return command
}

type kbCreateValues struct {
	Name        string
	Description string
	ForestType  string
	AvatarURL   string
}

func (a *app) readKBCreateValues(command *cobra.Command, args []string, reader *bufio.Reader, description, avatarURL string, yes bool) (kbCreateValues, error) {
	values := kbCreateValues{
		Description: strings.TrimSpace(description),
		ForestType:  defaultKBForestType,
		AvatarURL:   strings.TrimSpace(avatarURL),
	}
	if len(args) == 1 {
		values.Name = strings.TrimSpace(args[0])
		if values.Name == "" {
			return kbCreateValues{}, clierr.Usage("kb_name_required", "knowledge base name must not be empty")
		}
	}
	if yes {
		if values.Name == "" {
			return kbCreateValues{}, clierr.Usage("kb_name_required", "provide NAME when using --yes")
		}
		return validateKBCreateValues(values)
	}

	var err error
	if len(args) == 0 {
		values.Name, err = promptValue(reader, a.errOut, "Knowledge base name", "", "kb_create_input_required")
		if err != nil {
			return kbCreateValues{}, err
		}
	}
	if !command.Flags().Changed("description") {
		values.Description, err = promptValue(reader, a.errOut, "Description", "", "kb_create_input_required")
		if err != nil {
			return kbCreateValues{}, err
		}
	}
	if !command.Flags().Changed("avatar-url") {
		values.AvatarURL, err = promptValue(reader, a.errOut, "Avatar URL", "", "kb_create_input_required")
		if err != nil {
			return kbCreateValues{}, err
		}
	}
	values.Name = strings.TrimSpace(values.Name)
	values.Description = strings.TrimSpace(values.Description)
	values.AvatarURL = strings.TrimSpace(values.AvatarURL)
	return validateKBCreateValues(values)
}

func validateKBCreateValues(values kbCreateValues) (kbCreateValues, error) {
	if values.Name == "" {
		return kbCreateValues{}, clierr.Usage("kb_name_required", "knowledge base name must not be empty")
	}
	values.ForestType = defaultKBForestType
	return values, nil
}

func (a *app) writeKBCreateConfirmation(values kbCreateValues, use bool) error {
	_, err := fmt.Fprintf(a.errOut, "Create knowledge base:\n  Name: %s\n  Description: %s\n  Type: %s\n  Avatar URL: %s\n  Set as current: %t\n", values.Name, emptyDisplay(values.Description), values.ForestType, emptyDisplay(values.AvatarURL), use)
	return err
}

func emptyDisplay(value string) string {
	if value == "" {
		return "(empty)"
	}
	return value
}

func promptValue(reader *bufio.Reader, writer io.Writer, label, defaultValue, inputErrorCode string) (string, error) {
	if defaultValue == "" {
		if _, err := fmt.Fprintf(writer, "%s: ", label); err != nil {
			return "", err
		}
	} else if _, err := fmt.Fprintf(writer, "%s [%s]: ", label, defaultValue); err != nil {
		return "", err
	}
	value, err := reader.ReadString('\n')
	if err != nil && len(value) == 0 {
		if errors.Is(err, io.EOF) {
			return "", clierr.Usage(inputErrorCode, "kb create requires interactive input")
		}
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}

func promptConfirmation(reader *bufio.Reader, writer io.Writer, prompt, inputErrorCode string) (string, error) {
	if _, err := fmt.Fprint(writer, prompt); err != nil {
		return "", err
	}
	value, err := reader.ReadString('\n')
	if err != nil && len(value) == 0 {
		if errors.Is(err, io.EOF) {
			return "", clierr.Usage(inputErrorCode, "kb create requires interactive confirmation")
		}
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func saveKnowledgeBaseSelection(active *activeProfile, forestID uint, name string) error {
	return store.WithLock(active.Paths, func() error {
		settings, err := store.LoadState(active.Paths)
		if err != nil {
			return err
		}
		definition := settings.Profiles[active.Name]
		definition.KnowledgeBaseID = strconv.FormatUint(uint64(forestID), 10)
		definition.KnowledgeBaseName = name
		settings.Profiles[active.Name] = definition
		return store.SaveState(active.Paths, settings)
	})
}

func (a *app) writeKBCreateOutput(value kbCreateOutput) error {
	format, err := a.format()
	if err != nil {
		return err
	}
	if format == "json" {
		return output.WriteJSON(a.out, value)
	}
	if format == "id" {
		return output.WriteNames(a.out, []string{strconv.FormatUint(uint64(value.ForestID), 10)})
	}
	return output.WriteTable(a.out, []string{"ID", "NAME", "TYPE", "DESCRIPTION"}, [][]string{{
		strconv.FormatUint(uint64(value.ForestID), 10),
		value.Name,
		value.ForestType,
		value.Description,
	}})
}

func (a *app) kbListCommand() *cobra.Command {
	var offset, limit int
	command := &cobra.Command{
		Use:   "list",
		Short: "List knowledge bases",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if offset < 0 || limit < 0 || limit > api.MaxPageSize {
				return clierr.Usage("invalid_pagination", fmt.Sprintf("offset must not be negative and limit must be between 0 and %d", api.MaxPageSize))
			}
			active, err := a.loadActiveProfile(a.profileName)
			if err != nil {
				return err
			}
			var page api.ForestPage
			if err := active.Client.DoJSON(cmd.Context(), active.Credential.APIKey, "keapi.ListForest", map[string]any{
				"offset": offset,
				"limit":  limit,
			}, &page); err != nil {
				return clierr.Wrap("kb_list_failed", err)
			}
			return a.writeForestPage(page)
		},
	}
	command.Flags().IntVar(&offset, "offset", 0, "Starting offset")
	command.Flags().IntVar(&limit, "limit", 0, fmt.Sprintf("Maximum number of knowledge bases (up to %d)", api.MaxPageSize))
	return command
}

func (a *app) kbUseCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "use ID_OR_NAME",
		Short: "Set the default knowledge base for a profile",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			active, err := a.loadActiveProfile(a.profileName)
			if err != nil {
				return err
			}
			forests, err := a.listAllForests(cmd.Context(), active)
			if err != nil {
				return err
			}
			matches := make([]api.Forest, 0, 1)
			for _, forest := range forests {
				if strconv.FormatUint(uint64(forest.ForestID), 10) == args[0] || forest.Name == args[0] {
					matches = append(matches, forest)
				}
			}
			if len(matches) == 0 {
				return clierr.New("kb_not_found", fmt.Sprintf("knowledge base %q does not exist", args[0]))
			}
			if len(matches) > 1 {
				return clierr.New("kb_name_ambiguous", fmt.Sprintf("knowledge base name %q matches multiple records; use its ID", args[0]))
			}
			selected := matches[0]
			if err := saveKnowledgeBaseSelection(active, selected.ForestID, selected.Name); err != nil {
				return err
			}
			return a.writeOutput(kbSelectionOutput{
				Profile:           active.Name,
				KnowledgeBaseID:   strconv.FormatUint(uint64(selected.ForestID), 10),
				KnowledgeBaseName: selected.Name,
			})
		},
	}
}

func (a *app) listAllForests(ctx context.Context, active *activeProfile) ([]api.Forest, error) {
	forests := make([]api.Forest, 0)
	for offset := 0; ; offset += api.MaxPageSize {
		var page api.ForestPage
		if err := active.Client.DoJSON(ctx, active.Credential.APIKey, "keapi.ListForest", map[string]any{
			"offset": offset,
			"limit":  api.MaxPageSize,
		}, &page); err != nil {
			return nil, clierr.Wrap("kb_list_failed", err)
		}
		forests = append(forests, page.Data...)
		if len(page.Data) == 0 || int64(offset+len(page.Data)) >= page.Total || len(page.Data) < api.MaxPageSize {
			return forests, nil
		}
	}
}

func (a *app) writeForestPage(page api.ForestPage) error {
	format, err := a.format()
	if err != nil {
		return err
	}
	if format == "json" {
		return output.WriteJSON(a.out, page)
	}
	if format == "id" {
		names := make([]string, 0, len(page.Data))
		for _, forest := range page.Data {
			names = append(names, strconv.FormatUint(uint64(forest.ForestID), 10))
		}
		return output.WriteNames(a.out, names)
	}
	rows := make([][]string, 0, len(page.Data))
	for _, forest := range page.Data {
		rows = append(rows, []string{
			strconv.FormatUint(uint64(forest.ForestID), 10),
			forest.Name,
			forest.KnowledgeStatus,
			forest.Description,
			strconv.FormatInt(forest.FileCount, 10),
		})
	}
	if len(rows) == 0 {
		_, err := fmt.Fprintln(a.out, "No knowledge bases found.")
		return err
	}
	return output.WriteTable(a.out, []string{"ID", "NAME", "STATUS", "DESCRIPTION", "FILES"}, rows)
}
