// Command stylesvg draws code samples in the bundled styles, as SVG files,
// so that a person can compare the styles and see how each grammar
// highlights its sample.
//
// Run it in the test module:
//
//	cd test && go run ./cmd/stylesvg
//
// It writes two kinds of file to the folder that -out names:
//
//   - lang-<grammar>.svg draws the sample of one grammar in each style.
//   - style-<style>.svg draws the sample of each grammar in one style.
//
// It also writes index.html, which shows every file. The highlights come
// from the highlighter of the highlight test, with the queries and the
// injections of each grammar module. A red line marks the text of an ERROR
// or a MISSING node, so a sample that a grammar does not parse shows it.
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xo/transit"
	"github.com/xo/transit/grammars/bash"
	"github.com/xo/transit/grammars/c"
	"github.com/xo/transit/grammars/cpp"
	"github.com/xo/transit/grammars/embeddedtemplate"
	golang "github.com/xo/transit/grammars/go"
	ghtml "github.com/xo/transit/grammars/html"
	"github.com/xo/transit/grammars/java"
	"github.com/xo/transit/grammars/javascript"
	"github.com/xo/transit/grammars/jsdoc"
	"github.com/xo/transit/grammars/json"
	"github.com/xo/transit/grammars/php/php"
	"github.com/xo/transit/grammars/php/phponly"
	"github.com/xo/transit/grammars/python"
	"github.com/xo/transit/grammars/ruby"
	"github.com/xo/transit/grammars/rust"
	"github.com/xo/transit/grammars/typescript/tsx"
	"github.com/xo/transit/grammars/typescript/typescript"
	"github.com/xo/transit/grammars/usql/usqlmysql"
	"github.com/xo/transit/grammars/usql/usqlpostgres"
	"github.com/xo/transit/internal/grammartest"
	"github.com/xo/transit/styles"
)

// samples holds a code sample for each grammar, in samples/<name>.txt.
//
//go:embed samples/*.txt
var samples embed.FS

// grammar is a grammar package that the command draws.
type grammar struct {
	name     string
	folder   string
	language func() *transit.Language
	queries  fs.FS
}

// grammars holds the grammar packages, in the order of the files.
var grammars = []grammar{
	{"bash", "bash", bash.Language, bash.Queries},
	{"c", "c", c.Language, c.Queries},
	{"cpp", "cpp", cpp.Language, cpp.Queries},
	{"embeddedtemplate", "embeddedtemplate", embeddedtemplate.Language, embeddedtemplate.Queries},
	{"go", "go", golang.Language, golang.Queries},
	{"html", "html", ghtml.Language, ghtml.Queries},
	{"java", "java", java.Language, java.Queries},
	{"javascript", "javascript", javascript.Language, javascript.Queries},
	{"jsdoc", "jsdoc", jsdoc.Language, jsdoc.Queries},
	{"json", "json", json.Language, json.Queries},
	{"php", "php/php", php.Language, php.Queries},
	{"phponly", "php/phponly", phponly.Language, phponly.Queries},
	{"python", "python", python.Language, python.Queries},
	{"ruby", "ruby", ruby.Language, ruby.Queries},
	{"rust", "rust", rust.Language, rust.Queries},
	{"typescript", "typescript/typescript", typescript.Language, typescript.Queries},
	{"tsx", "typescript/tsx", tsx.Language, tsx.Queries},
	{"usqlpostgres", "usql/usqlpostgres", usqlpostgres.Language, usqlpostgres.Queries},
	{"usqlmysql", "usql/usqlmysql", usqlmysql.Language, usqlmysql.Queries},
}

func main() {
	langs := flag.String("lang", "", "the grammars to draw, separated by commas (default: every grammar)")
	names := flag.String("style", "", "the styles to draw, separated by commas (default: every style)")
	file := flag.String("file", "", "a file to draw in place of the sample, with one grammar in -lang")
	out := flag.String("out", filepath.Join(os.TempDir(), "transit-stylesvg"), "the folder to write the files to")
	cols := flag.Int("cols", 4, "the number of columns of a file")
	flag.Parse()
	if err := run(os.Stdout, *langs, *names, *file, *out, *cols); err != nil {
		fmt.Fprintln(os.Stderr, "stylesvg:", err)
		os.Exit(1)
	}
}

// sample is a highlighted code sample.
type sample struct {
	name  string
	src   []byte
	names []string // the capture name of each byte, or ""
	errs  []bool   // whether each byte is in an ERROR or a MISSING node
}

// run draws the samples of the grammars langs in the styles names, writes
// the files to out, and reports them to w.
func run(w io.Writer, langs, names, file, out string, cols int) error {
	gs, err := selectGrammars(langs)
	if err != nil {
		return err
	}
	ss, err := selectStyles(names)
	if err != nil {
		return err
	}
	if file != "" && len(gs) != 1 {
		return errors.New("-file needs exactly one grammar in -lang")
	}
	root, err := findRoot()
	if err != nil {
		return err
	}
	var all []*sample
	for _, g := range gs {
		var src []byte
		if file != "" {
			src, err = os.ReadFile(file)
		} else {
			src, err = samples.ReadFile("samples/" + g.name + ".txt")
		}
		if err != nil {
			return fmt.Errorf("reading the sample of %s: %w", g.name, err)
		}
		s, err := highlight(root, g, src)
		if err != nil {
			return err
		}
		all = append(all, s)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return fmt.Errorf("making %s: %w", out, err)
	}
	var files []string
	for _, s := range all {
		tiles := make([]tile, len(ss))
		for i, st := range ss {
			tiles[i] = tile{title: st.Name, style: st, sample: s}
		}
		name := "lang-" + s.name + ".svg"
		if err := os.WriteFile(filepath.Join(out, name), drawSheet(tiles, cols), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
		files = append(files, name)
	}
	for _, st := range ss {
		tiles := make([]tile, len(all))
		for i, s := range all {
			tiles[i] = tile{title: s.name, style: st, sample: s}
		}
		name := "style-" + st.Name + ".svg"
		if err := os.WriteFile(filepath.Join(out, name), drawSheet(tiles, cols), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
		files = append(files, name)
	}
	if err := os.WriteFile(filepath.Join(out, "index.html"), index(files), 0o644); err != nil {
		return fmt.Errorf("writing index.html: %w", err)
	}
	if _, err := fmt.Fprintf(w, "wrote %d files to %s\n", len(files)+1, out); err != nil {
		return fmt.Errorf("writing the report: %w", err)
	}
	return nil
}

// selectGrammars returns the grammars that langs names, or every grammar
// when langs is empty.
func selectGrammars(langs string) ([]grammar, error) {
	if langs == "" {
		return grammars, nil
	}
	var gs []grammar
	for name := range strings.SplitSeq(langs, ",") {
		i := slices.IndexFunc(grammars, func(g grammar) bool { return g.name == strings.TrimSpace(name) })
		if i < 0 {
			return nil, fmt.Errorf("finding the grammar %q: no such grammar", name)
		}
		gs = append(gs, grammars[i])
	}
	return gs, nil
}

// selectStyles returns the bundled styles that names names, or every style
// when names is empty.
func selectStyles(names string) ([]*styles.Style, error) {
	list := styles.Names()
	if names != "" {
		list = strings.Split(names, ",")
	}
	ss := make([]*styles.Style, 0, len(list))
	for _, name := range list {
		st, ok := styles.Get(strings.TrimSpace(name))
		if !ok {
			return nil, fmt.Errorf("finding the style %q: no such style", name)
		}
		ss = append(ss, st)
	}
	return ss, nil
}

// findRoot returns the root of the repository: the working folder or the
// nearest folder above it that holds grammars/grammars.json.
func findRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("finding the working folder: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "grammars", "grammars.json")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("finding the repository: no folder above the working folder holds grammars/grammars.json")
		}
		dir = parent
	}
}

// highlight highlights src with the grammar g, and marks the bytes of its
// ERROR and MISSING nodes.
func highlight(root string, g grammar, src []byte) (*sample, error) {
	ctx := context.Background()
	lang := g.language()
	dir := filepath.Join(root, "grammars", filepath.FromSlash(g.folder))
	spans, err := grammartest.Spans(ctx, dir, grammartest.Grammar{Language: lang, Queries: g.queries}, nil, src)
	if err != nil {
		return nil, fmt.Errorf("highlighting the sample of %s: %w", g.name, err)
	}
	s := &sample{name: g.name, src: src, names: make([]string, len(src)), errs: make([]bool, len(src))}
	for _, sp := range spans {
		for i := sp.Start; i < sp.End && i < len(src); i++ {
			s.names[i] = sp.Name
		}
	}
	p := transit.NewParser()
	if err := p.SetLanguage(lang); err != nil {
		return nil, fmt.Errorf("setting the language of %s: %w", g.name, err)
	}
	tree, err := p.Parse(ctx, src, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing the sample of %s: %w", g.name, err)
	}
	defer tree.Close()
	markErrors(tree.RootNode(), s.errs)
	return s, nil
}

// markErrors marks the bytes of each ERROR and MISSING node under n. A
// MISSING node has no bytes, so it marks the byte before it.
func markErrors(n transit.Node, errs []bool) {
	if !n.HasError() && !n.IsMissing() {
		return
	}
	if n.IsError() || n.IsMissing() {
		start, end := n.StartByte(), n.EndByte()
		if start == end && start > 0 {
			start--
		}
		for i := start; i < end && i < len(errs); i++ {
			errs[i] = true
		}
		if n.IsMissing() {
			return
		}
	}
	for i := range n.ChildCount() {
		if child, ok := n.Child(i); ok {
			markErrors(child, errs)
		}
	}
}

// index returns a page that shows each file.
func index(files []string) []byte {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html><head><meta charset=\"utf-8\"><title>stylesvg</title>\n")
	b.WriteString("<style>body{font-family:sans-serif;margin:16px}img{max-width:100%;display:block;margin:8px 0 32px}</style>\n</head><body>\n")
	for _, f := range files {
		fmt.Fprintf(&b, "<h2 id=%q>%s</h2>\n<img src=%q alt=%q>\n", f, html.EscapeString(f), f, f)
	}
	b.WriteString("</body></html>\n")
	return []byte(b.String())
}
