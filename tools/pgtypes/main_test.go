package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRewriteTag(t *testing.T) {
	cases := map[string]string{
		"type:tinyint(1);default:1":      "type:smallint;default:1",
		"type:tinyint;not null":          "type:smallint;not null",
		"type:tinyint unsigned;not null": "type:smallint;not null",
		"type:bigint unsigned;not null":  "type:bigint;not null",
		"type:int unsigned;not null":     "type:bigint;not null",
		"type:longtext;comment:x":        "type:text;comment:x",
		"type:mediumtext;comment:x":      "type:text;comment:x",
		"type:datetime(3);not null":      "type:timestamptz;not null",
		"type:datetime;not null":         "type:timestamptz;not null",
		"type:int(11);not null":          "type:integer;not null",
		"type:uint;not null":             "type:bigint;not null",
		"type:TINYINT;not null":          "type:smallint;not null",
		"type:datetime(3);default:CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)": "type:timestamptz;default:CURRENT_TIMESTAMP(3)",
	}
	for in, want := range cases {
		require.Equal(t, want, RewriteTag(in))
		require.Equal(t, want, RewriteTag(want)) // idempotent
	}
}

func TestRewriteFileLeavesRawSQL(t *testing.T) {
	src := []byte("const ddl = \"BIGINT UNSIGNED NOT NULL\"\n" +
		"var x struct{ ID uint `gorm:\"type:bigint unsigned;not null\"` }\n")
	out, changed := RewriteFile(src)
	require.True(t, changed)
	require.Contains(t, string(out), "BIGINT UNSIGNED NOT NULL")
	require.Contains(t, string(out), "type:bigint;not null")
}

func TestRewriteFileSkipsCommentLines(t *testing.T) {
	src := []byte("// gorm:\"column:xxxx;type:TINYINT;default:-1\"\n" +
		"var x struct{ Flag int32 `gorm:\"column:flag;type:TINYINT\"` }\n")
	out, changed := RewriteFile(src)
	require.True(t, changed)
	require.Contains(t, string(out), "// gorm:\"column:xxxx;type:TINYINT;default:-1\"")
	require.Contains(t, string(out), "column:flag;type:smallint")
}

func TestRewriteFileIdempotent(t *testing.T) {
	src := []byte("var x struct{ ID uint `gorm:\"type:bigint unsigned;not null\"` }\n" +
		"var y struct{ At time.Time `gorm:\"type:datetime(3);default:CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)\"` }\n")
	out, changed := RewriteFile(src)
	require.True(t, changed)
	out2, changed2 := RewriteFile(out)
	require.False(t, changed2)
	require.Equal(t, out, out2)
}
