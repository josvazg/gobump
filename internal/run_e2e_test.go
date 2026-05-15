package internal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunner_bumpsGoVersion(t *testing.T) {
	dir := t.TempDir()
	modPath := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(modPath, []byte("module example.com/m\n\ngo 1.21.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := time.Now().Add(-100 * 24 * time.Hour)
	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour, TestCmd: "go test ./...", Force: true},
		path: dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: "go1.22.3", Date: old, Stable: true}}, nil
		},
	}

	if code := r.run(context.Background()); code != 0 {
		t.Fatalf("run returned %d", code)
	}

	got, err := ReadGoVersion(modPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.22.3" {
		t.Errorf("go version = %q, want %q", got, "1.22.3")
	}
}

func TestRunner_skipsWhenSoaking(t *testing.T) {
	dir := t.TempDir()
	modPath := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(modPath, []byte("module example.com/m\n\ngo 1.21.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fresh := time.Now().Add(-10 * 24 * time.Hour)
	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour, TestCmd: "go test ./...", Force: true},
		path: dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: "go1.22.3", Date: fresh, Stable: true}}, nil
		},
	}

	if code := r.run(context.Background()); code != 0 {
		t.Fatalf("run returned %d", code)
	}

	got, _ := ReadGoVersion(modPath)
	if got != "1.21.0" {
		t.Errorf("go version should be unchanged, got %q", got)
	}
}

func TestRunner_skipsWhenUpToDate(t *testing.T) {
	dir := t.TempDir()
	modPath := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(modPath, []byte("module example.com/m\n\ngo 1.22.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := time.Now().Add(-100 * 24 * time.Hour)
	r := &runner{
		cfg:  Config{Soak: 90 * 24 * time.Hour, TestCmd: "go test ./...", Force: true},
		path: dir,
		fetchReleases: func(_ context.Context) ([]Release, error) {
			return []Release{{Version: "go1.22.3", Date: old, Stable: true}}, nil
		},
	}

	if code := r.run(context.Background()); code != 0 {
		t.Fatalf("run returned %d", code)
	}

	got, _ := ReadGoVersion(modPath)
	if got != "1.22.3" {
		t.Errorf("go version should be unchanged, got %q", got)
	}
}
