package main

// Every NAME a walk reached, and what a path nobody could look inside means to a
// rule that never looks inside anything. These are the escapes an adversarial
// review proved against this analyzer: a link is a committable, cloneable entry
// (mode 120000), so each of them was a durable way to hold a banned document in
// a repository with the gate green.

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestASymlinkedDocumentIsJudgedByTheNameItIsPublishedUnder pins that
// publishing a document through a link does not launder it, in BOTH directions.
// The link's own name is what the repository holds and what a reader opens, so
// a `guide.adoc` pointing at anything at all is an AsciiDoc document here — and
// an ordinary `readme.md` link to an innocent document is not, which is what
// keeps this from being a rule about links.
func TestASymlinkedDocumentIsJudgedByTheNameItIsPublishedUnder(t *testing.T) {
	dir := t.TempDir()
	target := writeDoc(t, dir, "content/source.md", "# Title\n")
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "guide.adoc")))
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "readme.md")))

	assert.Equal(t, []string{"guide.adoc"}, reported(t, dir))
}

// TestEveryNameOfOneDocumentIsItsOwnFinding pins the escape that made this rule
// bypassable by anyone who could commit a link.
//
// A hard link and an aliasing symlink are ONE file with two directory entries.
// The walk deduplicates FILES by identity — right for a rule that reads bytes,
// where one inode analyzed twice reports one defect as two — and that dropped
// every name but one, so deleting the reported name left the other in place with
// the gate green. Both names are the violation, so both are reported: the rule's
// finding IS the name, and a symlink survives a clone as mode 120000.
func TestEveryNameOfOneDocumentIsItsOwnFinding(t *testing.T) {
	dir := t.TempDir()
	target := writeDoc(t, dir, "guide.rst", prose)
	require.NoError(t, os.Link(target, filepath.Join(dir, "notes.adoc")))
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "alias.rst")))

	assert.ElementsMatch(t, []string{"guide.rst", "notes.adoc", "alias.rst"}, reported(t, dir),
		"removing any one of the three leaves a banned document in the tree")
}

// TestATreeBehindALinkIsReportedRatherThanSilentlySkipped pins the widest of the
// three escapes. A link to a directory is neither descended nor a file, so it
// fell through the walk in silence — and one entry hid an UNBOUNDED number of
// documents. The walk now names what it declines to enter, and this rule calls
// it what it is: a tree it could not look inside, which is not the same answer
// as clean.
func TestATreeBehindALinkIsReportedRatherThanSilentlySkipped(t *testing.T) {
	dir, elsewhere := t.TempDir(), t.TempDir()
	writeDoc(t, elsewhere, "hidden.rst", prose)
	writeDoc(t, elsewhere, "deeper/also-hidden.adoc", prose)
	require.NoError(t, os.Symlink(elsewhere, filepath.Join(dir, "docs")))
	buf := swapStdout(t)

	require.Equal(t, 0, run([]string{dir}))

	diags := decode(t, buf).Diagnostics
	require.Len(t, diags, 1)
	assert.Equal(t, filepath.Join(dir, "docs"), diags[0].Path)
	assert.Contains(t, diags[0].Message, "could not be looked inside",
		"the documents behind it were never named, and silence would say there are none")
}

// TestABannedNameOnATreeItCouldNotEnterIsStillConvictedByItsName pins the
// ordering [isOpaque] depends on. `GUIDE.rst -> ../docs` is both a tree this
// walk declined to enter and a reStructuredText document by name, and the name
// is the actionable answer: the author deletes the entry either way, and burying
// that under "could not be looked inside" would hide a certainty behind a
// caveat.
func TestABannedNameOnATreeItCouldNotEnterIsStillConvictedByItsName(t *testing.T) {
	dir, elsewhere := t.TempDir(), t.TempDir()
	require.NoError(t, os.Symlink(elsewhere, filepath.Join(dir, "GUIDE.rst")))
	buf := swapStdout(t)

	require.Equal(t, 0, run([]string{dir}))

	diags := decode(t, buf).Diagnostics
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "is a document written in reStructuredText")
	assert.NotContains(t, diags[0].Message, "could not be looked inside",
		"the name convicts it; nothing about it is unknown")
}

// linkChainPastTheKernelLimit is how many links deep a chain must be for the
// kernel to refuse to resolve it. The real limit is well under this on every
// platform the gate runs on, and overshooting costs nothing.
const linkChainPastTheKernelLimit = 40

// TestATreeNobodyCanFollowIsABlindSpotRatherThanACleanName pins the arm that was
// a working bypass.
//
// A link the walk named and NOBODY can follow is not a link to nothing. The
// documents behind it are there; the gate simply cannot see their names, which
// is the definition of a blind spot. Reading every stat failure as "not there"
// judged such a link on its own innocent name and cleared it, so a tree of
// banned documents behind `docs -> <unfollowable>` produced no finding at all
// while the identical tree reached without a link was reported.
//
// The chain is built past the kernel's link limit rather than with chmod,
// because the gate's own CI container runs as ROOT, where a permission bit does
// not bite and the test would assert nothing on the machine that matters. This
// failure is refused for everyone.
func TestATreeNobodyCanFollowIsABlindSpotRatherThanACleanName(t *testing.T) {
	dir, elsewhere := t.TempDir(), t.TempDir()
	writeDoc(t, elsewhere, "README.rst", prose)
	writeDoc(t, elsewhere, "GUIDE.adoc", prose)
	deepest := filepath.Join(elsewhere, "hop0")
	require.NoError(t, os.Symlink(elsewhere, deepest))
	for hop := 1; hop < linkChainPastTheKernelLimit; hop++ {
		next := filepath.Join(elsewhere, "hop"+strconv.Itoa(hop))
		require.NoError(t, os.Symlink(deepest, next))
		deepest = next
	}
	require.NoError(t, os.Symlink(deepest, filepath.Join(dir, "docs")))
	buf := swapStdout(t)

	require.Equal(t, 0, run([]string{dir}))

	diags := decode(t, buf).Diagnostics
	require.Len(t, diags, 1)
	assert.Equal(t, filepath.Join(dir, "docs"), diags[0].Path)
	assert.Contains(t, diags[0].Message, "could not be looked inside",
		"a tree nobody can follow is unread, not clean")
}

// TestAPathThatExistsAndIsNotATreeIsJudgedByItsName pins the other side of the
// same predicate. A FIFO inside a walked tree is an entry nothing can READ, so
// the walk hands it back as unreadable — but it exists, it is not a directory,
// and its name was read out loud, which is this rule's whole decision. Calling
// it a blind spot would announce an unknown about a path the gate knows exactly
// as much about as any other.
func TestAPathThatExistsAndIsNotATreeIsJudgedByItsName(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, syscall.Mkfifo(filepath.Join(dir, "notes.rst"), 0o600))
	require.NoError(t, syscall.Mkfifo(filepath.Join(dir, "op-run"), 0o600))
	buf := swapStdout(t)

	require.Equal(t, 0, run([]string{dir}))

	diags := decode(t, buf).Diagnostics
	require.Len(t, diags, 1)
	assert.Equal(t, filepath.Join(dir, "notes.rst"), diags[0].Path)
	assert.Contains(t, diags[0].Message, "is a document written in reStructuredText",
		"the name convicts one and clears the other; neither is an unknown")
}

// walkDeadline bounds the run in [TestALinkGoingNowhereOrRoundInCirclesStillTerminates].
// It is far longer than the walk it bounds, because what it must catch is a run
// that never ends at all rather than a slow one.
const walkDeadline = 30 * time.Second

// TestALinkGoingNowhereOrRoundInCirclesStillTerminates pins that a tree of links
// resolving to nothing is ANSWERED rather than hung on, and pins WHICH answer
// each shape gets.
//
// A gate that hangs is the one failure nobody can diagnose: it has no exit code,
// no output and no finding to read. Every link here is a shape that could
// produce one — a link to a path that is not there, a pair pointing at each
// other, and a link to the walk's own root.
//
// A CYCLE IS NOT A BROKEN LINK, and the two get different answers deliberately.
// "Not there" is a complete answer: nothing is behind the link, so its own name
// is the whole of what it can hide, and this rule reads names. A cycle is not an
// answer at all — the kernel stopped resolving, so what lies at the end is
// unknown, and an adversarial review built a chain past the link limit with real
// banned documents behind it. This test first asserted two findings, which was
// this rule answering "clean" about exactly what it could not determine.
func TestALinkGoingNowhereOrRoundInCirclesStillTerminates(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Symlink(filepath.Join(dir, "absent"), filepath.Join(dir, "broken.rst")))
	require.NoError(t, os.Symlink(filepath.Join(dir, "here"), filepath.Join(dir, "there")))
	require.NoError(t, os.Symlink(filepath.Join(dir, "there"), filepath.Join(dir, "here")))
	require.NoError(t, os.Symlink(dir, filepath.Join(dir, "self")))
	buf := swapStdout(t)

	finished := make(chan int, 1)
	go func() { finished <- run([]string{dir}) }()

	select {
	case code := <-finished:
		require.Equal(t, 0, code)
	case <-time.After(walkDeadline):
		t.Fatalf("the walk did not terminate within %s", walkDeadline)
	}
	diags := decode(t, buf).Diagnostics
	require.Len(t, diags, 4)
	assert.Equal(t, filepath.Join(dir, "broken.rst"), diags[0].Path)
	assert.Contains(t, diags[0].Message, "is a document written in reStructuredText",
		"a link to nothing is named by the walk, and the name is this rule's whole decision")
	for _, unresolved := range []int{1, 2, 3} {
		assert.Contains(t, diags[unresolved].Message, "could not be looked inside",
			"a link nothing can follow, and a link to a tree, are both unread rather than clean")
	}
	assert.Equal(t, []string{
		filepath.Join(dir, "here"), filepath.Join(dir, "self"), filepath.Join(dir, "there"),
	}, []string{diags[1].Path, diags[2].Path, diags[3].Path})
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
