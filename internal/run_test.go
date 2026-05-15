package internal_test

import (
	"context"
	"testing"

	"github.com/josvazg/gobump/internal"
)

func TestRun_noArgs(t *testing.T) {
	code := internal.Run(context.Background(), nil, func(string) string { return "" })
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}
