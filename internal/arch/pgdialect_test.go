package arch

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	baselineFindInSet   = 0
	baselineRegexp      = 0
	baselineMySQLTypTag = 218
)

var typeTagSkipDirs = []string{"workflow", "testdata", "docs", "mysqlplugin", "svcexcelforest", "excelparser"}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "go.mod not found above %s", file)
		dir = parent
	}
}

func walkGoLines(t *testing.T, roots []string, skipDirs []string, visit func(line string)) {
	t.Helper()
	root := moduleRoot(t)
	skip := make(map[string]struct{}, len(skipDirs))
	for _, d := range skipDirs {
		skip[d] = struct{}{}
	}
	for _, r := range roots {
		err := filepath.WalkDir(filepath.Join(root, r), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if _, ok := skip[d.Name()]; ok {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".go" {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, line := range strings.Split(string(content), "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "//") {
					continue
				}
				visit(line)
			}
			return nil
		})
		require.NoError(t, err, "walk root %s", r)
	}
}

func countPattern(t *testing.T, roots []string, pattern string, skipDirs []string) int {
	t.Helper()
	re := regexp.MustCompile(pattern)
	total := 0
	walkGoLines(t, roots, skipDirs, func(line string) {
		if len(re.FindAllString(line, -1)) > 0 {
			total++
		}
	})
	return total
}

func countMySQLTypeTags(t *testing.T) int {
	t.Helper()
	gormTagRe := regexp.MustCompile(`gorm:"([^"]*)"`)
	mysqlTypeTagRe := regexp.MustCompile(`type:(bigint unsigned|int unsigned|tinyint\(1\)|tinyint unsigned|tinyint|longtext|mediumtext|datetime\(3\)|datetime|int\(11\)|uint|TINYINT)([;"]|$)`)
	total := 0
	walkGoLines(t, []string{"apps", "pkgs"}, typeTagSkipDirs, func(line string) {
		for _, m := range gormTagRe.FindAllStringSubmatch(line, -1) {
			total += len(mysqlTypeTagRe.FindAllString(m[1], -1))
		}
	})
	return total
}

func TestPGDialectRatchet(t *testing.T) {
	findInSet := countPattern(t,
		[]string{"apps/kecore/models/forest", "apps/kesearch/models/globalsearch"},
		`FIND_IN_SET\(`, []string{"workflow", "testdata", "docs"})
	regexpCount := countPattern(t,
		[]string{"apps/kesearch/models/globalsearch"},
		`\bREGEXP\b`, []string{"workflow", "testdata", "docs"})
	typeTags := countMySQLTypeTags(t)
	t.Logf("FIND_IN_SET=%d REGEXP=%d typeTags=%d", findInSet, regexpCount, typeTags)
	require.LessOrEqual(t, findInSet, baselineFindInSet)
	require.LessOrEqual(t, regexpCount, baselineRegexp)
	require.LessOrEqual(t, typeTags, baselineMySQLTypTag)
}
