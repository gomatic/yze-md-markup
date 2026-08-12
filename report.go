package markup

// Aggregating one run: what the walk found, what it could not look at, and what
// a report says when it carries less than it found.

import (
	"fmt"

	errs "github.com/gomatic/go-error"
	goyze "github.com/gomatic/go-yze"
)

// ErrNoPaths reports a run given nothing to analyze. A runner whose root
// placeholder expands to nothing would otherwise green the gate over a
// repository no analyzer ever looked at — indistinguishable, at the exit code,
// from one it read and found clean.
const ErrNoPaths errs.Const = "no paths to analyze"

// findingCount is how many findings a run produced.
type findingCount int

// reportLimit bounds how many findings ONE RUN carries.
//
// The amplification a sibling analyzer needed this for cannot happen here: one
// path yields at most one finding, so a run's findings are bounded by the files
// on disk rather than by the bytes inside one of them. It is here for the other
// half of that lesson — a vendored documentation tree the prune list does not
// name is thousands of files, and a report naming every one of them is one
// problem printed thousands of times. The true total is still named, so nothing
// is lost by being unprinted.
const reportLimit findingCount = 10_000

// runTruncationMessage formats the finding that stands for the rest of a run.
const runTruncationMessage = "%d banned-markup findings across this run, of which %d are reported; findings from %s " +
	"onward are omitted, because a tree with this many is one problem rather than that many"

// unexaminableMessage formats the finding for a path the walk could not look
// inside.
const unexaminableMessage = "%s could not be looked inside, so any document it holds is unaccounted for; the gate " +
	"cannot vouch for a tree it could not read, so this is reported rather than passed over"

// Report is the whole run's report: every NAME the walk saw, and the paths it
// could not look inside.
//
// It reads [goyze.Expansion.Names] and never [goyze.Expansion.Files], which is
// this rule's shape rather than a preference. Files is one spelling per FILE —
// the right question for a rule that reads bytes, where analyzing one inode
// twice under two names reports one defect as two — and the wrong one here,
// because a name finding's subject IS the name. A `guide.adoc` symlink and a
// `notes.adoc` hard link are each a banned document in a repository under their
// own name; collapsing them to one reported spelling leaves the other in place
// with the gate green, and a symlink survives a clone as mode 120000. Reading
// BOTH lists is the other way to be wrong: Names is a superset of Files, so
// every file would be reported twice.
//
// [goyze.Expansion.Unreadable] means something narrower here than the walk hands
// back, and the caller narrows it. A rule that must READ to decide is blind to
// every path it could not open; this rule decides by NAME, so a path the walk
// merely failed to read has still been seen and belongs with the names. What is
// left — a directory nobody could enumerate, whose children's names were never
// seen — is the only genuine blind spot, and it is what this list must carry.
//
// It takes the expansion WHOLE, rather than a list of names and a list of
// failures, so both go through ONE counter, ONE limit and ONE truncation
// notice. Split across two entry points they disagreed in the sibling this rule
// was modelled on: read failures were appended past a limit that had already
// been applied to the findings, and a tree holding ten thousand unreadable
// files and one real violation reported the ten thousand, dropped the one,
// announced nothing and exited zero.
func Report(found goyze.Expansion) goyze.Report {
	run := collecting{}
	for _, path := range found.Names {
		run = run.add(Path(path), examined)
	}
	for _, path := range found.Unreadable {
		run = run.add(Path(path), unexaminable)
	}
	return run.done()
}

// finder is one path's findings. It is a function so [collecting.add] holds the
// counting, the limit and the notice once, whether the path was examined or
// merely named.
type finder func(at Path) []goyze.Diagnostic

// collecting accumulates one [Report]. It is a value: every method returns the
// next accumulation rather than mutating this one, so no caller can hold a
// half-counted run.
type collecting struct {
	truncatedAt Path
	diags       []goyze.Diagnostic
	total       findingCount
}

// add folds one path's findings into the run.
//
// The total is incremented BEFORE the limit is consulted, so a truncated run
// still reports the number of findings it FOUND rather than the number it
// printed. Summing what was collected counted the truncation instead of the
// findings, and the sentence naming the true count carried the false one.
func (c collecting) add(at Path, find finder) collecting {
	found := find(at)
	c.total += findingCount(len(found))
	if c.truncatedAt != "" || len(found) == 0 {
		return c
	}
	if findingCount(len(c.diags))+findingCount(len(found)) > reportLimit {
		c.truncatedAt = at
		return c
	}
	c.diags = append(c.diags, found...)
	return c
}

// done is the finished report, with the truncation notice when the run carried
// less than it found.
//
// An empty report is an empty ARRAY, not a JSON null. A nil slice encodes as
// `null`, which is a different document from the `[]` every sibling analyzer
// emits — and a consumer that iterates the field fails on it. For THIS rule
// that is not an edge case but the ordinary output: measured 2026-08-12, it is
// clean in 715 of the fleet's 716 repositories, so the clean report is the one
// shape it emits most.
func (c collecting) done() goyze.Report {
	if c.truncatedAt == "" {
		return goyze.Report{Diagnostics: emptied(c.diags)}
	}
	return goyze.Report{Diagnostics: append(c.diags, truncation(c.truncatedAt, c.total))}
}

// emptied is diags as a slice an encoder renders as `[]` rather than `null`.
func emptied(diags []goyze.Diagnostic) []goyze.Diagnostic {
	if diags == nil {
		return []goyze.Diagnostic{}
	}
	return diags
}

// examined is the finding for a path the discovery looked at: the document
// itself, when its name says it is written in a markup this repository does not
// write prose in.
func examined(at Path) []goyze.Diagnostic {
	written, banned := bannedDialect(at)
	if !banned {
		return nil
	}
	return []goyze.Diagnostic{diagnostic(at, finding(fmt.Sprintf(bannedMessage, judgedName(at), written)))}
}

// unexaminable is the finding for a path the walk could not look inside.
//
// A banned NAME is convicted here exactly as it is anywhere else, and that is
// this rule's defining property rather than a nicety: the decision is the path,
// so a `README.rst` the gate could not open is as certainly a reStructuredText
// document as one it could. A rule that must read to decide has to report such
// a path as an unknown; this one already knows.
//
// Anything else is reported as the blind spot it is. The children of a
// directory nobody could enumerate were never named, so a banned document among
// them would be a silent pass — which is the one outcome a gate must never
// produce.
func unexaminable(at Path) []goyze.Diagnostic {
	if diags := examined(at); diags != nil {
		return diags
	}
	return []goyze.Diagnostic{diagnostic(at, finding(fmt.Sprintf(unexaminableMessage, at)))}
}

// truncation is the finding that stands for everything past the run's limit,
// carrying the true total so nothing is silently lost.
//
// It is attributed to the PATH the run stopped collecting at, not to no path at
// all. A diagnostic with an empty path is one no runner can attribute, baseline
// or ratchet, and this one has a truthful path available: the file whose
// finding was the first to be dropped.
func truncation(at Path, found findingCount) goyze.Diagnostic {
	return diagnostic(at, finding(fmt.Sprintf(runTruncationMessage, found, reportLimit, at)))
}
