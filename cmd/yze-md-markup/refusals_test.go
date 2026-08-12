package main

import (
	"bytes"
	"testing"

	errs "github.com/gomatic/go-error"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	markup "github.com/gomatic/yze-md-markup"
)

// swapStderr captures what the command says when it refuses, restoring the real
// writer after so tests cannot leak into one another — nor into the gate's own
// output, where a stray refusal used to print.
func swapStderr(t *testing.T) *bytes.Buffer {
	t.Helper()
	original := stderr
	buf := &bytes.Buffer{}
	stderr = buf
	t.Cleanup(func() { stderr = original })
	return buf
}

// errWriteFailed is what a closed pipe hands back. It is a sentinel rather than
// an ad-hoc error so the test can assert that this exact cause reached the
// operator, and because the fleet standard has no room for `errors.New` — in a
// test any more than anywhere else.
const errWriteFailed errs.Const = "write failed"

// failingWriter refuses every write, standing in for a closed pipe.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWriteFailed }

// TestRunFailsWhenEncodeErrors pins that a report which cannot be written is a
// failure: exiting zero would tell the runner a check passed when its result
// never arrived.
func TestRunFailsWhenEncodeErrors(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, dir, "guide.rst", prose)
	original := stdout
	stdout = failingWriter{}
	t.Cleanup(func() { stdout = original })
	said := swapStderr(t)

	assert.Equal(t, 1, run([]string{dir}))
	assert.Contains(t, said.String(), string(errWriteFailed), "the operator is told what failed")
}

// TestARefusalNamesTheCommandAndTheCause pins what a failure SAYS, not merely
// that it happened.
//
// A gate's stderr is read by a person looking at a hundred interleaved tool
// outputs, so a line that does not name its command is noise. Nothing asserted
// this: an adversarial mutation deleted the prefix from every refusal and the
// suite stayed green.
func TestARefusalNamesTheCommandAndTheCause(t *testing.T) {
	said := swapStderr(t)

	require.Equal(t, 1, run(nil))

	assert.Contains(t, said.String(), "yze-md-markup:", "the command names itself")
	assert.Contains(t, said.String(), string(markup.ErrNoPaths), "and the refusal it made")
}
