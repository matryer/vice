//go:build tools

// Package tools is a stub for development tooling to be tracked by Gomod
package tools

import (
	// Static analysis tools
	_ "golang.org/x/lint/golint"
	_ "honnef.co/go/tools/cmd/staticcheck"
)
