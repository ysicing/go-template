package app

// BuildInfo contains build-time metadata exposed by runtime and version endpoints.
type BuildInfo struct {
	Version   string
	GitCommit string
	BuildDate string
}
