package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestGrammarPathsHasNoRepeats makes sure that a grammar that tree-sitter.json
// names more than once is generated once.
func TestGrammarPathsHasNoRepeats(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := `{"grammars": [{"name": "typescript", "path": "typescript"}, {"name": "tsx", "path": "tsx"},
		{"name": "tsx", "path": "tsx"}, {"name": "erb"}], "metadata": {"license": "MIT"}}`
	if err := os.WriteFile(filepath.Join(dir, "tree-sitter.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, license, err := grammarPaths(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"typescript", "tsx", "."}; !slices.Equal(paths, want) {
		t.Errorf("expected %v, got: %v", want, paths)
	}
	if license != "MIT" {
		t.Errorf("expected %q, got: %q", "MIT", license)
	}
}

// TestPortedCommitReadsUpstreamTxt makes sure that the harness wants the
// commit in upstream.txt, and the base commit while upstream.txt does not
// exist (D116).
func TestPortedCommitReadsUpstreamTxt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	got, err := portedCommit(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != baseCommit {
		t.Errorf("expected %s, got: %s", baseCommit, got)
	}
	const want = "436c42ff56b28a2016e88a09683a6cd8dbe04e7a"
	if err := os.WriteFile(filepath.Join(dir, "upstream.txt"), []byte(want+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err = portedCommit(dir); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("expected %s, got: %s", want, got)
	}
	if err := os.WriteFile(filepath.Join(dir, "upstream.txt"), []byte("436c42ff\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := portedCommit(dir); err == nil {
		t.Error("expected an error for a short hash, got none")
	}
}

// TestMergeReplacesAndSorts makes sure that a new entry replaces the old entry
// of the same repository and path, and that the record is sorted.
func TestMergeReplacesAndSorts(t *testing.T) {
	t.Parallel()
	old := []grammar{
		{Repository: "b", Path: ".", Commit: "old"},
		{Repository: "a", Path: ".", Commit: "keep"},
	}
	got := merge(old, []grammar{{Repository: "b", Path: ".", Commit: "new"}})
	want := []string{"keep", "new"}
	var commits []string
	for _, g := range got {
		commits = append(commits, g.Commit)
	}
	if !slices.Equal(commits, want) {
		t.Errorf("expected %v, got: %v", want, commits)
	}
}

// TestMergeKeepsThePackageAndTheCorpus makes sure that a new entry keeps the
// package name and the corpus result of the old entry, which the sets that
// make entries do not compute.
func TestMergeKeepsThePackageAndTheCorpus(t *testing.T) {
	t.Parallel()
	corpus := &corpusResult{Tests: 3}
	old := []grammar{{Repository: "go", Path: ".", Package: "golang", Corpus: corpus, Commit: "old"}}
	got := merge(old, []grammar{{Repository: "go", Path: ".", Package: "go", Commit: "new"}})
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got: %d", len(got))
	}
	if g := got[0]; g.Package != "golang" || g.Corpus != corpus || g.Commit != "new" {
		t.Errorf("expected the package golang, the old corpus and the commit new, got: %q %v %q", g.Package, g.Corpus, g.Commit)
	}
}

// TestReachedPaths makes sure that the markers find the paths of render.rs in
// a piece of a parser.c.
func TestReachedPaths(t *testing.T) {
	t.Parallel()
	c := []byte("static const TSLexerMode ts_lex_modes[STATE_COUNT] = {\n" +
		"  [1] = {.entry = {.count = 1, .reusable = true}}, REDUCE(sym_x, 1, -2, 3),\n" +
		"    .metadata = {\n      .major_version = 0,\n      .minor_version = 0,\n      .patch_version = 0,\n")
	got := reachedPaths(c)
	want := []string{"dynamic precedence", "ABI 15 lex modes"}
	if !slices.Equal(got, want) {
		t.Errorf("expected %v, got: %v", want, got)
	}
}

// TestReadCandidatesReadsTheWholeSet makes sure that the harness reads every
// grammar of docs/CANDIDATES.md: the 171 of the tiers, the SQL grammars that
// are not archived (D18) and the grammars for the languages of dbmeta (D23).
// A grammar that two tables name counts once.
func TestReadCandidatesReadsTheWholeSet(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "CANDIDATES.md"))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := readCandidates(string(b))
	if err != nil {
		t.Fatal(err)
	}
	n := map[string]int{}
	for _, c := range cs {
		n[c.set]++
	}
	expected := map[string]int{"tier1": 30, "tier2": 83, "tier3": 58, "sql": 8, "dbmeta": 5}
	if !maps.Equal(n, expected) {
		t.Errorf("expected the sets %v, got: %v", expected, n)
	}
	for _, c := range cs {
		if c.repo == "dhcmrlchtdj/tree-sitter-sqlite" {
			t.Error("expected the archived SQLite grammar to be left out")
		}
	}
	for _, c := range cs {
		if c.repo == "gmr/tree-sitter-postgres" && c.name == "" {
			t.Error("expected the grammars of gmr/tree-sitter-postgres to be named")
		}
	}
}

// TestCollectOutcomesFindsTheFailures makes sure that the harness records the
// path of each failed test of the summary of tree-sitter test, with the names
// of its groups, and no test with another outcome.
func TestCollectOutcomesFindsTheFailures(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"parse_results": [
		{"name": "main", "children": [
			{"name": "A", "outcome": "Passed", "parse_rate": null, "test_num": 0},
			{"name": "B", "outcome": "Failed", "parse_rate": null, "test_num": 1}
		]},
		{"name": "sub", "children": [
			{"name": "file", "children": [
				{"name": "C", "outcome": "Skipped", "parse_rate": null, "test_num": 2},
				{"name": "D / E", "outcome": "Failed", "parse_rate": null, "test_num": 3}
			]}
		]}
	]}`)
	var summary struct {
		ParseResults []json.RawMessage `json:"parse_results"`
	}
	if err := json.Unmarshal(raw, &summary); err != nil {
		t.Fatal(err)
	}
	var s testSummary
	for _, r := range summary.ParseResults {
		if err := collectOutcomes(r, "", &s); err != nil {
			t.Fatal(err)
		}
	}
	if want := []string{"main/B", "sub/file/D / E"}; !slices.Equal(s.Failing, want) {
		t.Errorf("expected %q, got: %q", want, s.Failing)
	}
	if len(s.Outcomes) != 4 {
		t.Errorf("expected 4 outcomes, got: %q", s.Outcomes)
	}
}

// phpCorpus is a corpus of two cases: A, which runs in the package of the
// first grammar of tree-sitter.json, and B, which names php_only (D83).
const phpCorpus = "===\nA\n===\n<?php 1;\n---\n(program)\n\n===\nB\n:language(php_only)\n===\n1;\n---\n(program)\n"

// TestWriteFailingFiles makes sure that the harness writes
// testdata/failing.txt of each grammar package from its entry in the record
// (D88): the failures in the order of the record, one on each line, and no
// file for a package whose entry has no failure. The entry of a package is
// the entry with its name, and of the repository of its module when two
// entries have the name, and of the repository that tree-sitter.json names
// when two of those give the folder of the module. In a module with two grammars, each package gets
// only the cases that it runs (D93).
func TestWriteFailingFiles(t *testing.T) {
	t.Parallel()
	grammars := filepath.Join(t.TempDir(), "grammars")
	failing := filepath.Join("testdata", "failing.txt")
	for name, text := range map[string]string{
		"json/go.mod":                          "module example.com/json\n",
		"json/grammar.json":                    `{"name": "json"}`,
		"typescript/go.mod":                    "module example.com/typescript\n",
		"typescript/typescript/grammar.json":   `{"name": "typescript"}`,
		"typescript/tsx/grammar.json":          `{"name": "tsx"}`,
		"typescript/tsx/" + failing:            "stale\n",
		"sql/go.mod":                           "module example.com/sql\n",
		"sql/grammar.json":                     `{"name": "sql"}`,
		"sql/tree-sitter.json":                 `{"metadata": {"links": {"repository": "git+https://github.com/B/tree-sitter-sql.git"}}}`,
		"php/go.mod":                           "module example.com/php\n",
		"php/tree-sitter.json":                 `{"grammars": [{"name": "php", "path": "php"}, {"name": "php_only", "path": "php_only"}]}`,
		"php/php/grammar.json":                 `{"name": "php"}`,
		"php/php/testdata/corpus/main.txt":     phpCorpus,
		"php/phponly/grammar.json":             `{"name": "php_only"}`,
		"php/phponly/testdata/corpus/main.txt": phpCorpus,
	} {
		p := filepath.Join(grammars, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rec := &record{Grammars: []grammar{
		{Name: "json", Repository: "https://github.com/tree-sitter/tree-sitter-json", Corpus: &corpusResult{Failing: []string{"main/B", "sub/file/D / E", "main/A"}}},
		{Name: "typescript", Repository: "https://github.com/tree-sitter/tree-sitter-typescript", Path: "typescript", Corpus: &corpusResult{Failing: []string{"x"}}},
		{Name: "tsx", Repository: "https://github.com/tree-sitter/tree-sitter-typescript", Path: "tsx", Corpus: &corpusResult{}},
		{Name: "sql", Repository: "https://github.com/a/tree-sitter-sql-a", Corpus: &corpusResult{Failing: []string{"wrong"}}},
		{Name: "sql", Repository: "https://github.com/b/tree-sitter-sql", Corpus: &corpusResult{Failing: []string{"right"}}},
		{Name: "sql", Repository: "https://github.com/c/tree-sitter-sql", Corpus: &corpusResult{Failing: []string{"other"}}},
		{Name: "php", Repository: "https://github.com/tree-sitter/tree-sitter-php", Path: "php", Corpus: &corpusResult{Failing: []string{"main/A", "main/B"}}},
		{Name: "php_only", Repository: "https://github.com/tree-sitter/tree-sitter-php", Path: "php_only", Corpus: &corpusResult{Failing: []string{"main/A", "main/B"}}},
	}}
	if err := writeFailingFiles(grammars, rec); err != nil {
		t.Fatal(err)
	}
	for dir, want := range map[string]string{
		"json":                  "main/B\nsub/file/D / E\nmain/A\n",
		"typescript/typescript": "x\n",
		"typescript/tsx":        "",
		"sql":                   "right\n",
		"php/php":               "main/A\n",
		"php/phponly":           "main/B\n",
	} {
		b, err := os.ReadFile(filepath.Join(grammars, dir, failing))
		switch {
		case want == "" && !errors.Is(err, fs.ErrNotExist):
			t.Errorf("%s: expected no file, got: %q, %v", dir, b, err)
		case want != "" && (err != nil || string(b) != want):
			t.Errorf("%s: expected %q, got: %q, %v", dir, want, b, err)
		}
	}

	rec.Grammars = rec.Grammars[:1]
	if err := writeFailingFiles(grammars, rec); err == nil {
		t.Error("expected an error for a package with no entry")
	}
}
