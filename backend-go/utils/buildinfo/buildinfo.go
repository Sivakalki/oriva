// Package buildinfo exposes build metadata injected at link time via -ldflags.
package buildinfo

// These are overridden by the Makefile build target:
//
//	-X 'oriva/backend-go/utils/buildinfo.Version=...'
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
