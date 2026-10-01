package cgrammar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/xo/transit"
	cgram "github.com/xo/transit/grammars/c"
	"github.com/xo/transit/grammars/javascript"
	jsongram "github.com/xo/transit/grammars/json"
	"github.com/xo/transit/grammars/python"
	"github.com/xo/transit/grammars/rust"
)

// This file holds the benchmarks and the test of the speed targets of D37.
// Each benchmark parses with a grammar package and the Go runtime, and with
// the C grammar of the same grammar.json and the C runtime, on the same
// machine. The line "cpu:" in the output of go test names the machine. The
// name of each benchmark names the grammar and the size of the input, and
// the log of each input names its size, its number of nodes and the offset
// of the key. speed_input_test.go writes the inputs.
//
// The benchmarks and TestSpeedTargets use the C runtime and the C grammars
// of the build with -O2, as upstream builds a grammar (D92). TestSpeedInputs
// compares trees, so it uses the build of the tests.

// speedGrammar is a grammar package of the speed benchmarks, and the input
// that it parses.
type speedGrammar struct {
	name     string
	language func() *transit.Language
	queries  fs.FS
	input    func(size int) string
}

// speedGrammars are the grammar packages of the speed benchmarks. python,
// javascript and rust have an external scanner.
var speedGrammars = []speedGrammar{
	{"json", jsongram.Language, jsongram.Queries, speedJSON},
	{"c", cgram.Language, cgram.Queries, speedC},
	{"javascript", javascript.Language, javascript.Queries, speedJavaScript},
	{"python", python.Language, python.Queries, speedPython},
	{"rust", rust.Language, rust.Queries, speedRust},
}

// speedSizes are the sizes of the inputs in KB. The targets name 10 KB, and
// the other sizes show whether the cost of a key grows with the size.
var speedSizes = []int{5, 10, 20, 40}

// speedInput is an input of the speed benchmarks, and the two edits of one
// key: an x that the key inserts, and the x that the next key deletes.
type speedInput struct {
	src, inserted  []byte
	offset         int
	insert, remove transit.InputEdit
}

// newSpeedInput returns the input of a grammar of about kb KB, and the
// edits of a key in its middle.
func newSpeedInput(sg speedGrammar, kb int) speedInput {
	src := []byte(sg.input(kb * 1024))
	offset := speedKeyOffset(src)
	var at transit.Point
	for _, c := range src[:offset] {
		if c == '\n' {
			at.Row, at.Column = at.Row+1, 0
		} else {
			at.Column++
		}
	}
	after := transit.Point{Row: at.Row, Column: at.Column + 1}
	return speedInput{
		src:      src,
		inserted: slices.Concat(src[:offset], []byte("x"), src[offset:]),
		offset:   offset,
		insert:   transit.InputEdit{StartByte: offset, OldEndByte: offset, NewEndByte: offset + 1, StartPoint: at, OldEndPoint: at, NewEndPoint: after},
		remove:   transit.InputEdit{StartByte: offset, OldEndByte: offset + 1, NewEndByte: offset, StartPoint: at, OldEndPoint: after, NewEndPoint: at},
	}
}

// runtimeO2Once loads the C runtime of the build with -O2 once for all
// benchmarks.
var runtimeO2Once = sync.OnceValues(func() (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	cache, err := CacheDir()
	if err != nil {
		return "", err
	}
	so, err := BuildRuntimeO2(context.Background(), root, cache)
	if err != nil {
		return "", err
	}
	return root, LoadRuntimeO2(so)
})

// loadSpeedGrammar builds and loads the C runtime and the C grammar of a
// grammar package, with the version of its tree-sitter.json, as
// loadGoPackage does. When o2 is true, it uses the build with -O2, and
// otherwise the build of the tests. It skips when the checkout of upstream
// or the grammar is missing.
func loadSpeedGrammar(tb testing.TB, name string, o2 bool) *Grammar {
	tb.Helper()
	build, load, once := BuildGrammarVersion, Load, runtimeOnce
	if o2 {
		build, load, once = BuildGrammarVersionO2, LoadO2, runtimeO2Once
	}
	root, err := once()
	if errors.Is(err, ErrMissing) {
		tb.Skipf("skipping: %v", err)
	}
	if err != nil {
		tb.Fatal(err)
	}
	cache, err := CacheDir()
	if err != nil {
		tb.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "grammars", "grammars.json"))
	if err != nil {
		tb.Fatal(err)
	}
	var rec struct {
		Grammars []fixture `json:"grammars"`
	}
	if err := json.Unmarshal(b, &rec); err != nil {
		tb.Fatal(err)
	}
	i := slices.IndexFunc(rec.Grammars, func(f fixture) bool { return f.Name == name && f.Set == "fixture" })
	if i < 0 {
		tb.Fatalf("no fixture grammar %s in grammars/grammars.json", name)
	}
	dir, _ := grammarDirs(cache, rec.Grammars[i])
	so, err := build(context.Background(), dir, cache)
	if errors.Is(err, ErrMissing) {
		tb.Skipf("skipping: %v", err)
	}
	if err != nil {
		tb.Fatal(err)
	}
	g, err := load(so, name)
	if err != nil {
		tb.Fatal(err)
	}
	return g
}

// speedHighlights compiles queries/highlights.scm of a grammar package.
func speedHighlights(tb testing.TB, sg speedGrammar) *transit.Query {
	tb.Helper()
	src, err := fs.ReadFile(sg.queries, "queries/highlights.scm")
	if err != nil {
		tb.Fatal(err)
	}
	q, err := transit.NewQuery(sg.language(), string(src))
	if err != nil {
		tb.Fatal(err)
	}
	return q
}

// newSpeedParser returns a parser of the Go runtime with the language.
func newSpeedParser(tb testing.TB, language *transit.Language) *transit.Parser {
	tb.Helper()
	p := transit.NewParser()
	if err := p.SetLanguage(language); err != nil {
		tb.Fatal(err)
	}
	return p
}

// benchFirstGo measures a first parse of src with the Go runtime.
func benchFirstGo(b *testing.B, language *transit.Language, src []byte) {
	b.Helper()
	p := newSpeedParser(b, language)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := p.Parse(context.Background(), src, nil); err != nil {
			b.Fatal(err)
		}
	}
}

// benchFirstC measures a first parse of src with the C runtime. The parser
// deletes the tree of the parse before, as a program that drops the tree
// does.
func benchFirstC(b *testing.B, g *Grammar, src []byte) {
	b.Helper()
	s, err := g.NewSpeedSession(src)
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	b.SetBytes(int64(len(src)))
	for b.Loop() {
		if err := s.Parse(0, false); err != nil {
			b.Fatal(err)
		}
	}
}

// benchKeyGo measures a parse after one key with the Go runtime: an edit of
// the tree, a parse with the old tree, and Close of the old tree, as
// benchKeyC deletes it (D91). Each iteration is one key, which inserts the x
// or deletes it again. When q is not nil, each iteration also runs the
// captures of q over the whole new tree.
func benchKeyGo(b *testing.B, language *transit.Language, in speedInput, q *transit.Query) {
	b.Helper()
	p := newSpeedParser(b, language)
	tree, err := p.Parse(context.Background(), in.src, nil)
	if err != nil {
		b.Fatal(err)
	}
	cursor := transit.NewQueryCursor()
	texts := [2][]byte{in.inserted, in.src}
	edits := [2]transit.InputEdit{in.insert, in.remove}
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		tree.Edit(edits[i%2])
		text := texts[i%2]
		old := tree
		if tree, err = p.Parse(context.Background(), text, old); err != nil {
			b.Fatal(err)
		}
		old.Close()
		if q != nil {
			for range cursor.Captures(context.Background(), q, tree.RootNode(), text) {
			}
		}
		i++
	}
}

// benchKeyC measures a parse after one key with the C runtime, as
// benchKeyGo does with no query.
func benchKeyC(b *testing.B, g *Grammar, in speedInput) {
	b.Helper()
	s, err := g.NewSpeedSession(in.inserted, in.src)
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	if err := s.Parse(1, false); err != nil {
		b.Fatal(err)
	}
	edits := [2]transit.InputEdit{in.insert, in.remove}
	i := 0
	for b.Loop() {
		s.Edit(edits[i%2])
		if err := s.Parse(i%2, true); err != nil {
			b.Fatal(err)
		}
		i++
	}
}

// BenchmarkSpeed measures the speed targets of D37 for each grammar of
// speedGrammars, at each size of speedSizes: a first parse with Go and with
// C, a parse after one key with Go and with C, and a parse after one key and
// the highlight query of the grammar over the whole tree with Go. The
// allocations are those of the Go runtime. The C runtime allocates with
// malloc, which the benchmarks do not count.
func BenchmarkSpeed(b *testing.B) {
	for _, sg := range speedGrammars {
		b.Run(sg.name, func(b *testing.B) {
			g := loadSpeedGrammar(b, sg.name, true)
			q := speedHighlights(b, sg)
			for _, kb := range speedSizes {
				in := newSpeedInput(sg, kb)
				b.Run(fmt.Sprintf("%dKB", kb), func(b *testing.B) {
					tree, err := newSpeedParser(b, sg.language()).Parse(context.Background(), in.src, nil)
					if err != nil {
						b.Fatal(err)
					}
					b.Logf("%s: an input of %d bytes, %d nodes, the key at byte %d", sg.name, len(in.src), tree.RootNode().DescendantCount(), in.offset)
					b.Run("first/C", func(b *testing.B) { benchFirstC(b, g, in.src) })
					b.Run("first/Go", func(b *testing.B) { benchFirstGo(b, sg.language(), in.src) })
					b.Run("key/C", func(b *testing.B) { benchKeyC(b, g, in) })
					b.Run("key/Go", func(b *testing.B) { benchKeyGo(b, sg.language(), in, nil) })
					b.Run("key+highlight/Go", func(b *testing.B) { benchKeyGo(b, sg.language(), in, q) })
				})
			}
		})
	}
}

// TestSpeedInputs makes sure that each input of the speed benchmarks parses
// with no error, before and after the key, and that the Go tree after the
// keys is the C tree. It closes each old tree, as the C session deletes it.
func TestSpeedInputs(t *testing.T) {
	t.Parallel()
	setup(t)
	for _, sg := range speedGrammars {
		t.Run(sg.name, func(t *testing.T) {
			t.Parallel()
			g := loadSpeedGrammar(t, sg.name, false)
			for _, kb := range speedSizes {
				in := newSpeedInput(sg, kb)
				p := newSpeedParser(t, sg.language())
				s, err := g.NewSpeedSession(in.inserted, in.src)
				if err != nil {
					t.Fatal(err)
				}
				tree, err := p.Parse(context.Background(), in.src, nil)
				if err == nil {
					err = s.Parse(1, false)
				}
				for i, edit := range []transit.InputEdit{in.insert, in.remove, in.insert} {
					if err != nil {
						break
					}
					if tree.RootNode().HasError() {
						t.Errorf("%d KB, after %d keys: the tree has an error", kb, i)
					}
					tree.Edit(edit)
					s.Edit(edit)
					text := [2][]byte{in.inserted, in.src}[i%2]
					old := tree
					if tree, err = p.Parse(context.Background(), text, old); err == nil {
						old.Close()
						err = s.Parse(i%2, true)
					}
					if err == nil {
						if d := Diff(s.Snapshot(), GoSnapshot(tree.RootNode(), "")); d != "" {
							t.Errorf("%d KB, after %d keys: the trees differ at %s", kb, i+1, d)
						}
					}
				}
				s.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// bestOf runs a benchmark n times and returns the least time of an
// iteration, so that another program on the machine changes the result less.
func bestOf(n int, f func(b *testing.B)) time.Duration {
	best := time.Duration(-1)
	for range n {
		d := time.Duration(testing.Benchmark(f).NsPerOp())
		if best < 0 || d < best {
			best = d
		}
	}
	return best
}

// TestSpeedTargets checks targets 1 and 2 of D37 on the inputs of 10 KB, so
// that a change that makes the Go runtime too slow fails a test. Target 1: a
// parse after one key and the highlight query take less than 2 ms. Target 2:
// a first parse runs at no less than half the speed of the C runtime. Each
// time is the best of three runs. The test does not run in parallel with the
// other tests, because they take the processors. It skips in short mode and
// with the race detector, which makes Go and not C slower.
func TestSpeedTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("the speed targets take a minute")
	}
	if raceEnabled {
		t.Skip("the race detector makes the Go runtime slower")
	}
	for _, sg := range speedGrammars {
		t.Run(sg.name, func(t *testing.T) {
			g := loadSpeedGrammar(t, sg.name, true)
			q := speedHighlights(t, sg)
			in := newSpeedInput(sg, 10)
			key := bestOf(3, func(b *testing.B) { b.Helper(); benchKeyGo(b, sg.language(), in, q) })
			firstGo := bestOf(3, func(b *testing.B) { b.Helper(); benchFirstGo(b, sg.language(), in.src) })
			firstC := bestOf(3, func(b *testing.B) { b.Helper(); benchFirstC(b, g, in.src) })
			ratio := float64(firstGo) / float64(firstC)
			t.Logf("%d bytes: a key and the highlight query %v, a first parse %v in Go and %v in C, %.2f times as long in Go", len(in.src), key, firstGo, firstC, ratio)
			if key >= 2*time.Millisecond {
				t.Errorf("target 1: a key and the highlight query take %v, and the target is less than 2ms", key)
			}
			if ratio > 2 {
				t.Errorf("target 2: a first parse takes %.2f times as long in Go as in C, and the target is at most 2", ratio)
			}
		})
	}
}
