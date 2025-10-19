// mtg is just a command-line application that starts a proxy.
//
// Application logic is how to read a config and configure mtglib.Proxy.
// So, probably you need to read the documentation for mtglib package
// first.
//
// mtglib is a core of the application. The rest of the packages provide
// some default implementations for the interfaces, defined in mtglib.
package main

import (
	"runtime"

	"github.com/alecthomas/kong"
	"github.com/dewadaru/mtgo/internal/cli"
)

func init() {
	// Optimize for multi-core systems
	// Use all available CPU cores for better concurrency
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	// Note: rand.Seed() is deprecated in Go 1.20+
	// The global random number generator is automatically seeded since Go 1.20
	// No manual seeding required for Go 1.25.3

	// Allocate CLI instance on stack for better memory locality
	cliInstance := cli.CLI{}
	currentVersion := getVersion()

	// Parse command line arguments with version info
	ctx := kong.Parse(&cliInstance, kong.Vars{
		"version": currentVersion,
	})

	// Execute and handle errors
	ctx.FatalIfErrorf(ctx.Run(&cliInstance, currentVersion))
}
