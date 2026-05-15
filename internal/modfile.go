package internal

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/mod/modfile"
)

// ReadGoVersion returns the bare go directive version from a go.mod file
// (e.g. "1.22.3"). Returns "" if no go directive is present.
func ReadGoVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	f, err := modfile.Parse(path, data, nil)
	if err != nil {
		return "", fmt.Errorf("parsing %s: %w", path, err)
	}
	if f.Go == nil {
		return "", nil
	}
	return f.Go.Version, nil
}

// WriteGoVersion updates the go directive in a go.mod file, preserving all
// other content. version may include a "go" prefix (e.g. "go1.22.3"); it will
// be stripped automatically.
func WriteGoVersion(path, version string) error {
	version = strings.TrimPrefix(version, "go")

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	f, err := modfile.Parse(path, data, nil)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	if err := f.AddGoStmt(version); err != nil {
		return fmt.Errorf("updating go directive: %w", err)
	}
	return os.WriteFile(path, modfile.Format(f.Syntax), 0o644)
}
