// Command railroad draws a railroad diagram of each rule of a grammar, as
// the manuals of SQLite and PostgreSQL show the syntax of SQL.
//
// Run it in the test module:
//
//	cd test && go run ./cmd/railroad -grammar usql
//
// The flag -grammar names a folder under grammars/, such as usql, sql or
// postgres/postgres, or the path of a grammar.json file or of the folder
// that holds one. The command writes to the folder that -out names:
//
//   - index.html shows the diagram of each rule, in the order of the
//     grammar. The name of a rule in a diagram links to its diagram.
//   - <rule>.svg draws one rule, and links to the files of the other rules.
//
// A terminal is a box with round ends, and the name of a rule is a box with
// square corners. A token of the external scanner is a box with a color of
// its own. To make the diagrams short, the command:
//
//   - draws a hidden rule, whose name starts with _, in place, unless it
//     refers to itself or has more than 12 boxes (-inline);
//   - drops the wrappers of precedence and of tokens;
//   - shows an alias by the name that it gives;
//   - shows a rule that only matches one keyword, such as keyword_select, as
//     that keyword in capitals, and draws no diagram for it;
//   - draws a loop of an item with a separator, such as a list with commas,
//     as one loop with the separator on its track back;
//   - wraps a long row into several rows, and a long choice of single boxes
//     into columns;
//   - draws the extras of the grammar once, at the end of the page.
//
// The style sheet is light or dark (-style). The SVG elements use classes,
// so the style sheet sets each color.
package main

import (
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	gram := flag.String("grammar", "", "the folder of the grammar under grammars/, such as usql or postgres/postgres, or the path of a grammar.json file")
	out := flag.String("out", filepath.Join(os.TempDir(), "transit-railroad"), "the folder to write the files to")
	style := flag.String("style", "light", "the style sheet: light or dark")
	width := flag.Float64("width", 1000, "the width in pixels over which a row of a diagram wraps")
	limit := flag.Int("inline", inlineLimit, "the largest number of boxes of a hidden rule that is drawn in place")
	flag.Parse()
	if err := run(os.Stdout, *gram, *out, *style, *width, *limit); err != nil {
		fmt.Fprintln(os.Stderr, "railroad:", err)
		os.Exit(1)
	}
}

// run draws the rules of the grammar gram and writes the files to out.
func run(w io.Writer, gram, out, style string, width float64, limit int) error {
	css, ok := styleSheets[style]
	if !ok {
		return fmt.Errorf("finding the style sheet %q: use light or dark", style)
	}
	path, err := grammarPath(gram)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening the grammar: %w", err)
	}
	g, err := readGrammar(f)
	_ = f.Close()
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return fmt.Errorf("making %s: %w", out, err)
	}
	d := draw(g, width, limit)
	page := d.page(css)
	if err := os.WriteFile(filepath.Join(out, "index.html"), []byte(page), 0o644); err != nil {
		return fmt.Errorf("writing index.html: %w", err)
	}
	for _, r := range d.rules {
		svg := diagram(r.node, width, func(name string) string { return name + ".svg" }, css)
		name := r.name + ".svg"
		if err := os.WriteFile(filepath.Join(out, name), []byte(svg), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}
	if _, err := fmt.Fprintf(w, "wrote the diagrams of %d rules of %s to %s\n", len(d.rules), g.Name, out); err != nil {
		return fmt.Errorf("writing the report: %w", err)
	}
	return nil
}

// grammarPath returns the path of the grammar.json file that gram names.
func grammarPath(gram string) (string, error) {
	if gram == "" {
		return "", errors.New("-grammar is required")
	}
	if st, err := os.Stat(gram); err == nil {
		if st.IsDir() {
			return filepath.Join(gram, "grammar.json"), nil
		}
		return gram, nil
	}
	root, err := findRoot()
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, "grammars", filepath.FromSlash(gram), "grammar.json")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("finding the grammar %q: %w", gram, err)
	}
	return path, nil
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

// drawing holds the diagrams of a grammar.
type drawing struct {
	name     string
	width    float64
	rules    []drawnRule
	extras   *node
	keywords []string // the rules that match one keyword
	inlined  []string // the hidden rules that have no diagram
}

// drawnRule is the diagram of one rule.
type drawnRule struct {
	name string
	node *node
}

// draw makes the diagrams of g. A rule has a diagram when it is visible and
// does not match one keyword, or when a box links to it.
func draw(g *grammarFile, width float64, limit int) *drawing {
	c := newConverter(g, limit)
	nodes := make(map[string]*node)
	var queue []string
	for _, name := range g.Rules.names {
		if !strings.HasPrefix(name, "_") && c.keywords[name] == "" {
			queue = append(queue, name)
		}
	}
	extras := c.extras()
	for len(queue) > 0 || len(c.refs) > 0 {
		for name := range c.refs {
			if _, ok := nodes[name]; !ok {
				queue = append(queue, name)
			}
			delete(c.refs, name)
		}
		if len(queue) == 0 {
			break
		}
		name := queue[0]
		queue = queue[1:]
		if _, ok := nodes[name]; !ok {
			nodes[name] = c.rule(name)
		}
	}
	d := &drawing{name: g.Name, width: width, extras: extras}
	for _, name := range g.Rules.names {
		switch n, ok := nodes[name]; {
		case ok:
			d.rules = append(d.rules, drawnRule{name: name, node: n})
		case c.keywords[name] != "":
			d.keywords = append(d.keywords, name)
		default:
			d.inlined = append(d.inlined, name)
		}
	}
	return d
}

// page returns the HTML page of the diagrams, with the style sheet css.
func (d *drawing) page(css string) string {
	link := func(name string) string { return "#" + name }
	var b strings.Builder
	title := html.EscapeString(d.name)
	fmt.Fprintf(&b, "<!DOCTYPE html>\n<html><head><meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n<title>%s</title>\n<style>\n%s%s</style>\n</head><body>\n", title, css, pageCSS)
	fmt.Fprintf(&b, "<h1>The rules of %s</h1>\n<nav>\n", title)
	for _, r := range d.rules {
		fmt.Fprintf(&b, "<a href=\"#%s\">%s</a>\n", html.EscapeString(r.name), html.EscapeString(r.name))
	}
	b.WriteString("<a href=\"#-extras\">extras</a>\n</nav>\n")
	for _, r := range d.rules {
		name := html.EscapeString(r.name)
		fmt.Fprintf(&b, "<section id=\"%s\">\n<h2><a href=\"#%s\">%s</a></h2>\n", name, name, name)
		b.WriteString(diagram(r.node, d.width, link, ""))
		b.WriteString("</section>\n")
	}
	b.WriteString("<section id=\"-extras\">\n<h2><a href=\"#-extras\">extras</a></h2>\n")
	b.WriteString("<p>The grammar allows these extras, such as white space and comments, between any two tokens. The diagrams do not show them.</p>\n")
	b.WriteString(diagram(d.extras, d.width, link, ""))
	b.WriteString("</section>\n")
	if len(d.keywords) > 0 {
		fmt.Fprintf(&b, "<section id=\"-keywords\">\n<h2>Keywords</h2>\n<p>%d rules match one keyword each. The diagrams show each one as its keyword in capitals: %s.</p>\n</section>\n",
			len(d.keywords), html.EscapeString(strings.Join(d.keywords, ", ")))
	}
	if len(d.inlined) > 0 {
		fmt.Fprintf(&b, "<section id=\"-hidden\">\n<h2>Hidden rules</h2>\n<p>The diagrams show these %d hidden rules in place: %s.</p>\n</section>\n",
			len(d.inlined), html.EscapeString(strings.Join(d.inlined, ", ")))
	}
	b.WriteString("</body></html>\n")
	return b.String()
}
