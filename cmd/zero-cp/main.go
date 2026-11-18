// zero-cp is the zero service (control plane): push gate and forge token
// upkeep for all sandboxes of this user. zero up starts it; it takes no
// arguments.
package main

import (
	"fmt"
	"os"

	"github.com/Breee/z3r0/internal/zero"
)

func main() {
	if len(os.Args) > 1 {
		fmt.Fprintln(os.Stderr, "zero-cp: the zero service (push gate, token upkeep); takes no arguments, zero up starts it")
		os.Exit(2)
	}
	if err := zero.Serve(); err != nil {
		fmt.Fprintln(os.Stderr, "zero-cp:", err)
		os.Exit(1)
	}
}
