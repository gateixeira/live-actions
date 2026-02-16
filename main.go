package main

import (
	"embed"
	"fmt"
	"runtime"

	"github.com/gateixeira/live-actions/cmd/server"
)

var (
	// These will be set by build flags
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

//go:embed frontend/dist
var staticFS embed.FS

//go:embed config
var configFS embed.FS

func main() {
	fmt.Printf("Live Actions %s (commit: %s, built: %s)\n", version, commit, date)
	fmt.Printf("Go version: %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	server.SetupAndRun(staticFS, configFS)
}
