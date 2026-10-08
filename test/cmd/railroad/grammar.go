package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// rule is one node of the rules of a grammar.json file.
type rule struct {
	Type    string          `json:"type"`
	Name    string          `json:"name"`
	Value   json.RawMessage `json:"value"`
	Flags   string          `json:"flags"`
	Named   bool            `json:"named"`
	Content *rule           `json:"content"`
	Members []*rule         `json:"members"`
}

// text returns the value of a STRING, a PATTERN or an ALIAS. The value of a
// PREC is a number or a name, and text returns "" for a number.
func (r *rule) text() string {
	var s string
	if err := json.Unmarshal(r.Value, &s); err != nil {
		return ""
	}
	return printable(s)
}

// printable writes each character of s that is not a printable character as
// an escape sequence, such as \uffff for U+FFFF. XML forbids some of them,
// such as U+FFFF, which the patterns of a grammar use as the end of a range,
// so an SVG with them does not parse.
func printable(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case unicode.IsPrint(r) || r == ' ':
			b.WriteRune(r)
		case r <= 0xffff:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			fmt.Fprintf(&b, `\U%08x`, r)
		}
	}
	return b.String()
}

// grammarFile is a grammar.json file. Its rules keep the order of the file.
type grammarFile struct {
	Name      string   `json:"name"`
	Rules     rules    `json:"rules"`
	Extras    []*rule  `json:"extras"`
	Externals []*rule  `json:"externals"`
	Inline    []string `json:"inline"`
}

// rules holds the rules of a grammar in the order of the file.
type rules struct {
	names  []string
	byName map[string]*rule
}

// UnmarshalJSON reads the object of the rules and keeps the order of its
// keys, which a map does not keep.
func (rs *rules) UnmarshalJSON(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return errors.New("reading the rules: expected an object")
	}
	rs.byName = make(map[string]*rule)
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("reading the rules: %w", err)
		}
		name, ok := tok.(string)
		if !ok {
			return errors.New("reading the rules: expected the name of a rule")
		}
		r := new(rule)
		if err := dec.Decode(r); err != nil {
			return fmt.Errorf("reading the rule %s: %w", name, err)
		}
		if _, ok := rs.byName[name]; !ok {
			rs.names = append(rs.names, name)
		}
		rs.byName[name] = r
	}
	return nil
}

// readGrammar reads a grammar.json file.
func readGrammar(rd io.Reader) (*grammarFile, error) {
	g := new(grammarFile)
	if err := json.NewDecoder(rd).Decode(g); err != nil {
		return nil, fmt.Errorf("reading the grammar: %w", err)
	}
	if len(g.Rules.names) == 0 {
		return nil, errors.New("reading the grammar: it has no rules")
	}
	return g, nil
}

// The kinds of node of a diagram.
const (
	kEmpty    = iota // matches nothing, and draws a plain track
	kTerm            // a terminal: a box with round ends
	kNonterm         // the name of a rule: a box with square corners
	kExternal        // a token of the external scanner
	kSeq             // a row of items
	kChoice          // branches, one of which matches
	kOptional        // an item with a track around it
	kLoop            // an item that repeats one or more times, with sep between
	kField           // an item with the name of a field over it
	kLabel           // the name of the rule at the start of a diagram
)

// node is a node of a diagram, made from the rules of a grammar.
type node struct {
	kind  int
	text  string  // the text of a box, a field or a label
	class string  // the class of a box: terminal, keyword, pattern
	href  string  // the rule that the box links to, or ""
	title string  // the full text of a box when text is shortened
	items []*node // the items of a seq or a choice, or the one item
	sep   *node   // the separator of a loop, or nil
}

// The limits of the diagrams.
const (
	// inlineLimit is the largest number of boxes that a hidden rule can have
	// and still be drawn in place.
	inlineLimit = 12
	// patternLimit is the longest pattern, in characters, that a box shows
	// in full.
	patternLimit = 24
)

// typeSymbol is the type of a reference to a rule in grammar.json.
const typeSymbol = "SYMBOL"

// classKeyword is the class of a box that shows a keyword.
const classKeyword = "keyword"

var empty = &node{kind: kEmpty}

// key returns a string that is the same for two nodes when they draw the
// same diagram.
func (n *node) key() string {
	var b strings.Builder
	n.writeKey(&b)
	return b.String()
}

func (n *node) writeKey(b *strings.Builder) {
	fmt.Fprintf(b, "(%d %q %q %q", n.kind, n.text, n.class, n.href)
	for _, it := range n.items {
		it.writeKey(b)
	}
	if n.sep != nil {
		b.WriteString(" sep ")
		n.sep.writeKey(b)
	}
	b.WriteByte(')')
}

// boxes returns the number of boxes in n.
func (n *node) boxes() int {
	switch n.kind {
	case kTerm, kNonterm, kExternal:
		return 1
	}
	c := 0
	for _, it := range n.items {
		c += it.boxes()
	}
	if n.sep != nil {
		c += n.sep.boxes()
	}
	return c
}

// mkSeq returns a row of items. It flattens rows in rows, drops empty
// items, and turns an item that a repeat of a separator and the same item
// follows into a loop with that separator.
func mkSeq(items ...*node) *node {
	var flat []*node
	for _, it := range items {
		switch it.kind {
		case kEmpty:
		case kSeq:
			flat = append(flat, it.items...)
		default:
			flat = append(flat, it)
		}
	}
	flat = joinSeparators(flat)
	switch len(flat) {
	case 0:
		return empty
	case 1:
		return flat[0]
	}
	return &node{kind: kSeq, items: flat}
}

// joinSeparators finds the items a, then (s a)* in items, and replaces them
// with a loop of a with the separator s. The item a can be several items.
func joinSeparators(items []*node) []*node {
	for i := 1; i < len(items); i++ {
		opt := items[i]
		if opt.kind != kOptional || opt.items[0].kind != kLoop || opt.items[0].sep != nil {
			continue
		}
		body := opt.items[0].items[0]
		if body.kind != kSeq {
			continue
		}
		inner := body.items
		// Take the longest repeated part, so that the separator is short.
		for m := min(len(inner)-1, i); m >= 1; m-- {
			if seqKey(inner[len(inner)-m:]) != seqKey(items[i-m:i]) {
				continue
			}
			loop := mkLoop(mkSeq(items[i-m:i]...), mkSeq(inner[:len(inner)-m]...))
			out := append(append(append([]*node{}, items[:i-m]...), loop), items[i+1:]...)
			return joinSeparators(out)
		}
	}
	return items
}

// seqKey returns the key of a row of items.
func seqKey(items []*node) string {
	var b strings.Builder
	for _, it := range items {
		it.writeKey(&b)
	}
	return b.String()
}

// mkChoice returns the branches items. It flattens choices in choices,
// drops repeated branches, and turns an empty branch into an optional part.
func mkChoice(items ...*node) *node {
	var flat []*node
	seen := make(map[string]bool)
	opt := false
	var add func(it *node)
	add = func(it *node) {
		switch it.kind {
		case kEmpty:
			opt = true
		case kChoice:
			for _, sub := range it.items {
				add(sub)
			}
		case kOptional:
			opt = true
			add(it.items[0])
		default:
			if k := it.key(); !seen[k] {
				seen[k] = true
				flat = append(flat, it)
			}
		}
	}
	for _, it := range items {
		add(it)
	}
	var n *node
	switch len(flat) {
	case 0:
		return empty
	case 1:
		n = flat[0]
	default:
		n = &node{kind: kChoice, items: flat}
	}
	if opt {
		return mkOptional(n)
	}
	return n
}

// mkOptional returns item with a track around it.
func mkOptional(item *node) *node {
	switch item.kind {
	case kEmpty, kOptional:
		return item
	}
	return &node{kind: kOptional, items: []*node{item}}
}

// mkLoop returns item repeated one or more times, with sep between the
// repeats when sep is not nil.
func mkLoop(item, sep *node) *node {
	if sep != nil && sep.kind == kEmpty {
		sep = nil
	}
	switch item.kind {
	case kEmpty:
		return empty
	case kOptional:
		// (a?)+ matches what a* matches.
		return mkOptional(mkLoop(item.items[0], sep))
	}
	return &node{kind: kLoop, items: []*node{item}, sep: sep}
}

// converter turns the rules of a grammar into the nodes of diagrams.
type converter struct {
	g         *grammarFile
	externals map[string]bool
	inline    map[string]bool // the rules that the grammar inlines
	keywords  map[string]string
	limit     int

	expanded  map[string]*node
	recursive map[string]bool
	refs      map[string]bool // the rules that a box links to
}

// newConverter returns a converter for g, which draws a hidden rule in
// place when it has limit boxes or fewer.
func newConverter(g *grammarFile, limit int) *converter {
	c := &converter{
		g:         g,
		externals: make(map[string]bool),
		inline:    make(map[string]bool),
		keywords:  make(map[string]string),
		limit:     limit,
		expanded:  make(map[string]*node),
		recursive: make(map[string]bool),
		refs:      make(map[string]bool),
	}
	for _, e := range g.Externals {
		if e.Type == typeSymbol {
			c.externals[e.Name] = true
		}
	}
	for _, name := range g.Inline {
		c.inline[name] = true
	}
	for _, name := range g.Rules.names {
		if kw := keywordRule(name, g.Rules.byName[name]); kw != "" {
			c.keywords[name] = kw
		}
	}
	for _, name := range g.Rules.names {
		if c.candidate(name) {
			c.recursive[name] = c.reaches(name)
		}
	}
	return c
}

// candidate reports whether the rule name can be drawn in place: it is
// hidden, or the grammar inlines it.
func (c *converter) candidate(name string) bool {
	_, ok := c.g.Rules.byName[name]
	return ok && (strings.HasPrefix(name, "_") || c.inline[name]) && c.keywords[name] == ""
}

// reaches reports whether the rule name refers to itself through rules that
// can be drawn in place.
func (c *converter) reaches(name string) bool {
	seen := map[string]bool{}
	stack := symbols(c.g.Rules.byName[name], nil)
	for len(stack) > 0 {
		s := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if s == name {
			return true
		}
		if seen[s] || !c.candidate(s) {
			continue
		}
		seen[s] = true
		stack = symbols(c.g.Rules.byName[s], stack)
	}
	return false
}

// symbols appends the names of the symbols in r to list.
func symbols(r *rule, list []string) []string {
	if r == nil {
		return list
	}
	if r.Type == typeSymbol {
		list = append(list, r.Name)
	}
	list = symbols(r.Content, list)
	for _, m := range r.Members {
		list = symbols(m, list)
	}
	return list
}

// inlined reports whether a reference to the rule name is drawn in place.
func (c *converter) inlined(name string) bool {
	return c.candidate(name) && !c.recursive[name] && c.expand(name).boxes() <= c.limit
}

// expand returns the nodes of the rule name, which must not refer to itself.
func (c *converter) expand(name string) *node {
	if n, ok := c.expanded[name]; ok {
		return n
	}
	n := c.convert(c.g.Rules.byName[name])
	c.expanded[name] = n
	return n
}

// rule returns the diagram of the rule name, with its name at the start.
func (c *converter) rule(name string) *node {
	return mkSeq(&node{kind: kLabel, text: name}, c.convert(c.g.Rules.byName[name]))
}

// extras returns the diagram of the extras of the grammar.
func (c *converter) extras() *node {
	items := make([]*node, len(c.g.Extras))
	for i, e := range c.g.Extras {
		items[i] = c.convert(e)
	}
	return mkSeq(&node{kind: kLabel, text: "extras"}, mkChoice(items...))
}

// convert returns the nodes of r. It drops the wrappers of precedence and
// of tokens, and draws their content.
func (c *converter) convert(r *rule) *node {
	if r == nil {
		return empty
	}
	switch r.Type {
	case "BLANK":
		return empty
	case "STRING":
		return &node{kind: kTerm, text: r.text(), class: "terminal"}
	case "PATTERN":
		return pattern(r.text(), r.Flags)
	case typeSymbol:
		return c.symbol(r.Name)
	case "SEQ":
		items := make([]*node, len(r.Members))
		for i, m := range r.Members {
			items[i] = c.convert(m)
		}
		return mkSeq(items...)
	case "CHOICE":
		items := make([]*node, len(r.Members))
		for i, m := range r.Members {
			items[i] = c.convert(m)
		}
		return mkChoice(items...)
	case "REPEAT":
		return mkOptional(mkLoop(c.convert(r.Content), nil))
	case "REPEAT1":
		return mkLoop(c.convert(r.Content), nil)
	case "FIELD":
		item := c.convert(r.Content)
		if item.kind == kEmpty {
			return item
		}
		return &node{kind: kField, text: r.Name, items: []*node{item}}
	case "ALIAS":
		return c.alias(r)
	}
	// TOKEN, IMMEDIATE_TOKEN, RESERVED and the PREC wrappers.
	return c.convert(r.Content)
}

// symbol returns the nodes of a reference to the rule name.
func (c *converter) symbol(name string) *node {
	if kw, ok := c.keywords[name]; ok {
		return &node{kind: kTerm, text: kw, class: classKeyword}
	}
	if _, ok := c.g.Rules.byName[name]; ok {
		if c.inlined(name) {
			return c.expand(name)
		}
		c.refs[name] = true
		return &node{kind: kNonterm, text: name, href: name}
	}
	if c.externals[name] {
		return &node{kind: kExternal, text: name}
	}
	return &node{kind: kNonterm, text: name}
}

// alias returns the nodes of an ALIAS. An alias that is not named is a
// terminal with the text that it gives. A named alias is a box with the
// name that it gives, which links to the rule under it.
func (c *converter) alias(r *rule) *node {
	value := r.text()
	if !r.Named {
		class := "terminal"
		if in := c.convert(r.Content); in.kind == kTerm && in.class == classKeyword {
			class = classKeyword
		}
		return &node{kind: kTerm, text: value, class: class}
	}
	n := &node{kind: kNonterm, text: value}
	target := ""
	if r.Content != nil && r.Content.Type == typeSymbol {
		target = r.Content.Name
	}
	switch {
	case c.linkable(target):
		n.href = target
	case c.linkable(value):
		n.href = value
	case target != "" && c.externals[target]:
		n.kind = kExternal
	}
	if n.href != "" {
		c.refs[n.href] = true
	}
	return n
}

// linkable reports whether name is a rule that can have a diagram.
func (c *converter) linkable(name string) bool {
	_, ok := c.g.Rules.byName[name]
	return ok && c.keywords[name] == ""
}

// pattern returns the nodes of a PATTERN: the keyword when it matches one
// word in any case, or else the regular expression.
func pattern(value, flags string) *node {
	if kw := keyword(value, flags); kw != "" {
		return &node{kind: kTerm, text: kw, class: classKeyword}
	}
	n := &node{kind: kTerm, text: value, class: "pattern"}
	if r := []rune(value); len(r) > patternLimit {
		n.text = string(r[:patternLimit-1]) + "…"
		n.title = value
	}
	return n
}

// keyword returns the word in capitals when the pattern value matches only
// that word in any case, such as [sS][eE][lL][eE][cC][tT]. Words can be
// separated by \s+ or \s*. It returns "" for any other pattern.
func keyword(value, flags string) string {
	fold := strings.Contains(flags, "i")
	var b strings.Builder
	letters := 0
	r := []rune(value)
	for i := 0; i < len(r); {
		switch {
		case r[i] == '[' && i+3 < len(r) && r[i+3] == ']' && unicode.ToUpper(r[i+1]) == unicode.ToUpper(r[i+2]):
			// A class of the two cases of a letter, such as [sS], or of one
			// character twice, such as [__].
			c := r[i+1]
			switch {
			case unicode.IsLetter(c) && c != r[i+2]:
				letters++
			case !unicode.IsDigit(c) && c != '_':
				return ""
			}
			b.WriteRune(unicode.ToUpper(c))
			i += 4
		case fold && unicode.IsLetter(r[i]):
			b.WriteRune(unicode.ToUpper(r[i]))
			letters++
			i++
		case unicode.IsDigit(r[i]) || r[i] == '_':
			b.WriteRune(r[i])
			i++
		case r[i] == '\\' && i+2 < len(r) && r[i+1] == 's' && (r[i+2] == '+' || r[i+2] == '*'):
			if b.Len() == 0 || i+3 >= len(r) {
				return ""
			}
			b.WriteByte(' ')
			i += 3
		default:
			return ""
		}
	}
	if letters == 0 {
		return ""
	}
	return b.String()
}

// keywordRule returns the keyword that the rule r matches, or "" when it
// matches more than one keyword. A rule matches one keyword when it is a
// pattern that keyword accepts, or a string of letters and the name of the
// rule starts with kw_ or keyword_.
func keywordRule(name string, r *rule) string {
	for r != nil && r.Content != nil && (r.Type == "TOKEN" || r.Type == "IMMEDIATE_TOKEN" || strings.HasPrefix(r.Type, "PREC")) {
		r = r.Content
	}
	if r == nil {
		return ""
	}
	switch r.Type {
	case "PATTERN":
		return keyword(r.text(), r.Flags)
	case "STRING":
		bare := strings.TrimPrefix(name, "_")
		if !strings.HasPrefix(bare, "kw_") && !strings.HasPrefix(bare, "keyword_") {
			return ""
		}
		s := r.text()
		if s == "" || strings.IndexFunc(s, func(c rune) bool { return !unicode.IsLetter(c) && c != '_' }) >= 0 {
			return ""
		}
		return strings.ToUpper(s)
	case "ALIAS":
		if !r.Named && r.Content != nil && keywordRule(name, r.Content) != "" {
			return r.text()
		}
	}
	return ""
}
