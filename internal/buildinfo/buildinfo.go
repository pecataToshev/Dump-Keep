// Package buildinfo holds build metadata.
// These dev defaults are overwritten by the CI workflow before the
// Docker image is built and pushed.
package buildinfo

const (
	Commit    = "dev"
	BuildTime = "now"
	Source    = "local"
)
