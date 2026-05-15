package internal

import (
	"io/fs"
	"path/filepath"
)

// FindModFiles walks root and returns the path of every go.mod file found,
// skipping vendor directories. Defaults to "." when root is empty.
func FindModFiles(root string) ([]string, error) {
	if root == "" {
		root = "."
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
