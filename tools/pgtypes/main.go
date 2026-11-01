package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var typeMappings = [][2]string{
	{"type:bigint unsigned", "type:bigint"},
	{"type:int unsigned", "type:bigint"},
	{"type:tinyint(1)", "type:smallint"},
	{"type:tinyint unsigned", "type:smallint"},
	{"type:tinyint", "type:smallint"},
	{"type:longtext", "type:text"},
	{"type:mediumtext", "type:text"},
	{"type:datetime(3)", "type:timestamptz"},
	{"type:datetime", "type:timestamptz"},
	{"type:int(11)", "type:integer"},
	{"type:uint", "type:bigint"},
	{"type:TINYINT", "type:smallint"},
}

var (
	gormTagRe  = regexp.MustCompile(`gorm:"([^"]*)"`)
	onUpdateRe = regexp.MustCompile(`\s+ON UPDATE CURRENT_TIMESTAMP(\(3\))?`)
)

var skipDirs = map[string]struct{}{
	"workflow":       {},
	"testdata":       {},
	"docs":           {},
	"mysqlplugin":    {},
	"svcexcelforest": {},
	"excelparser":    {},
}

// RewriteTag normalizes MySQL-specific GORM type tags to PostgreSQL equivalents.
func RewriteTag(tag string) string {
	for _, m := range typeMappings {
		tag = strings.ReplaceAll(tag, m[0], m[1])
	}
	return onUpdateRe.ReplaceAllString(tag, "")
}

// RewriteFile rewrites GORM struct tags in src, skipping comment lines.
func RewriteFile(src []byte) ([]byte, bool) {
	lines := strings.Split(string(src), "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		lines[i] = gormTagRe.ReplaceAllStringFunc(line, func(m string) string {
			sub := gormTagRe.FindStringSubmatch(m)
			return `gorm:"` + RewriteTag(sub[1]) + `"`
		})
	}
	out := []byte(strings.Join(lines, "\n"))
	return out, !bytes.Equal(out, src)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}
	changed := 0
	for _, r := range []string{"apps", "pkgs"} {
		err := filepath.WalkDir(filepath.Join(root, r), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if _, ok := skipDirs[d.Name()]; ok {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".go" {
				return nil
			}
			rewritten, err := rewriteGoFile(path, d)
			if err != nil {
				return err
			}
			if rewritten {
				changed++
				rel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					rel = path
				}
				fmt.Println(rel)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	fmt.Printf("changed %d files\n", changed)
	return nil
}

func rewriteGoFile(path string, d fs.DirEntry) (bool, error) {
	info, err := d.Info()
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	out, changed := RewriteFile(src)
	if !changed {
		return false, nil
	}
	formatted, err := format.Source(out)
	if err != nil {
		return false, fmt.Errorf("format %s: %w", path, err)
	}
	if err := os.WriteFile(path, formatted, info.Mode()); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found at or above %s", dir)
		}
		dir = parent
	}
}
