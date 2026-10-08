package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// parse reads a grammar.json file from src.
func parse(t *testing.T, src string) *grammarFile {
	t.Helper()
	g, err := readGrammar(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// convertRule returns the nodes of the rule r of the grammar src, without
// the label of its name.
func convertRule(t *testing.T, src, r string) *node {
	t.Helper()
	g := parse(t, src)
	return newConverter(g, inlineLimit).convert(g.Rules.byName[r])
}

// validSVG expects s to be well formed XML with an svg element at its root.
func validSVG(t *testing.T, name, s string) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(s))
	root := ""
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("%s: expected well formed XML, got %v", name, err)
		}
		if se, ok := tok.(xml.StartElement); ok && root == "" {
			root = se.Name.Local
		}
	}
	if root != "svg" {
		t.Errorf("%s: expected an svg element, got %q", name, root)
	}
}

func TestOrder(t *testing.T) {
	g := parse(t, `{"name":"t","rules":{"zeta":{"type":"BLANK"},"alpha":{"type":"BLANK"},"mid":{"type":"BLANK"}}}`)
	if got, want := strings.Join(g.Rules.names, ","), "zeta,alpha,mid"; got != want {
		t.Errorf("expected the rules %s, got %s", want, got)
	}
}

func TestChoice(t *testing.T) {
	n := convertRule(t, `{"name":"t","rules":{"r":{"type":"CHOICE","members":[
		{"type":"STRING","value":"a"},{"type":"STRING","value":"b"},{"type":"STRING","value":"a"}]}}}`, "r")
	if n.kind != kChoice || len(n.items) != 2 {
		t.Fatalf("expected a choice of two branches, got %s", n.key())
	}
	b := layout(n, 1000)
	if b.down < boxHeight/2+gap+boxHeight {
		t.Errorf("expected the second branch under the first, got %v below the rail", b.down)
	}
	svg := diagram(n, 1000, nil, "")
	validSVG(t, "choice", svg)
	if c := strings.Count(svg, "<rect x="); c != 2 {
		t.Errorf("expected 2 boxes, got %d", c)
	}
}

func TestRepeat(t *testing.T) {
	src := `{"name":"t","rules":{"r":{"type":"REPEAT","content":{"type":"SYMBOL","name":"x"}},
		"s":{"type":"REPEAT1","content":{"type":"SYMBOL","name":"x"}},"x":{"type":"STRING","value":"x"}}}`
	n := convertRule(t, src, "r")
	if n.kind != kOptional || n.items[0].kind != kLoop || n.items[0].items[0].kind != kNonterm {
		t.Errorf("expected an optional loop of a rule name, got %s", n.key())
	}
	n = convertRule(t, src, "s")
	if n.kind != kLoop {
		t.Fatalf("expected a loop, got %s", n.key())
	}
	b := layout(n, 1000)
	if b.down < 2*radius {
		t.Errorf("expected the track back under the item, got %v below the rail", b.down)
	}
	validSVG(t, "repeat", diagram(n, 1000, nil, ""))
}

func TestOptional(t *testing.T) {
	n := convertRule(t, `{"name":"t","rules":{"r":{"type":"SEQ","members":[{"type":"STRING","value":"a"},
		{"type":"CHOICE","members":[{"type":"STRING","value":"b"},{"type":"BLANK"}]}]}}}`, "r")
	if n.kind != kSeq || n.items[1].kind != kOptional || n.items[1].items[0].text != "b" {
		t.Fatalf("expected a row of a and an optional b, got %s", n.key())
	}
	b := layout(n, 1000)
	if b.up < 2*radius {
		t.Errorf("expected the track around b over the rail, got %v above it", b.up)
	}
}

func TestSeparator(t *testing.T) {
	n := convertRule(t, `{"name":"t","rules":{"r":{"type":"SEQ","members":[{"type":"SYMBOL","name":"x"},
		{"type":"REPEAT","content":{"type":"SEQ","members":[{"type":"STRING","value":","},{"type":"SYMBOL","name":"x"}]}}]},
		"x":{"type":"PATTERN","value":"[0-9]+"}}}`, "r")
	if n.kind != kLoop || n.sep == nil || n.sep.text != "," || n.items[0].text != "x" {
		t.Errorf("expected a loop of x with the separator \",\", got %s", n.key())
	}
}

func TestField(t *testing.T) {
	n := convertRule(t, `{"name":"t","rules":{"r":{"type":"FIELD","name":"left","content":{"type":"SYMBOL","name":"x"}},
		"x":{"type":"STRING","value":"x"}}}`, "r")
	if n.kind != kField || n.text != "left" {
		t.Fatalf("expected the field left, got %s", n.key())
	}
	svg := diagram(n, 1000, nil, "")
	validSVG(t, "field", svg)
	if !strings.Contains(svg, `<text class="field"`) || !strings.Contains(svg, ">left</text>") {
		t.Error("expected the label of the field in the diagram")
	}
}

func TestAlias(t *testing.T) {
	src := `{"name":"t","rules":{
		"r":{"type":"ALIAS","named":true,"value":"table_name","content":{"type":"SYMBOL","name":"_name"}},
		"s":{"type":"ALIAS","named":false,"value":"WITH","content":{"type":"PATTERN","value":"[wW][iI][tT][hH]"}},
		"_name":{"type":"PATTERN","value":"[a-z]+"}}}`
	n := convertRule(t, src, "r")
	if n.kind != kNonterm || n.text != "table_name" || n.href != "_name" {
		t.Errorf("expected the box table_name that links to _name, got %s", n.key())
	}
	n = convertRule(t, src, "s")
	if n.kind != kTerm || n.text != "WITH" || n.class != "keyword" {
		t.Errorf("expected the keyword WITH, got %s", n.key())
	}
	d := draw(parse(t, src), 1000, inlineLimit)
	if !hasRule(d, "_name") {
		t.Error("expected a diagram of _name, which an alias links to")
	}
}

func TestKeyword(t *testing.T) {
	src := `{"name":"t","rules":{
		"r":{"type":"SEQ","members":[{"type":"SYMBOL","name":"keyword_select"},{"type":"SYMBOL","name":"kw_low"},{"type":"SYMBOL","name":"kw_order"}]},
		"keyword_select":{"type":"PATTERN","value":"[sS][eE][lL][eE][cC][tT]"},
		"kw_low":{"type":"TOKEN","content":{"type":"PREC","value":1,"content":{"type":"PATTERN","value":"[lL][oO][wW][__][pP][rR][iI]"}}},
		"kw_order":{"type":"PATTERN","value":"[oO][rR][dD][eE][rR]\\s+[bB][yY]"}}}`
	n := convertRule(t, src, "r")
	var got []string
	for _, it := range n.items {
		if it.kind != kTerm || it.class != "keyword" {
			t.Errorf("expected a keyword, got %s", it.key())
		}
		got = append(got, it.text)
	}
	if s, want := strings.Join(got, ","), "SELECT,LOW_PRI,ORDER BY"; s != want {
		t.Errorf("expected the keywords %s, got %s", want, s)
	}
	d := draw(parse(t, src), 1000, inlineLimit)
	if hasRule(d, "keyword_select") || len(d.keywords) != 3 {
		t.Errorf("expected no diagram of the 3 keyword rules, got %d rules and %d keywords", len(d.rules), len(d.keywords))
	}
	for _, p := range []string{"[sS][eE]x", "[a-z]+", "[sS]\\s+", "[ss]"} {
		if kw := keyword(p, ""); kw != "" {
			t.Errorf("%s: expected no keyword, got %q", p, kw)
		}
	}
	if kw := keyword("select", "i"); kw != "SELECT" {
		t.Errorf("expected SELECT for a pattern with the flag i, got %q", kw)
	}
}

func TestHidden(t *testing.T) {
	src := `{"name":"t","rules":{
		"r":{"type":"SEQ","members":[{"type":"SYMBOL","name":"_small"},{"type":"SYMBOL","name":"_rec"}]},
		"_small":{"type":"SEQ","members":[{"type":"STRING","value":"a"},{"type":"STRING","value":"b"}]},
		"_rec":{"type":"CHOICE","members":[{"type":"STRING","value":"c"},{"type":"SEQ","members":[{"type":"STRING","value":"("},{"type":"SYMBOL","name":"_rec"},{"type":"STRING","value":")"}]}]}}}`
	n := convertRule(t, src, "r")
	if n.kind != kSeq || len(n.items) != 3 || n.items[0].text != "a" || n.items[2].text != "_rec" {
		t.Errorf("expected a, b and the box _rec, got %s", n.key())
	}
	d := draw(parse(t, src), 1000, inlineLimit)
	if !hasRule(d, "_rec") || hasRule(d, "_small") {
		t.Error("expected a diagram of _rec and none of _small")
	}
	g := parse(t, src)
	if n := newConverter(g, 1).convert(g.Rules.byName["r"]); n.items[0].text != "_small" {
		t.Errorf("expected the box _small over the limit, got %s", n.key())
	}
}

func TestWrap(t *testing.T) {
	var members []string
	for range 30 {
		members = append(members, `{"type":"STRING","value":"word"}`)
	}
	n := convertRule(t, `{"name":"t","rules":{"r":{"type":"SEQ","members":[`+strings.Join(members, ",")+`]}}}`, "r")
	b := layout(n, 600)
	if b.w > 600 || b.down < 3*boxHeight {
		t.Errorf("expected several rows of 600 or less, got %v wide and %v below the rail", b.w, b.down)
	}
	validSVG(t, "wrap", diagram(n, 600, nil, ""))
}

func TestColumns(t *testing.T) {
	var members []string
	for range 40 {
		members = append(members, `{"type":"STRING","value":"word"}`)
	}
	members = append(members, `{"type":"STRING","value":"other"}`)
	n := convertRule(t, `{"name":"t","rules":{"r":{"type":"CHOICE","members":[`+strings.Join(members, ",")+`]}}}`, "r")
	if n.kind != kChoice || len(n.items) != 2 {
		t.Fatalf("expected the repeated branches to merge, got %s", n.key())
	}
	n.items = nil
	for i := range 40 {
		n.items = append(n.items, &node{kind: kTerm, class: "keyword", text: strings.Repeat("W", i%7+2)})
	}
	b := layout(n, 1000)
	if b.down > 15*(boxHeight+gap) {
		t.Errorf("expected columns, got %v below the rail", b.down)
	}
	validSVG(t, "columns", diagram(n, 1000, nil, ""))
}

// hasRule reports whether d has a diagram of the rule name.
func hasRule(d *drawing, name string) bool {
	for _, r := range d.rules {
		if r.name == name {
			return true
		}
	}
	return false
}

var (
	svgElement = regexp.MustCompile(`(?s)<svg .*?</svg>`)
	sectionID  = regexp.MustCompile(`<section id="([^"]+)">`)
	hrefAttr   = regexp.MustCompile(`href="#([^"]*)"`)
	idAttr     = regexp.MustCompile(`id="([^"]+)"`)
)

// TestRun draws the grammars json and usql, and expects a diagram of each
// visible rule, valid SVG, and links to anchors of the page.
func TestRun(t *testing.T) {
	for _, gram := range []string{"json", "usql", "mysql"} {
		t.Run(gram, func(t *testing.T) {
			out := t.TempDir()
			for _, style := range []string{"dark", "light"} {
				if err := run(io.Discard, gram, out, style, 1000, inlineLimit); err != nil {
					t.Fatal(err)
				}
			}
			path, err := grammarPath(gram)
			if err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			g, err := readGrammar(f)
			_ = f.Close()
			if err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(filepath.Join(out, "index.html"))
			if err != nil {
				t.Fatal(err)
			}
			page := string(b)
			sections := make(map[string]bool)
			for _, m := range sectionID.FindAllStringSubmatch(page, -1) {
				sections[m[1]] = true
			}
			c := newConverter(g, inlineLimit)
			for _, name := range g.Rules.names {
				if strings.HasPrefix(name, "_") || c.keywords[name] != "" {
					continue
				}
				if !sections[name] {
					t.Errorf("expected a diagram of the rule %s", name)
				}
				svg, err := os.ReadFile(filepath.Join(out, name+".svg"))
				if err != nil {
					t.Error(err)
					continue
				}
				validSVG(t, name+".svg", string(svg))
			}
			// The sections of the keywords and of the hidden rules hold no
			// diagram.
			want := len(sections)
			for _, id := range []string{"-keywords", "-hidden"} {
				if sections[id] {
					want--
				}
			}
			svgs := svgElement.FindAllString(page, -1)
			if len(svgs) != want {
				t.Errorf("expected %d diagrams, got %d", want, len(svgs))
			}
			for i, s := range svgs {
				validSVG(t, fmt.Sprintf("diagram %d", i), s)
			}
			ids := make(map[string]bool)
			for _, m := range idAttr.FindAllStringSubmatch(page, -1) {
				ids[m[1]] = true
			}
			links := hrefAttr.FindAllStringSubmatch(page, -1)
			if len(links) == 0 {
				t.Fatal("expected links in the page, got none")
			}
			for _, m := range links {
				if !ids[m[1]] {
					t.Errorf("expected an anchor for the link #%s", m[1])
				}
			}
		})
	}
}

// TestPrintable writes the characters of a pattern that XML forbids, or that
// do not print, as escape sequences.
func TestPrintable(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"abc", "abc"},
		{"[a-zé]", "[a-zé]"},
		{"[\u0080-\uffff]", `[\u0080-\uffff]`},
		{"a\tb\nc", `a\tb\nc`},
		{"\x00", `\u0000`},
		{"\U0010ffff", `\U0010ffff`},
	} {
		if got := printable(tc.in); got != tc.want {
			t.Errorf("printable(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
