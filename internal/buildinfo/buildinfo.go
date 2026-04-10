package buildinfo

import "fmt"

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

type Info struct {
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	BuildTime   string `json:"build_time"`
	FullVersion string `json:"full_version"`
}

func Current() Info {
	return Info{
		Version:     Version,
		Commit:      Commit,
		BuildTime:   BuildTime,
		FullVersion: FullVersion(),
	}
}

func FullVersion() string {
	return fmt.Sprintf("%s-%s-%s", Version, Commit, BuildTime)
}
