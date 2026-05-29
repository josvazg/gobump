package internal

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
)

// VulnReport holds findings from a govulncheck -json run.
type VulnReport struct {
	Findings []Finding
}

// Finding is a single vulnerability entry from govulncheck output.
type Finding struct {
	OSV          string
	FixedVersion string
	Module       string
	Package      string
}

// IsStdlib reports whether the finding is in the Go standard library.
func (f Finding) IsStdlib() bool {
	return f.Module == "stdlib"
}

// HasStdlib reports whether any finding is in the standard library.
func (r VulnReport) HasStdlib() bool {
	for _, f := range r.Findings {
		if f.IsStdlib() {
			return true
		}
	}
	return false
}

// LibFindings returns a module→fixedVersion map for non-stdlib findings.
// When the same module appears multiple times the lexicographically greatest
// fixed version is kept (works correctly for semver strings sharing a major.minor).
func (r VulnReport) LibFindings() map[string]string {
	m := make(map[string]string)
	for _, f := range r.Findings {
		if f.IsStdlib() {
			continue
		}
		if cur, ok := m[f.Module]; !ok || f.FixedVersion > cur {
			m[f.Module] = f.FixedVersion
		}
	}
	return m
}

// parseVulnReport parses govulncheck -json output from r.
// Non-finding lines (config, progress) are silently skipped.
// Findings with an empty trace are skipped.
func parseVulnReport(r io.Reader) (VulnReport, error) {
	var report VulnReport
	dec := json.NewDecoder(r)
	for dec.More() {
		var line struct {
			Finding *struct {
				OSV          string `json:"osv"`
				FixedVersion string `json:"fixed_version"`
				Trace        []struct {
					Module  string `json:"module"`
					Package string `json:"package"`
				} `json:"trace"`
			} `json:"finding"`
		}
		if err := dec.Decode(&line); err != nil {
			return VulnReport{}, err
		}
		if line.Finding == nil || len(line.Finding.Trace) == 0 {
			continue
		}
		report.Findings = append(report.Findings, Finding{
			OSV:          line.Finding.OSV,
			FixedVersion: line.Finding.FixedVersion,
			Module:       line.Finding.Trace[0].Module,
			Package:      line.Finding.Trace[0].Package,
		})
	}
	return report, nil
}

// defaultGovulncheck runs govulncheck -json ./... in dir.
// It captures stdout for parsing even when the command exits non-zero.
func defaultGovulncheck(dir string) (VulnReport, error) {
	cmd := exec.Command("govulncheck", "-json", "./...")
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	runErr := cmd.Run()
	report, parseErr := parseVulnReport(&stdout)
	if parseErr != nil {
		return VulnReport{}, parseErr
	}
	return report, runErr
}
