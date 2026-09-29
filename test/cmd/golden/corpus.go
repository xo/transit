package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// corpusResult is the result of the corpus of a grammar with the upstream C
// runtime, as tree-sitter test reports it. It is what "The gate for the Go
// backend" in docs/PLAN.md counts (D9).
type corpusResult struct {
	// Tests is the number of corpus tests that ran, and Failures the number
	// that failed, with the parser.c that the upstream tool writes.
	Tests    int `json:"tests"`
	Failures int `json:"failures"`
	// Transit is "same" when the parser.c that transit writes gives the same
	// result, and "different" when it does not.
	Transit string `json:"transit,omitempty"`
	// Error is the first line of the error when the upstream tool cannot
	// build the grammar or run its corpus, and then the grammar has no
	// result and cannot count toward the gate.
	Error string `json:"error,omitempty"`
}

// testSummary is the part of the JSON summary of tree-sitter test that
// stays the same from run to run: the name and the outcome of each test, and
// the failures. The rates and the durations change from run to run, so the
// harness leaves them out.
type testSummary struct {
	Outcomes []string
	Failures []string
	Parses   [2]int
}

// corpusGrammars runs the corpus of each recorded grammar that has one, with
// the parser.c of the upstream tool and with the parser.c of transit, and
// writes the result to the record. A grammar with no test/corpus, or one that
// the upstream tool rejects, gets no result.
func (h *harness) corpusGrammars(ctx context.Context) error {
	rec, err := h.readRecord()
	if err != nil {
		return err
	}
	transit, err := h.buildTransit(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(filepath.Dir(transit)) }()

	byRepo := map[string][]int{}
	var repos []string
	for i, g := range rec.Grammars {
		if g.Status != statusAvailable || !h.wanted(g.Name) && !h.wanted(strings.TrimPrefix(g.Repository, "https://github.com/")) {
			continue
		}
		if _, ok := byRepo[g.Repository]; !ok {
			repos = append(repos, g.Repository)
		}
		byRepo[g.Repository] = append(byRepo[g.Repository], i)
	}
	h.logf("running the corpus of %d repositories\n", len(repos))
	var mu sync.Mutex
	var failed, different []string
	err = parallel(ctx, h.jobs, repos, func(ctx context.Context, repo string) error {
		results, err := h.corpusRepo(ctx, rec, byRepo[repo], transit)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			// one repository that fails must not stop the run of the others
			h.logf("  %s: %v\n", repo, err)
			failed = append(failed, repo)
			return nil
		}
		for i, r := range results {
			g := &rec.Grammars[i]
			g.Corpus = r
			switch {
			case r == nil:
				h.logf("  %s %s: no corpus\n", repo, g.Path)
				continue
			case r.Error != "":
				h.logf("  %s %s: the upstream tool cannot run the corpus: %s\n", repo, g.Path, r.Error)
				continue
			}
			h.logf("  %s %s: %d tests, %d failures, transit %s\n", repo, g.Path, r.Tests, r.Failures, r.Transit)
			if r.Transit != "same" {
				different = append(different, g.Name)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := writeJSON(h.recordPath(), rec); err != nil {
		return err
	}
	var problems []string
	if len(failed) > 0 {
		slices.Sort(failed)
		problems = append(problems, fmt.Sprintf("the harness could not run the corpus of %d repositories: %s", len(failed), strings.Join(failed, ", ")))
	}
	if len(different) > 0 {
		slices.Sort(different)
		problems = append(problems, fmt.Sprintf("the parser.c of transit gives another corpus result for %d grammars: %s", len(different), strings.Join(different, ", ")))
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, ". "))
	}
	return nil
}

// buildTransit builds cmd/transit of the root module into a new folder, and
// returns its path. The test module does not import the root module (D12),
// so the harness runs the command.
func (h *harness) buildTransit(ctx context.Context) (string, error) {
	dir, err := os.MkdirTemp("", "golden-transit-")
	if err != nil {
		return "", fmt.Errorf("making a folder for transit: %w", err)
	}
	bin := filepath.Join(dir, "transit")
	if _, err := command(ctx, h.root, "go", "build", "-o", bin, "./cmd/transit"); err != nil {
		return "", fmt.Errorf("building cmd/transit: %w", err)
	}
	return bin, nil
}

// corpusRepo runs the corpus of the recorded grammars of one repository,
// the entries at indices of rec, and returns the result of each index. A
// repository whose grammars share test/corpus in its root runs it once, and
// each grammar gets that result.
func (h *harness) corpusRepo(ctx context.Context, rec *record, indices []int, transit string) (map[int]*corpusResult, error) {
	first := rec.Grammars[indices[0]]
	cached := filepath.Join(h.cache, "grammars", cacheName(first.Repository))
	if _, err := os.Stat(cached); err != nil {
		return nil, fmt.Errorf("finding the checkout of %s in the cache, which the candidates or fixtures set makes: %w", first.Repository, err)
	}
	// the folder in which each grammar runs its corpus
	runDirs := map[int]string{}
	var accepted []int
	for _, i := range indices {
		g := rec.Grammars[i]
		// the upstream tool rejects the grammar, so it has no parser.c to
		// test, and it cannot count toward the gate
		if g.Golden["abi15"].Error != "" {
			continue
		}
		accepted = append(accepted, i)
		switch {
		case isDir(filepath.Join(cached, g.Path, "test", "corpus")):
			runDirs[i] = g.Path
		case isDir(filepath.Join(cached, "test", "corpus")):
			runDirs[i] = "."
		}
	}
	results := map[int]*corpusResult{}
	for _, i := range indices {
		results[i] = nil
	}
	if len(runDirs) == 0 {
		return results, nil
	}

	var summaries [2]map[string]testSummary
	var transitErr error
	for v, tool := range []string{h.tool, transit} {
		copyDir, err := os.MkdirTemp("", "golden-corpus-")
		if err != nil {
			return nil, fmt.Errorf("making a folder for the corpus: %w", err)
		}
		defer func() { _ = os.RemoveAll(copyDir) }()
		if _, err := command(ctx, "", "cp", "-a", cached+"/.", copyDir); err != nil {
			return nil, err
		}
		// Generate each recorded grammar of the repository into its src,
		// from its grammar.json, as the golden files were made.
		for _, i := range accepted {
			g := rec.Grammars[i]
			dir := filepath.Join(copyDir, g.Path)
			cmd := exec.CommandContext(ctx, tool, "generate", filepath.Join("src", "grammar.json"), "--abi", "15")
			cmd.Dir = dir
			cmd.Env = cleanEnv()
			if b, err := cmd.CombinedOutput(); err != nil {
				return nil, fmt.Errorf("running %s generate in %s: %w: %s", filepath.Base(tool), g.Path, err, b)
			}
		}
		libDir := filepath.Join(copyDir, ".golden-lib")
		summaries[v] = map[string]testSummary{}
		for _, dir := range uniqueValues(runDirs) {
			s, err := h.runCorpus(ctx, filepath.Join(copyDir, dir), libDir)
			switch {
			case err != nil && v == 0:
				// the upstream tool cannot build the grammar or run its
				// corpus, so there is no result for transit to match
				for i := range runDirs {
					results[i] = &corpusResult{Error: firstLine(err.Error())}
				}
				return results, nil
			case err != nil:
				transitErr = err
			}
			summaries[v][dir] = s
		}
	}
	for i, dir := range runDirs {
		up, tr := summaries[0][dir], summaries[1][dir]
		r := &corpusResult{Tests: len(up.Outcomes), Failures: len(up.Failures), Transit: "different"}
		if transitErr == nil && slices.Equal(up.Outcomes, tr.Outcomes) && slices.Equal(up.Failures, tr.Failures) && up.Parses == tr.Parses {
			r.Transit = "same"
		}
		results[i] = r
	}
	return results, nil
}

// runCorpus runs tree-sitter test of the upstream tool in a folder, with its
// own folder for the compiled parsers, and returns the parts of the summary
// that stay the same from run to run.
func (h *harness) runCorpus(ctx context.Context, dir, libDir string) (testSummary, error) {
	cmd := exec.CommandContext(ctx, h.tool, "test", "--json-summary")
	cmd.Dir = dir
	cmd.Env = append(cleanEnv(), "TREE_SITTER_LIBDIR="+libDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	// a failed test exits with an error, and the summary still holds the
	// result, so only a summary that does not decode is an error
	runErr := cmd.Run()
	var raw struct {
		ParseResults  []json.RawMessage `json:"parse_results"`
		ParseFailures []json.RawMessage `json:"parse_failures"`
		ParseStats    struct {
			Successful int `json:"successful_parses"`
			Total      int `json:"total_parses"`
		} `json:"parse_stats"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return testSummary{}, fmt.Errorf("running tree-sitter test in %s: %w: %s", dir, runErrOr(runErr, err), strings.TrimSpace(stderr.String()))
	}
	var s testSummary
	for _, r := range raw.ParseResults {
		if err := collectOutcomes(r, "", &s.Outcomes); err != nil {
			return testSummary{}, err
		}
	}
	for _, f := range raw.ParseFailures {
		s.Failures = append(s.Failures, string(f))
	}
	s.Parses = [2]int{raw.ParseStats.Successful, raw.ParseStats.Total}
	return s, nil
}

// collectOutcomes adds the name and the outcome of each test of one node of
// the parse results, with the names of the groups above it, and recurses
// into its children. A node without an outcome is a group, such as a file.
func collectOutcomes(raw json.RawMessage, prefix string, out *[]string) error {
	var n struct {
		Name     string            `json:"name"`
		Outcome  json.RawMessage   `json:"outcome"`
		Children []json.RawMessage `json:"children"`
	}
	if err := json.Unmarshal(raw, &n); err != nil {
		return fmt.Errorf("decoding a parse result of tree-sitter test: %w", err)
	}
	name := prefix + "/" + n.Name
	if len(n.Outcome) > 0 {
		*out = append(*out, name+" "+string(n.Outcome))
	}
	for _, c := range n.Children {
		if err := collectOutcomes(c, name, out); err != nil {
			return err
		}
	}
	return nil
}

// runErrOr returns the error of the run when there is one, and err when
// there is not.
func runErrOr(runErr, err error) error {
	if runErr != nil {
		return runErr
	}
	return err
}

// isDir reports whether a path is a folder.
func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// uniqueValues returns the values of a map, sorted, with no duplicates.
func uniqueValues(m map[int]string) []string {
	var out []string
	for _, v := range m {
		out = append(out, v)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// firstLine returns the first line of a text.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
