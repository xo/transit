package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// record is grammars/grammars.json, the list of every grammar in transit
// (docs/GRAMMAR.md, D40, D51).
type record struct {
	Upstream string    `json:"upstream"`
	Tool     string    `json:"tool"`
	Rust     string    `json:"rust"`
	Grammars []grammar `json:"grammars"`
}

// statusAvailable is the status of a grammar whose repository exists (D51).
const statusAvailable = "available"

// grammar is one entry of the record.
type grammar struct {
	Name       string            `json:"name"`
	Package    string            `json:"package"`
	Repository string            `json:"repository"`
	Tag        string            `json:"tag,omitempty"`
	Branch     string            `json:"branch,omitempty"`
	Commit     string            `json:"commit"`
	Path       string            `json:"path"`
	License    string            `json:"license"`
	Scanner    bool              `json:"scanner"`
	Set        string            `json:"set"`
	Status     string            `json:"status"`
	Golden     map[string]golden `json:"golden"`
	// Corpus is the result of the corpus of the grammar, which the set
	// corpus writes, or nil when the grammar has no corpus or the set has
	// not run.
	Corpus *corpusResult `json:"corpus,omitempty"`
}

// golden is what the upstream tool writes for one grammar in one variant.
type golden struct {
	ParserC   string   `json:"parser_c,omitempty"`
	NodeTypes string   `json:"node_types,omitempty"`
	Error     string   `json:"error,omitempty"`
	Paths     []string `json:"paths,omitempty"`
}

// recordPath is the path of the record in the repository.
func (h *harness) recordPath() string {
	return filepath.Join(h.root, "grammars", "grammars.json")
}

// readRecord reads the record, or returns an empty one.
func (h *harness) readRecord() (*record, error) {
	b, err := os.ReadFile(h.recordPath())
	if errors.Is(err, os.ErrNotExist) {
		return &record{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading grammars/grammars.json: %w", err)
	}
	var r record
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("decoding grammars/grammars.json: %w", err)
	}
	return &r, nil
}

// fixture is one line of tree-sitter/test/fixtures/fixtures.json: the name of
// the grammar, its tag, and a branch that wins over the tag.
type fixture struct {
	name, tag, branch string
}

// readFixtures reads the fixture grammars of upstream.
func (h *harness) readFixtures() ([]fixture, error) {
	b, err := os.ReadFile(filepath.Join(h.ts, "test", "fixtures", "fixtures.json"))
	if err != nil {
		return nil, fmt.Errorf("reading fixtures.json: %w", err)
	}
	var rows [][]*string
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, fmt.Errorf("decoding fixtures.json: %w", err)
	}
	var out []fixture
	for _, r := range rows {
		if len(r) < 2 || r[0] == nil || r[1] == nil {
			return nil, fmt.Errorf("decoding fixtures.json: the row %v has no name or no tag", r)
		}
		f := fixture{name: *r[0], tag: *r[1]}
		if len(r) > 2 && r[2] != nil {
			f.branch = *r[2]
		}
		out = append(out, f)
	}
	return out, nil
}

// fixtureGrammars runs the upstream tool on each fixture grammar, at ABI 14
// and ABI 15, and writes the hashes to the record.
func (h *harness) fixtureGrammars(ctx context.Context, report *coverage) error {
	fixtures, err := h.readFixtures()
	if err != nil {
		return err
	}
	rec, err := h.readRecord()
	if err != nil {
		return err
	}
	var names []string
	byName := map[string]fixture{}
	for _, f := range fixtures {
		if h.wanted(f.name) {
			names = append(names, f.name)
			byName[f.name] = f
		}
	}
	h.logf("generating %d fixture grammars\n", len(names))
	var mu sync.Mutex
	var entries []grammar
	err = parallel(ctx, h.jobs, names, func(ctx context.Context, name string) error {
		es, err := h.fixtureGrammar(ctx, byName[name], report)
		if err != nil {
			return err
		}
		mu.Lock()
		entries = append(entries, es...)
		mu.Unlock()
		return nil
	})
	if err != nil {
		return err
	}
	rec.Upstream, rec.Tool, rec.Rust = baseCommit, h.toolVersion, h.rustVersion
	rec.Grammars = merge(rec.Grammars, entries)
	return writeJSON(h.recordPath(), rec)
}

// fixtureGrammar fetches one fixture repository, and runs the upstream tool on
// each grammar in it.
func (h *harness) fixtureGrammar(ctx context.Context, f fixture, report *coverage) ([]grammar, error) {
	repo := "https://github.com/tree-sitter/tree-sitter-" + f.name
	ref := "refs/tags/" + f.tag
	if f.branch != "" {
		ref = "refs/heads/" + f.branch
	}
	dir, commit, err := h.fetch(ctx, repo, ref)
	if err != nil {
		return nil, err
	}
	paths, license, err := grammarPaths(dir)
	if err != nil {
		return nil, err
	}
	var out []grammar
	for _, p := range paths {
		g, err := h.realGrammar(ctx, dir, p, report)
		if err != nil {
			return nil, err
		}
		g.Repository, g.Tag, g.Branch, g.Commit, g.License = repo, f.tag, f.branch, commit, license
		g.Set, g.Status = "fixture", statusAvailable
		out = append(out, g)
		h.logf("  %s %s\n", f.name, p)
	}
	return out, nil
}

// realGrammar runs the upstream tool on the grammar in one folder of a
// repository, from its committed src/grammar.json, at ABI 14 and ABI 15.
func (h *harness) realGrammar(ctx context.Context, repo, path string, report *coverage) (grammar, error) {
	dir := filepath.Join(repo, path)
	file := filepath.Join(dir, "src", "grammar.json")
	b, err := os.ReadFile(file)
	if err != nil {
		// D51 runs grammar.js for such a grammar, which the fixtures do not need
		return grammar{}, fmt.Errorf("reading %s, which a fixture grammar commits: %w", file, err)
	}
	var head struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return grammar{}, fmt.Errorf("decoding %s: %w", file, err)
	}
	g := grammar{
		Name:    head.Name,
		Package: strings.ReplaceAll(head.Name, "_", ""),
		Path:    path,
		Golden:  map[string]golden{},
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "scanner.c")); err == nil {
		g.Scanner = true
	}
	for _, v := range []variant{abi15, abi14} {
		o, err := h.generateTwice(ctx, dir, file, v)
		if err != nil {
			return grammar{}, err
		}
		if o.err != "" {
			g.Golden[v.name] = golden{Error: o.err}
			continue
		}
		g.Golden[v.name] = golden{ParserC: sum(o.parserC), NodeTypes: sum(o.nodeTypes), Paths: reachedPaths(o.parserC)}
		report.add(g.Name, o.parserC)
	}
	return g, nil
}

// grammarPaths reads tree-sitter.json of a repository, and returns the folder
// of each grammar in it and the license of the repository.
func grammarPaths(repo string) ([]string, string, error) {
	b, err := os.ReadFile(filepath.Join(repo, "tree-sitter.json"))
	if errors.Is(err, os.ErrNotExist) {
		return []string{"."}, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("reading tree-sitter.json of %s: %w", repo, err)
	}
	var cfg struct {
		Grammars []struct {
			Path string `json:"path"`
		} `json:"grammars"`
		Metadata struct {
			License string `json:"license"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, "", fmt.Errorf("decoding tree-sitter.json of %s: %w", repo, err)
	}
	// one grammar can have more than one entry, as tree-sitter-embedded-template
	// has for ERB and EJS, and tree-sitter-typescript names tsx twice
	var paths []string
	for _, g := range cfg.Grammars {
		if p := cmp.Or(g.Path, "."); !slices.Contains(paths, p) {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		paths = []string{"."}
	}
	return paths, cfg.Metadata.License, nil
}

// fetch fetches one ref of a repository into the cache, and returns its
// folder and the commit of the ref.
func (h *harness) fetch(ctx context.Context, repo, ref string) (string, string, error) {
	dir := filepath.Join(h.cache, "grammars", cacheName(repo))
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", "", fmt.Errorf("making %s: %w", dir, err)
		}
		if _, err := command(ctx, dir, "git", "init", "-q"); err != nil {
			return "", "", err
		}
	}
	if _, err := command(ctx, dir, "git", "fetch", "-q", "--depth", "1", repo, ref); err != nil {
		return "", "", fmt.Errorf("fetching %s of %s: %w", ref, repo, err)
	}
	if _, err := command(ctx, dir, "git", "checkout", "-q", "--force", "FETCH_HEAD"); err != nil {
		return "", "", err
	}
	commit, err := command(ctx, dir, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", "", err
	}
	return dir, strings.TrimSpace(commit), nil
}

// merge puts new entries into the record in place of the old entries of the
// same repository and path, and sorts the record.
func merge(old, updates []grammar) []grammar {
	key := func(g grammar) string { return g.Repository + " " + g.Path }
	seen := map[string]bool{}
	for _, g := range updates {
		seen[key(g)] = true
	}
	out := slices.Clone(updates)
	for _, g := range old {
		if !seen[key(g)] {
			out = append(out, g)
		}
	}
	slices.SortFunc(out, func(a, b grammar) int {
		return cmp.Or(cmp.Compare(a.Repository, b.Repository), cmp.Compare(a.Path, b.Path))
	})
	return out
}

// cacheName returns the folder of a repository in the cache: its owner and
// its name, such as tree-sitter/tree-sitter-json, so that two repositories
// with one name, such as the two tree-sitter-sql, do not share a folder.
func cacheName(repo string) string {
	return filepath.Join(filepath.Base(filepath.Dir(repo)), filepath.Base(repo))
}
