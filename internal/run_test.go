package internal_test

import (
	"context"
	"testing"

	"github.com/josvazg/gobump/internal"
)

func TestRun_noArgs(t *testing.T) {
	// -force bypasses git checks so this test is not sensitive to the state
	// of whatever repo the tests happen to run inside.
	code := internal.Run(context.Background(), []string{"-force"}, func(string) string { return "" })
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}
