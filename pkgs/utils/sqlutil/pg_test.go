package sqlutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContainsCSV(t *testing.T) {
	require.Equal(t, "?::text = ANY(string_to_array(f.manager_ids, ','))", ContainsCSV("f.manager_ids"))
	require.Equal(t, "?::text = ANY(string_to_array(forest.manager_ids, ','))", ContainsCSV("forest.manager_ids"))
}
