package migrate

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// Plan is a read-only dry-run migration plan for one or more MySQL schemas.
type Plan struct {
	GeneratedAt time.Time    `json:"generated_at"`
	Schemas     []SchemaPlan `json:"schemas"`
}

// SchemaPlan describes the planned migration of a single MySQL schema.
type SchemaPlan struct {
	Name   string      `json:"name"`
	Tables []TablePlan `json:"tables"`
}

// TablePlan describes one table's estimated row count, columns and warnings.
type TablePlan struct {
	Name     string       `json:"name"`
	RowCount int64        `json:"row_count"`
	Columns  []ColumnPlan `json:"columns"`
	Warnings []string     `json:"warnings,omitempty"`
}

// ColumnPlan describes one column's MySQL type and its PostgreSQL mapping.
type ColumnPlan struct {
	Name      string `json:"name"`
	MySQLType string `json:"mysql_type"`
	PGType    string `json:"pg_type"`
	Nullable  bool   `json:"nullable"`
	Default   string `json:"default,omitempty"`
	Extra     string `json:"extra,omitempty"`
}

// MapType maps a MySQL column type to its PostgreSQL equivalent. It returns an
// empty string for types that have no known mapping.
func MapType(mysqlType string) string {
	name, params, unsigned := splitType(mysqlType)

	switch name {
	case "tinyint":
		return "smallint"
	case "smallint":
		if unsigned {
			return "integer"
		}

		return "smallint"
	case "int", "integer", "mediumint":
		if unsigned {
			return "bigint"
		}

		return "integer"
	case "bigint":
		return "bigint"
	case "longtext", "mediumtext", "text", "tinytext":
		return "text"
	case "datetime", "timestamp":
		return "timestamptz"
	case "date":
		return "date"
	case "time":
		return "time"
	case "year":
		return "smallint"
	case "json":
		return "jsonb"
	case "blob", "longblob", "mediumblob", "tinyblob", "binary", "varbinary":
		return "bytea"
	case "double":
		return "double precision"
	case "float":
		return "real"
	case "decimal":
		if params != "" {
			return "numeric" + params
		}

		return "numeric"
	case "varchar", "char":
		return name + params
	case "enum", "set":
		return "text"
	default:
		return ""
	}
}

// BuildPlan inspects the given MySQL schemas through information_schema and
// returns a migration plan. It performs no writes against the database.
func BuildPlan(ctx context.Context, db *sql.DB, schemas []string, exactCount bool) (*Plan, error) {
	plan := &Plan{
		GeneratedAt: time.Now().UTC(),
		Schemas:     make([]SchemaPlan, 0, len(schemas)),
	}

	for _, schema := range schemas {
		tables, err := listTables(ctx, db, schema)
		if err != nil {
			return nil, err
		}

		schemaPlan := SchemaPlan{
			Name:   schema,
			Tables: make([]TablePlan, 0, len(tables)),
		}

		for _, table := range tables {
			tablePlan, err := buildTablePlan(ctx, db, schema, table, exactCount)
			if err != nil {
				return nil, err
			}

			schemaPlan.Tables = append(schemaPlan.Tables, *tablePlan)
		}

		plan.Schemas = append(plan.Schemas, schemaPlan)
	}

	return plan, nil
}

func buildTablePlan(ctx context.Context, db *sql.DB, schema string, table tableInfo, exactCount bool) (*TablePlan, error) {
	rowCount := table.TableRows

	if exactCount {
		count, err := countRows(ctx, db, schema, table.Name)
		if err != nil {
			return nil, err
		}

		rowCount = count
	}

	columns, warnings, err := listColumns(ctx, db, schema, table.Name)
	if err != nil {
		return nil, err
	}

	return &TablePlan{
		Name:     table.Name,
		RowCount: rowCount,
		Columns:  columns,
		Warnings: warnings,
	}, nil
}

func splitType(mysqlType string) (name, params string, unsigned bool) {
	t := strings.ToLower(strings.TrimSpace(mysqlType))

	if trimmed, found := strings.CutSuffix(t, " unsigned"); found {
		unsigned = true
		t = strings.TrimSpace(trimmed)
	}

	name = t

	if idx := strings.Index(t, "("); idx >= 0 {
		name = strings.TrimSpace(t[:idx])
		params = t[idx:]
	}

	return name, params, unsigned
}

func isBigintUnsigned(mysqlType string) bool {
	name, _, unsigned := splitType(mysqlType)

	return unsigned && name == "bigint"
}

func isEnumOrSet(mysqlType string) bool {
	name, _, _ := splitType(mysqlType)

	return name == "enum" || name == "set"
}
