// Command highlight highlights the input of usql, as rline will for usql
// (D53). It is a sample program of transit.
//
// It types a text of usql one key at a time. After each key, it tells the
// tree of the usql grammar what changed and parses again with the old tree.
// It then finds the layers of the text with inject.Layers: the usql layer,
// and a layer of the SQL grammar of a dialect for each statement, with a
// placeholder for each variable (D101). It runs the highlight query of the
// usql grammar on the usql tree and the highlight query of the SQL grammar
// on each statement. At the end, it prints the text in the colors of a style
// of the module styles, with the escapes of a terminal of 256 or 16 colors,
// and the time of each edit.
//
// inject.Layers parses each layer with no old tree (D72). So only the usql
// tree is parsed again with its old tree, and the time of an edit counts
// the parse of every layer.
//
// Run it in the folder _example of the repository:
//
//	go run ./highlight -dialect mysql -style monokai -colors 16
//
// The dialects are cql, generic, mysql, oracle, postgres and sqlserver.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xo/transit"
	"github.com/xo/transit/_example/internal/dialect"
	"github.com/xo/transit/grammars/usql"
	"github.com/xo/transit/styles"
)

// sample is the text that the program types when -text is not given.
const sample = `\set id 10
-- the user and the orders
select u.name, count(*) as n
  from users u join orders o on o.user_id = u.id
  where u.id = :id and o.note <> 'none'
  group by u.name;
\echo done
`

func main() {
	switch err := run(context.Background(), os.Args[1:], os.Stdout); {
	case errors.Is(err, flag.ErrHelp):
		// the flag package printed the help
	case err != nil:
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run reads the flags in args, types the text, and writes the text in color
// and the time of each edit to w.
func run(ctx context.Context, args []string, w io.Writer) error {
	fs := flag.NewFlagSet("highlight", flag.ContinueOnError)
	name := fs.String("dialect", "postgres", "the SQL dialect: "+strings.Join(dialect.Names(), ", "))
	styleName := fs.String("style", "monokai", "the style of the module styles")
	colors := fs.Int("colors", 256, "the colors of the terminal: 256 or 16")
	text := fs.String("text", sample, "the text of usql")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("reading the flags: %w", err)
	}
	d, ok := dialect.Get(*name)
	if !ok {
		return fmt.Errorf("no dialect %q: choose one of %s", *name, strings.Join(dialect.Names(), ", "))
	}
	style, ok := styles.Get(*styleName)
	if !ok {
		return fmt.Errorf("no style %q: choose one of %s", *styleName, strings.Join(styles.Names(), ", "))
	}
	if *colors != 256 && *colors != 16 {
		return fmt.Errorf("the colors are 256 or 16, not %d", *colors)
	}
	h, err := newHighlighter(d)
	if err != nil {
		return err
	}
	// Type the text one key at a time, and keep the highlights of the last
	// key.
	var src []byte
	var names []string
	var times []edit
	for _, r := range *text {
		src = utf8.AppendRune(src, r)
		var e edit
		if names, e, err = h.key(ctx, src); err != nil {
			return err
		}
		times = append(times, e)
	}
	_, _ = io.WriteString(w, paint(src, names, style, *colors))
	if len(src) > 0 && src[len(src)-1] != '\n' {
		_, _ = io.WriteString(w, "\n")
	}
	report(w, src, times)
	return nil
}

// highlighter keeps the parse of the text from the last key, and the
// queries that it runs after each key. It compiles each query once.
type highlighter struct {
	dialect  dialect.Dialect
	injector *dialect.Injector
	parser   *transit.Parser
	cursor   *transit.QueryCursor

	// usqlQuery and sqlQuery are the highlight queries of the usql grammar
	// and of the SQL grammar. sqlQuery is nil for a grammar that has no
	// highlight query. The statements of such a grammar keep the highlights
	// of the usql layer: the strings, the comments and the variables.
	usqlQuery *transit.Query
	sqlQuery  *transit.Query

	src  []byte
	tree *transit.Tree
}

// newHighlighter makes a highlighter for the dialect d.
func newHighlighter(d dialect.Dialect) (*highlighter, error) {
	injector, err := dialect.NewInjector(d)
	if err != nil {
		return nil, err
	}
	usqlQuery, err := transit.NewQuery(injector.Usql(), dialect.Query(usql.Queries, "highlights.scm"))
	if err != nil {
		return nil, fmt.Errorf("compiling the highlight query of usql: %w", err)
	}
	h := &highlighter{
		dialect:   d,
		injector:  injector,
		parser:    transit.NewParser(),
		cursor:    transit.NewQueryCursor(),
		usqlQuery: usqlQuery,
	}
	if source := dialect.Query(d.Queries, "highlights.scm"); source != "" {
		if h.sqlQuery, err = transit.NewQuery(d.Language(), source); err != nil {
			return nil, fmt.Errorf("compiling the highlight query of %s: %w", d.Name, err)
		}
	}
	return h, nil
}

// edit is the time that each step of one key took.
type edit struct {
	parse, layers, highlight time.Duration
}

// key parses src, which is the text of the last key with more text at its
// end, and returns the capture name of each byte of src.
func (h *highlighter) key(ctx context.Context, src []byte) ([]string, edit, error) {
	var e edit
	start := time.Now()
	if h.tree != nil {
		h.tree.Edit(insertAtEnd(h.src, src))
	}
	if err := h.parser.SetLanguage(h.injector.Usql()); err != nil {
		return nil, e, fmt.Errorf("setting the language usql: %w", err)
	}
	tree, err := h.parser.Parse(ctx, src, h.tree)
	if err != nil {
		return nil, e, fmt.Errorf("parsing: %w", err)
	}
	h.src, h.tree = src, tree
	e.parse = time.Since(start)

	start = time.Now()
	layers, err := h.injector.Layers(ctx, h.parser, src)
	if err != nil {
		return nil, e, err
	}
	e.layers = time.Since(start)

	start = time.Now()
	names := make([]string, len(src))
	h.capture(ctx, h.usqlQuery, h.tree.RootNode(), names)
	// The root layer comes first, so the layers of the statements paint
	// over it.
	if h.sqlQuery != nil {
		for _, l := range layers[1:] {
			h.capture(ctx, h.sqlQuery, l.Tree.RootNode(), names)
		}
	}
	e.highlight = time.Since(start)
	return names, e, nil
}

// capture runs the highlight query q on the tree of root, and writes the
// capture name of each byte that a capture covers to names. A later capture
// is inside an earlier one, after it, or a later pattern of the same node, so
// the innermost capture wins, and of two patterns that capture one node the
// last one wins, as the highlight test keeps it (D80). A name that starts
// with an underscore belongs to a predicate, and it is not a highlight.
func (h *highlighter) capture(ctx context.Context, q *transit.Query, root transit.Node, names []string) {
	captureNames := q.CaptureNames()
	h.cursor.SetByteRange(root.StartByte(), root.EndByte())
	for m, i := range h.cursor.Captures(ctx, q, root, h.src) {
		c := m.Captures[i]
		name := captureNames[c.Index]
		if strings.HasPrefix(name, "_") {
			continue
		}
		for b := c.Node.StartByte(); b < c.Node.EndByte() && b < len(names); b++ {
			names[b] = name
		}
	}
}

// insertAtEnd describes the edit that turns old into next, where next is
// old with more text at its end.
func insertAtEnd(old, next []byte) transit.InputEdit {
	return transit.InputEdit{
		StartByte:   len(old),
		OldEndByte:  len(old),
		NewEndByte:  len(next),
		StartPoint:  pointAt(old, len(old)),
		OldEndPoint: pointAt(old, len(old)),
		NewEndPoint: pointAt(next, len(next)),
	}
}

// pointAt returns the row and the column, in bytes, of offset in src.
func pointAt(src []byte, offset int) transit.Point {
	before := string(src[:offset])
	return transit.Point{
		Row:    strings.Count(before, "\n"),
		Column: offset - (strings.LastIndexByte(before, '\n') + 1),
	}
}

// paint returns src with the escapes that draw the capture name of each byte
// in style, at a depth of 256 or 16 colors. Text that no capture names
// keeps the colors of the terminal, as a line editor keeps them.
func paint(src []byte, names []string, style *styles.Style, colors int) string {
	var b strings.Builder
	var current string
	for i := 0; i < len(src); {
		name := names[i]
		j := i + 1
		for j < len(src) && names[j] == name {
			j++
		}
		if escape := sgr(style, name, colors); escape != current {
			b.WriteString("\x1b[0m")
			b.WriteString(escape)
			current = escape
		}
		b.Write(src[i:j])
		i = j
	}
	b.WriteString("\x1b[0m")
	return b.String()
}

// sgr returns the escape that selects the entry of the capture name in
// style, or "" for text that no capture names.
func sgr(style *styles.Style, name string, colors int) string {
	if name == "" {
		return ""
	}
	e := style.Lookup(name)
	var codes []string
	if e.Bold {
		codes = append(codes, "1")
	}
	if e.Italic {
		codes = append(codes, "3")
	}
	if e.Underline {
		codes = append(codes, "4")
	}
	codes = appendColor(codes, e.Fg, colors, false)
	codes = appendColor(codes, e.Bg, colors, true)
	if len(codes) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(codes, ";") + "m"
}

// appendColor appends the codes of the color c, as a foreground or as a
// background, at a depth of 256 or 16 colors.
func appendColor(codes []string, c styles.Color, colors int, background bool) []string {
	if !c.IsSet() {
		return codes
	}
	if colors == 256 {
		c = c.To256()
		if background {
			return append(codes, fmt.Sprintf("48;5;%d", c.Value))
		}
		return append(codes, fmt.Sprintf("38;5;%d", c.Value))
	}
	c = c.To16()
	base := uint32(30)
	if background {
		base = 40
	}
	if c.Value >= 8 {
		base += 60 - 8
	}
	return append(codes, strconv.FormatUint(uint64(base+c.Value), 10))
}

// report writes the time of each edit, with the key that made it, and the
// mean and the slowest of them.
func report(w io.Writer, src []byte, times []edit) {
	if len(times) == 0 {
		return
	}
	var keys []string
	for _, r := range string(src) {
		keys = append(keys, fmt.Sprintf("%q", r))
	}
	var total, slowest time.Duration
	for i, e := range times {
		all := e.parse + e.layers + e.highlight
		total += all
		slowest = max(slowest, all)
		_, _ = fmt.Fprintf(w, "edit %d, key %s: %s (parse %s, layers %s, highlight %s)\n",
			i+1, keys[i], round(all), round(e.parse), round(e.layers), round(e.highlight))
	}
	_, _ = fmt.Fprintf(w, "%d edits, %s on average, %s at the most\n",
		len(times), round(total/time.Duration(len(times))), round(slowest))
}

// round rounds d to a microsecond.
func round(d time.Duration) time.Duration {
	return d.Round(time.Microsecond)
}
