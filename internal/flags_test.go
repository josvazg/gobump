package internal_test

import (
	"strings"
	"testing"
	"time"

	"github.com/josvazg/gobump/internal"
)

func TestParseFlags_defaults(t *testing.T) {
	cfg, path, err := internal.ParseFlags([]string{})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if path != "" {
		t.Errorf("path = %q, want empty", path)
	}
	if cfg.TestCmd != "go test ./..." {
		t.Errorf("TestCmd = %q, want %q", cfg.TestCmd, "go test ./...")
	}
	if cfg.Soak != 90*24*time.Hour {
		t.Errorf("Soak = %v, want 90d", cfg.Soak)
	}
	if cfg.Skip != "" {
		t.Errorf("Skip = %q, want empty", cfg.Skip)
	}
	if cfg.Custom != "" {
		t.Errorf("Custom = %q, want empty", cfg.Custom)
	}
}

func TestParseFlags_pathArg(t *testing.T) {
	_, path, err := internal.ParseFlags([]string{"./myproject"})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if path != "./myproject" {
		t.Errorf("path = %q, want %q", path, "./myproject")
	}
}

func TestParseFlags_flags(t *testing.T) {
	cfg, _, err := internal.ParseFlags([]string{
		"-test=make test",
		"-soak=30d",
		"-skip=govulncheck",
		"-custom=make generate",
	})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if cfg.TestCmd != "make test" {
		t.Errorf("TestCmd = %q", cfg.TestCmd)
	}
	if cfg.Soak != 30*24*time.Hour {
		t.Errorf("Soak = %v, want 30d", cfg.Soak)
	}
	if cfg.Skip != "govulncheck" {
		t.Errorf("Skip = %q", cfg.Skip)
	}
	if cfg.Custom != "make generate" {
		t.Errorf("Custom = %q", cfg.Custom)
	}
}

func TestParseFlags_soakDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"90d", 90 * 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"1d", 24 * time.Hour},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			cfg, _, err := internal.ParseFlags([]string{"-soak=" + tc.input})
			if err != nil {
				t.Fatalf("ParseFlags: %v", err)
			}
			if cfg.Soak != tc.want {
				t.Errorf("Soak = %v, want %v", cfg.Soak, tc.want)
			}
		})
	}
}

func TestParseFlags_soakMaximum(t *testing.T) {
	cfg, _, err := internal.ParseFlags([]string{"-soak=165d"})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if cfg.Soak != internal.MaxSoakDays*24*time.Hour {
		t.Errorf("Soak = %v, want %dd", cfg.Soak, internal.MaxSoakDays)
	}

	_, _, err = internal.ParseFlags([]string{"-soak=166d"})
	if err == nil {
		t.Fatal("expected error for soak above the maximum, got nil")
	}
	wantErr := "-soak=166d exceeds the maximum of 165d"
	if !strings.Contains(err.Error(), wantErr) {
		t.Errorf("error %q does not contain %q", err.Error(), wantErr)
	}
}
