// Command yze-md-markup reports a prose document written in a markup dialect
// other than markdown, emitting the lean stickler-json report the stickler
// runner consumes.
package main

import (
	"encoding/json"
	"io"
	"os"

	goyze "github.com/gomatic/go-yze"

	markup "github.com/gomatic/yze-md-markup"
)

// Injected collaborators, so the command is testable without real I/O.
//
// The filesystem is ONE value rather than a scatter of package variables,
// matching the sibling analyzers: each of them once kept its own set and they
// drifted, and the drift was invisible until an adversarial review found the
// same defect twice in two of them.
//
// There is no reader here, which is this rule's shape rather than an omission —
// it decides from the path, so the command opens nothing. The bounded read the
// siblings inject is machinery for a rule that has to look inside a file, and
// TestTheCommandOpensNothing holds this one to deciding without it.
var (
	osExit           = os.Exit
	files            = goyze.OSFileSystem()
	stdout io.Writer = os.Stdout
)

func main() { osExit(run(os.Args[1:])) }

// run expands the arguments to paths, judges them, and emits the report.
func run(args []string) int {
	if err := report(args); err != nil {
		return fail(err)
	}
	return 0
}

// report is the run itself, as an ERROR rather than an exit code. run answers
// the process, which cannot be matched: with the refusal only ever reaching an
// int and a line of stderr, this command's sentinel could be swapped for any
// other with the whole suite green, so the failure the sentinel exists to name
// would have nothing behind it.
func report(args []string) error {
	if len(args) == 0 {
		// Being given nothing is an error, not a clean pass. A runner whose
		// root placeholder expands to nothing would otherwise green the gate
		// over a repository no analyzer ever looked at.
		return markup.ErrNoPaths.With(nil)
	}
	found, err := discovery().Expand(args)
	if err != nil {
		// An argument that cannot be resolved fails the RUN rather than being
		// contained like an unreadable tree, and the difference is deliberate:
		// a tree the walk cannot enter is a fact about the repository and is
		// reported as a finding, while an argument that is not there is a fact
		// about the INVOCATION. Reporting on the rest and exiting zero would
		// tell a runner its request was answered when part of it never was.
		return err
	}
	// The expansion goes through WHOLE. What the walk could not enter is part
	// of the run's answer, not a leftover: a directory nobody could enumerate
	// is exactly where a banned document would sit unreported. [judged] is what
	// separates that from a path whose name the walk saw and could not read,
	// which this rule decides like any other.
	return json.NewEncoder(stdout).Encode(markup.Report(judged(found)))
}
