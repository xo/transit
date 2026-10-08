// Command complete completes at a cursor in the input of usql, as usql will
// (D53). It is a sample program of transit.
//
// It parses a text of usql with the usql grammar, with the options of a SQL
// dialect, and parses each SQL statement with the SQL grammar of that
// dialect, with a placeholder for each variable. At each cursor, it says
// whether the cursor is in a meta command, in a variable or in a SQL
// statement. In a meta command, it names the command and the argument. In a
// SQL statement, it lists the keywords and the kinds of node that the parser
// can accept at the cursor, from Layer.StatesAt and the lookahead iterator
// (D57, D70, D111). It also prints the nodes around the cursor, from
// DescendantForByteRange, Parent and FieldNameForChild.
//
// transit gives parsing information only. The program does not know whether
// a name is a table or a column, and it gets no names from a database. usql
// does that (D6).
//
// Run it in the folder _example of the repository, with the cursors as byte
// offsets of the text:
//
//	go run ./complete -dialect mysql -text 'select * from `t` where ' 24
//
// With no offset, the cursor is at the end of the text.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xo/transit"
	"github.com/xo/transit/_example/internal/dialect"
	"github.com/xo/transit/inject"
)

// sample is the text that the program completes in when -text is not given.
const sample = "\\set tbl users\n\\dt+ public.* \nselect id, name from :tbl where id = :'id' and "

func main() {
	switch err := run(context.Background(), os.Args[1:], os.Stdout); {
	case errors.Is(err, flag.ErrHelp):
		// the flag package printed the help
	case err != nil:
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run reads the flags and the offsets in args, and writes the completions at
// each offset to w.
func run(ctx context.Context, args []string, w io.Writer) error {
	fs := flag.NewFlagSet("complete", flag.ContinueOnError)
	name := fs.String("dialect", "postgres", "the SQL dialect: "+strings.Join(dialect.Names(), ", "))
	text := fs.String("text", sample, "the text of usql")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("reading the flags: %w", err)
	}
	d, ok := dialect.Get(*name)
	if !ok {
		return fmt.Errorf("no dialect %q: choose one of %s", *name, strings.Join(dialect.Names(), ", "))
	}
	src := []byte(*text)
	cursors := []int{len(src)}
	if fs.NArg() > 0 {
		cursors = cursors[:0]
		for _, arg := range fs.Args() {
			cursor, err := strconv.Atoi(arg)
			if err != nil || cursor < 0 || cursor > len(src) {
				return fmt.Errorf("the cursor %q is not an offset from 0 to %d", arg, len(src))
			}
			cursors = append(cursors, cursor)
		}
	}
	c, err := newCompleter(d)
	if err != nil {
		return err
	}
	layers, err := c.injector.Layers(ctx, c.parser, src)
	if err != nil {
		return err
	}
	for _, cursor := range cursors {
		if err := c.complete(ctx, w, src, layers, cursor); err != nil {
			return err
		}
	}
	return nil
}

// completer holds what the completion of one dialect needs.
type completer struct {
	dialect  dialect.Dialect
	injector *dialect.Injector
	parser   *transit.Parser
	language *transit.Language
	keywords map[string]bool
}

// newCompleter makes a completer for the dialect d.
func newCompleter(d dialect.Dialect) (*completer, error) {
	injector, err := dialect.NewInjector(d)
	if err != nil {
		return nil, err
	}
	keywords := make(map[string]bool)
	for _, k := range d.Keywords() {
		keywords[k] = true
	}
	return &completer{
		dialect:  d,
		injector: injector,
		parser:   transit.NewParser(),
		language: d.Language(),
		keywords: keywords,
	}, nil
}

// complete writes what goes at cursor in src. layers are the layers of src,
// and the first one is the usql layer.
func (c *completer) complete(ctx context.Context, w io.Writer, src []byte, layers []inject.Layer, cursor int) error {
	start := wordStart(src, cursor)
	word := string(src[start:cursor])
	_, _ = fmt.Fprintf(w, "cursor %d, after %q:\n", cursor, tail(src[:cursor]))
	root := layers[0].Tree.RootNode()
	top, ok := topLevel(root, start)
	switch {
	case ok && top.Kind() == "meta_command" && !strings.Contains(string(src[min(top.EndByte(), start):start]), "\n"):
		c.metaCommand(w, src, top, start)
		return nil
	case ok && top.Kind() == "statement" && (start < top.EndByte() || src[top.EndByte()-1] != ';'):
		if v, ok := enclosing(root, start, "variable"); ok {
			variable(w, src, root, v, word)
			return nil
		}
		i := slices.IndexFunc(layers, func(l inject.Layer) bool {
			return l.Name == "sql" && l.Ranges[0].StartByte == top.StartByte()
		})
		if i < 0 {
			_, _ = fmt.Fprintf(w, "  in a statement that the grammar %s did not parse\n", c.dialect.Name)
			return nil
		}
		return c.statement(ctx, w, layers[i], start, word)
	}
	// The cursor is after the last statement, or before the first one. A
	// new statement starts at the word.
	if err := c.parser.SetLanguage(c.language); err != nil {
		return fmt.Errorf("setting the language %s: %w", c.dialect.Name, err)
	}
	states, err := c.parser.StatesAt(ctx, src[start:], 0, nil)
	if err != nil {
		return fmt.Errorf("finding the states at %d: %w", cursor, err)
	}
	_, _ = fmt.Fprintf(w, "  at the start of a SQL statement of %s\n", c.dialect.Name)
	c.candidates(w, states, word)
	return nil
}

// metaCommand writes the meta command top that the cursor at start is in,
// and the number of its argument there. usql completes a meta command from
// its own list of commands, so the program lists no candidates.
func (c *completer) metaCommand(w io.Writer, src []byte, top transit.Node, start int) {
	name, _ := top.ChildByFieldName("name")
	command := string(src[name.StartByte():name.EndByte()])
	if start <= name.EndByte() {
		_, _ = fmt.Fprintf(w, "  in the name of the meta command %s\n", command)
		return
	}
	argument := 1
	for n := range top.NamedChildren() {
		if n.Equal(name) || n.Kind() == "modifier" {
			continue
		}
		if n.EndByte() < start {
			argument++
		}
	}
	_, _ = fmt.Fprintf(w, "  in argument %d of the meta command %s\n", argument, command)
	_, _ = fmt.Fprintf(w, "  context: %s\n", around(src, rootOf(top), start))
}

// variable writes the variable v that the cursor is in, and as candidates
// the names that the \set commands of src set and that start with word.
func variable(w io.Writer, src []byte, root, v transit.Node, word string) {
	name, _ := v.ChildByFieldName("name")
	_, _ = fmt.Fprintf(w, "  in the variable %s\n", src[name.StartByte():name.EndByte()])
	var names []string
	for n := range root.NamedChildren() {
		if n.Kind() != "meta_command" {
			continue
		}
		command, _ := n.ChildByFieldName("name")
		first, ok := n.NamedChild(1)
		if !ok || string(src[command.StartByte():command.EndByte()]) != `\set` || first.Kind() != "word" {
			continue
		}
		set := string(src[first.StartByte():first.EndByte()])
		if strings.HasPrefix(set, word) && !slices.Contains(names, set) {
			names = append(names, set)
		}
	}
	_, _ = fmt.Fprintf(w, "  variables: %s\n", list(names))
}

// statement writes the context and the candidates at the word that starts at
// start, in the SQL layer l.
func (c *completer) statement(ctx context.Context, w io.Writer, l inject.Layer, start int, word string) error {
	// After the last token of a statement with no semicolon, the cursor is
	// past the end of the layer. The states there are the states at its end.
	offset := min(start, l.Ranges[len(l.Ranges)-1].EndByte)
	states, err := l.StatesAt(ctx, c.parser, offset)
	if err != nil {
		return fmt.Errorf("finding the states at %d: %w", offset, err)
	}
	_, _ = fmt.Fprintf(w, "  in a SQL statement of %s\n", c.dialect.Name)
	_, _ = fmt.Fprintf(w, "  context: %s\n", around(l.Text, l.Tree.RootNode(), offset))
	c.candidates(w, states, word)
	return nil
}

// candidates writes the keywords and the kinds of node that the states
// accept. A keyword must match word. A kind of node has no fixed text, so
// it is listed only when no word is typed.
func (c *completer) candidates(w io.Writer, states []transit.StateID, word string) {
	var keywords, nodes []string
	for _, state := range states {
		it, ok := c.language.LookaheadIterator(state)
		if !ok {
			continue
		}
		for sym := range it.Symbols() {
			name := c.language.SymbolName(sym)
			kind := c.language.SymbolType(sym)
			switch {
			case c.keywords[name] || kind == transit.SymbolAnonymous && isWord(name):
				text := keywordText(name)
				if strings.HasPrefix(strings.ToUpper(text), strings.ToUpper(word)) && !slices.Contains(keywords, text) {
					keywords = append(keywords, text)
				}
			case kind == transit.SymbolRegular && word == "" && !strings.HasPrefix(name, "_"):
				if !slices.Contains(nodes, name) {
					nodes = append(nodes, name)
				}
			}
		}
	}
	slices.Sort(keywords)
	slices.Sort(nodes)
	_, _ = fmt.Fprintf(w, "  keywords: %s\n", list(keywords))
	_, _ = fmt.Fprintf(w, "  nodes: %s\n", list(nodes))
}

// keywordText returns the text of the keyword of the symbol name. The
// grammars of the dialects name a keyword in three ways: by its text, such
// as SELECT, or as kw_select or keyword_select. This is knowledge of the
// grammars that usql keeps, and not of transit.
func keywordText(name string) string {
	for _, prefix := range []string{"keyword_", "kw_"} {
		if s, ok := strings.CutPrefix(name, prefix); ok {
			return strings.ToUpper(s)
		}
	}
	return strings.ToUpper(name)
}

// isWord says whether name is letters and underscores only, as the text of a
// keyword is.
func isWord(name string) bool {
	return name != "" && !strings.ContainsFunc(name, func(r rune) bool {
		return r != '_' && !unicode.IsLetter(r)
	})
}

// topLevel returns the last child of the root that starts at offset or
// before it, and false when none does.
func topLevel(root transit.Node, offset int) (transit.Node, bool) {
	var top transit.Node
	var ok bool
	for n := range root.Children() {
		if n.StartByte() > offset {
			break
		}
		top, ok = n, true
	}
	return top, ok
}

// enclosing returns the node of the kind that holds offset, and false when
// no such node does.
func enclosing(root transit.Node, offset int, kind string) (transit.Node, bool) {
	n, ok := root.DescendantForByteRange(offset, offset)
	for ok {
		if n.Kind() == kind && n.StartByte() <= offset && offset < n.EndByte() {
			return n, true
		}
		n, ok = n.Parent()
	}
	return transit.Node{}, false
}

// around names the nodes that hold offset in src, from the root to the
// innermost, each with the field of its parent that holds it. When only the
// root holds offset, as in the spaces after the last token, the nodes are
// those of the last byte before offset that is not a space.
func around(src []byte, root transit.Node, offset int) string {
	n, ok := root.DescendantForByteRange(offset, offset)
	if ok && n.Equal(root) {
		if i := bytes.LastIndexFunc(src[:offset], func(r rune) bool { return !unicode.IsSpace(r) }); i >= 0 {
			n, ok = root.DescendantForByteRange(i, i)
		}
	}
	if !ok {
		return ""
	}
	var parts []string
	for {
		part := n.Kind()
		parent, ok := n.Parent()
		if !ok {
			parts = append(parts, part)
			break
		}
		if field := fieldOf(parent, n); field != "" {
			part = field + ": " + part
		}
		parts = append(parts, part)
		n = parent
	}
	slices.Reverse(parts)
	return strings.Join(parts, " > ")
}

// fieldOf returns the name of the field of parent that holds child, or "" for
// a child that no field holds.
func fieldOf(parent, child transit.Node) string {
	for i := range parent.ChildCount() {
		if n, ok := parent.Child(i); ok && n.Equal(child) {
			return parent.FieldNameForChild(i)
		}
	}
	return ""
}

// rootOf returns the root node of the tree of n.
func rootOf(n transit.Node) transit.Node {
	for {
		parent, ok := n.Parent()
		if !ok {
			return n
		}
		n = parent
	}
}

// wordStart returns where the word that ends at cursor starts. A word is
// letters, digits and underscores, as rline completes a word (D70).
func wordStart(src []byte, cursor int) int {
	for cursor > 0 {
		r, size := utf8.DecodeLastRune(src[:cursor])
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		cursor -= size
	}
	return cursor
}

// tail returns the last line of text, up to 30 bytes of it.
func tail(text []byte) string {
	s := string(text[bytes.LastIndexByte(text, '\n')+1:])
	if len(s) > 30 {
		s = "..." + s[len(s)-30:]
	}
	return s
}

// list joins names for the output, or gives "none".
func list(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, " ")
}
