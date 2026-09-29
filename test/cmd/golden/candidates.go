package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
)

// candidate is one grammar of docs/CANDIDATES.md: its repository, the folder
// of the grammar in it, and the set that it is in.
type candidate struct {
	// repo is the owner and the name of the repository, such as
	// tree-sitter/tree-sitter-json.
	repo string
	// path is the folder of the grammar, or empty when the table names none,
	// and then every grammar of tree-sitter.json is in the set.
	path string
	// name is the name in tree-sitter.json of the grammar, when a table
	// names the grammar by its name and not by its folder.
	name string
	set  string
}

// candidateRepo matches the cell of a repository in the table of the
// candidates: the repository, and a folder in backquotes after it.
var candidateRepo = regexp.MustCompile("^([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)(?: `([^`]+)`)?$")

// sqlRepo matches the first cell of the tables of SQL and of the languages
// of dbmeta: the repository in backquotes, and a grammar name after it.
var sqlRepo = regexp.MustCompile("^`([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)`(?:, `([^`]+)`)?$")

// readCandidates reads the grammars of docs/CANDIDATES.md that are in the
// set: the three tiers of "The candidates", the SQL grammars that are not
// archived (D18), and the grammars for the languages of dbmeta (D23). A
// grammar that two tables name is read once, with the set of the first.
func readCandidates(doc string) ([]candidate, error) {
	var out []candidate
	seen := map[string]bool{}
	add := func(c candidate) {
		key := c.repo + " " + c.path + " " + c.name
		if !seen[key] {
			seen[key] = true
			out = append(out, c)
		}
	}

	for cells := range tableRows(section(doc, "## The candidates")) {
		if len(cells) < 3 || cells[0] == "Tier" {
			continue
		}
		m := candidateRepo.FindStringSubmatch(cells[2])
		if m == nil {
			return nil, fmt.Errorf("reading the candidate %s: the repository %q has no form that the harness knows", cells[1], cells[2])
		}
		add(candidate{repo: m[1], path: m[2], set: "tier" + cells[0]})
	}

	for cells := range tableRows(section(doc, "## SQL")) {
		if len(cells) < 4 || cells[0] == "Grammar" {
			continue
		}
		m := sqlRepo.FindStringSubmatch(cells[0])
		if m == nil {
			return nil, fmt.Errorf("reading the SQL grammar %q: it has no form that the harness knows", cells[0])
		}
		// D18 takes every SQL grammar that is not archived
		if strings.Contains(cells[3], "Archived") {
			continue
		}
		// the second name is a grammar of tree-sitter.json, whose folder the
		// harness finds when it has the repository
		add(candidate{repo: m[1], name: m[2], set: "sql"})
	}

	for cells := range tableRows(section(doc, "### The grammars for dbmeta's languages")) {
		if len(cells) < 2 || cells[0] == "Grammar" {
			continue
		}
		m := sqlRepo.FindStringSubmatch(cells[0])
		if m == nil {
			return nil, fmt.Errorf("reading the grammar %q for a language of dbmeta: it has no form that the harness knows", cells[0])
		}
		add(candidate{repo: m[1], name: m[2], set: "dbmeta"})
	}
	return out, nil
}

// section returns the text of a document from a heading to the next heading
// of the same level or higher.
func section(doc, heading string) string {
	i := strings.Index(doc, "\n"+heading+"\n")
	if i < 0 {
		return ""
	}
	rest := doc[i+1+len(heading)+1:]
	level := strings.Index(heading, " ")
	for _, h := range []string{"\n## ", "\n### "} {
		if len(h)-2 > level {
			continue
		}
		if j := strings.Index(rest, h); j >= 0 {
			rest = rest[:j]
		}
	}
	return rest
}

// tableRows returns the cells of each row of each Markdown table in text,
// without the separator rows.
func tableRows(text string) func(func([]string) bool) {
	return func(yield func([]string) bool) {
		for line := range strings.Lines(text) {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "|") || strings.HasPrefix(line, "| ---") {
				continue
			}
			cells := strings.Split(strings.Trim(line, "|"), "|")
			for i := range cells {
				cells[i] = strings.TrimSpace(cells[i])
			}
			if !yield(cells) {
				return
			}
		}
	}
}

// candidateGrammars runs the upstream tool on each candidate grammar, at ABI
// 14 and ABI 15, and writes the hashes to the record. A grammar that the
// record holds already is fetched at its commit (D51). A new one is fetched at
// the latest release of its repository, or at its latest tag, or at its
// default branch. A fixture grammar keeps the tag of fixtures.json, so the
// candidates skip it.
func (h *harness) candidateGrammars(ctx context.Context, report *coverage) error {
	doc, err := os.ReadFile(filepath.Join(h.root, "docs", "CANDIDATES.md"))
	if err != nil {
		return fmt.Errorf("reading docs/CANDIDATES.md: %w", err)
	}
	candidates, err := readCandidates(string(doc))
	if err != nil {
		return err
	}
	rec, err := h.readRecord()
	if err != nil {
		return err
	}
	byRepo := map[string][]candidate{}
	var repos []string
	for _, c := range candidates {
		url := "https://github.com/" + c.repo
		if slices.ContainsFunc(rec.Grammars, func(g grammar) bool { return g.Repository == url && g.Set == "fixture" }) {
			continue
		}
		if !h.wanted(c.repo) && !h.wanted(filepath.Base(c.repo)) && !h.wanted(c.path) {
			continue
		}
		if _, ok := byRepo[c.repo]; !ok {
			repos = append(repos, c.repo)
		}
		byRepo[c.repo] = append(byRepo[c.repo], c)
	}
	h.logf("generating the grammars of %d candidate repositories\n", len(repos))
	var mu sync.Mutex
	var entries []grammar
	var failed []string
	err = parallel(ctx, h.jobs, repos, func(ctx context.Context, repo string) error {
		es, err := h.candidateRepo(ctx, rec, repo, byRepo[repo], report)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			// one repository that fails must not stop the run of the others
			h.logf("  %s: %v\n", repo, err)
			failed = append(failed, repo)
			return nil
		}
		entries = append(entries, es...)
		return nil
	})
	if err != nil {
		return err
	}
	rec.Upstream, rec.Tool, rec.Rust = baseCommit, h.toolVersion, h.rustVersion
	rec.Grammars = merge(rec.Grammars, entries)
	if err := writeJSON(h.recordPath(), rec); err != nil {
		return err
	}
	if len(failed) > 0 {
		slices.Sort(failed)
		return fmt.Errorf("the harness could not generate %d candidate repositories: %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}

// candidateRepo fetches one candidate repository, and runs the upstream tool
// on each of its grammars in the set.
func (h *harness) candidateRepo(ctx context.Context, rec *record, repo string, cs []candidate, report *coverage) ([]grammar, error) {
	url := "https://github.com/" + repo
	tag, branch, ref := "", "", ""
	for _, g := range rec.Grammars {
		if g.Repository == url {
			tag, branch, ref = g.Tag, g.Branch, g.Commit
			break
		}
	}
	if ref == "" {
		var err error
		if tag, branch, err = latestRef(ctx, repo); err != nil {
			return nil, err
		}
		ref = "refs/tags/" + tag
		if tag == "" {
			ref = "refs/heads/" + branch
		}
	}
	dir, commit, err := h.fetch(ctx, url, ref)
	if err != nil {
		return nil, err
	}
	allPaths, license, err := grammarPaths(dir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, c := range cs {
		switch {
		case c.path != "":
			paths = append(paths, c.path)
		case c.name != "":
			p, err := grammarPathByName(dir, c.name)
			if err != nil {
				return nil, err
			}
			paths = append(paths, p)
		default:
			paths = append(paths, allPaths...)
		}
	}
	var out []grammar
	for _, p := range slices.Compact(paths) {
		if err := h.writeGrammarJSON(ctx, dir, p); err != nil {
			return nil, err
		}
		g, err := h.realGrammar(ctx, dir, p, report)
		if err != nil {
			return nil, err
		}
		g.Repository, g.Tag, g.Branch, g.Commit, g.License = url, tag, branch, commit, license
		g.Set, g.Status = cs[0].set, "available"
		out = append(out, g)
		h.logf("  %s %s\n", repo, p)
	}
	return out, nil
}

// latestRef returns the latest release tag of a repository, or its latest tag
// when it has no release, or its default branch when it has no tag.
func latestRef(ctx context.Context, repo string) (string, string, error) {
	if tag, err := command(ctx, "", "gh", "api", "repos/"+repo+"/releases/latest", "--jq", ".tag_name"); err == nil && strings.TrimSpace(tag) != "" {
		return strings.TrimSpace(tag), "", nil
	}
	tags, err := command(ctx, "", "git", "ls-remote", "--tags", "--sort=-v:refname", "--refs", "https://github.com/"+repo)
	if err != nil {
		return "", "", fmt.Errorf("listing the tags of %s: %w", repo, err)
	}
	if first, _, _ := strings.Cut(tags, "\n"); first != "" {
		if _, name, ok := strings.Cut(first, "refs/tags/"); ok {
			return strings.TrimSpace(name), "", nil
		}
	}
	branch, err := command(ctx, "", "gh", "api", "repos/"+repo, "--jq", ".default_branch")
	if err != nil {
		return "", "", fmt.Errorf("finding the default branch of %s: %w", repo, err)
	}
	return "", strings.TrimSpace(branch), nil
}

// writeGrammarJSON makes src/grammar.json for a grammar that commits none, by
// running grammar.js with the upstream tool (D51). It first installs the npm
// packages that package.json of the repository names, with no install
// scripts, because grammar.js can require another grammar. The tool writes
// grammar.json, and the harness keeps it in the checkout in the cache, so
// that the generator test reads it as it reads a committed one.
func (h *harness) writeGrammarJSON(ctx context.Context, repo, path string) error {
	dir := filepath.Join(repo, path)
	file := filepath.Join(dir, "src", "grammar.json")
	if _, err := os.Stat(file); err == nil {
		return nil
	}
	js := filepath.Join(dir, "grammar.js")
	if _, err := os.Stat(js); err != nil {
		return fmt.Errorf("finding %s or %s: %w", file, js, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "package.json")); err == nil {
		if _, err := os.Stat(filepath.Join(repo, "node_modules")); errors.Is(err, os.ErrNotExist) {
			if _, err := command(ctx, repo, "npm", "install", "--ignore-scripts", "--no-audit", "--no-fund"); err != nil {
				return fmt.Errorf("installing the npm packages of %s: %w", repo, err)
			}
		}
	}
	out, err := os.MkdirTemp("", "golden-js-")
	if err != nil {
		return fmt.Errorf("making a folder for grammar.json: %w", err)
	}
	defer func() { _ = os.RemoveAll(out) }()
	cmd := exec.CommandContext(ctx, h.tool, "generate", "--no-parser", js, "-o", out)
	cmd.Dir = dir
	cmd.Env = cleanEnv()
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("running %s with the upstream tool: %w: %s", js, err, b)
	}
	b, err := os.ReadFile(filepath.Join(out, "grammar.json"))
	if err != nil {
		return fmt.Errorf("reading the grammar.json of %s: %w", js, err)
	}
	h.logf("  %s: made src/grammar.json from grammar.js\n", dir)
	return writeFile(file, b)
}

// grammarPathByName returns the folder of the grammar with a name in
// tree-sitter.json of a repository, or the folder with the name of the
// grammar when tree-sitter.json does not list it.
func grammarPathByName(repo, name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(repo, "tree-sitter.json"))
	if err != nil {
		return "", fmt.Errorf("reading tree-sitter.json of %s, to find the grammar %s: %w", repo, name, err)
	}
	var cfg struct {
		Grammars []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"grammars"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return "", fmt.Errorf("decoding tree-sitter.json of %s: %w", repo, err)
	}
	for _, g := range cfg.Grammars {
		if g.Name == name {
			return cmp.Or(g.Path, "."), nil
		}
	}
	// A repository can hold a grammar that its tree-sitter.json does not
	// list, as gmr/tree-sitter-postgres holds plpgsql. Then the folder with
	// the name of the grammar holds it.
	for _, f := range []string{"grammar.js", filepath.Join("src", "grammar.json")} {
		if _, err := os.Stat(filepath.Join(repo, name, f)); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("finding the grammar %s in tree-sitter.json of %s, or in its folder %s", name, repo, name)
}
