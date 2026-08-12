package main

// How this command REFUSES: what a failure says, and to whom.
//
// It is its own file because a refusal is its own contract. It reached only an
// exit code and an unasserted line of text for as long as it sat beside the
// wiring, and an adversarial mutation that stripped the command's name from
// every error line left the whole suite green.

import (
	"fmt"
	"io"
	"os"
)

// stderr is where a refusal is named, injected like [stdout] so what the
// command SAYS is a contract a test can hold rather than text nobody reads.
var stderr io.Writer = os.Stderr

// fail names the refusal on stderr and returns the failure exit code.
//
// The writer is INJECTED like stdout's. It was the one piece of real I/O left in
// a file whose comment claims there is none, so nothing asserted what a refusal
// says: an adversarial mutation deleted the command's name from every error
// line and the whole suite stayed green, while the stray text leaked into the
// gate's own output.
func fail(err error) int {
	_, _ = fmt.Fprintln(stderr, "yze-md-markup:", err)
	return 1
}
