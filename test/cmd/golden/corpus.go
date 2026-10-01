package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
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
	// Failing holds the name of each test that fails with the parser.c of
	// the upstream tool, in the order of the run. A name is the path of the
	// test in the corpus: the names of its groups and its own name, joined
	// by "/", such as "expressions/Binary operators". The corpus test of a
	// grammar package expects these tests to fail, and no other test. The
	// package holds them in testdata/failing.txt (D79, D88).
	Failing []string `json:"failing,omitempty"`
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
	// Failing holds the path of each test whose outcome is Failed, as
	// corpusResult.Failing records it.
	Failing []string
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
		if dir, ok := corpusFolder(cached, g.Path); ok {
			runDirs[i] = dir
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
		r := &corpusResult{Tests: len(up.Outcomes), Failures: len(up.Failures), Failing: up.Failing, Transit: "different"}
		if transitErr == nil && slices.Equal(up.Outcomes, tr.Outcomes) && slices.Equal(up.Failures, tr.Failures) && up.Parses == tr.Parses {
			r.Transit = "same"
		}
		results[i] = r
	}
	return results, nil
}

// corpusFolder returns the folder in which a grammar at the path p of a
// checkout runs its corpus: the nearest folder, from p up to the root of
// the checkout, that holds test/corpus. A grammar that xo writes finds the
// corpus of its module this way.
func corpusFolder(checkout, p string) (string, bool) {
	for p = filepath.Clean(p); ; p = filepath.Dir(p) {
		if isDir(filepath.Join(checkout, p, "test", "corpus")) {
			return p, true
		}
		if p == "." || p == string(filepath.Separator) {
			return "", false
		}
	}
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
		if err := collectOutcomes(r, "", &s); err != nil {
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
// It also adds the path of each failed test to s.Failing.
func collectOutcomes(raw json.RawMessage, prefix string, s *testSummary) error {
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
		s.Outcomes = append(s.Outcomes, name+" "+string(n.Outcome))
		if string(n.Outcome) == `"Failed"` {
			s.Failing = append(s.Failing, strings.TrimPrefix(name, "/"))
		}
	}
	for _, c := range n.Children {
		if err := collectOutcomes(c, name, s); err != nil {
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

// writeFailing writes testdata/failing.txt of each grammar package under
// grammars/ from the record (D88).
func (h *harness) writeFailing() error {
	rec, err := h.readRecord()
	if err != nil {
		return err
	}
	return writeFailingFiles(filepath.Join(h.root, "grammars"), rec)
}

// writeFailingFiles writes testdata/failing.txt of each grammar package
// under the folder grammars: each folder grammars/<module>,
// grammars/<module>/<grammar> that holds a grammar.json. The file holds the
// names of the corpus tests of the entry of the grammar that fail upstream,
// in the order of the record, each on a line of its own that ends with a
// newline. A package whose entry names no such test gets no file, and the
// function deletes a file that is there.
func writeFailingFiles(grammars string, rec *record) error {
	var files []string
	for _, pattern := range []string{"*/grammar.json", "*/*/grammar.json"} {
		found, err := filepath.Glob(filepath.Join(grammars, pattern))
		if err != nil {
			return fmt.Errorf("finding the grammar packages: %w", err)
		}
		files = append(files, found...)
	}
	for _, f := range files {
		dir := filepath.Dir(f)
		g, err := packageEntry(grammars, dir, rec)
		if err != nil {
			return err
		}
		var text strings.Builder
		if g.Corpus != nil {
			own, err := ownCases(grammars, dir, g.Corpus.Failing)
			if err != nil {
				return err
			}
			for _, name := range own {
				text.WriteString(name + "\n")
			}
		}
		p := filepath.Join(dir, "testdata", "failing.txt")
		if text.Len() == 0 {
			if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("deleting %s: %w", p, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return fmt.Errorf("making %s: %w", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(text.String()), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", p, err)
		}
	}
	return nil
}

// ownCases returns the names of cases that the grammar package in dir runs
// (D83, D93). In a module with more than one grammar, a case runs in the
// package of the grammar that its :language names, or of the first grammar
// of tree-sitter.json when it names none, or of the package itself when no
// entry of tree-sitter.json has its folder. The harness imports only the
// standard library (D58), so this reads the header of each case itself,
// with the rule of internal/grammartest. A case that the corpus does not
// hold, or whose :language names no grammar of the module, stays, because
// the package fails it.
func ownCases(grammars, dir string, names []string) ([]string, error) {
	moduleDir := dir
	for moduleDir != grammars && !isFile(filepath.Join(moduleDir, "go.mod")) {
		moduleDir = filepath.Dir(moduleDir)
	}
	if moduleDir == dir {
		return names, nil
	}
	b, err := os.ReadFile(filepath.Join(moduleDir, "tree-sitter.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return names, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the tree-sitter.json of %s: %w", moduleDir, err)
	}
	var cfg struct {
		Grammars []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"grammars"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("decoding the tree-sitter.json of %s: %w", moduleDir, err)
	}
	if len(cfg.Grammars) == 0 {
		return names, nil
	}
	folder := func(p string) string {
		p = strings.TrimPrefix(path.Clean(p), "./")
		return strings.NewReplacer("_", "", "-", "").Replace(p)
	}
	own := filepath.Base(dir)
	// A package of a grammar that tree-sitter.json does not list, such as
	// plpgsql of tree-sitter-postgres, runs each case of its own corpus.
	listed := false
	for _, g := range cfg.Grammars {
		listed = listed || folder(g.Path) == own
	}
	if !listed {
		return names, nil
	}
	var out []string
	for _, name := range names {
		language, found, err := caseLanguage(filepath.Join(dir, "testdata", "corpus"), name)
		if err != nil {
			return nil, err
		}
		owner := cfg.Grammars[0]
		known := true
		if language != "" {
			known = false
			for _, g := range cfg.Grammars {
				if g.Name == language {
					owner, known = g, true
					break
				}
			}
		}
		if !found || !known || folder(owner.Path) == own {
			out = append(out, name)
		}
	}
	return out, nil
}

// caseLanguage returns the language that the :language attribute of a
// corpus case names, or "" when it names none, and whether the corpus holds
// the case. name is the path of the case: the path of its file in corpus
// with no .txt, a "/", and the name of the case.
func caseLanguage(corpus, name string) (string, bool, error) {
	for i := len(name) - 1; i > 0; i-- {
		if name[i] != '/' {
			continue
		}
		b, err := os.ReadFile(filepath.Join(corpus, filepath.FromSlash(name[:i])+".txt"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", false, fmt.Errorf("reading the corpus of %s: %w", name, err)
		}
		language, found := headerLanguage(string(b), name[i+1:])
		return language, found, nil
	}
	return "", false, nil
}

// headerLanguage finds the header of the case title in a corpus file, the
// title between two lines of "=" and its attributes, and returns the
// language of its :language attribute.
func headerLanguage(content, title string) (string, bool) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != title || !strings.HasPrefix(lines[i-1], "===") {
			continue
		}
		for _, line := range lines[i+1:] {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "===") {
				return "", true
			}
			if rest, ok := strings.CutPrefix(line, ":language("); ok {
				if lang, _, ok := strings.Cut(rest, ")"); ok {
					return lang, true
				}
			}
		}
	}
	return "", false
}

// packageEntry returns the entry of the record for the grammar package in
// dir, as golang.FindRecord of the root module finds it: the entry with the
// name of grammar.json. When several entries have the name, the entry is
// the one whose repository gives the name of the folder of the module, the
// nearest folder that holds a go.mod, as docs/GRAMMAR.md says. When several
// entries give that name too, the entry is the one whose repository the
// tree-sitter.json of the module names (D106).
func packageEntry(grammars, dir string, rec *record) (*grammar, error) {
	b, err := os.ReadFile(filepath.Join(dir, "grammar.json"))
	if err != nil {
		return nil, fmt.Errorf("reading the grammar of %s: %w", dir, err)
	}
	var head struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return nil, fmt.Errorf("decoding the grammar of %s: %w", dir, err)
	}
	moduleDir := dir
	for moduleDir != grammars && !isFile(filepath.Join(moduleDir, "go.mod")) {
		moduleDir = filepath.Dir(moduleDir)
	}
	var found []*grammar
	for i := range rec.Grammars {
		if rec.Grammars[i].Name == head.Name {
			found = append(found, &rec.Grammars[i])
		}
	}
	if len(found) > 1 {
		found = slices.DeleteFunc(found, func(g *grammar) bool {
			return moduleFolderName(g.Repository) != filepath.Base(moduleDir)
		})
	}
	if len(found) > 1 {
		repository, err := moduleRepository(moduleDir)
		if err != nil {
			return nil, err
		}
		found = slices.DeleteFunc(found, func(g *grammar) bool {
			return !sameRepository(g.Repository, repository)
		})
	}
	if len(found) != 1 {
		return nil, fmt.Errorf("finding the entry of the grammar package %s: the record has %d entries for the grammar %s in the module folder %s", dir, len(found), head.Name, filepath.Base(moduleDir))
	}
	return found[0], nil
}

// moduleRepository returns the repository that the tree-sitter.json in the
// folder of a module names in metadata.links.repository, or "" when the
// folder has no tree-sitter.json or the file names no repository, as
// golang.FindRecord of the root module reads it.
func moduleRepository(moduleDir string) (string, error) {
	p := filepath.Join(moduleDir, "tree-sitter.json")
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", p, err)
	}
	var cfg struct {
		Metadata struct {
			Links struct {
				Repository string `json:"repository"`
			} `json:"links"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return "", fmt.Errorf("decoding %s: %w", p, err)
	}
	return cfg.Metadata.Links.Repository, nil
}

// sameRepository reports whether two URLs name the same repository, as
// golang.FindRecord of the root module compares them. It ignores the case,
// a prefix git+ and a suffix .git or /. An empty URL names no repository.
func sameRepository(a, b string) bool {
	clean := func(u string) string {
		u = strings.ToLower(strings.TrimPrefix(u, "git+"))
		return strings.TrimSuffix(strings.TrimSuffix(u, "/"), ".git")
	}
	return a != "" && b != "" && clean(a) == clean(b)
}

// moduleFolderName returns the name of the folder of the module of a
// repository: its name without the prefix tree-sitter- and with each "-"
// removed, as docs/GRAMMAR.md says.
func moduleFolderName(repository string) string {
	name := strings.TrimPrefix(path.Base(repository), "tree-sitter-")
	return strings.ReplaceAll(name, "-", "")
}

// isFile reports whether a file exists at a path.
func isFile(name string) bool {
	fi, err := os.Stat(name)
	return err == nil && !fi.IsDir()
}
