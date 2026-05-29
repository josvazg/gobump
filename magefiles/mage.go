//go:build mage

package main

import (
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Build compiles gobump into ./gobump.
func Build() error {
	return sh.Run("go", "build", "-o", "gobump", ".")
}

// Test runs the full test suite.
func Test() error {
	return sh.Run("go", "test", "./...")
}

// Lint runs golangci-lint.
func Lint() error {
	return sh.Run("golangci-lint", "run", "./...")
}

// Install installs gobump to GOPATH/bin.
func Install() error {
	return sh.Run("go", "install", ".")
}

// CI runs the full pipeline: Build, Test, and Lint in sequence.
func CI() {
	mg.SerialDeps(Build, Test, Lint)
}
