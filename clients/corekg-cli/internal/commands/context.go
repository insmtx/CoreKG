package commands

import (
	"fmt"
	"sort"

	"github.com/insmtx/corekg/clients/corekg-cli/internal/clierr"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/output"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/store"
	"github.com/spf13/cobra"
)

type profileRow struct {
	Name             string `json:"name"`
	Current          bool   `json:"current"`
	Effective        bool   `json:"effective,omitempty"`
	ServerURL        string `json:"server"`
	Credential       string `json:"credential"`
	OrganizationName string `json:"organization_name,omitempty"`
	KnowledgeBase    string `json:"knowledge_base,omitempty"`
}

type profileOutput struct {
	Name              string `json:"name"`
	Current           bool   `json:"current"`
	Effective         bool   `json:"effective,omitempty"`
	ServerURL         string `json:"server"`
	Credential        string `json:"credential"`
	OrganizationID    string `json:"organization_id,omitempty"`
	OrganizationName  string `json:"organization_name,omitempty"`
	KnowledgeBaseID   string `json:"knowledge_base_id,omitempty"`
	KnowledgeBaseName string `json:"knowledge_base_name,omitempty"`
}

func (a *app) profileCommand() *cobra.Command {
	command := &cobra.Command{Use: "profile", Short: "Manage CoreKG server, account and organization profiles"}
	command.AddCommand(a.profileListCommand())
	command.AddCommand(a.profileShowCommand())
	command.AddCommand(a.profileUseCommand())
	command.AddCommand(a.profileRenameCommand())
	command.AddCommand(a.profileDeleteCommand())
	return command
}

func (a *app) profileShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show a profile without revealing its API Key",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := a.resolvePaths()
			if err != nil {
				return err
			}
			settings, err := store.LoadState(paths)
			if err != nil {
				return err
			}
			name := a.selectedProfile()
			if name == "" {
				name = settings.CurrentProfile
			}
			if name == "" {
				return clierr.New("profile_not_found", "no active profile")
			}
			definition, ok := settings.Profiles[name]
			if !ok {
				return clierr.New("profile_not_found", fmt.Sprintf("profile %q does not exist", name))
			}
			value := profileOutput{
				Name:              name,
				Current:           name == settings.CurrentProfile,
				Effective:         name == a.selectedProfile() && a.selectedProfile() != "",
				ServerURL:         definition.ServerURL,
				Credential:        definition.Credential,
				OrganizationID:    definition.OrganizationID,
				OrganizationName:  definition.OrganizationName,
				KnowledgeBaseID:   definition.KnowledgeBaseID,
				KnowledgeBaseName: definition.KnowledgeBaseName,
			}
			format, err := a.format()
			if err != nil {
				return err
			}
			switch format {
			case "json":
				return output.WriteJSON(a.out, value)
			case "id":
				return output.WriteNames(a.out, []string{value.Name})
			default:
				return output.WriteTable(a.out, []string{"NAME", "CURRENT", "EFFECTIVE", "SERVER", "CREDENTIAL", "ORGANIZATION", "KNOWLEDGE BASE"}, [][]string{{value.Name, fmt.Sprint(value.Current), fmt.Sprint(value.Effective), value.ServerURL, value.Credential, value.OrganizationName, value.KnowledgeBaseName}})
			}
		},
	}
}

func (a *app) profileListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured profiles",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := a.resolvePaths()
			if err != nil {
				return err
			}
			settings, err := store.LoadState(paths)
			if err != nil {
				return err
			}
			rows := profileRows(settings, a.selectedProfile())
			format, err := a.format()
			if err != nil {
				return err
			}
			switch format {
			case "json":
				return output.WriteJSON(a.out, rows)
			case "id":
				names := make([]string, 0, len(rows))
				for _, row := range rows {
					names = append(names, row.Name)
				}
				return output.WriteNames(a.out, names)
			default:
				if len(rows) == 0 {
					_, err := fmt.Fprintln(a.out, "No profiles configured.")
					return err
				}
				tableRows := make([][]string, 0, len(rows))
				for _, row := range rows {
					current, effective := "", ""
					if row.Current {
						current = "*"
					}
					if row.Effective {
						effective = "*"
					}
					tableRows = append(tableRows, []string{current, effective, row.Name, row.ServerURL, row.Credential, row.OrganizationName, row.KnowledgeBase})
				}
				return output.WriteTable(a.out, []string{"CURRENT", "EFFECTIVE", "NAME", "SERVER", "CREDENTIAL", "ORGANIZATION", "KNOWLEDGE BASE"}, tableRows)
			}
		},
	}
}

func (a *app) profileUseCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "use NAME",
		Short: "Set the active profile; use '-' to switch back",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := a.resolvePaths()
			if err != nil {
				return err
			}
			if err := store.WithLock(paths, func() error {
				settings, loadErr := store.LoadState(paths)
				if loadErr != nil {
					return loadErr
				}
				name := args[0]
				if name == "-" {
					name = settings.PreviousProfile
					if name == "" {
						return clierr.New("profile_not_found", "no previous profile to switch back to")
					}
				}
				if err := store.SetCurrentProfile(&settings, name); err != nil {
					return clierr.New("profile_not_found", err.Error())
				}
				return store.SaveState(paths, settings)
			}); err != nil {
				return err
			}
			name := args[0]
			if name == "-" {
				latest, loadErr := store.LoadState(paths)
				if loadErr != nil {
					return loadErr
				}
				name = latest.CurrentProfile
			}
			if format, formatErr := a.format(); formatErr != nil {
				return formatErr
			} else if format == "json" {
				return output.WriteJSON(a.out, map[string]string{"current_profile": name})
			} else if format == "id" {
				return output.WriteNames(a.out, []string{name})
			}
			_, err = fmt.Fprintf(a.out, "Switched to profile %q.\n", name)
			return err
		},
	}
}

func (a *app) profileRenameCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rename OLD_NAME NEW_NAME",
		Short: "Rename a profile",
		Args:  exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := a.resolvePaths()
			if err != nil {
				return err
			}
			if err := store.WithLock(paths, func() error {
				settings, loadErr := store.LoadState(paths)
				if loadErr != nil {
					return loadErr
				}
				if renameErr := store.RenameProfile(&settings, args[0], args[1]); renameErr != nil {
					return clierr.New("profile_rename_failed", renameErr.Error())
				}
				return store.SaveState(paths, settings)
			}); err != nil {
				return err
			}
			return a.writeOutput(map[string]string{"old_name": args[0], "new_name": args[1], "status": "renamed"})
		},
	}
}

func (a *app) profileDeleteCommand() *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a local profile",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return clierr.Confirm("confirmation_required", "profile delete requires --yes")
			}
			paths, err := a.resolvePaths()
			if err != nil {
				return err
			}
			if err := store.WithLock(paths, func() error {
				settings, loadErr := store.LoadState(paths)
				if loadErr != nil {
					return loadErr
				}
				definition, deleteErr := store.DeleteProfile(&settings, args[0])
				if deleteErr != nil {
					return clierr.Wrap("profile_delete_failed", deleteErr)
				}
				auth, authErr := store.LoadAuth(paths)
				if authErr != nil {
					return authErr
				}
				stillReferenced := false
				for _, profile := range settings.Profiles {
					if profile.Credential == definition.Credential {
						stillReferenced = true
						break
					}
				}
				if saveErr := store.SaveState(paths, settings); saveErr != nil {
					return saveErr
				}
				if !stillReferenced {
					delete(auth.Credentials, definition.Credential)
					return store.SaveAuth(paths, auth)
				}
				return nil
			}); err != nil {
				return err
			}
			return a.writeOutput(map[string]string{"profile": args[0], "status": "deleted"})
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "Confirm deleting the profile")
	return command
}

func profileRows(settings store.State, effectiveName string) []profileRow {
	settings = settings.Normalize()
	names := store.ProfileNames(settings)
	rows := make([]profileRow, 0, len(names))
	for _, name := range names {
		profile := settings.Profiles[name]
		rows = append(rows, profileRow{
			Name:             name,
			Current:          name == settings.CurrentProfile,
			Effective:        name == effectiveName && effectiveName != "",
			ServerURL:        profile.ServerURL,
			Credential:       profile.Credential,
			OrganizationName: profile.OrganizationName,
			KnowledgeBase:    profile.KnowledgeBaseName,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Effective != rows[j].Effective {
			return rows[i].Effective
		}
		if rows[i].Current != rows[j].Current {
			return rows[i].Current
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}
