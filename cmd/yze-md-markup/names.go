package main

// Asking the walk the NAME question, and deciding what a path nobody could look
// inside means to a rule that never looks inside anything.
//
// This rule is path-only in its ENTIRETY — it opens no file, and every finding
// it makes is a name finding — which makes it the analyzer most exposed to a
// walk that answers the wrong question about a name. Three escapes were proven
// here by an adversarial review and stood recorded as unfixed: a symlink's name
// collapsed away by the identity dedup, two names for one file yielding one
// finding, and a symlink to a directory neither descended nor reported. All
// three belonged to the shared discovery and are repaired there in `go-yze`
// v0.13.0 rather than worked around here — an analyzer that disagreed with its
// siblings about what a tree contains would be the defect the shared discovery
// exists to prevent.
//
// The library now hands back NAMES beside FILES: one entry per spelling,
// absolutised and NOT resolved through symlinks, and a superset of the one
// spelling per file that Files carries. This command asks Names everywhere and
// reads Files nowhere. It also reports the directory links it declines to enter,
// through the unreadable list, which leaves exactly ONE decision here that the
// library cannot make for it: which of those paths is a blind spot for a rule
// that decides by name.

import (
	"errors"
	"io/fs"

	goyze "github.com/gomatic/go-yze"

	markup "github.com/gomatic/yze-md-markup"
)

// judged re-expresses an expansion for a rule that decides by NAME.
//
// It carries NAMES forward, never files. Files is one spelling per inode, and
// collapsing two spellings of one file discards the evidence a name finding IS:
// a `notes.adoc` hard link beside `guide.rst`, or an `alias.rst` symlink beside
// the document it points at, is a banned document in this repository under its
// own name, and reporting one while dropping the other leaves the gate green
// over a file that is still there.
//
// The walk hands back one list of paths it could not read, mixing two things
// this rule answers differently: a tree whose children's names it never saw, and
// an entry it could not have READ — a FIFO, a device, a link resolving to
// nothing — whose own name it saw perfectly well. For a rule whose entire
// decision is the name, seeing the name IS examining it, so the second kind
// joins the names and is judged like any other path.
//
// Reporting both as blind spots was measured wrong: the first sweep of the
// whole fleet produced exactly one finding, and it was a dangling `op-run`
// symlink — a rule that has never seen a first-party violation, whose only
// output anywhere was a broken link that has nothing to do with markup.
func judged(found goyze.Expansion) goyze.Expansion {
	seen := make([]string, 0, len(found.Names)+len(found.Unreadable))
	seen = append(seen, found.Names...)
	opaque := make([]string, 0, len(found.Unreadable))
	for _, path := range found.Unreadable {
		if isOpaque(markup.Path(path)) {
			opaque = append(opaque, path)
			continue
		}
		seen = append(seen, path)
	}
	return goyze.Expansion{Names: seen, Unreadable: opaque}
}

// isOpaque reports a path this rule could not look INSIDE.
//
// THE BANNED NAME IS JUDGED FIRST, AND NOT HERE. [markup.Report] convicts a
// banned name before it calls any path a blind spot, so a `GUIDE.rst` link to a
// directory is reported as the reStructuredText document it is whichever list it
// arrives in, and nothing this predicate answers can turn a banned name into
// "could not be looked inside". That ordering is what makes this question
// answerable at all: it decides only what becomes of the names that are NOT
// banned, and there the two failures differ, so the two are asked differently.
//
//   - The directory question is STAT'd, so a link to a directory is opaque. The
//     walk names such a link and refuses to follow it, so the names behind it —
//     an unbounded number of them — were never seen. `docs -> ../rst-docs` hid
//     every document under it from every report, and answering "clean" about a
//     tree nobody enumerated is the one outcome a gate must never invent. This
//     is the arm that changed when the walk began reporting the trees it
//     declines: lstat'ing here called such a link a plain entry, judged its
//     innocent name clean, and put the silence straight back.
//   - The existence question is asked of the STAT ERROR, and only "not there"
//     answers it. A DANGLING link is not opaque: its own name the walk read out
//     loud, and the name is this rule's whole decision, so treating it as a
//     blind spot would turn every broken link in every repository into a finding
//     about markup. Measured: the first sweep of the whole fleet produced
//     exactly one finding, and it was a dangling `op-run` symlink.
//   - EVERY OTHER STAT FAILURE IS OPAQUE, and reading them as "not there" was a
//     working bypass. `err == nil && info.IsDir()` calls a link nobody can
//     FOLLOW an ordinary entry — a link into a directory that cannot be
//     traversed, a chain past the kernel's link limit, a stale network mount —
//     so `docs -> /locked/rst-docs` was judged on its innocent name and cleared,
//     while the identical tree reached without a link was reported. An
//     adversarial review built both and got one finding where there should have
//     been two. The distinction is the ERROR, never the mere fact of one.
//
// A path that cannot be lstat'd at all is opaque for the same reason: a question
// nobody could answer is not an answer of "clean".
func isOpaque(at markup.Path) bool {
	if _, err := files.Lstat(string(at)); err != nil {
		return true
	}
	info, err := files.Stat(string(at))
	if err != nil {
		return !errors.Is(err, fs.ErrNotExist)
	}
	return info.IsDir()
}
