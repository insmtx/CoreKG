package arch

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	appsPrefix   = "github.com/insmtx/corekg/apps/"
	infraPrefix  = "github.com/insmtx/corekg/internal/infra/"
	apiPkg       = "github.com/insmtx/corekg/internal/api"
	apiPrefix    = "github.com/insmtx/corekg/internal/api/"
	bootstrapPkg = "github.com/insmtx/corekg/internal/bootstrap"
)

func listImports(t *testing.T, pkg string) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-f",
		`{{range .Imports}}{{println .}}{{end}}`, pkg).Output()
	require.NoError(t, err)
	return strings.Fields(string(out))
}

func TestConfigDoesNotImportApps(t *testing.T) {
	for _, imp := range listImports(t, "github.com/insmtx/corekg/internal/config") {
		require.False(t, strings.HasPrefix(imp, appsPrefix),
			"internal/config must not import legacy apps: "+imp)
	}
}

func TestConfigDoesNotImportAPIOrBootstrap(t *testing.T) {
	for _, imp := range listImports(t, "github.com/insmtx/corekg/internal/config") {
		require.False(t, imp == apiPkg || strings.HasPrefix(imp, apiPrefix),
			"internal/config must not import internal/api: "+imp)
		require.False(t, imp == bootstrapPkg,
			"internal/config must not import internal/bootstrap: "+imp)
	}
}

func TestAPIDoesNotImportAppsOrInfra(t *testing.T) {
	for _, imp := range listImports(t, "github.com/insmtx/corekg/internal/api/...") {
		require.False(t, strings.HasPrefix(imp, appsPrefix),
			"internal/api must not import legacy apps: "+imp)
		require.False(t, strings.HasPrefix(imp, infraPrefix),
			"internal/api must not import internal/infra: "+imp)
	}
}
