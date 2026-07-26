package main

import "fmt"

// version is overwritten at build time via
// `-ldflags "-X main.version=..."` (see .github/workflows/release.yml) —
// "dev" is what a plain local `go build` produces.
var version = "dev"

func runVersionCommand() {
	fmt.Println("nullsec-server", version)
}
