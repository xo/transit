package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// testGrammars runs the upstream tool on each test grammar of upstream, in
// tree-sitter/test/fixtures/test_grammars, and writes the golden files to
// generate/testdata (D40). A test grammar holds only grammar.js, so the tool
// writes grammar.json too (D17).
func (h *harness) testGrammars(ctx context.Context, report *coverage) error {
	src := filepath.Join(h.ts, "test", "fixtures", "test_grammars")
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("reading the test grammars: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && h.wanted(e.Name()) {
			names = append(names, e.Name())
		}
	}
	dst := filepath.Join(h.root, "generate", "testdata")
	h.logf("generating %d test grammars\n", len(names))
	err = parallel(ctx, h.jobs, names, func(ctx context.Context, name string) error {
		return h.testGrammar(ctx, filepath.Join(src, name), filepath.Join(dst, name), report)
	})
	if err != nil {
		return err
	}
	return writeJSON(filepath.Join(dst, "upstream.json"), map[string]string{
		"upstream": h.upstream,
		"tool":     h.toolVersion,
		"rust":     h.rustVersion,
	})
}

// testGrammar runs the upstream tool on one test grammar in each variant, and
// writes its golden files. A grammar that the tool rejects gets error.txt in
// place of parser.c and node-types.json.
func (h *harness) testGrammar(ctx context.Context, src, dst string, report *coverage) error {
	name := filepath.Base(src)
	work, err := os.MkdirTemp("", "golden-"+name+"-")
	if err != nil {
		return fmt.Errorf("making a folder for %s: %w", name, err)
	}
	defer func() { _ = os.RemoveAll(work) }()
	if err := os.CopyFS(work, os.DirFS(src)); err != nil {
		return fmt.Errorf("copying the test grammar %s: %w", name, err)
	}
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("removing the old golden files of %s: %w", name, err)
	}
	for _, v := range []variant{abi15, abi14, abi15NoMerge} {
		o, err := h.generateTwice(ctx, work, "grammar.js", v)
		if err != nil {
			return err
		}
		dir := filepath.Join(dst, v.name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("making %s: %w", dir, err)
		}
		if o.grammarJSON != nil {
			if err := writeFile(filepath.Join(dst, "grammar.json"), o.grammarJSON); err != nil {
				return err
			}
		}
		if o.err != "" {
			if err := writeFile(filepath.Join(dir, "error.txt"), []byte(o.err+"\n")); err != nil {
				return err
			}
			continue
		}
		if err := writeFile(filepath.Join(dir, "parser.c"), o.parserC); err != nil {
			return err
		}
		if err := writeFile(filepath.Join(dir, "node-types.json"), o.nodeTypes); err != nil {
			return err
		}
		report.add(name, o.parserC)
	}
	h.logf("  %s\n", name)
	return nil
}

// parallel runs f for each name, with at most jobs at once, and returns the
// first error.
func parallel(ctx context.Context, jobs int, names []string, f func(context.Context, string) error) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	work := make(chan string)
	var wg sync.WaitGroup
	for range jobs {
		wg.Go(func() {
			for name := range work {
				if err := f(ctx, name); err != nil {
					cancel(err)
				}
			}
		})
	}
	for _, name := range names {
		select {
		case work <- name:
		case <-ctx.Done():
		}
	}
	close(work)
	wg.Wait()
	if err := context.Cause(ctx); err != nil && ctx.Err() != nil {
		return err
	}
	return nil
}

// writeFile writes a file and its folder.
func writeFile(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("making the folder of %s: %w", path, err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// writeJSON writes a value as indented JSON, with a newline at the end.
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}
	return writeFile(path, append(b, '\n'))
}
