// Package markup reports a prose document written in a markup dialect other
// than markdown.
//
// Prose in these repositories is markdown, and only markdown. A
// reStructuredText or AsciiDoc document is not a document to be parsed
// differently — it IS the violation, because a second prose dialect splits
// every convention that follows from the first: what a heading is, what a link
// is, what the docs site renders, what a reviewer can read without switching
// tools. So this rule is about EXISTENCE, and its whole decision is the file's
// name.
//
// # THIS RULE IS PREVENTIVE
//
// It detects nothing today, and it exists to keep it that way. Measured over
// the fleet on 2026-08-12 — every file under ~/src/github.com carrying one of
// the banned suffixes, excluding the off-limits account — there are exactly
// three, all reStructuredText inside Python `.venv/site-packages` trees, all
// third-party, and all already invisible to this rule because the discovery
// prunes those trees. First-party instances: ZERO.
//
// Re-measured 2026-08-12 after the shared walk began reporting the directory
// links it declines to enter: over the same 716 repositories, the whole fleet
// produces ONE diagnostic, and it is not a document. It is a blind-spot notice
// for `tsvsheet/_projects/.obsidian/plugins/tsvsheet`, a link to a tree outside
// the repository, in a repository whose `.gitignore` is a deny-all allowlist —
// the shape where the shared discovery deliberately stops honouring the filter
// rather than answer a whole run with a silent clean pass. Banned documents
// found: still ZERO.
//
// The standard this fleet is written to says to measure before mechanizing,
// because an analyzer for a rule nobody breaks costs runtime forever and
// detects nothing. This one is built anyway, deliberately, on two grounds:
//
//  1. Its cost is a map lookup on a file extension during a directory walk. It
//     opens nothing, reads nothing and parses nothing, so the runtime it costs
//     forever is a hash of a four-byte string per file the walk was already
//     visiting.
//  2. The pressure is live rather than hypothetical. The fleet contains Python
//     subprojects — `sqlrest/sqlid/py`, `uplang/up.py` — and reStructuredText
//     is the historical default for a PyPI long description, arriving with
//     whatever `README.rst` a project template writes. The zero above is a
//     state to hold, not a fact to celebrate.
//
// Do not read that measurement as more than it says: this rule has never fired
// on a file anyone here wrote.
//
// # NO PARSER, NO CONTENTS
//
// The decision is the path, so nothing in this module opens a file. That is not
// an economy, it is the rule's shape: a `README.rst` holding zero bytes, holding
// invalid UTF-8, or holding eight megabytes of legal text is equally a
// reStructuredText document in a repository whose prose is markdown.
//
// The size bound and the UTF-8 guard the sibling analyzers carry are therefore
// absent DELIBERATELY, not by omission. [goyze.BoundedRead] and
// [goyze.ErrTooLarge] exist for a rule that must read to decide; a rule that
// never reads cannot exceed a read bound, and declaring a sentinel nothing can
// return would be a contract with no behavior behind it. The shared discovery
// this analyzer walks with requires neither — its [goyze.Claim] may open a file
// and this one does not.
//
// # THE WAY OUT
//
// A repository that genuinely must ship one of these formats takes the route
// every other rule in this suite takes: the runner's per-repo exemption
// machinery, keyed on this analyzer's flat rule id ([Rule]) — a `.standards.yaml`
// entry carrying a stated reason, or a `.stickler.yaml` soft entry and baseline.
// Both are committed, reviewable, and ratcheted.
//
// This analyzer adds no way out of its own: no in-file marker, no magic comment,
// no per-directory dotfile, because a second way out is a second answer to "is
// this repository exempt" and the two drift. What it does NOT claim is that no
// other way out exists — the shared walk's tree rules are one, and an
// adversarial review demonstrated the sharpest of them: an EMPTY FILE named
// `.git` makes the walk treat a directory as a nested checkout and skip it
// entirely, while git itself never reports a path of that name in `status`,
// `ls-files` or `--ignored`. That is an unauditable exemption, it belongs to
// `go-yze`'s discovery rather than to this rule, and it is written down here so
// nobody reads the paragraph above as a guarantee it is not.
package markup

import (
	"path/filepath"
	"strings"
	"unicode"

	goyze "github.com/gomatic/go-yze"
)

// Name is the analyzer's stable identifier — the suffix of its flat rule id and
// the key the yze suite catalogs it under.
const Name = "markup"

// Tool is the suite name stamped on every diagnostic.
const Tool = "yze"

// Rule is the stable, flat rule id every diagnostic carries: "yze/" + [Name].
// It is what a runner softens, baselines, attributes and exempts on, which is
// why this rule needs no opt-out of its own.
const Rule = Tool + "/" + Name

// Category is the language group this analyzer belongs to, used by the yze
// suite to run it only when processing documentation.
const Category = "docs"

// Path is the file path stamped on each diagnostic's location.
type Path string

// extension is a file's suffix, case-folded, including its leading dot.
type extension string

// dialect is a markup language's name, as the finding calls it. The finding
// names the dialect rather than the suffix because the suffix is what the
// author sees and the dialect is what they have to stop writing.
type dialect string

// markup is one suffix's standing with this rule: what writing it means, and
// whether it may exist here.
type markup struct {
	dialect  dialect
	isBanned bool
}

// markups is the whole decision, one entry per suffix, each decided
// deliberately — including the ones that are ALLOWED. A suffix absent from this
// table is permitted by default, and that default is exactly the failure mode
// this table exists against: an extension nobody wrote down is a decision
// nobody made. Adding a markup means adding a line here with the reasoning
// beside it.
//
// Every key is case-folded and dot-led, because that is the only form
// [extensionOf] can produce, and an internal test pins the spelling of every
// entry: a key typed as `RST` or `rst` would sit in the table looking
// authoritative and never match a single file.
var markups = map[extension]markup{
	// BANNED — prose markups that compete with markdown.
	//
	// Each is a language for writing the same documents this fleet writes in
	// markdown: a README, a guide, a design note. That overlap is the whole
	// reason they cannot be here.
	".rst":       {dialect: "reStructuredText", isBanned: true},
	".adoc":      {dialect: "AsciiDoc", isBanned: true},
	".asciidoc":  {dialect: "AsciiDoc", isBanned: true},
	".textile":   {dialect: "Textile", isBanned: true},
	".mediawiki": {dialect: "MediaWiki markup", isBanned: true},
	".wiki":      {dialect: "wiki markup", isBanned: true},
	".creole":    {dialect: "Creole", isBanned: true},
	".pod":       {dialect: "Perl POD", isBanned: true},
	".rdoc":      {dialect: "RDoc", isBanned: true},
	// Emacs Org Mode. This entry is the reason the rule judges FILES and never
	// directory names: the fleet is built out of directories called
	// `www.<domain>.org` and `infra.<domain>.org`, and git metadata is full of
	// paths like `google.golang.org` — a rule reading directory names would
	// report scores of them, in every org, on its first run. Measured
	// 2026-08-12: zero FILES in the fleet end in `.org`.
	".org": {dialect: "Emacs Org Mode", isBanned: true},

	// ALLOWED — considered, and deliberately left alone.
	//
	// Each of these was a candidate. None is a competing prose markup, so
	// banning it would be this rule reaching past the standard it mechanizes.
	// They are recorded rather than omitted so the next reader can tell a
	// decision from an oversight.
	// The standard itself, in both its spellings.
	".md":       {dialect: "markdown", isBanned: false},
	".markdown": {dialect: "markdown", isBanned: false},
	// Plain text is not a markup at all, and a `.txt` is as often data as
	// prose: `requirements.txt` is a Python dependency list.
	".txt": {dialect: "plain text", isBanned: false},
	// LaTeX is a typesetting system for documents markdown does not replace —
	// a paper, a thesis, anything with a bibliography and numbered figures.
	".tex": {dialect: "LaTeX", isBanned: false},
	// HTML is rendered output rather than authored prose. `.htm` is the same
	// decision in the older spelling; absent, it would read as an oversight
	// standing next to `.html`.
	".html": {dialect: "HTML", isBanned: false},
	".htm":  {dialect: "HTML", isBanned: false},
}

// bannedMessage formats the finding.
//
// It states what the file IS and what prose here has to be, and stops there. It
// recommends no conversion tool, because the fault is not that the document was
// written with the wrong program — it is that the document is in the
// repository, and the author is the one who decides what replaces it.
//
// The dialect follows "written in" rather than sitting in front of the word
// "document", because an article in front of a name the table supplies is an
// English agreement problem no compiler catches: the first run of this rule
// produced "guide.adoc is a AsciiDoc document". Carrying an article per entry
// would put grammar in the decision table; naming the dialect after a
// preposition removes the question.
const bannedMessage = "%s is a document written in %s; prose in this repository is markdown, and only markdown, " +
	"so a document in another markup dialect must not exist here"

// Banned reports a path this rule convicts, by its name alone. It is the
// analyzer's whole decision, exported because the command's discovery claims
// files with it — one predicate, so the walk and the report cannot disagree
// about what is banned.
func Banned(at Path) bool {
	_, found := bannedDialect(at)
	return found
}

// bannedDialect is the markup a path is written in, when that markup is banned.
func bannedDialect(at Path) (dialect, bool) {
	entry, known := markups[extensionOf(at)]
	if !known || !entry.isBanned {
		return "", false
	}
	return entry.dialect, true
}

// extensionOf is the suffix this rule decides on: the final element's
// extension, case-FOLDED so every spelling a filesystem treats as one file
// arrives at one entry.
//
// Folded, not lower-cased, and the difference is a working bypass rather than a
// nicety. `strings.ToLower` is strictly narrower than the case folding a
// case-insensitive volume applies: U+017F LATIN SMALL LETTER LONG S folds to
// `s`, so on the default macOS APFS volume `README.rſt` and `README.rst` are
// ONE file — the same inode, `cat README.rst` prints it — while ToLower left the
// long s alone and the rule saw no `.rst` anywhere. An adversarial review found
// it by scanning every codepoint against every banned suffix; U+017F is the only
// one that reaches one, and folding closes the whole class rather than that
// character.
func extensionOf(at Path) extension {
	return folded(extension(filepath.Ext(judgedName(at))))
}

// folded is a suffix with every rune replaced by its case-folding
// representative: the smallest rune of its simple-folding orbit, lower-cased so
// the table's keys are written the way a path is. `S`, `s` and `ſ` share one
// orbit and all three arrive as `s`.
//
// The mapping is a literal because its signature is [strings.Map]'s rather than
// this package's, which is the one case a domain type cannot be declared for a
// parameter.
func folded(raw extension) extension {
	return extension(strings.Map(func(r rune) rune {
		smallest := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			if next < smallest {
				smallest = next
			}
		}
		return unicode.ToLower(smallest)
	}, string(raw)))
}

// judgedName is a path's final element as the FILESYSTEM will spell it, with
// the trailing dots and spaces Windows discards removed.
//
// `README.rst.` and `README.rst ` are distinct files on Linux and macOS and are
// both `README.rst` once checked out on Windows, where the Win32 layer strips
// them. Deciding on the name as written would leave a one-keystroke bypass of a
// rule whose entire decision is the name.
//
// It is [filepath.Base], not [path.Base], and that choice reaches only the
// FINDING'S WORDING, not the decision: because the base is taken first, the
// extension is read from a string with no separator left in it, where the two
// packages cannot disagree. On Windows `path.Base` would return the whole
// backslash-separated path, and the message would name `C:\src\docs\README.rst`
// where it should say `README.rst`. Neither the gate's platforms nor its tests
// can tell the two apart, so no test here kills that mutation — said plainly
// rather than left as a rationale nothing stands behind.
func judgedName(at Path) string {
	return strings.TrimRight(filepath.Base(string(at)), ". ")
}

// diagnostic builds one finding.
//
// Every finding of this rule is against a FILE rather than anything inside one
// — nothing here reads a line — so the position is always the first line and
// the first column, which is where an editor should land when the answer is to
// open the file and decide what replaces it.
func diagnostic(at Path, message finding) goyze.Diagnostic {
	return goyze.Diagnostic{
		Tool:     Tool,
		Rule:     Rule,
		Path:     string(at),
		Line:     1,
		Col:      1,
		Severity: goyze.SeverityError,
		Message:  string(message),
	}
}

// finding is one diagnostic's rendered message.
type finding string
