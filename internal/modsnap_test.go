package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadModSumBytes_bothPresent(t *testing.T) {
	dir := t.TempDir()
	mod := []byte("module example.com/m\n\ngo 1.22.0\n")
	sum := []byte("h1:abc123\n")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), mod, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), sum, 0o644); err != nil {
		t.Fatal(err)
	}

	gotMod, gotSum := readModSumBytes(dir)
	if string(gotMod) != string(mod) {
		t.Errorf("mod: got %q, want %q", gotMod, mod)
	}
	if string(gotSum) != string(sum) {
		t.Errorf("sum: got %q, want %q", gotSum, sum)
	}
}

func TestReadModSumBytes_missingSum(t *testing.T) {
	dir := t.TempDir()
	mod := []byte("module example.com/m\n\ngo 1.22.0\n")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), mod, 0o644); err != nil {
		t.Fatal(err)
	}

	gotMod, gotSum := readModSumBytes(dir)
	if string(gotMod) != string(mod) {
		t.Errorf("mod: got %q, want %q", gotMod, mod)
	}
	if gotSum != nil {
		t.Errorf("sum: got %q, want nil", gotSum)
	}
}
