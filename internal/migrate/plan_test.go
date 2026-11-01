package migrate

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func TestMapType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"tinyint with width", "tinyint(1)", "smallint"},
		{"tinyint plain", "tinyint", "smallint"},
		{"tinyint unsigned", "tinyint(1) unsigned", "smallint"},
		{"tinyint unsigned no width", "tinyint unsigned", "smallint"},
		{"smallint plain", "smallint", "smallint"},
		{"smallint with width", "smallint(5)", "smallint"},
		{"smallint unsigned", "smallint unsigned", "integer"},
		{"smallint width unsigned", "smallint(5) unsigned", "integer"},
		{"int plain", "int", "integer"},
		{"int with width", "int(11)", "integer"},
		{"int unsigned", "int unsigned", "bigint"},
		{"int width unsigned", "int(10) unsigned", "bigint"},
		{"mediumint plain", "mediumint", "integer"},
		{"mediumint unsigned", "mediumint(8) unsigned", "bigint"},
		{"bigint plain", "bigint", "bigint"},
		{"bigint with width", "bigint(20)", "bigint"},
		{"bigint unsigned", "bigint(20) unsigned", "bigint"},
		{"longtext", "longtext", "text"},
		{"mediumtext", "mediumtext", "text"},
		{"text", "text", "text"},
		{"tinytext", "tinytext", "text"},
		{"datetime", "datetime", "timestamptz"},
		{"datetime with precision", "datetime(3)", "timestamptz"},
		{"timestamp", "timestamp", "timestamptz"},
		{"timestamp with precision", "timestamp(3)", "timestamptz"},
		{"date", "date", "date"},
		{"time", "time", "time"},
		{"time with precision", "time(6)", "time"},
		{"year", "year", "smallint"},
		{"year with width", "year(4)", "smallint"},
		{"json", "json", "jsonb"},
		{"blob", "blob", "bytea"},
		{"longblob", "longblob", "bytea"},
		{"mediumblob", "mediumblob", "bytea"},
		{"tinyblob", "tinyblob", "bytea"},
		{"binary", "binary(16)", "bytea"},
		{"varbinary", "varbinary(16)", "bytea"},
		{"double", "double", "double precision"},
		{"double with params", "double(10,2)", "double precision"},
		{"float", "float", "real"},
		{"float with params", "float(5,2)", "real"},
		{"decimal with params", "decimal(10,2)", "numeric(10,2)"},
		{"decimal plain", "decimal", "numeric"},
		{"varchar", "varchar(255)", "varchar(255)"},
		{"varchar case normalization", "VARCHAR(64)", "varchar(64)"},
		{"char", "char(36)", "char(36)"},
		{"enum", "enum('a','b')", "text"},
		{"set", "set('x')", "text"},
		{"unknown geometry", "geometry", ""},
		{"unknown point", "point", ""},
		{"empty", "", ""},
		{"surrounding whitespace", "  text  ", "text"},
		{"uppercase with unsigned", "BIGINT(20) UNSIGNED", "bigint"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MapType(tt.input); got != tt.want {
				t.Errorf("MapType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestQuoteIdent(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"core", "`core`"},
		{"user_order", "`user_order`"},
		{"weird`name", "`weird``name`"},
	}

	for _, tt := range tests {
		if got := quoteIdent(tt.input); got != tt.want {
			t.Errorf("quoteIdent(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildPlanIntegration(t *testing.T) {
	dsn := os.Getenv("CORekG_MYSQL_DSN")
	if dsn == "" {
		t.Skip("CORekG_MYSQL_DSN not set")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping mysql: %v", err)
	}

	plan, err := BuildPlan(ctx, db, []string{"core", "account", "knownow", "chat", "sale"}, false)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}

	if len(plan.Schemas) < 5 {
		t.Fatalf("expected at least 5 schemas in plan, got %d", len(plan.Schemas))
	}

	for _, schema := range plan.Schemas {
		for _, table := range schema.Tables {
			for _, column := range table.Columns {
				if column.PGType == "" {
					t.Errorf("schema %q table %q column %q: empty PGType for MySQL type %q",
						schema.Name, table.Name, column.Name, column.MySQLType)
				}
			}
		}
	}
}
