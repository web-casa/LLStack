package main

import (
	"os"

	"github.com/web-casa/llstack/internal/app"
	"github.com/web-casa/llstack/internal/buildinfo"
)

var (
	version    = "0.1.0-dev"
	commit     = "unknown"
	buildDate  = "unknown"
	targetOS   = ""
	targetArch = ""
)

func main() {
	application := app.New(app.Options{
		Build: buildinfo.Info{
			Version:    version,
			Commit:     commit,
			BuildDate:  buildDate,
			TargetOS:   targetOS,
			TargetArch: targetArch,
		},
	})

	if err := application.Run(os.Args[1:]); err != nil {
		os.Exit(1)
	}
}
