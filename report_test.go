package markup_test

import (
	"testing"

	goyze "github.com/gomatic/go-yze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	markup "github.com/gomatic/yze-md-markup"
)

// paths is every path a report named, in the order it named them.
func paths(report goyze.Report) []string {
	named := make([]string, 0, len(report.Diagnostics))
	for _, d := range report.Diagnostics {
		named = append(named, d.Path)
	}
	return named
}

// TestAnEmptyExpansionIsAnEmptyReport pins that a walk that found nothing to
// judge says so. This rule finds no banned document anywhere in the fleet as it
// stands, so the clean report is its ordinary output rather than an edge case.
func TestAnEmptyExpansionIsAnEmptyReport(t *testing.T) {
	assert.Empty(t, markup.Report(goyze.Expansion{}).Diagnostics)
	assert.Empty(t, markup.Report(goyze.Expansion{Names: []string{"README.md", "main.go"}}).Diagnostics)
}

// TestOnlyTheBannedFilesAreReported pins that the report applies the rule
// again rather than trusting what it was handed. The command's walk claims only
// banned suffixes, so this can only be exercised deliberately — and a [Report]
// that reported whatever arrived would be a rule that any other caller, and any
// future change to the walk's claim, could turn into a report about every
// markdown file in the repository.
func TestOnlyTheBannedFilesAreReported(t *testing.T) {
	found := goyze.Expansion{Names: []string{"README.md", "docs/guide.adoc", "main.go", "notes.rst"}}

	report := markup.Report(found)

	assert.Equal(t, []string{"docs/guide.adoc", "notes.rst"}, paths(report),
		"the banned files, in the order the walk found them")
}

// TestEverySpellingOfOneDocumentIsItsOwnFinding pins the question this report
// asks of an expansion.
//
// A hard link and an aliasing symlink are ONE file under two names, and both
// names are violations: deleting the reported one leaves the other in place,
// still a banned document, now with a green gate. The expansion's file list
// collapses them by design — one spelling per inode, which is right for a rule
// that reads bytes — so this report reads its NAME list, where each spelling
// stands on its own.
//
// The same expansion's file half must not be a second source of findings. Names
// is a superset of Files, so reading both would report every ordinary document
// twice — the duplicate this rule's whole aggregation exists to avoid.
func TestEverySpellingOfOneDocumentIsItsOwnFinding(t *testing.T) {
	found := goyze.Expansion{
		Names: []string{"guide.rst", "notes.adoc", "alias.rst"},
		Files: []string{"guide.rst"},
	}

	report := markup.Report(found)

	assert.Equal(t, []string{"guide.rst", "notes.adoc", "alias.rst"}, paths(report),
		"three names, three findings, and the file list is not a fourth")
}

// TestAnUnexaminablePathIsReportedRatherThanLost pins the blind spot. A
// directory nobody could enumerate is exactly where a banned document sits
// unreported, and a gate that says nothing about it is indistinguishable from
// one that looked and found nothing.
func TestAnUnexaminablePathIsReportedRatherThanLost(t *testing.T) {
	found := goyze.Expansion{Unreadable: []string{"docs/locked"}}

	report := markup.Report(found)

	require.Len(t, report.Diagnostics, 1)
	assert.Equal(t, "docs/locked", report.Diagnostics[0].Path)
	assert.Contains(t, report.Diagnostics[0].Message, "could not be looked inside")
	assert.Contains(t, report.Diagnostics[0].Message, "unaccounted for")
}

// TestABannedNameIsConvictedEvenWhenNothingCouldReadIt pins this rule's
// defining property.
//
// The decision is the path, so a `README.rst` behind a permission error, a
// FIFO, or a link resolving to nothing is as certainly a reStructuredText
// document as one the walk read. A rule that must open a file to decide can
// only report such a path as an unknown; this one already knows, and reporting
// it as an unknown would bury the finding under a caveat that does not apply.
func TestABannedNameIsConvictedEvenWhenNothingCouldReadIt(t *testing.T) {
	found := goyze.Expansion{Unreadable: []string{"docs/README.rst"}}

	report := markup.Report(found)

	require.Len(t, report.Diagnostics, 1)
	assert.Contains(t, report.Diagnostics[0].Message, "is a document written in reStructuredText")
	assert.NotContains(t, report.Diagnostics[0].Message, "could not be looked inside",
		"the name convicts it; nothing about it is unknown")
}

// TestEveryPathOfARunIsJudgedOnce pins that both halves of an expansion reach
// the report, each exactly once. Splitting them across two entry points is how
// the sibling analyzer lost a finding: one half was appended past a limit the
// other half had already consumed.
func TestEveryPathOfARunIsJudgedOnce(t *testing.T) {
	found := goyze.Expansion{
		Names:      []string{"README.md", "notes.rst"},
		Unreadable: []string{"locked", "hidden.adoc"},
	}

	report := markup.Report(found)

	assert.Equal(t, []string{"notes.rst", "locked", "hidden.adoc"}, paths(report))
}
