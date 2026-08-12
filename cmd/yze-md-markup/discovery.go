package main

// What this command claims, and whose prose is somebody else's. Everything else
// about turning arguments into paths — the symlinked root, the identity of a
// path reached two ways, the tree that cannot be read, the ignore filter —
// belongs to the shared discovery, because three analyzers answering those
// questions separately answered them differently.

import (
	goyze "github.com/gomatic/go-yze"

	markup "github.com/gomatic/yze-md-markup"
)

// Two properties of the SHARED walk under-report this rule specifically, and
// both were proven by an adversarial review rather than reasoned about. They are
// recorded here so nobody reads this analyzer as name-complete, and they are not
// worked around locally: the walk is one implementation on purpose, and an
// analyzer that disagreed with its siblings about what a tree contains would be
// the defect the shared discovery exists to prevent.
//
//   - Deduplication is by file IDENTITY (device+inode), so two NAMES for one
//     file collapse to one finding: a hard link `notes.adoc` beside
//     `guide.rst`, or an `alias.rst` symlink, is dropped even when named
//     outright. For a rule whose finding IS the name, both names are violations
//     — deleting one leaves the other, still a banned document in the tree.
//   - A symlink to a DIRECTORY is neither descended nor reported, so a
//     `docs -> ../rst-docs` link hides every banned document beneath it with no
//     finding and no unreadable notice. Naming that same link as an argument
//     reports them, which is the proof the tool can see them at all.
//
// Both belong to `go-yze`'s discovery and are fixes to make there.

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

// judged re-expresses an expansion for a rule that decides by NAME.
//
// The walk hands back one list of paths it could not read, mixing two things
// this rule answers differently: a directory it was refused entry to, whose
// children's names it never saw, and an entry it could not have READ — a FIFO,
// a device, a link resolving to nothing — whose own name it saw perfectly well.
// For a rule whose entire decision is the name, seeing the name IS examining
// it, so the second kind joins the files and is judged like any other path.
//
// Reporting both as blind spots was measured wrong: the first sweep of the
// whole fleet produced exactly one finding, and it was a dangling `op-run`
// symlink — a rule that has never seen a first-party violation, whose only
// output anywhere was a broken link that has nothing to do with markup.
func judged(found goyze.Expansion) goyze.Expansion {
	seen := make([]string, 0, len(found.Files)+len(found.Unreadable))
	seen = append(seen, found.Files...)
	opaque := make([]string, 0, len(found.Unreadable))
	for _, path := range found.Unreadable {
		if isOpaque(markup.Path(path)) {
			opaque = append(opaque, path)
			continue
		}
		seen = append(seen, path)
	}
	return goyze.Expansion{Files: seen, Unreadable: opaque}
}

// isOpaque reports a path this rule could not look inside: a directory nobody
// enumerated, or a path that is no longer there at all.
//
// It LSTATS. Following the link instead would answer "gone" for a dangling
// symlink — a path whose own name the walk read out loud — and turn every
// broken link in every repository into a finding about markup. A path that
// cannot be lstat'd is genuinely unknown and is treated as opaque, because a
// question nobody could answer is not an answer of "clean".
func isOpaque(at markup.Path) bool {
	info, err := files.Lstat(string(at))
	return err != nil || info.IsDir()
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
