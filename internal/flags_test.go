package internal_test

import (
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
	if cfg.Push {
		t.Error("Push should default to false")
	}
	if cfg.PR != "" {
		t.Errorf("PR = %q, want empty", cfg.PR)
	}
	if cfg.TestCmd != "go test ./..." {
		t.Errorf("TestCmd = %q, want %q", cfg.TestCmd, "go test ./...")
	}
	if cfg.Soak != 90*24*time.Hour {
		t.Errorf("Soak = %v, want 90d", cfg.Soak)
	}
	if cfg.Protected != "main,master,trunk" {
		t.Errorf("Protected = %q, want %q", cfg.Protected, "main,master,trunk")
	}
	if cfg.Force {
		t.Error("Force should default to false")
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
		"-push",
		"-pr=gh pr create --fill",
		"-test=make test",
		"-soak=30d",
		"-protected=main",
		"-force",
		"-skip=govulncheck",
		"-custom=make generate",
	})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if !cfg.Push {
		t.Error("Push should be true")
	}
	if cfg.PR != "gh pr create --fill" {
		t.Errorf("PR = %q", cfg.PR)
	}
	if cfg.TestCmd != "make test" {
		t.Errorf("TestCmd = %q", cfg.TestCmd)
	}
	if cfg.Soak != 30*24*time.Hour {
		t.Errorf("Soak = %v, want 30d", cfg.Soak)
	}
	if !cfg.Force {
		t.Error("Force should be true")
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
