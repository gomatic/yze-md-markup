package markup_test

import (
	"path"
	"strings"
	"testing"

	goyze "github.com/gomatic/go-yze"

	markup "github.com/gomatic/yze-md-markup"
)

// bannedSuffixes is the decision written out a SECOND time, independently, for
// the fuzz target to disagree with. It is deliberately not the package's table:
// a fuzz target that asks the implementation what the implementation thinks
// holds for every input ever generated and proves nothing.
var bannedSuffixes = []string{
	".rst", ".adoc", ".asciidoc", ".textile", ".org",
	".mediawiki", ".wiki", ".creole", ".pod", ".rdoc",
}

// wantBanned decides a path the long way round: the final element, with the
// trailing characters Windows discards removed, from its last dot onward,
// compared case-insensitively. It shares no code with [markup.Banned] — that is
// the point.
//
// It compares with [strings.EqualFold] and NOT by lower-casing, and the
// difference is the whole reason the fold exists. `strings.ToLower` is strictly
// narrower than the case folding a case-insensitive volume applies: it leaves
// U+017F LATIN SMALL LETTER LONG S alone, so an oracle built on it decides
// `README.rſt` is clean — and on APFS that is the SAME FILE as `README.rst`.
// This oracle was written that way, which made it a second copy of the exact
// bypass [markup.Banned] was repaired to close: the fuzzer would have reported
// the correct implementation as the failure the moment its corpus reached that
// rune, and the obvious way to make the two agree would have reinstated the
// bypass. EqualFold is stdlib simple case folding — the contract, reached by a
// different mechanism than the orbit walk it checks.
func wantBanned(at string) bool {
	name := strings.TrimRight(path.Base(at), ". ")
	dot := strings.LastIndex(name, ".")
	if dot < 0 {
		return false
	}
	for _, suffix := range bannedSuffixes {
		if strings.EqualFold(name[dot:], suffix) {
			return true
		}
	}
	return false
}

// FuzzReport drives arbitrary paths through the whole rule. The contract under
// fuzz, asserted on every input rather than merely exercised:
//
//   - The decision agrees with an independent reading of the same rule, for any
//     path a filesystem can hold. This rule's entire job is that decision, and
//     its entire input is a name — so a name is exactly what it must be fuzzed
//     with.
//   - An examined path yields at most ONE finding, and yields one only when it
//     is banned.
//   - A path the caller could not look inside ALWAYS yields exactly one finding.
//     Nothing is passed over in silence, which is the one outcome a gate must
//     never produce, and it is the arm most easily lost to a refactor because it
//     fires on nothing anybody writes.
//   - Every finding carries this rule's identity, the path it was given, and a
//     position an editor can open.
func FuzzReport(f *testing.F) {
	for _, seed := range []string{
		"", ".", "..", "...", "/", "   ", "\n", "\x00",
		"README.md", "README.rst", "README.RST", "README.Rst",
		// The folded spellings, seeded rather than left to the fuzzer to
		// discover. A corpus without them left the oracle and the rule free to
		// disagree about the one rune that is a proven bypass, for 20 million
		// executions and counting.
		"README.rſt", "README.rſT", "notes.ORG", "guide.AdOc",
		"README.rst.", "README.rst ", "README.rst..", "README.rst . ",
		".rst", ".md", "docs/.adoc", "README.rst.md", "README.md.rst",
		"notes.org", "www.uplang.org/index.md", "google.golang.org/grpc/README.md",
		"docs/guide.adoc", "a/b/c/d/e/deep.rst", "my docs/release notes.adoc",
		"documentación/guía.rst", "requirements.txt", "paper.tex", "index.html",
		"CHANGELOG", "Makefile", "release.rst.tar.gz", "trailing/", "..rst",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, at string) {
		want := wantBanned(at)
		if got := markup.Banned(markup.Path(at)); got != want {
			t.Fatalf("Banned(%q) = %v, independently decided %v", at, got, want)
		}

		examined := markup.Report(goyze.Expansion{Names: []string{at}}).Diagnostics
		if len(examined) > 1 {
			t.Fatalf("one path yields at most one finding, got %d for %q", len(examined), at)
		}
		if (len(examined) == 1) != wantBanned(at) {
			t.Fatalf("an examined path is reported when it is banned; %q gave %d", at, len(examined))
		}

		unexaminable := markup.Report(goyze.Expansion{Unreadable: []string{at}}).Diagnostics
		if len(unexaminable) != 1 {
			t.Fatalf("a path the caller could not look inside is always reported; %q gave %d", at, len(unexaminable))
		}

		for _, d := range append(examined, unexaminable...) {
			if d.Rule != markup.Rule || d.Tool != markup.Tool {
				t.Fatalf("diagnostic must carry this rule's identity, got %q/%q", d.Tool, d.Rule)
			}
			if d.Path != at {
				t.Fatalf("diagnostic must name the path it was given, got %q for %q", d.Path, at)
			}
			if d.Line != 1 || d.Col != 1 {
				t.Fatalf("every finding is against the file, got line %d col %d", d.Line, d.Col)
			}
			if d.Severity != goyze.SeverityError {
				t.Fatalf("every finding here is an error, got %q", d.Severity)
			}
			if d.Message == "" {
				t.Fatalf("a finding nobody can act on is not a finding")
			}
		}
	})
}
