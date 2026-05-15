package internal

import (
	"context"
	"fmt"
)

// Run is the program entry point. It receives a cancelable context, raw CLI
// args, and an env lookup func so all three are injectable in tests.
func Run(ctx context.Context, args []string, env func(string) string) int {
	fmt.Println("gobump: not yet implemented")
	return 0
}
