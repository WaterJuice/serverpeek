// ---------------------------------------------------------------------------------------
//
//	main.go
//	-------
//
//	Entry point for the serverpeek binary.
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
// ---------------------------------------------------------------------------------------
package main

import "github.com/WaterJuice/serverpeek/internal"

// Version is set at build time via -ldflags
var Version = "dev"

// main is the entry point. It delegates to internal.Run with the build-time version string.
func main() {
	internal.Run(Version)
}
