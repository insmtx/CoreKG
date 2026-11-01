package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insmtx/corekg/internal/migrate"

	_ "github.com/go-sql-driver/mysql"
)

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Schema migration tooling",
	}

	cmd.AddCommand(newMigrateFromMySQLCmd())

	return cmd
}

func newMigrateFromMySQLCmd() *cobra.Command {
	var (
		dsn        string
		schemas    string
		out        string
		dryRun     bool
		apply      bool
		exactCount bool
	)

	cmd := &cobra.Command{
		Use:   "from-mysql",
		Short: "Generate a read-only MySQL to PostgreSQL migration plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			if apply || !dryRun {
				return errors.New("apply is not implemented in phase 0")
			}

			if dsn == "" {
				return errors.New("--dsn is required (or set CORekG_MYSQL_DSN)")
			}

			schemaList := splitSchemas(schemas)
			if len(schemaList) == 0 {
				return errors.New("--schemas must contain at least one schema name")
			}

			db, err := sql.Open("mysql", dsn)
			if err != nil {
				return fmt.Errorf("open mysql: %w", err)
			}
			defer db.Close()

			if err := db.PingContext(cmd.Context()); err != nil {
				return fmt.Errorf("ping mysql: %w", err)
			}

			plan, err := migrate.BuildPlan(cmd.Context(), db, schemaList, exactCount)
			if err != nil {
				return err
			}

			data, err := json.MarshalIndent(plan, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal plan: %w", err)
			}

			if out == "" {
				if _, err := os.Stdout.Write(append(data, '\n')); err != nil {
					return fmt.Errorf("write plan to stdout: %w", err)
				}

				return nil
			}

			if dir := filepath.Dir(out); dir != "" {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return fmt.Errorf("create output directory %q: %w", dir, err)
				}
			}

			if err := os.WriteFile(out, data, 0o644); err != nil {
				return fmt.Errorf("write plan to %q: %w", out, err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&dsn, "dsn", os.Getenv("CORekG_MYSQL_DSN"), "MySQL DSN to inspect")
	cmd.Flags().StringVar(&schemas, "schemas", "core,account,knownow,chat,sale", "comma-separated MySQL schemas to plan")
	cmd.Flags().StringVar(&out, "out", "", "write the plan to this file instead of stdout")
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "generate the plan without applying it")
	cmd.Flags().BoolVar(&apply, "apply", false, "apply the migration (not implemented in phase 0)")
	cmd.Flags().BoolVar(&exactCount, "exact-count", false, "count rows with SELECT COUNT(*) instead of information_schema estimates")

	return cmd
}

func splitSchemas(schemas string) []string {
	parts := strings.Split(schemas, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
