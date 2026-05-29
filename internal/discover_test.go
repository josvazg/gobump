package internal_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/josvazg/gobump/internal"
)

func TestFindModFiles(t *testing.T) {
	const mod = "module example.com/m\n\ngo 1.22.0\n"

	tests := []struct {
		name    string
		files   []string                   // relative paths to create in the temp dir
		pattern func(root string) string   // pattern passed to FindModFiles
		want    []string                   // expected relative paths; nil skips result check
	}{
		{
			name:    "recursive finds all modules skipping vendor",
			files:   []string{"go.mod", "sub/go.mod", "sub/vendor/pkg/go.mod"},
			pattern: func(root string) string { return root + "/..." },
			want:    []string{"go.mod", "sub/go.mod"},
		},
		{
			name:    "non-recursive finds only the root module",
			files:   []string{"go.mod", "sub/go.mod"},
			pattern: func(root string) string { return root },
			want:    []string{"go.mod"},
		},
		{
			name:    "non-recursive on empty directory returns nothing",
			pattern: func(root string) string { return root },
			want:    []string{},
		},
		{
			name:    "empty pattern defaults to current directory without error",
			pattern: func(string) string { return "" },
			want:    nil, // only verify no error
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for _, f := range tc.files {
				write(t, filepath.Join(root, f), mod)
			}

			got, err := internal.FindModFiles(tc.pattern(root))
			if err != nil {
				t.Fatalf("FindModFiles: %v", err)
			}
			if tc.want == nil {
				return
			}

			sort.Strings(got)
			want := make([]string, len(tc.want))
			for i, w := range tc.want {
				want[i] = filepath.Join(root, w)
			}
			if len(got) != len(want) {
				t.Fatalf("got %v, want %v", got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("[%d] got %q, want %q", i, got[i], want[i])
				}
			}
		})
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
