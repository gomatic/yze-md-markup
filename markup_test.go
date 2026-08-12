package markup_test

import (
	"path/filepath"
	"testing"

	goyze "github.com/gomatic/go-yze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	markup "github.com/gomatic/yze-md-markup"
)

// judge is the whole rule under test, reached the way the command reaches it:
// one examined path, and whatever it produced.
func judge(path string) []goyze.Diagnostic {
	return markup.Report(goyze.Expansion{Files: []string{path}}).Diagnostics
}

// TestEveryBannedMarkupIsReportedAndNamed pins the banned table entry by entry,
// naming the dialect each finding must carry.
//
// The whole table is walked rather than a sample of it. A suffix with nothing
// behind it can be deleted from the table with the suite green, and deleting
// one is precisely how a banned document arrives unreported — which, for a rule
// that has never fired on a first-party file, nobody would notice for years.
func TestEveryBannedMarkupIsReportedAndNamed(t *testing.T) {
	for path, dialect := range map[string]string{
		"README.rst":     "reStructuredText",
		"guide.adoc":     "AsciiDoc",
		"guide.asciidoc": "AsciiDoc",
		"notes.textile":  "Textile",
		"page.mediawiki": "MediaWiki markup",
		"page.wiki":      "wiki markup",
		"page.creole":    "Creole",
		"Module.pod":     "Perl POD",
		"class.rdoc":     "RDoc",
		"notes.org":      "Emacs Org Mode",
	} {
		t.Run(path, func(t *testing.T) {
			diags := judge(path)

			require.Len(t, diags, 1, "%s is a document in a markup this repository does not write prose in", path)
			assert.True(t, markup.Banned(markup.Path(path)))
			assert.Contains(t, diags[0].Message, "is a document written in "+dialect,
				"the finding names what the file is")
			assert.Contains(t, diags[0].Message, "prose in this repository is markdown, and only markdown",
				"and what prose must be instead")
			assert.NotContains(t, diags[0].Message, "convert",
				"the standard is stated; no tool is recommended")
		})
	}
}

// TestEveryAllowedExtensionIsLeftAlone pins the other half of the table: the
// suffixes that were considered and deliberately permitted. A rule that reached
// past its standard would be a worse failure than one that never fires, because
// it would fire on documents nobody agreed to ban.
func TestEveryAllowedExtensionIsLeftAlone(t *testing.T) {
	for _, path := range []string{
		"README.md",             // The standard itself.
		"guide.markdown",        // The same, spelled out.
		"requirements.txt",      // Plain text, and here not even prose.
		"paper.tex",             // Typesetting markdown does not replace.
		"index.html", "old.htm", // Rendered output, not authored prose.
		"main.go", "data.json", "photo.png", "Makefile", ".gitignore",
	} {
		t.Run(path, func(t *testing.T) {
			assert.False(t, markup.Banned(markup.Path(path)))
			assert.Empty(t, judge(path))
		})
	}
}

// TestTheDecisionIsCaseInsensitive pins that the suffix is judged lower-cased.
// A checkout on a case-insensitive filesystem spells `README.RST` and
// `README.rst` as the same file, so a rule that only knew one of them would be
// bypassed by the shift key.
func TestTheDecisionIsCaseInsensitive(t *testing.T) {
	for _, path := range []string{"README.RST", "README.Rst", "GUIDE.ADOC", "NOTES.ORG"} {
		t.Run(path, func(t *testing.T) {
			assert.True(t, markup.Banned(markup.Path(path)))
			assert.Len(t, judge(path), 1)
		})
	}
}

// TestACaseFoldedSpellingIsTheSameDocument pins the decision against the
// folding a FILESYSTEM applies, which is wider than lower-casing.
//
// This is a bypass that was demonstrated, not imagined. `ſ` (U+017F LATIN SMALL
// LETTER LONG S) folds to `s`, so on the default macOS APFS volume
// `README.rſt` and `README.rst` are one file with one inode — `cat README.rst`
// prints it — and while the rule lower-cased instead of folding, it reported
// nothing at all for that repository. Every codepoint was scanned against every
// banned suffix and U+017F is the only one that reaches one, which is exactly
// why this is asserted rather than left to the next reader to notice.
func TestACaseFoldedSpellingIsTheSameDocument(t *testing.T) {
	for _, path := range []string{"README.rſt", "guide.aſciidoc", "GUIDE.AſCIIDOC"} {
		t.Run(path, func(t *testing.T) {
			assert.True(t, markup.Banned(markup.Path(path)))
			assert.Len(t, judge(path), 1)
		})
	}
	for _, path := range []string{"README.md", "notes.txt", "guide.adoſ"} {
		t.Run(path, func(t *testing.T) {
			assert.False(t, markup.Banned(markup.Path(path)), "folding must not invent a violation")
		})
	}
}

// TestTrailingDotsAndSpacesDoNotHideAMarkup pins the one-keystroke bypass of a
// rule whose entire decision is a name.
//
// `README.rst.` and `README.rst ` are distinct files on Linux and macOS, and
// BOTH are `README.rst` the moment the repository is checked out on Windows,
// where the Win32 layer strips trailing dots and spaces. Judging the name
// exactly as written would let a document into every repository through a
// keystroke that costs nothing to type and nothing to explain away.
func TestTrailingDotsAndSpacesDoNotHideAMarkup(t *testing.T) {
	for _, path := range []string{"README.rst.", "README.rst..", "README.rst ", "README.rst . "} {
		t.Run(path, func(t *testing.T) {
			assert.True(t, markup.Banned(markup.Path(path)))
		})
	}
	for _, path := range []string{"README.md.", "README.md ", "notes.txt.."} {
		t.Run(path, func(t *testing.T) {
			assert.False(t, markup.Banned(markup.Path(path)), "trimming must not invent a violation")
		})
	}
}

// TestTheLastExtensionIsTheDocument pins which half of a double extension
// decides. `README.rst.md` is a markdown file whose stem happens to end in
// `.rst`; `README.md.rst` is a reStructuredText document however its stem
// reads. Deciding from anywhere but the final suffix gets one of the two wrong.
func TestTheLastExtensionIsTheDocument(t *testing.T) {
	assert.False(t, markup.Banned("README.rst.md"), "the file is markdown")
	assert.True(t, markup.Banned("README.md.rst"), "the file is reStructuredText")
	assert.False(t, markup.Banned("release.rst.tar.gz"), "an archive is not a document")
}

// TestAStemlessNameIsStillItsMarkup pins the deliberate decision about a file
// named exactly `.rst`, which Go's path handling calls all-extension.
//
// It is REPORTED. A hidden reStructuredText document is still one, and the
// alternative — exempting any name whose extension is the whole of it — is a
// hole written into a rule whose entire decision is the name.
func TestAStemlessNameIsStillItsMarkup(t *testing.T) {
	assert.True(t, markup.Banned(".rst"))
	assert.True(t, markup.Banned("docs/.adoc"))
	assert.False(t, markup.Banned(".md"))
}

// TestTheNameIsJudgedWhereverThePathLeads pins that only the final element
// decides. A directory named for a domain — which this fleet is built out of,
// `www.uplang.org` and `infra.xto.email` — must not make every file beneath it
// a violation, and a nested path must not hide one.
func TestTheNameIsJudgedWhereverThePathLeads(t *testing.T) {
	assert.False(t, markup.Banned(markup.Path(filepath.Join("www.uplang.org", "content", "index.md"))))
	assert.False(t, markup.Banned(markup.Path(filepath.Join("google.golang.org", "grpc", "README.md"))))
	assert.True(t, markup.Banned(markup.Path(filepath.Join("a", "b", "c", "d", "e", "deep.rst"))))
	assert.True(t, markup.Banned(markup.Path(filepath.Join("my docs", "café", "guía.rst"))),
		"spaces and non-ASCII are ordinary in a path")
}

// TestEveryDiagnosticIsNavigable pins the diagnostic's shape: the identity a runner
// softens, baselines and attributes on, the path exactly as it was handed in,
// and a position an editor can open.
func TestEveryDiagnosticIsNavigable(t *testing.T) {
	path := filepath.Join("docs", "guide.adoc")

	diags := judge(path)

	require.Len(t, diags, 1)
	assert.Equal(t, markup.Tool, diags[0].Tool)
	assert.Equal(t, markup.Rule, diags[0].Rule)
	assert.Equal(t, "yze/markup", diags[0].Rule, "the flat rule id is the exemption key and must not drift")
	assert.Equal(t, path, diags[0].Path, "reported under the path the caller gave, not a resolved one")
	assert.Equal(t, 1, diags[0].Line)
	assert.Equal(t, 1, diags[0].Col)
	assert.Equal(t, goyze.SeverityError, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "guide.adoc", "the finding names the file")
	assert.NotContains(t, diags[0].Message, "docs/", "by its own name, not the path already on the diagnostic")
}

// TestTheAnalyzerIdentityIsWhatTheSuiteCatalogs pins the constants the yze
// suite indexes this analyzer by. They are a contract with the runner, not
// decoration: [markup.Category] is what makes it run over documentation, and a
// drift here silently removes the rule from the gate.
func TestTheAnalyzerIdentityIsWhatTheSuiteCatalogs(t *testing.T) {
	assert.Equal(t, "markup", markup.Name)
	assert.Equal(t, "yze", markup.Tool)
	assert.Equal(t, "yze/markup", markup.Rule)
	assert.Equal(t, "docs", markup.Category)
}
