package transit_test

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// update tells TestTheLedgerIsComplete to add the upstream commits that the
// ledger does not hold yet:
//
//	go test -run TestTheLedgerIsComplete -update .
var update = flag.Bool("update", false, "add new upstream commits to docs/upstream/ledger.tsv")

// baseCommit is the upstream commit that phases 1 to 5 port (D30).
const baseCommit = "dcdc8cc55e5dfedfc858080835f153999a29ec40"

// ledgerPath is the ledger of upstream commits (D30).
var ledgerPath = filepath.Join("docs", "upstream", "ledger.tsv")

// ledgerHeader is the first line of the ledger.
const ledgerHeader = "upstream\tdate\tstatus\ttransit\tnote"

// The statuses of a line of the ledger, as docs/UPSTREAM.md says.
const (
	statusPorted        = "ported"
	statusNotApplicable = "not-applicable"
	statusPending       = "pending"
)

// ledgerLine is one line of the ledger.
type ledgerLine struct {
	upstream string
	date     string
	status   string
	transit  string
	note     string
}

// TestTheLedgerIsComplete holds D30. The ledger has one line for each upstream
// commit after the base commit, in order, with no gap and no line twice.
//
// The test reads the history of upstream from the checkout in tree-sitter/
// (D4). CI has no checkout, so there it tests only the form of the ledger.
func TestTheLedgerIsComplete(t *testing.T) {
	lines := readLedger(t)
	if !strings.Contains(read(t, filepath.Join("docs", "UPSTREAM.md")), baseCommit) {
		t.Errorf("docs/UPSTREAM.md does not name the base commit %s", baseCommit)
	}
	if _, err := os.Stat(filepath.Join("tree-sitter", ".git")); err != nil {
		t.Skip("tree-sitter/ is not a checkout, so the history of upstream is not here to compare")
	}
	commits := upstreamCommits(t)
	for i, l := range lines {
		switch {
		case i >= len(commits):
			t.Fatalf("%s: line %d names %s, which is not in the history of upstream after %s. "+
				"Fetch upstream in tree-sitter/ and run the test again", ledgerPath, i+2, l.upstream, baseCommit[:7])
		case l.upstream != commits[i].hash:
			t.Fatalf("%s: line %d names %s, and upstream commit %d after %s is %s. "+
				"The ledger follows the order of upstream, with no gap", ledgerPath, i+2, l.upstream, i+1, baseCommit[:7], commits[i].hash)
		case l.date != commits[i].date:
			t.Errorf("%s: line %d dates %s at %s, and upstream dates it at %s", ledgerPath, i+2, l.upstream[:7], l.date, commits[i].date)
		}
	}
	missing := commits[min(len(lines), len(commits)):]
	if len(missing) == 0 {
		return
	}
	if !*update {
		t.Fatalf("%s lacks %d upstream commits, from %s. Add them with:\n"+
			"\tgo test -run TestTheLedgerIsComplete -update .\n"+
			"Then read each line that says \"pending\" and \"review\", as docs/UPSTREAM.md says",
			ledgerPath, len(missing), missing[0].hash[:7])
	}
	f, err := os.OpenFile(ledgerPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range missing {
		status, note := sortCommit(c.paths)
		if _, err := fmt.Fprintf(f, "%s\t%s\t%s\t\t%s\n", c.hash, c.date, status, note); err != nil {
			t.Fatal(err)
		}
	}
	// a write that fails can report only on close
	if err := f.Close(); err != nil {
		t.Fatalf("writing %s: %v", ledgerPath, err)
	}
	t.Logf("added %d upstream commits to %s", len(missing), ledgerPath)
}

// readLedger reads the ledger and makes sure of its form.
func readLedger(t *testing.T) []ledgerLine {
	t.Helper()
	hash := regexp.MustCompile(`^[0-9a-f]{40}$`)
	date := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	var lines []ledgerLine
	seen := map[string]bool{}
	s := bufio.NewScanner(strings.NewReader(read(t, ledgerPath)))
	for n := 1; s.Scan(); n++ {
		if n == 1 {
			if s.Text() != ledgerHeader {
				t.Fatalf("%s: the first line is %q, and it must be %q", ledgerPath, s.Text(), ledgerHeader)
			}
			continue
		}
		cols := strings.Split(s.Text(), "\t")
		if len(cols) != 5 {
			t.Errorf("%s: line %d has %d columns, and it must have 5, separated by a tab", ledgerPath, n, len(cols))
			continue
		}
		l := ledgerLine{upstream: cols[0], date: cols[1], status: cols[2], transit: cols[3], note: cols[4]}
		switch {
		case !hash.MatchString(l.upstream):
			t.Errorf("%s: line %d: %q is not a full commit hash", ledgerPath, n, l.upstream)
		case seen[l.upstream]:
			t.Errorf("%s: line %d: %s is in the ledger twice", ledgerPath, n, l.upstream)
		case !date.MatchString(l.date):
			t.Errorf("%s: line %d: %q is not a date in the form YYYY-MM-DD", ledgerPath, n, l.date)
		case !slices.Contains([]string{statusPorted, statusNotApplicable, statusPending}, l.status):
			t.Errorf("%s: line %d: the status %q is not ported, not-applicable or pending", ledgerPath, n, l.status)
		case l.status == statusPorted && l.transit == "":
			t.Errorf("%s: line %d: %s is ported, and it names no transit commit", ledgerPath, n, l.upstream[:7])
		case l.status != statusPorted && l.note == "":
			t.Errorf("%s: line %d: %s is %s, and it gives no reason in the note", ledgerPath, n, l.upstream[:7], l.status)
		}
		seen[l.upstream] = true
		lines = append(lines, l)
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	return lines
}

// upstreamCommit is one commit of upstream and the paths that it changes.
type upstreamCommit struct {
	hash  string
	date  string
	paths []string
}

// upstreamCommits returns the commits of upstream after the base commit,
// oldest first, from the checkout in tree-sitter/. The history of upstream has
// no merges, so the order is the order of the commits.
func upstreamCommits(t *testing.T) []upstreamCommit {
	t.Helper()
	ref := "origin/master"
	if err := exec.CommandContext(t.Context(), "git", "-C", "tree-sitter", "rev-parse", "--verify", "-q", ref).Run(); err != nil {
		ref = "HEAD"
	}
	out, err := exec.CommandContext(t.Context(), "git", "-C", "tree-sitter", "log", "--reverse", "--no-merges",
		"--format=%x00%H%x09%cs", "--name-only", baseCommit+".."+ref).Output()
	if err != nil {
		t.Fatalf("reading the history of upstream in tree-sitter/: %v", err)
	}
	var commits []upstreamCommit
	for _, rec := range strings.Split(string(out), "\x00")[1:] {
		head, rest, _ := strings.Cut(rec, "\n")
		hash, date, _ := strings.Cut(head, "\t")
		c := upstreamCommit{hash: hash, date: date}
		for p := range strings.SplitSeq(rest, "\n") {
			if p = strings.TrimSpace(p); p != "" {
				c.paths = append(c.paths, p)
			}
		}
		commits = append(commits, c)
	}
	return commits
}

// The rules of docs/UPSTREAM.md, under "Where each upstream path goes". A
// commit takes the strongest rule of its paths.
const (
	ruleNotApplicable = iota
	ruleReview
	rulePort
)

// pathRule is one row of the table in docs/UPSTREAM.md. A pattern is a glob of
// the package path, where * matches within one folder, or a folder that ends
// in a slash, which matches everything under it.
type pathRule struct {
	pattern string
	rule    int
}

// pathRules holds the table of docs/UPSTREAM.md, in order. The first rule that
// matches a path wins. TestThePathRulesFollowUpstreamMD makes sure that each
// pattern is in that table.
var pathRules = []pathRule{
	{"lib/src/wasm_store.*", ruleNotApplicable},
	{"lib/src/wasm-stdlib/", ruleNotApplicable},
	{"lib/src/unicode/", ruleReview},
	{"lib/src/portable/", ruleReview},
	{"lib/src/*.c", rulePort},
	{"lib/src/*.h", rulePort},
	{"lib/include/tree_sitter/api.h", rulePort},
	{"crates/generate/src/dsl.js", ruleReview},
	{"crates/generate/src/quickjs.rs", ruleReview},
	{"crates/generate/src/", rulePort},
	{"crates/highlight/", ruleReview},
	{"crates/tags/", ruleNotApplicable},
	{"crates/cli/src/tests/", ruleReview},
	{"crates/cli/src/test.rs", rulePort},
	{"crates/cli/src/parse.rs", rulePort},
	{"crates/cli/src/query.rs", rulePort},
	{"crates/cli/", ruleNotApplicable},
	{"test/fixtures/test_grammars/", rulePort},
	{"test/fixtures/fixtures.json", rulePort},
	{"test/fixtures/error_corpus/", rulePort},
	{"test/fixtures/template_corpus/", rulePort},
	{"test/fixtures/rust_wasm_web/", ruleNotApplicable},
	{"lib/binding_rust/", ruleReview},
	{"docs/", ruleReview},
	{"lib/binding_web/", ruleNotApplicable},
	{"crates/loader/", ruleNotApplicable},
	{"crates/xtask/", ruleNotApplicable},
	{"crates/config/", ruleNotApplicable},
	{"crates/language/", ruleNotApplicable},
	{".github/", ruleNotApplicable},
	{"Cargo.*", ruleNotApplicable},
	{"flake.*", ruleNotApplicable},
	{"build.zig*", ruleNotApplicable},
	{"CMakeLists.txt", ruleNotApplicable},
	{"Makefile", ruleNotApplicable},
}

// ruleForPath returns the rule of one upstream path. A file in the root of
// upstream that no row names is not applicable, and any other path is
// reviewed, as the last two rows of the table say.
func ruleForPath(p string) int {
	for _, r := range pathRules {
		if strings.HasSuffix(r.pattern, "/") {
			if strings.HasPrefix(p, r.pattern) {
				return r.rule
			}
			continue
		}
		if ok, _ := path.Match(r.pattern, p); ok {
			return r.rule
		}
	}
	if !strings.Contains(p, "/") {
		return ruleNotApplicable
	}
	return ruleReview
}

// sortCommit gives the status and the note of a new line of the ledger, from
// the paths that the commit changes.
func sortCommit(paths []string) (string, string) {
	rule := ruleNotApplicable
	var named []string
	for _, p := range paths {
		r := ruleForPath(p)
		switch {
		case r > rule:
			rule, named = r, []string{p}
		case r == rule && len(named) < 3:
			named = append(named, p)
		}
	}
	list := strings.Join(named, ", ")
	switch rule {
	case rulePort:
		return statusPending, "port in phase 6: " + list
	case ruleReview:
		return statusPending, "review: " + list
	}
	if list == "" {
		return statusNotApplicable, "changes no file"
	}
	return statusNotApplicable, "changes only paths that transit does not port: " + list
}

// TestThePathRulesFollowUpstreamMD makes sure that each pattern of pathRules is
// in the table of docs/UPSTREAM.md, so that the ledger sorts commits by the
// rules that the document states.
func TestThePathRulesFollowUpstreamMD(t *testing.T) {
	t.Parallel()
	doc := read(t, filepath.Join("docs", "UPSTREAM.md"))
	_, table, ok := strings.Cut(doc, "## Where each upstream path goes")
	if !ok {
		t.Fatal("docs/UPSTREAM.md has no section called Where each upstream path goes")
	}
	for _, r := range pathRules {
		if !strings.Contains(table, strings.TrimSuffix(r.pattern, "/")) {
			t.Errorf("pathRules holds %q, and the table in docs/UPSTREAM.md does not name it", r.pattern)
		}
	}
	for _, c := range []struct {
		path string
		want int
	}{
		{"lib/src/parser.c", rulePort},
		{"lib/src/wasm_store.c", ruleNotApplicable},
		{"lib/src/unicode/utf8.h", ruleReview},
		{"crates/generate/src/render.rs", rulePort},
		{"crates/generate/src/build_tables/item.rs", rulePort},
		{"crates/generate/src/dsl.js", ruleReview},
		{"crates/cli/src/query.rs", rulePort},
		{"crates/cli/src/main.rs", ruleNotApplicable},
		{"crates/cli/src/tests/node_test.rs", ruleReview},
		{"README.md", ruleNotApplicable},
		{"Cargo.lock", ruleNotApplicable},
		{"test/fixtures/rust_wasm_web/Cargo.lock", ruleNotApplicable},
		{"somewhere/new.c", ruleReview},
	} {
		if got := ruleForPath(c.path); got != c.want {
			t.Errorf("ruleForPath(%q) = %d, expected %d", c.path, got, c.want)
		}
	}
}
