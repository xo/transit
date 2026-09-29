package transit_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// These tests read the repository and not the package. They hold the agent
// setup that D2 adopts from dbmeta D110, and the decision log that D5 keeps
// one file for each decision, as dbmeta D111 does.

// markdownLink matches a relative link, and leaves an absolute one alone.
var markdownLink = regexp.MustCompile(`\]\((?:\./)?([^)#:]+\.md)(#[^)]*)?\)`)

// TestEveryMarkdownLinkResolves makes sure that each relative link in a
// document names a file that exists.
func TestEveryMarkdownLinkResolves(t *testing.T) {
	t.Parallel()
	for _, path := range repoFiles(t, ".md") {
		dir := filepath.Dir(path)
		for _, m := range markdownLink.FindAllStringSubmatch(read(t, path), -1) {
			target := filepath.Join(dir, m[1])
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s: the link to %s does not resolve to %s", path, m[1], target)
			}
		}
	}
}

// decisionRef matches a reference to a decision by its number, such as D3.
var decisionRef = regexp.MustCompile(`\bD([1-9][0-9]*)\b`)

// repositories are the names that can come before a decision of another
// repository, as in "dbmeta D110".
var repositories = map[string]bool{
	"blitz": true, "dbimp": true, "dbmeta": true, "dbtpl": true, "dburl": true,
	"magic": true, "resvg": true, "rline": true, "tblfmt": true, "usql": true,
}

// TestEveryDecisionReferenceExists makes sure that a bare decision number in a
// document or a Go file is a decision in docs/decisions. A reference to a
// decision of another repository names that repository first, as in
// "dbmeta D110", and this test skips it.
func TestEveryDecisionReferenceExists(t *testing.T) {
	t.Parallel()
	written := make(map[string]bool)
	for _, d := range decisions(t) {
		written[d.num] = true
	}
	for _, path := range repoFiles(t, ".md", ".go") {
		body := read(t, path)
		for _, m := range decisionRef.FindAllStringSubmatchIndex(body, -1) {
			num := body[m[2]:m[3]]
			if written[num] {
				continue
			}
			// a code point in hex, such as \x{D800} in a test of regex
			// syntax, is not a decision
			if m[0] > 0 && body[m[0]-1] == '{' {
				continue
			}
			before := strings.Fields(body[max(0, m[0]-40):m[0]])
			if len(before) != 0 && repositories[strings.Trim(before[len(before)-1], "(`\"")] {
				continue
			}
			t.Errorf("%s: names D%s, which is not a decision in docs/decisions. "+
				"A decision of another repository names that repository, as in dbmeta D110", path, num)
		}
	}
}

// planned are the tests that a document names before they are written. Each
// one names the phase that writes it. No test waits today.
var planned = map[string]string{}

// TestEveryTestNameInTheDocsExists makes sure that a test that a document
// names is a test that is written. A rule is often paired with the test that
// holds it, and a renamed test breaks the pair without a sound.
func TestEveryTestNameInTheDocsExists(t *testing.T) {
	t.Parallel()
	written := make(map[string]bool)
	for _, path := range repoFiles(t, ".go") {
		for _, m := range regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9]+)\(`).
			FindAllStringSubmatch(read(t, path), -1) {
			written[m[1]] = true
		}
	}
	var named int
	for _, path := range repoFiles(t, ".md") {
		for _, name := range regexp.MustCompile(`\bTest[A-Z][A-Za-z0-9]*`).
			FindAllString(read(t, path), -1) {
			named++
			if !written[name] && planned[name] == "" {
				t.Errorf("%s: names %s, which no test defines. Renaming a test "+
					"means fixing the documents that tell a reader to trust it", path, name)
			}
		}
	}
	for name, phase := range planned {
		if written[name] {
			t.Errorf("%s is written now. Remove it from planned, which holds it for %s", name, phase)
		}
	}
	if named == 0 {
		t.Error("no document names a test any more, so this guards nothing")
	}
}

// decision is one file in docs/decisions.
type decision struct {
	num    string
	title  string
	status string
	file   string
}

// decisionFile names a decision file: D, the number in three digits, and the
// title in lower case words joined by hyphens.
var decisionFile = regexp.MustCompile(`^D(\d{3})-[a-z0-9-]+\.md$`)

// decisions reads every decision in docs/decisions, in order. Each opens with
// its number, its title and its status:
//
//	# D5. Each decision is a file of its own
//
//	Status: Decided.
func decisions(t *testing.T) []decision {
	t.Helper()
	dir := filepath.Join("docs", "decisions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	head := regexp.MustCompile(`\A# D(\d+)\. (.+)\n\nStatus: (.+)\.\n`)
	var out []decision
	for _, e := range entries {
		if e.Name() == "README.md" {
			continue
		}
		name := decisionFile.FindStringSubmatch(e.Name())
		if name == nil {
			t.Errorf("%s: a decision file is named D, three digits, a hyphen and the"+
				" title in lower case words, such as D005-each-decision-is-a-file-of-its-own.md", e.Name())
			continue
		}
		m := head.FindStringSubmatch(read(t, filepath.Join(dir, e.Name())))
		if m == nil {
			t.Errorf("%s: a decision opens with \"# D<n>. <title>\", a blank line and"+
				" \"Status: <status>.\"", e.Name())
			continue
		}
		if n, _ := strconv.Atoi(name[1]); strconv.Itoa(n) != m[1] {
			t.Errorf("%s holds D%s. The file name and the heading name one decision", e.Name(), m[1])
		}
		out = append(out, decision{num: m[1], title: m[2], status: m[3], file: e.Name()})
	}
	if len(out) == 0 {
		t.Fatalf("found no decision in %s", dir)
	}
	return out
}

// TestTheDecisionIndexIsComplete makes sure that the table in
// docs/decisions/README.md matches the decision files. A reader finds a
// decision by its number in that table, so a missing row or a stale status
// hides it.
func TestTheDecisionIndexIsComplete(t *testing.T) {
	t.Parallel()
	index := read(t, filepath.Join("docs", "decisions", "README.md"))
	rows := make(map[string]string)
	for _, m := range regexp.MustCompile(`(?m)^\| \[D(\d+)\]\(.*$`).FindAllStringSubmatch(index, -1) {
		rows[m[1]] = m[0]
	}
	written := make(map[string]bool)
	for _, d := range decisions(t) {
		written[d.num] = true
		want := fmt.Sprintf("| [D%s](%s) | %s | %s |", d.num, d.file, d.title, d.status)
		switch got, ok := rows[d.num]; {
		case !ok:
			t.Errorf("D%s has no row in docs/decisions/README.md. Add:\n%s", d.num, want)
		case got != want:
			t.Errorf("D%s: the row in docs/decisions/README.md is\n%s\nand the file says\n%s", d.num, got, want)
		}
	}
	for num := range rows {
		if !written[num] {
			t.Errorf("docs/decisions/README.md has a row for D%s, and no file holds it", num)
		}
	}
}

// TestAnAmendmentPointsBothWays makes sure that a decision that amends or
// supersedes another names it in its status, and that the other names it
// back. A reader who finds the older decision first then learns that it
// changed.
func TestAnAmendmentPointsBothWays(t *testing.T) {
	t.Parallel()
	status := make(map[string]string)
	for _, d := range decisions(t) {
		status[d.num] = d.status
	}
	naming := regexp.MustCompile(`(?i)\b(?:amends|supersedes|superseded by|amended by) ((?:D\d+(?:, | and )?)+)`)
	for num, s := range status {
		for _, m := range naming.FindAllStringSubmatch(s, -1) {
			for _, other := range regexp.MustCompile(`D(\d+)`).FindAllStringSubmatch(m[1], -1) {
				if _, ok := status[other[1]]; !ok {
					t.Errorf("D%s names D%s, which is not a decision", num, other[1])
					continue
				}
				if !regexp.MustCompile(`\bD` + num + `\b`).MatchString(status[other[1]]) {
					t.Errorf("D%s says %q, and the status of D%s does not name D%s", num, s, other[1], num)
				}
			}
		}
	}
}

// TestTheCountsInProseAreRight makes sure that the documents that name the
// range of decisions name the last one, and that they agree on the number of
// the next question. A number in prose goes stale when somebody adds one.
func TestTheCountsInProseAreRight(t *testing.T) {
	t.Parallel()
	last := 0
	for _, d := range decisions(t) {
		n, _ := strconv.Atoi(d.num)
		last = max(last, n)
	}
	ranges := regexp.MustCompile(`D1\s+to\s+D(\d+)`)
	next := regexp.MustCompile(`next\s+question\s+is\s+question\s+(\d+)`)
	var found int
	nexts := map[string]string{}
	for _, path := range repoFiles(t, ".md") {
		if strings.HasPrefix(path, filepath.Join("docs", "decisions")) {
			// a decision records the range on the day that it was written
			continue
		}
		body := read(t, path)
		for _, m := range ranges.FindAllStringSubmatch(body, -1) {
			found++
			if m[1] != strconv.Itoa(last) {
				t.Errorf("%s says D1 to D%s, and the last decision is D%d", path, m[1], last)
			}
		}
		for _, m := range next.FindAllStringSubmatch(body, -1) {
			nexts[path] = m[1]
		}
	}
	if found == 0 {
		t.Error("no document names the range of decisions any more, so this guards nothing")
	}
	var want string
	for path, n := range nexts {
		if want == "" {
			want = n
			continue
		}
		if n != want {
			t.Errorf("%s says the next question is question %s, and another document says %s", path, n, want)
		}
	}
}

// TestTheRootHoldsFourDocuments holds dbmeta D110, which D2 adopts. Only
// README.md, AGENTS.md, CLAUDE.md and CONTRIBUTING.md are in the root.
func TestTheRootHoldsFourDocuments(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"README.md": true, "AGENTS.md": true, "CLAUDE.md": true, "CONTRIBUTING.md": true,
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if !allowed[e.Name()] {
			t.Errorf("%s is in the repository root. Only README, AGENTS, CLAUDE and CONTRIBUTING belong there, "+
				"and every other document goes in docs/. See D2", e.Name())
		}
	}
	for name := range allowed {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("expected %s in the repository root", name)
		}
	}
}

// TestClaudeImportsAgents holds dbmeta D110, which D2 adopts. AGENTS.md holds
// the rules, and CLAUDE.md imports it, so that Claude Code and every other
// agent read the same rules. A symbolic link does not do, because a Windows
// checkout writes a link as a small text file.
func TestClaudeImportsAgents(t *testing.T) {
	t.Parallel()
	info, err := os.Lstat("CLAUDE.md")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("CLAUDE.md is a symbolic link. Make it a file that holds @AGENTS.md. See D2")
	}
	if got := strings.TrimSpace(read(t, "CLAUDE.md")); got != "@AGENTS.md" {
		t.Errorf("CLAUDE.md holds %q. It holds only @AGENTS.md, and the rules go in AGENTS.md. See D2", got)
	}
}

// TestNoSectionHeadingIsRepeated makes sure that a level two heading appears
// once in its document. Two sections with one name mean that a section landed
// in the wrong place.
func TestNoSectionHeadingIsRepeated(t *testing.T) {
	t.Parallel()
	for _, path := range repoFiles(t, ".md") {
		seen := make(map[string]bool)
		for _, m := range regexp.MustCompile(`(?m)^## (.+)$`).FindAllStringSubmatch(read(t, path), -1) {
			if seen[m[1]] {
				t.Errorf("%s has two sections called %q", path, m[1])
			}
			seen[m[1]] = true
		}
	}
}

// TestEveryDocumentIsInBothTables makes sure that each document in docs/ is
// named in the table of AGENTS.md and in the table of README.md. AGENTS.md
// says that a document that is not in its table does not exist, and this test
// keeps that true.
func TestEveryDocumentIsInBothTables(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir("docs")
	if err != nil {
		t.Fatal(err)
	}
	tables := map[string]string{
		"AGENTS.md": read(t, "AGENTS.md"),
		"README.md": read(t, "README.md"),
	}
	var found int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		found++
		for name, body := range tables {
			if !strings.Contains(body, "docs/"+e.Name()) {
				t.Errorf("%s does not name docs/%s. A document that nobody can find is "+
					"a document that nobody reads", name, e.Name())
			}
		}
	}
	if found == 0 {
		t.Error("docs/ holds no document, so this guards nothing")
	}
}

// repoFiles returns each file with one of the given extensions. It skips the
// folders whose names start with a period, such as .git and the agent skills,
// and the upstream checkout in tree-sitter/, which this repository does not
// write (D4).
func repoFiles(t *testing.T, exts ...string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && path != "." && (strings.HasPrefix(d.Name(), ".") || path == "tree-sitter"):
			return filepath.SkipDir
		case d.IsDir():
			return nil
		}
		for _, ext := range exts {
			if strings.HasSuffix(path, ext) {
				out = append(out, path)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// read returns the content of a file.
func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
