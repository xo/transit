package cgrammar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/xo/transit"
)

// fixture is a fixture grammar of grammars/grammars.json.
type fixture struct {
	Name       string `json:"name"`
	Repository string `json:"repository"`
	Path       string `json:"path"`
	Set        string `json:"set"`
	Status     string `json:"status"`
}

// runtimeOnce loads the C runtime once for all tests.
var runtimeOnce = sync.OnceValues(func() (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	cache, err := CacheDir()
	if err != nil {
		return "", err
	}
	so, err := BuildRuntime(context.Background(), root, cache)
	if err != nil {
		return "", err
	}
	return root, LoadRuntime(so)
})

// setup builds and loads the C runtime, and returns the root of the
// repository and the folder of the cache. It skips the test when the
// checkout of upstream is missing, or in short mode.
func setup(t *testing.T) (string, string) {
	t.Helper()
	if testing.Short() {
		t.Skip("the tree tests build every fixture grammar, which takes minutes")
	}
	root, err := runtimeOnce()
	if errors.Is(err, ErrMissing) {
		t.Skipf("skipping: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	cache, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	return root, cache
}

// fixtures returns the fixture grammars of grammars/grammars.json.
func fixtures(t *testing.T, root string) []fixture {
	t.Helper()
	return recorded(t, root, "fixture")
}

// recorded returns the available grammars of grammars/grammars.json that
// are in one of the sets.
func recorded(t *testing.T, root string, sets ...string) []fixture {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "grammars", "grammars.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rec struct {
		Grammars []fixture `json:"grammars"`
	}
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatal(err)
	}
	var out []fixture
	for _, g := range rec.Grammars {
		if slices.Contains(sets, g.Set) && g.Status == "available" {
			out = append(out, g)
		}
	}
	return out
}

// grammarDirs returns the folder of a grammar in the cache of the golden
// harness, and the folder of its corpus: test/corpus in the folder of the
// grammar, or in the nearest folder above it. A grammar that xo writes finds
// the corpus of its module this way.
func grammarDirs(cache string, f fixture) (string, string) {
	repo := strings.TrimPrefix(f.Repository, "https://github.com/")
	checkout := filepath.Join(cache, "grammars", filepath.FromSlash(repo))
	dir := filepath.Join(checkout, f.Path)
	for p := dir; ; p = filepath.Dir(p) {
		corpus := filepath.Join(p, "test", "corpus")
		if _, err := os.Stat(corpus); err == nil || p == checkout || p == filepath.Dir(p) {
			return dir, corpus
		}
	}
}

// loadFixture builds and loads a fixture grammar, and reads its corpus. It
// skips the test when the grammar is not in the cache.
func loadFixture(t *testing.T, cache string, f fixture) (*Grammar, []Example) {
	t.Helper()
	dir, corpusDir := grammarDirs(cache, f)
	so, err := BuildGrammar(context.Background(), dir, cache)
	if errors.Is(err, ErrMissing) {
		t.Skipf("skipping: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	g, err := Load(so, f.Name)
	if err != nil {
		t.Fatal(err)
	}
	examples, err := ReadCorpus(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	return g, examples
}

// errorCorpus returns the tests of test/fixtures/error_corpus of upstream
// for a grammar, which upstream writes to test the recovery from errors.
func errorCorpus(t *testing.T, root, name string) []Example {
	t.Helper()
	file := filepath.Join(root, "tree-sitter", "test", "fixtures", "error_corpus", name+"_errors.txt")
	b, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	examples := parseCorpus(string(b))
	for i := range examples {
		examples[i].File = file
	}
	return examples
}

// goParse parses src with the Go runtime, and turns a panic into an error.
func goParse(p *transit.Parser, src []byte) (tree *transit.Tree, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("the Go runtime panicked: %v\n%s", r, debug.Stack())
		}
	}()
	return p.Parse(context.Background(), src, nil)
}

// TestFixtureTreesMatchC parses each input of the corpus of each fixture
// grammar with the C runtime and with the Go runtime, and compares the two
// trees node by node, and their text. This is the test of the end of phase
// 3 in docs/PLAN.md.
func TestFixtureTreesMatchC(t *testing.T) {
	t.Parallel()
	root, cache := setup(t)
	for _, f := range fixtures(t, root) {
		t.Run(f.Name, func(t *testing.T) {
			t.Parallel()
			g, examples := loadFixture(t, cache, f)
			examples = append(examples, errorCorpus(t, root, f.Name)...)
			p := transit.NewParser()
			if err := p.SetLanguage(g.Language); err != nil {
				t.Fatal(err)
			}
			failures := 0
			for _, e := range examples {
				want, wantString, err := g.CParse(e.Input)
				if err != nil {
					t.Fatal(err)
				}
				tree, err := goParse(p, e.Input)
				if err != nil {
					failures++
					t.Errorf("%s: %s: %v", filepath.Base(e.File), e.Name, err)
					p = transit.NewParser()
					if err := p.SetLanguage(g.Language); err != nil {
						t.Fatal(err)
					}
					continue
				}
				got := GoSnapshot(tree.RootNode(), "")
				if d := Diff(want, got); d != "" {
					failures++
					t.Errorf("%s: %s: the trees differ at %s", filepath.Base(e.File), e.Name, d)
					continue
				}
				if gotString := tree.RootNode().String(); gotString != wantString {
					failures++
					t.Errorf("%s: %s: the text of the trees differs:\n  C:  %s\n  Go: %s", filepath.Base(e.File), e.Name, wantString, gotString)
				}
			}
			t.Logf("%d inputs, %d differ", len(examples), failures)
		})
	}
}

func TestReadCorpus(t *testing.T) {
	content := `===============
The first test
===============

a b c

---

(a
    (b c))

================
The second test
:skip
================
d
---
(d)
`
	checkCorpus(t, content, []Example{
		{Name: "The first test", Input: []byte("\na b c\n")},
		{Name: "The second test", Input: []byte("d")},
	})

	// When one header has a suffix, only the headers with that suffix
	// count, and a longer divider wins over a literal --- of the input.
	suffixed := `==|||
plain
==|||
====|||
suffixed
====|||
x
---
y
-----|||
(x)
`
	checkCorpus(t, suffixed, []Example{
		{Name: "suffixed", Input: []byte("x\n---\ny")},
	})
}

// checkCorpus checks that parseCorpus splits content into want.
func checkCorpus(t *testing.T, content string, want []Example) {
	t.Helper()
	got := parseCorpus(content)
	if len(got) != len(want) {
		t.Fatalf("parseCorpus found %d tests, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Name != want[i].Name || string(got[i].Input) != string(want[i].Input) {
			t.Errorf("test %d = %q %q, want %q %q", i, got[i].Name, got[i].Input, want[i].Name, want[i].Input)
		}
	}
}

// pointAt returns the point of a byte offset of a text.
func pointAt(text []byte, offset int) transit.Point {
	var p transit.Point
	for _, b := range text[:offset] {
		if b == '\n' {
			p.Row++
			p.Column = 0
		} else {
			p.Column++
		}
	}
	return p
}

// edit returns the edit that replaces the bytes from start to oldEnd of src
// with insert, and the text after it.
func edit(src []byte, start, oldEnd int, insert []byte) (transit.InputEdit, []byte) {
	next := make([]byte, 0, len(src)-(oldEnd-start)+len(insert))
	next = append(next, src[:start]...)
	next = append(next, insert...)
	next = append(next, src[oldEnd:]...)
	newEnd := start + len(insert)
	return transit.InputEdit{
		StartByte:   start,
		OldEndByte:  oldEnd,
		NewEndByte:  newEnd,
		StartPoint:  pointAt(src, start),
		OldEndPoint: pointAt(src, oldEnd),
		NewEndPoint: pointAt(next, newEnd),
	}, next
}

// TestFixtureEditsMatchC edits each input of the corpus of each fixture
// grammar: it deletes a few bytes from the middle and parses again with the
// old tree, and then it puts the bytes back and parses again. The C runtime
// and the Go runtime do the same steps, and the test compares the trees and
// the changed ranges after each step.
func TestFixtureEditsMatchC(t *testing.T) {
	t.Parallel()
	root, cache := setup(t)
	for _, f := range fixtures(t, root) {
		t.Run(f.Name, func(t *testing.T) {
			t.Parallel()
			g, examples := loadFixture(t, cache, f)
			examples = append(examples, errorCorpus(t, root, f.Name)...)
			failures := 0
			for _, e := range examples {
				if len(e.Input) == 0 {
					continue
				}
				if msg := compareEdits(g, e.Input); msg != "" {
					failures++
					t.Errorf("%s: %s: %s", filepath.Base(e.File), e.Name, msg)
				}
			}
			t.Logf("%d inputs, %d differ", len(examples), failures)
		})
	}
}

// compareEdits runs the steps of TestFixtureEditsMatchC on one input, and
// returns the first difference, or "".
func compareEdits(g *Grammar, src []byte) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprintf("the Go runtime panicked: %v\n%s", r, debug.Stack())
		}
	}()
	cs, err := g.NewCSession(src)
	if err != nil {
		return err.Error()
	}
	defer cs.Close()
	p := transit.NewParser()
	if err := p.SetLanguage(g.Language); err != nil {
		return err.Error()
	}
	tree, err := p.Parse(context.Background(), src, nil)
	if err != nil {
		return err.Error()
	}

	start := len(src) / 2
	oldEnd := min(start+3, len(src))
	removed := append([]byte(nil), src[start:oldEnd]...)
	del, deleted := edit(src, start, oldEnd, nil)
	ins, restored := edit(deleted, start, start, removed)
	for i, step := range []struct {
		edit transit.InputEdit
		text []byte
	}{{del, deleted}, {ins, restored}} {
		cs.Edit(step.edit)
		old := tree.Copy()
		old.Edit(step.edit)
		wantRanges, err := cs.Reparse(step.text)
		if err != nil {
			return err.Error()
		}
		newTree, err := p.Parse(context.Background(), step.text, old)
		if err != nil {
			return err.Error()
		}
		want, wantString := cs.Snapshot()
		if d := Diff(want, GoSnapshot(newTree.RootNode(), "")); d != "" {
			return fmt.Sprintf("step %d: the trees differ at %s", i, d)
		}
		if got := newTree.RootNode().String(); got != wantString {
			return fmt.Sprintf("step %d: the text of the trees differs:\n  C:  %s\n  Go: %s", i, wantString, got)
		}
		gotRanges := old.ChangedRanges(newTree)
		if !slices.Equal(gotRanges, wantRanges) {
			return fmt.Sprintf("step %d: the changed ranges are %v in C and %v in Go", i, wantRanges, gotRanges)
		}
		tree = newTree
	}
	return ""
}
