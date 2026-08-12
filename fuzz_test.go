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
var bannedSuffixes = map[string]bool{
	".rst": true, ".adoc": true, ".asciidoc": true, ".textile": true, ".org": true,
	".mediawiki": true, ".wiki": true, ".creole": true, ".pod": true, ".rdoc": true,
}

// wantBanned decides a path the long way round: the final element, with the
// trailing characters Windows discards removed, from its last dot onward,
// lower-cased. It shares no code with [markup.Banned] — that is the point.
func wantBanned(at string) bool {
	name := strings.TrimRight(path.Base(at), ". ")
	dot := strings.LastIndex(name, ".")
	if dot < 0 {
		return false
	}
	return bannedSuffixes[strings.ToLower(name[dot:])]
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

		examined := markup.Report(goyze.Expansion{Files: []string{at}}).Diagnostics
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
