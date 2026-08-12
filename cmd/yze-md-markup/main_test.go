package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	goyze "github.com/gomatic/go-yze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	markup "github.com/gomatic/yze-md-markup"
)

// TestMain neutralises the one seam that reaches outside this process. The real
// ignore filter runs `git check-ignore`, so every test here would spawn git, and
// the filter's verdict would then depend on whatever repository the temporary
// directory happened to sit in. It fails open, so nothing would be flaky; it
// would simply never be exercised deterministically, and a unit test that shells
// out is an integration test wearing one's name.
func TestMain(m *testing.M) {
	files.CheckIgnore = func(goyze.RepoDir, []string) (map[string]bool, error) {
		return map[string]bool{}, nil
	}
	os.Exit(m.Run())
}

// swapStdout captures what the command writes, restoring the real writer after
// so tests cannot leak into one another.
func swapStdout(t *testing.T) *bytes.Buffer {
	t.Helper()
	original := stdout
	buf := &bytes.Buffer{}
	stdout = buf
	t.Cleanup(func() { stdout = original })
	return buf
}

// writeDoc puts a file at a path relative to dir, creating the parents.
func writeDoc(t *testing.T, dir, rel, content string) string {
	t.Helper()
	path := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// decode is the report the command emitted, read back the way the runner reads
// it. Asserting on the substrings of a JSON blob passes for a report that is
// well-formed by accident; the runner unmarshals it, so the tests do too.
func decode(t *testing.T, buf *bytes.Buffer) goyze.Report {
	t.Helper()
	var report goyze.Report
	require.NoError(t, json.Unmarshal(buf.Bytes(), &report))
	return report
}

// prose is a reStructuredText document — the thing this rule exists to keep out
// of a repository.
const prose = "Title\n=====\n\nA paragraph.\n"

// TestTheRunsFailures pins every way a run refuses, each by its OWN sentinel.
// A refusal that only ever reaches an exit code cannot be matched, so any one of
// these could be swapped for any other with the suite green.
func TestTheRunsFailures(t *testing.T) {
	for name, test := range map[string]struct {
		args    func(t *testing.T) []string
		wantErr error
	}{
		"nothing to analyze": {
			args:    func(*testing.T) []string { return nil },
			wantErr: markup.ErrNoPaths,
		},
		"an empty argument list": {
			args:    func(*testing.T) []string { return []string{} },
			wantErr: markup.ErrNoPaths,
		},
		"a path that is not there": {
			args:    func(t *testing.T) []string { return []string{filepath.Join(t.TempDir(), "absent.rst")} },
			wantErr: fs.ErrNotExist,
		},
		"a named FIFO, which reading would hang on": {
			args: func(t *testing.T) []string {
				pipe := filepath.Join(t.TempDir(), "README.rst")
				require.NoError(t, syscall.Mkfifo(pipe, 0o600))
				return []string{pipe}
			},
			wantErr: goyze.ErrNotRegularFile,
		},
		"a tree it can read": {
			args:    func(t *testing.T) []string { return []string{t.TempDir()} },
			wantErr: nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			swapStdout(t)
			args := test.args(t)

			err := report(args)

			assert.ErrorIs(t, err, test.wantErr)
		})
	}
}

// TestRunEmitsReportForDirectory pins the ordinary invocation: a tree is walked
// and its findings reach stdout as the report the runner consumes.
func TestRunEmitsReportForDirectory(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, dir, "docs/guide.rst", prose)
	buf := swapStdout(t)

	require.Equal(t, 0, run([]string{dir}))

	report := decode(t, buf)
	require.Len(t, report.Diagnostics, 1)
	assert.Equal(t, markup.Rule, report.Diagnostics[0].Rule)
	assert.Equal(t, filepath.Join(dir, "docs/guide.rst"), report.Diagnostics[0].Path)
	assert.Contains(t, report.Diagnostics[0].Message, "is a document written in reStructuredText")
}

// TestACleanTreeReportsNothing pins this rule's ordinary answer over this fleet,
// which holds no first-party document in a banned markup: an empty report and a
// zero exit, not a silence the runner has to interpret.
func TestACleanTreeReportsNothing(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, dir, "README.md", "# Title\n")
	writeDoc(t, dir, "requirements.txt", "pytest\n")
	writeDoc(t, dir, "main.go", "package main\n")
	buf := swapStdout(t)

	require.Equal(t, 0, run([]string{dir}))

	assert.Empty(t, decode(t, buf).Diagnostics)
	// The BYTES, because they are what the runner reads. An empty run that
	// emitted `{"diagnostics":null}` would be a different document from the one
	// every sibling analyzer emits, and this rule emits the clean report more
	// often than any other.
	assert.JSONEq(t, `{"diagnostics":[]}`, buf.String())
}

// TestTheCommandOpensNothing pins this rule's shape at the only place it can be
// broken: the filesystem.
//
// The decision is the path, so a run must reach its answer without opening a
// single file. Nothing else in the suite is like this — every sibling reads —
// and the day someone adds a read to "just check the first line", every claim
// this module makes about its cost, its size bound and its UTF-8 handling stops
// being true at once. The counter is what notices.
func TestTheCommandOpensNothing(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, dir, "guide.rst", prose)
	writeDoc(t, dir, "README.md", "# Title\n")
	opened := 0
	original := files.Open
	files.Open = func(path string) (io.ReadCloser, error) {
		opened++
		return original(path)
	}
	t.Cleanup(func() { files.Open = original })
	buf := swapStdout(t)

	require.Equal(t, 0, run([]string{dir}))

	assert.Zero(t, opened, "a read here would be a rule that reads")
	assert.Len(t, decode(t, buf).Diagnostics, 1, "and it decided correctly without one")
}

// TestANamedFileIsJudgedVerbatim pins that naming a file works, and that naming
// one this rule does not judge is a clean pass rather than a finding. The
// analyzer's claim applies to a named path too: handing a markdown file to this
// rule must not produce an error-severity finding about a file that is not a
// competing markup.
func TestANamedFileIsJudgedVerbatim(t *testing.T) {
	dir := t.TempDir()
	banned := writeDoc(t, dir, "README.rst", prose)
	allowed := writeDoc(t, dir, "README.md", "# Title\n")
	buf := swapStdout(t)

	require.Equal(t, 0, run([]string{banned, allowed}))

	report := decode(t, buf)
	require.Len(t, report.Diagnostics, 1)
	assert.Equal(t, banned, report.Diagnostics[0].Path)
}

// TestAnUnreadableTreeIsReportedRatherThanLost pins that the command carries
// what the walk could not enter. A directory nobody could enumerate is exactly
// where a banned document sits unreported, and every other file keeps its
// findings.
func TestAnUnreadableTreeIsReportedRatherThanLost(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, dir, "guide.rst", prose)
	locked := filepath.Join(dir, "locked")
	require.NoError(t, os.Mkdir(locked, 0o750))
	original := files.WalkDir
	files.WalkDir = deniedTree(locked)
	t.Cleanup(func() { files.WalkDir = original })
	buf := swapStdout(t)

	require.Equal(t, 0, run([]string{dir}))

	report := decode(t, buf)
	require.Len(t, report.Diagnostics, 2)
	assert.Equal(t, filepath.Join(dir, "guide.rst"), report.Diagnostics[0].Path,
		"the document keeps its finding")
	assert.Equal(t, locked, report.Diagnostics[1].Path)
	assert.Contains(t, report.Diagnostics[1].Message, "could not be looked inside",
		"and the tree is reported rather than passed over")
}

// deniedTree walks for real but reports one path as unreadable.
//
// The failure is injected through the SEAM rather than with chmod, because a
// test that removes read permission does not test what it says on a machine
// whose user ignores permissions: the gate's own CI container runs as ROOT.
func deniedTree(at string) func(string, fs.WalkDirFunc) error {
	return func(root string, walk fs.WalkDirFunc) error {
		return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if path == at {
				return walk(path, entry, os.ErrPermission)
			}
			return walk(path, entry, err)
		})
	}
}

// TestMainExits pins the entry point's wiring: main runs the command and exits
// with its status rather than swallowing it.
func TestMainExits(t *testing.T) {
	original, originalArgs := osExit, os.Args
	code := -1
	osExit = func(status int) { code = status }
	os.Args = []string{"yze-md-markup", filepath.Join(t.TempDir(), "absent.rst")}
	t.Cleanup(func() { osExit, os.Args = original, originalArgs })
	// Captured, so a deliberate failure does not print into the gate's own
	// output, where a reader has to work out whether the tool broke.
	said := swapStderr(t)

	main()

	assert.Contains(t, said.String(), "yze-md-markup:")

	assert.Equal(t, 1, code)
}
