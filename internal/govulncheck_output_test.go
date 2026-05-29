package internal

import (
	"strings"
	"testing"
)

func TestParseVulnReport(t *testing.T) {
	const (
		configLine   = `{"config":{"protocol_version":"1.0.0","handler":"govulncheck"}}`
		progressLine = `{"progress":{"message":"Scanning..."}}`
		stdlibLine   = `{"finding":{"osv":"GO-2023-1988","fixed_version":"go1.21.1","trace":[{"module":"stdlib","version":"go1.21.0","package":"net/http"}]}}`
		libLine      = `{"finding":{"osv":"GO-2023-2041","fixed_version":"v0.18.0","trace":[{"module":"golang.org/x/net","version":"v0.10.0","package":"golang.org/x/net/http2"}]}}`
		emptyTrace   = `{"finding":{"osv":"GO-2023-9999","fixed_version":"v1.0.0","trace":[]}}`
	)

	tests := []struct {
		name    string
		input   string
		want    []Finding
		wantErr bool
	}{
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "non-finding lines are ignored",
			input: configLine + "\n" + progressLine,
			want:  nil,
		},
		{
			name:  "stdlib finding",
			input: stdlibLine,
			want: []Finding{
				{OSV: "GO-2023-1988", FixedVersion: "go1.21.1", Module: "stdlib", Package: "net/http"},
			},
		},
		{
			name:  "library finding",
			input: libLine,
			want: []Finding{
				{OSV: "GO-2023-2041", FixedVersion: "v0.18.0", Module: "golang.org/x/net", Package: "golang.org/x/net/http2"},
			},
		},
		{
			name:  "mixed findings",
			input: stdlibLine + "\n" + libLine,
			want: []Finding{
				{OSV: "GO-2023-1988", FixedVersion: "go1.21.1", Module: "stdlib", Package: "net/http"},
				{OSV: "GO-2023-2041", FixedVersion: "v0.18.0", Module: "golang.org/x/net", Package: "golang.org/x/net/http2"},
			},
		},
		{
			name:  "finding with empty trace is skipped",
			input: emptyTrace,
			want:  nil,
		},
		{
			name:    "malformed JSON returns error",
			input:   "not-json",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseVulnReport(strings.NewReader(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got.Findings) != len(tc.want) {
				t.Fatalf("findings = %v, want %v", got.Findings, tc.want)
			}
			for i, f := range got.Findings {
				w := tc.want[i]
				if f != w {
					t.Errorf("finding[%d] = %+v, want %+v", i, f, w)
				}
			}
		})
	}
}

func TestFinding_IsStdlib(t *testing.T) {
	tests := []struct {
		module string
		want   bool
	}{
		{"stdlib", true},
		{"golang.org/x/net", false},
		{"github.com/some/pkg", false},
		{"", false},
	}
	for _, tc := range tests {
		f := Finding{Module: tc.module}
		if got := f.IsStdlib(); got != tc.want {
			t.Errorf("Finding{Module:%q}.IsStdlib() = %v, want %v", tc.module, got, tc.want)
		}
	}
}
