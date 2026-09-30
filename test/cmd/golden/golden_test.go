package main

import (
	"encoding/json"
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
