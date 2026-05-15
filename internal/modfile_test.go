package internal_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/josvazg/gobump/internal"
)

func writeGoMod(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadGoVersion(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name:    "full semver",
			content: "module example.com/m\n\ngo 1.22.3\n",
			want:    "1.22.3",
		},
		{
			name:    "minor only",
			content: "module example.com/m\n\ngo 1.22\n",
			want:    "1.22",
		},
		{
			name:    "no go directive",
			content: "module example.com/m\n",
			want:    "",
		},
		{
			name:    "file not found",
			content: "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var path string
			if tc.wantErr {
				path = "/nonexistent/go.mod"
			} else {
				path = writeGoMod(t, tc.content)
			}

			got, err := internal.ReadGoVersion(path)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ReadGoVersion() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("ReadGoVersion() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWriteGoVersion(t *testing.T) {
	original := "module example.com/m\n\n// keep this comment\ngo 1.21.0\n\nrequire example.com/dep v1.0.0\n"
	path := writeGoMod(t, original)

	if err := internal.WriteGoVersion(path, "1.22.3"); err != nil {
		t.Fatalf("WriteGoVersion: %v", err)
	}

	got, err := internal.ReadGoVersion(path)
	if err != nil {
		t.Fatalf("ReadGoVersion after write: %v", err)
	}
	if got != "1.22.3" {
		t.Errorf("after write: got %q, want %q", got, "1.22.3")
	}

	// Verify require block survived the rewrite.
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "example.com/dep") {
		t.Error("require block was lost after WriteGoVersion")
	}
}

func TestWriteGoVersion_stripsGoPrefix(t *testing.T) {
	path := writeGoMod(t, "module example.com/m\n\ngo 1.21.0\n")

	if err := internal.WriteGoVersion(path, "go1.22.3"); err != nil {
		t.Fatalf("WriteGoVersion: %v", err)
	}
	got, _ := internal.ReadGoVersion(path)
	if got != "1.22.3" {
		t.Errorf("got %q, want %q", got, "1.22.3")
	}
}

