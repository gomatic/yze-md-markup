package markup

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	goyze "github.com/gomatic/go-yze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bannedPaths is n distinct banned documents, named so each is identifiable in
// a truncated report.
func bannedPaths(n findingCount) []string {
	paths := make([]string, 0, n)
	for i := findingCount(0); i < n; i++ {
		paths = append(paths, "docs/guide-"+strconv.Itoa(int(i))+".rst")
	}
	return paths
}

// TestARunPastTheReportLimitStillNamesWhatItFound pins the truncation: the report
// stops collecting, says so, and carries the TRUE total.
//
// The count is the point. A run that summed what it collected would report its
// own limit back as the number of findings — in the very sentence that claims
// to name how many there are — and an author reading "10000 findings, of which
// 10000 are reported" has been told the truncation did not happen.
func TestARunPastTheReportLimitStillNamesWhatItFound(t *testing.T) {
	over := reportLimit + 3
	found := goyze.Expansion{Names: bannedPaths(over)}

	report := Report(found)

	require.Len(t, report.Diagnostics, int(reportLimit)+1, "the limit, plus the notice that stands for the rest")
	notice := report.Diagnostics[len(report.Diagnostics)-1]
	assert.Contains(t, notice.Message, fmt.Sprintf("%d banned-markup findings across this run", over),
		"the true total, not the number printed")
	assert.Contains(t, notice.Message, fmt.Sprintf("of which %d are reported", reportLimit))
	assert.Equal(t, "docs/guide-"+strconv.Itoa(int(reportLimit))+".rst", notice.Path,
		"attributed to the file the run stopped at, so a runner can place it")
	assert.Equal(t, Rule, notice.Rule, "the notice is this rule's, and must be attributable like any finding")
}

// TestFindingsPastTheLimitAreCountedFromBothHalvesOfARun pins that a path the
// walk could not enter is counted after truncation exactly as a file is. Both
// halves go through one counter for this reason: counting only the half that
// was still being collected is how a total silently becomes a guess.
func TestFindingsPastTheLimitAreCountedFromBothHalvesOfARun(t *testing.T) {
	found := goyze.Expansion{
		Names:      bannedPaths(reportLimit + 1),
		Unreadable: []string{"docs/locked", "docs/hidden.rst"},
	}

	report := Report(found)

	notice := report.Diagnostics[len(report.Diagnostics)-1]
	assert.Contains(t, notice.Message, fmt.Sprintf("%d banned-markup findings across this run", reportLimit+3),
		"everything found is counted, whether or not it was collected")
}

// TestTheTotalCountsFindingsRatherThanPaths pins what the truncation notice's
// number MEANS, which every other corpus here leaves free.
//
// The other truncation tests hand [Report] nothing but banned names, so one
// finding per path and one increment per path are the same number and the claim
// at [collecting.add] — that the total is what the run FOUND — has nothing
// behind it. A mutation replacing the count of findings with a count of paths
// survives them all. Here the clean names outnumber the banned ones, so a run
// that counted paths would announce a total half again as large as the number of
// documents anybody has to fix, in the very sentence that claims to name it.
func TestTheTotalCountsFindingsRatherThanPaths(t *testing.T) {
	banned := bannedPaths(reportLimit + 1)
	clean := make([]string, 0, reportLimit)
	for i := findingCount(0); i < reportLimit; i++ {
		clean = append(clean, "docs/notes-"+strconv.Itoa(int(i))+".md")
	}
	found := goyze.Expansion{Names: append(banned, clean...)}

	report := Report(found)

	notice := report.Diagnostics[len(report.Diagnostics)-1]
	assert.Contains(t, notice.Message, fmt.Sprintf("%d banned-markup findings across this run", reportLimit+1),
		"the markdown files are paths this run judged, not findings it made")
}

// TestARunUnderItsLimitCarriesNoNotice pins the ordinary case: nothing is
// announced when nothing was dropped.
func TestARunUnderItsLimitCarriesNoNotice(t *testing.T) {
	report := Report(goyze.Expansion{Names: bannedPaths(3)})

	require.Len(t, report.Diagnostics, 3)
	for _, d := range report.Diagnostics {
		assert.NotContains(t, d.Message, "across this run")
	}
}

// TestACleanRunCollectsNothingAndStillEncodesAsAnArray pins the shape of this
// rule's ORDINARY output.
//
// It is clean in 715 of the fleet's 716 repositories, so the clean report is
// the document it emits most, and a nil slice would make that document
// `{"diagnostics":null}` — a different shape from every sibling analyzer's
// `{"diagnostics":[]}`, and one a consumer iterating the field fails on. The
// JSON is asserted, not the slice, because the JSON is what leaves the process.
func TestACleanRunCollectsNothingAndStillEncodesAsAnArray(t *testing.T) {
	for name, report := range map[string]goyze.Report{
		"a run given nothing":      collecting{}.done(),
		"a run finding nothing":    collecting{}.add("README.md", examined).done(),
		"an expansion of markdown": Report(goyze.Expansion{Names: []string{"README.md"}}),
	} {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, report.Diagnostics)

			encoded, err := json.Marshal(report)

			require.NoError(t, err)
			assert.JSONEq(t, `{"diagnostics":[]}`, string(encoded))
		})
	}
}
