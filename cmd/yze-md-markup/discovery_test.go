package main

// What this command claims, and nothing else. The walk itself — the symlinked
// root, the identity of a path reached two ways, the tree that cannot be read,
// the ignore filter — belongs to the shared discovery and is proven there,
// once, rather than three times in three ways that disagreed.

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	goyze "github.com/gomatic/go-yze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reported is every path a walk of dir produced a finding for.
func reported(t *testing.T, dir string) []string {
	t.Helper()
	buf := swapStdout(t)
	require.Equal(t, 0, run([]string{dir}))
	diags := decode(t, buf).Diagnostics
	named := make([]string, 0, len(diags))
	for _, d := range diags {
		rel, err := filepath.Rel(dir, d.Path)
		require.NoError(t, err)
		named = append(named, filepath.ToSlash(rel))
	}
	return named
}

// TestTheWalkClaimsOnlyTheBannedMarkups pins which files a walk stops on. The
// claim is the analyzer's own predicate, so this is also what keeps the walk
// cheap: every markdown file in a repository is declined on a map lookup and
// never stat'd, ignore-checked or deduplicated.
func TestTheWalkClaimsOnlyTheBannedMarkups(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, dir, "README.rst", prose)
	writeDoc(t, dir, "guide.ADOC", prose)
	writeDoc(t, dir, "notes.org", prose)
	writeDoc(t, dir, "README.md", "# Title\n")
	writeDoc(t, dir, "notes.txt", prose)
	writeDoc(t, dir, "index.html", "<p>rendered</p>\n")
	writeDoc(t, dir, "main.go", "package main\n")

	assert.ElementsMatch(t, []string{"README.rst", "guide.ADOC", "notes.org"}, reported(t, dir))
}

// TestTheWalkSkipsSomebodyElsesProse pins the pruning. A dependency's
// documentation is written in whatever dialect that dependency chose, and
// reporting it tells this repository to delete a file it does not own — the
// three reStructuredText files this rule can see anywhere in the fleet are all
// pip-vendored, inside a `.venv`. A Hugo theme and a nested checkout are
// somebody else's too, and `testdata` is where this family proves its analyzers
// in BOTH directions, so a fixture that MUST contain a violation is not one.
func TestTheWalkSkipsSomebodyElsesProse(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, dir, "own.rst", prose)
	writeDoc(t, dir, ".venv/lib/python3.14/site-packages/pip/_vendor/README.rst", prose)
	writeDoc(t, dir, "venv/lib/pkg/AUTHORS.rst", prose)
	writeDoc(t, dir, ".tox/py/pkg/README.rst", prose)
	writeDoc(t, dir, "node_modules/dep/README.rst", prose)
	writeDoc(t, dir, "vendor/dep/README.adoc", prose)
	writeDoc(t, dir, "themes/paper/README.adoc", prose)
	writeDoc(t, dir, "testdata/fixture.rst", prose)
	writeDoc(t, dir, ".git/description.rst", prose)
	writeDoc(t, dir, "submodule/.git/HEAD", "ref: refs/heads/main\n")
	writeDoc(t, dir, "submodule/README.rst", prose)

	assert.Equal(t, []string{"own.rst"}, reported(t, dir),
		"only this repository's own document is this repository's problem")
}

// TestADocumentIsFoundHoweverDeepAndHoweverNamed pins that nesting, spaces and
// non-ASCII names are ordinary rather than edge cases. A rule whose entire
// decision is a file's name has to survive every name a filesystem permits.
func TestADocumentIsFoundHoweverDeepAndHoweverNamed(t *testing.T) {
	dir := t.TempDir()
	deep := filepath.Join("a", "b", "c", "d", "e", "f", "guide.rst")
	spaced := filepath.Join("my docs", "release notes.adoc")
	unicode := filepath.Join("documentación", "guía.rst")
	for _, rel := range []string{deep, spaced, unicode} {
		writeDoc(t, dir, rel, prose)
	}

	assert.ElementsMatch(t,
		[]string{filepath.ToSlash(deep), filepath.ToSlash(spaced), filepath.ToSlash(unicode)},
		reported(t, dir))
}

// TestADomainNamedDirectoryIsNotADocument pins the one collision the banned
// table carries. This fleet is built out of directories called
// `www.<domain>.org` and `infra.<domain>.org`, and its git metadata is full of
// paths like `google.golang.org`: a rule that judged directory names would
// report scores of them, in every org, on its first run.
func TestADomainNamedDirectoryIsNotADocument(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, dir, filepath.Join("www.uplang.org", "content", "index.md"), "# Title\n")
	writeDoc(t, dir, filepath.Join("google.golang.org", "grpc", "README.md"), "# Title\n")

	assert.Empty(t, reported(t, dir))
}

// TestASymlinkedDocumentIsJudgedByTheNameItIsPublishedUnder pins that
// publishing a document through a link does not launder it. The link's own name
// is what the repository holds and what a reader opens, so a `guide.adoc`
// pointing at anything at all is an AsciiDoc document in this repository.
func TestASymlinkedDocumentIsJudgedByTheNameItIsPublishedUnder(t *testing.T) {
	dir := t.TempDir()
	target := writeDoc(t, dir, "content/source.md", "# Title\n")
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "guide.adoc")))

	assert.Equal(t, []string{"guide.adoc"}, reported(t, dir))
}

// TestAFoldedSpellingIsFoundOnDisk pins the fold against a real file rather
// than a string.
//
// On a case-insensitive volume — the macOS default, where most of this fleet is
// written — `README.rſt` and `README.rst` are ONE file: the shell opens it
// under either name. On a case-sensitive one they are two files and the folded
// spelling is still a reStructuredText document by any reading. Either way the
// walk must report it, and while the rule lower-cased instead of folding, a
// repository holding exactly this file reported nothing at all.
func TestAFoldedSpellingIsFoundOnDisk(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, dir, "README.rſt", prose)
	writeDoc(t, dir, "GUIDE.md", "# Title\n")

	assert.Equal(t, []string{"README.rſt"}, reported(t, dir))
}

// TestAPathThatCannotBeReadIsStillJudgedByItsName pins the narrowing this rule
// applies to what the walk could not read.
//
// A dangling symlink is a path whose own name the walk read out loud, and the
// name is this rule's whole decision — so `op-run` is clean and `notes.rst` is
// a reStructuredText document, exactly as they would be if both resolved. This
// is measured rather than theoretical: reporting every unreadable path as an
// unknown made a broken `op-run` link the ONLY finding this rule produced
// anywhere in the fleet.
func TestAPathThatCannotBeReadIsStillJudgedByItsName(t *testing.T) {
	dir := t.TempDir()
	absent := filepath.Join(dir, "nowhere")
	require.NoError(t, os.Symlink(absent, filepath.Join(dir, "op-run")))
	require.NoError(t, os.Symlink(absent, filepath.Join(dir, "notes.rst")))

	assert.Equal(t, []string{"notes.rst"}, reported(t, dir),
		"the name convicts one and clears the other; neither is an unknown")
}

// TestIsOpaqueTreatsAVanishedPathAsUnknown pins the conservative arm. A walk
// that named a path which has since gone answers neither "clean" nor "banned":
// nothing can be decided about a name nobody can confirm, and a question nobody
// could answer is not an answer of clean.
func TestIsOpaqueTreatsAVanishedPathAsUnknown(t *testing.T) {
	dir := t.TempDir()
	vanished := filepath.Join(dir, "vanished")
	original := files.WalkDir
	files.WalkDir = vanishedEntry(vanished)
	t.Cleanup(func() { files.WalkDir = original })
	buf := swapStdout(t)

	require.Equal(t, 0, run([]string{dir}))

	diags := decode(t, buf).Diagnostics
	require.Len(t, diags, 1)
	assert.Equal(t, vanished, diags[0].Path)
	assert.Contains(t, diags[0].Message, "could not be looked inside")
}

// vanishedEntry walks for real, then hands the walk one more path that is not
// there — the entry a concurrent delete removes between being listed and being
// stat'd, which no arrangement of a real tree can produce on demand.
func vanishedEntry(at string) func(string, fs.WalkDirFunc) error {
	return func(root string, walk fs.WalkDirFunc) error {
		if err := filepath.WalkDir(root, walk); err != nil {
			return err
		}
		// The callback answers a failure with fs.SkipDir, which has nothing
		// left to skip here: this is the last entry of the walk, not one inside
		// it.
		_ = walk(at, nil, os.ErrNotExist)
		return nil
	}
}

// TestTheClaimIsTheRulesOwnPredicate pins that the walk and the report cannot
// disagree about what is banned. They are one function; asking the claim
// directly is asking the rule.
func TestTheClaimIsTheRulesOwnPredicate(t *testing.T) {
	for _, path := range []string{"README.rst", "docs/guide.ADOC", "notes.org", ".rst", "README.rst."} {
		assert.True(t, claims(goyze.FilePath(path)), "%s is a banned markup", path)
	}
	for _, path := range []string{"README.md", "notes.txt", "main.go", "www.uplang.org/index.md"} {
		assert.False(t, claims(goyze.FilePath(path)), "%s is not", path)
	}
}

// TestPrunedNamesOnlyTheTreesThatAreAlwaysSomebodyElses pins the prune list
// itself, in both directions. A name added here is a tree this rule stops
// looking in — the widest possible silent pass — so each is asserted rather than
// sampled.
func TestPrunedNamesOnlyTheTreesThatAreAlwaysSomebodyElses(t *testing.T) {
	for _, name := range []string{"vendor", "node_modules", "themes", "testdata", ".venv", "venv", ".tox"} {
		assert.True(t, pruned(goyze.DirName(name)), "%s is somebody else's in every repository", name)
	}
	for _, name := range []string{"docs", "content", "internal", "cmd", ".github", "venv-tools", "themes-guide"} {
		assert.False(t, pruned(goyze.DirName(name)), "%s is this repository's own", name)
	}
}
