package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

type Format string

const (
	FormatTable = Format("table")
	FormatJSON  = Format("json")
	FormatID    = Format("id")
	FormatName  = FormatID
)

func ParseFormat(value string) (Format, error) {
	format := Format(strings.ToLower(strings.TrimSpace(value)))
	if format != FormatTable && format != FormatJSON && format != FormatID {
		return "", fmt.Errorf("unsupported output format %q; use table, json, or id", value)
	}
	return format, nil
}

func WriteJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(map[string]any{"ok": true, "data": value})
}

func WriteNames(w io.Writer, names []string) error {
	for _, name := range names {
		if _, err := fmt.Fprintln(w, name); err != nil {
			return err
		}
	}
	return nil
}

func WriteTable(w io.Writer, headers []string, rows [][]string) error {
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, strings.Join(headers, "\t")); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintln(table, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return table.Flush()
}
