package markup

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEveryMarkupsEntryIsSpelledAsALookupCanFindIt pins the table's spelling against
// the only form [extensionOf] can produce.
//
// A key typed as `RST`, `rst` or `.RST` sits in the table looking authoritative
// and never matches a single file. For a rule that has never fired on a
// first-party document, a suffix that silently cannot match is indistinguishable
// from the rule working — which is why the spelling is asserted rather than
// trusted to review.
func TestEveryMarkupsEntryIsSpelledAsALookupCanFindIt(t *testing.T) {
	for key, entry := range markups {
		t.Run(string(key), func(t *testing.T) {
			assert.Equal(t, strings.ToLower(string(key)), string(key), "keys are lower-cased")
			assert.True(t, strings.HasPrefix(string(key), "."), "keys carry their leading dot")
			assert.Equal(t, key, extensionOf(Path("document"+string(key))),
				"a file with this suffix must reach this entry")
			assert.NotEmpty(t, entry.dialect, "every entry names the markup it stands for")
		})
	}
}

// TestTheBannedSetIsExactlyWhatWasDecided pins the table against its own proof.
//
// The external suite asserts that each of these is reported and names its
// dialect; this asserts there is nothing ELSE. Without it a suffix could be
// banned by a one-line edit with no test behind it, and the first anyone would
// learn of it is a repository failing its gate over a document nobody agreed to
// ban — the opposite failure to the one this rule was built for, and a worse
// one, because it fires.
func TestTheBannedSetIsExactlyWhatWasDecided(t *testing.T) {
	var banned, allowed []string
	for key, entry := range markups {
		if entry.isBanned {
			banned = append(banned, string(key))
			continue
		}
		allowed = append(allowed, string(key))
	}
	sort.Strings(banned)
	sort.Strings(allowed)

	assert.Equal(t, []string{
		".adoc", ".asciidoc", ".creole", ".mediawiki", ".org",
		".pod", ".rdoc", ".rst", ".textile", ".wiki",
	}, banned, "the markups that compete with markdown as prose")
	assert.Equal(t, []string{
		".htm", ".html", ".markdown", ".md", ".tex", ".txt",
	}, allowed, "considered, and deliberately left alone")
}

// TestJudgedNameSurvivesTheDegenerateNames pins the trimming against the names
// that trim to nothing. A path made entirely of the characters Windows discards
// must produce no extension at all rather than an accidental match, and the
// empty path must not panic.
func TestJudgedNameSurvivesTheDegenerateNames(t *testing.T) {
	for _, path := range []string{"", ".", "..", "...", "  ", "/", ". . ."} {
		t.Run(path, func(t *testing.T) {
			assert.Empty(t, string(extensionOf(Path(path))))
			assert.False(t, Banned(Path(path)))
		})
	}
}
