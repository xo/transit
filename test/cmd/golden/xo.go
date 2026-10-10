package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// xoRepository is the repository of the grammars that xo writes (D42, D104).
const xoRepository = "https://github.com/xo/transit"

// xoGrammars runs the upstream tool on each grammar that xo writes, at
// ABI 14 and ABI 15, and writes the hashes to the record. Such a grammar is
// a module grammars/<name> whose folder or whose packages hold their
// grammar.js, which no module of another repository holds (D104). It copies
// each such module that it runs into the cache, as part of the checkout of
// the repository xo/transit in which the other sets of the harness and the
// tests find a grammar. It replaces only the copies of the modules that it
// runs, so a run with -only keeps the copies of the other modules. The
// upstream tool makes the grammar.json of each grammar from its grammar.js,
// as for a candidate that commits none (D51). The harness also writes it
// into the folder of the grammar package, which holds grammar.json as a
// package of another repository does. Such an entry has no tag and no
// commit, because the grammar is in the repository that holds the record.
func (h *harness) xoGrammars(ctx context.Context, report *coverage) error {
	modules, err := xoModules(filepath.Join(h.root, "grammars"))
	if err != nil {
		return err
	}
	checkout := filepath.Join(h.cache, "grammars", cacheName(xoRepository))
	if err := os.MkdirAll(filepath.Join(checkout, "grammars"), 0o755); err != nil {
		return fmt.Errorf("making %s: %w", filepath.Join(checkout, "grammars"), err)
	}
	var names []string
	for _, name := range modules {
		if !h.wanted(name) {
			continue
		}
		dst := filepath.Join(checkout, "grammars", name)
		if err := os.RemoveAll(dst); err != nil {
			return fmt.Errorf("deleting the old copy of grammars/%s: %w", name, err)
		}
		src := filepath.Join(h.root, "grammars", name)
		if _, err := command(ctx, "", "cp", "-a", src, dst); err != nil {
			return fmt.Errorf("copying grammars/%s to the cache: %w", name, err)
		}
		names = append(names, name)
	}
	rec, err := h.readRecord()
	if err != nil {
		return err
	}
	h.logf("generating the grammars of %d modules that xo writes\n", len(names))
	var mu sync.Mutex
	var entries []grammar
	err = parallel(ctx, h.jobs, names, func(ctx context.Context, name string) error {
		es, err := h.xoModule(ctx, checkout, name, report)
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
	rec.Upstream, rec.Tool, rec.Rust = h.upstream, h.toolVersion, h.rustVersion
	rec.Grammars = merge(rec.Grammars, entries)
	return writeJSON(h.recordPath(), rec)
}

// xoModules returns the name of each module of the folder grammars that xo
// writes: a folder grammars/<name> with a tree-sitter.json, whose folder or
// whose package folders hold a grammar.js.
func xoModules(grammars string) ([]string, error) {
	var names []string
	for _, pattern := range []string{"*/grammar.js", "*/*/grammar.js"} {
		found, err := filepath.Glob(filepath.Join(grammars, pattern))
		if err != nil {
			return nil, fmt.Errorf("finding the grammars that xo writes: %w", err)
		}
		for _, f := range found {
			rel, err := filepath.Rel(grammars, f)
			if err != nil {
				return nil, fmt.Errorf("finding the module of %s: %w", f, err)
			}
			name, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
			if isFile(filepath.Join(grammars, name, "tree-sitter.json")) && !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
	}
	slices.Sort(names)
	return names, nil
}

// xoModule runs the upstream tool on each grammar of the module
// grammars/<name> in the copy of the repository in checkout, and copies
// the grammar.json of each one into the repository.
func (h *harness) xoModule(ctx context.Context, checkout, name string, report *coverage) ([]grammar, error) {
	rel := filepath.Join("grammars", name)
	dir := filepath.Join(checkout, rel)
	paths, license, err := grammarPaths(dir)
	if err != nil {
		return nil, err
	}
	var out []grammar
	for _, p := range paths {
		if err := h.writeGrammarJSON(ctx, dir, p); err != nil {
			return nil, err
		}
		g, err := h.realGrammar(ctx, dir, p, report)
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(filepath.Join(dir, p, "src", "grammar.json"))
		if err != nil {
			return nil, fmt.Errorf("reading the grammar.json of %s: %w", filepath.Join(rel, p), err)
		}
		if err := writeFile(filepath.Join(h.root, rel, p, "grammar.json"), b); err != nil {
			return nil, err
		}
		g.Repository, g.Path, g.License = xoRepository, filepath.ToSlash(filepath.Join(rel, p)), license
		g.Set, g.Status = "xo", statusAvailable
		out = append(out, g)
		h.logf("  %s %s\n", rel, p)
	}
	return out, nil
}
