package internal

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FindModFiles returns the path of every go.mod found under root.
//
// If root ends with "/..." (e.g. "./..." or "sub/...") the search recurses
// through the whole subtree, skipping vendor directories. Otherwise only the
// immediate directory is checked for a go.mod file.
// An empty root defaults to "." and is treated as recursive.
func FindModFiles(root string) ([]string, error) {
	recursive := root == "" || strings.HasSuffix(root, "/...")
	if recursive {
		root = strings.TrimSuffix(root, "/...")
	}
	if root == "" {
		root = "."
	}

	if !recursive {
		path := filepath.Join(root, "go.mod")
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		return []string{path}, nil
	}

	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "vendor" {
			return filepath.SkipDir
		}
		if !d.IsDir() && d.Name() == "go.mod" {
			paths = append(paths, path)
		}
		return nil
	})
	return paths, err
}
