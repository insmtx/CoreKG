package sqlutil

import "fmt"

// ContainsCSV returns a PostgreSQL predicate for a comma-separated column.
func ContainsCSV(column string) string {
	return fmt.Sprintf("?::text = ANY(string_to_array(%s, ','))", column)
}
