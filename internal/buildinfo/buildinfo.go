package buildinfo

import "runtime"

// Info captures release/build metadata for the current binary.
type Info struct {
	Version    string `json:"version"`
	Commit     string `json:"commit"`
	BuildDate  string `json:"build_date"`
	TargetOS   string `json:"target_os"`
	TargetArch string `json:"target_arch"`
	GoVersion  string `json:"go_version"`
}

// Normalize fills empty build metadata with stable defaults.
func Normalize(info Info) Info {
	if info.Version == "" {
		info.Version = "0.1.0-dev"
	}
	if info.Commit == "" {
		info.Commit = "unknown"
	}
	if info.BuildDate == "" {
		info.BuildDate = "unknown"
	}
	if info.TargetOS == "" {
		info.TargetOS = runtime.GOOS
	}
	if info.TargetArch == "" {
		info.TargetArch = runtime.GOARCH
	}
	if info.GoVersion == "" {
		info.GoVersion = runtime.Version()
	}
	return info
}
