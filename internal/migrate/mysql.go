package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type tableInfo struct {
	Name      string
	TableRows int64
}

func listTables(ctx context.Context, db *sql.DB, schema string) ([]tableInfo, error) {
	const query = `SELECT table_name, table_rows FROM information_schema.tables WHERE table_schema = ? AND table_type = 'BASE TABLE' ORDER BY table_name`

	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("query tables for schema %q: %w", schema, err)
	}
	defer rows.Close()

	tables := make([]tableInfo, 0)

	for rows.Next() {
		var name string

		var tableRows sql.NullInt64

		if err := rows.Scan(&name, &tableRows); err != nil {
			return nil, fmt.Errorf("scan table row in schema %q: %w", schema, err)
		}

		tables = append(tables, tableInfo{Name: name, TableRows: tableRows.Int64})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tables for schema %q: %w", schema, err)
	}

	return tables, nil
}

func listColumns(ctx context.Context, db *sql.DB, schema, table string) ([]ColumnPlan, []string, error) {
	const query = `SELECT column_name, column_type, is_nullable, column_default, extra FROM information_schema.columns WHERE table_schema = ? AND table_name = ? ORDER BY ordinal_position`

	rows, err := db.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, nil, fmt.Errorf("query columns for table %q.%q: %w", schema, table, err)
	}
	defer rows.Close()

	columns := make([]ColumnPlan, 0)

	var warnings []string

	for rows.Next() {
		var name, columnType, isNullable string

		var columnDefault, extra sql.NullString

		if err := rows.Scan(&name, &columnType, &isNullable, &columnDefault, &extra); err != nil {
			return nil, nil, fmt.Errorf("scan column in table %q.%q: %w", schema, table, err)
		}

		pgType := MapType(columnType)

		switch {
		case pgType == "":
			warnings = append(warnings, fmt.Sprintf("column %s: unmapped MySQL type %q", name, columnType))
		case isBigintUnsigned(columnType):
			warnings = append(warnings, fmt.Sprintf(
				"column %s: bigint unsigned mapped to bigint; verify max value <= 9223372036854775807 before migration", name))
		}

		if isEnumOrSet(columnType) {
			warnings = append(warnings, fmt.Sprintf("column %s: %s mapped to text; verify allowed values", name, columnType))
		}

		columns = append(columns, ColumnPlan{
			Name:      name,
			MySQLType: columnType,
			PGType:    pgType,
			Nullable:  strings.EqualFold(isNullable, "YES"),
			Default:   columnDefault.String,
			Extra:     extra.String,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate columns for table %q.%q: %w", schema, table, err)
	}

	return columns, warnings, nil
}

func countRows(ctx context.Context, db *sql.DB, schema, table string) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", quoteIdent(schema), quoteIdent(table))

	var count int64

	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("count rows in table %q.%q: %w", schema, table, err)
	}

	return count, nil
}

func quoteIdent(ident string) string {
	return "`" + strings.ReplaceAll(ident, "`", "``") + "`"
}
