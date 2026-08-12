package main

// What this command claims, and whose prose is somebody else's. Everything else
// about turning arguments into paths — the symlinked root, the identity of a
// path reached two ways, the tree that cannot be read, the ignore filter —
// belongs to the shared discovery, because three analyzers answering those
// questions separately answered them differently. Which of the shared walk's
// two answers a NAME rule reads, and what it makes of a path nobody could look
// inside, is [judged]'s — see names.go.

import (
	goyze "github.com/gomatic/go-yze"

	markup "github.com/gomatic/yze-md-markup"
)

// discovery is this command's file discovery: the shared walk, told what this
// rule judges and whose trees to skip.
func discovery() goyze.Discovery {
	return goyze.Discovery{Files: files, Claims: claims, Prunes: pruned}
}

// claims reports a path this rule judges.
//
// It is the analyzer's own predicate, not a second copy of it. Claiming ONLY
// the banned suffixes is what keeps the walk cheap — every markdown file in the
// fleet is declined on a map lookup and never stat'd, ignore-checked or
// deduplicated — and the report applies the same predicate again, so a caller
// that hands [markup.Report] a file this walk would not have claimed still gets
// the right answer.
func claims(path goyze.FilePath) bool {
	return markup.Banned(markup.Path(path))
}

// pruned reports the trees that hold somebody else's prose. A dependency's
// documentation is written in whatever dialect that dependency chose, and
// reporting it tells this repository to delete a file it does not own.
//
// The Python entries are the ones that carry this rule today: every banned
// document in the fleet as measured is a `.rst` inside a `.venv`, vendored by
// pip and belonging to the package that shipped it.
//
// It matches a directory's own NAME at any depth, which is wider than it reads.
// A first-party `content/blog/themes/`, `docs/vendor/` or `internal/testdata/`
// is dropped as silently as a dependency's, and an adversarial review confirmed
// it — the fleet has no such tree today, so this is a latent over-prune rather
// than a live one, and it is recorded because the alternative, a depth-1 rule,
// would disagree with every sibling analyzer about what a walk owns. What a
// particular repository ignores is git's answer, not a list's, and `.git` and
// nested checkouts are the shared walk's business. `testdata` is here for a
// reason specific to this family: it is where an analyzer is proven in both
// directions, so a fixture that MUST contain a violation would otherwise be
// reported as one.
func pruned(name goyze.DirName) bool {
	switch name {
	case "vendor", "node_modules", "themes", "testdata", ".venv", "venv", ".tox":
		return true
	}
	return false
}
