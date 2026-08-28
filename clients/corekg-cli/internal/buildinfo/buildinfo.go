package buildinfo

// Build values are replaced by the release build through -ldflags.
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuiltAt   = "unknown"
)

type Info struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuiltAt   string `json:"built_at"`
}
