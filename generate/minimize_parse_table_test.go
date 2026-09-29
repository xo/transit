package generate

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"testing"
)

// This file tests minimize_parse_table.go. minimize_parse_table.rs has no
// tests upstream, so these tests compare the port with the golden files, and
// the sort with the sort of the Rust standard library.

// minimizeTablesForTest runs the generator on a test grammar up to
// MinimizeParseTable, with the calls and the arguments of build_tables.rs,
// and returns the minimized table. It returns false when an earlier step
// rejects the grammar.
func minimizeTablesForTest(t *testing.T, file string, optimizations OptLevel) (*ParseTable[ActionListID], bool) {
	t.Helper()
	built, err := buildParseTableForTest(t, file)
	if err != nil {
		return nil, false
	}
	prepared, builder, parseTable := built.prepared, built.builder, built.table
	syntaxGrammar, lexicalGrammar := &prepared.SyntaxGrammar, &prepared.LexicalGrammar
	followingTokens := getFollowingTokens(syntaxGrammar, lexicalGrammar, builder)
	tokenConflictMap := NewTokenConflictMap(lexicalGrammar, followingTokens)
	coincidentTokenIndex := NewCoincidentTokenIndex(&parseTable, lexicalGrammar, syntaxGrammar.WordToken, syntaxGrammar.HasWordToken)
	keywords := identifyKeywords(lexicalGrammar, syntaxGrammar.WordToken, syntaxGrammar.HasWordToken, tokenConflictMap, coincidentTokenIndex)
	populateErrorState(&parseTable, syntaxGrammar, lexicalGrammar, coincidentTokenIndex, tokenConflictMap, &keywords)
	populateUsedSymbols(&parseTable, syntaxGrammar, lexicalGrammar)
	table := InternTable(parseTable)
	MinimizeParseTable(
		&table,
		syntaxGrammar,
		lexicalGrammar,
		prepared.DefaultAliases,
		tokenConflictMap,
		&keywords,
		prepared.StrPool,
		optimizations,
	)
	return &table, true
}

// stateCountRE finds the number of parse states in a parser.c.
var stateCountRE = regexp.MustCompile(`(?m)^#define STATE_COUNT (\d+)$`)

// TestMinimizeParseTableStateCount minimizes the table of each test grammar,
// with and without the merge of states, and compares the number of states
// with STATE_COUNT in the golden parser.c. The steps after the minimization
// do not add or remove states.
func TestMinimizeParseTableStateCount(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("testdata", "*", "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, f := range files {
		dir := filepath.Dir(f)
		for _, test := range []struct {
			golden        string
			optimizations OptLevel
		}{
			{"abi15", OptLevelMergeStates},
			{"abi15-nomerge", 0},
		} {
			parser, err := os.ReadFile(filepath.Join(dir, test.golden, "parser.c"))
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			m := stateCountRE.FindSubmatch(parser)
			if m == nil {
				t.Fatalf("%s/%s: parser.c has no STATE_COUNT", dir, test.golden)
			}
			expected, _ := strconv.Atoi(string(m[1]))
			table, ok := minimizeTablesForTest(t, f, test.optimizations)
			if !ok {
				t.Errorf("%s: the grammar has a golden parser.c in %s, and the generator rejects it", dir, test.golden)
				continue
			}
			if len(table.States) != expected {
				t.Errorf("%s/%s: expected %d states, got %d", dir, test.golden, expected, len(table.States))
			}
			checked++
		}
	}
	if checked == 0 {
		t.Error("expected at least one golden parser.c")
	}
}

// TestMinimizeParseTableStateOrder checks the order of the states after the
// minimization: the error state, then the start state, then the other states
// by descending number of entries. It also checks that every reference to a
// state is in the table.
func TestMinimizeParseTableStateOrder(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("testdata", "*", "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		table, ok := minimizeTablesForTest(t, f, OptLevelMergeStates)
		if !ok {
			continue
		}
		for i := range table.States {
			state := &table.States[i]
			if i > 2 {
				prev := &table.States[i-1]
				if prev.TerminalEntries.Len()+prev.NonterminalEntries.Len() < state.TerminalEntries.Len()+state.NonterminalEntries.Len() {
					t.Errorf("%s: state %d has more entries than state %d", f, i, i-1)
				}
			}
			for id := range ReferencedStates(state, &table.ActionLists) {
				if int(id) >= len(table.States) {
					t.Errorf("%s: state %d refers to state %d, which is not in the table", f, i, id)
				}
			}
		}
	}
}

// sortCaseKeysForTest returns the keys of a case of
// TestMinimizeSortUnstableMatchesRust. The Rust program that made the
// expected hash computes the same keys: a linear congruential generator,
// with the key of reorderStatesByDescendingSize in mode 0, ascending runs in
// mode 1, descending runs in mode 2, and random keys in mode 3.
func sortCaseKeysForTest(n int, keyRange, seed uint64, mode int) []int64 {
	keys := make([]int64, n)
	state := seed
	for i := range keys {
		state = state*6364136223846793005 + 1442695040888963407
		r := int64((state >> 33) % keyRange)
		switch mode {
		case 0:
			if i <= 1 {
				keys[i] = int64(i) - 1_000_000
			} else {
				keys[i] = -r
			}
		case 1:
			keys[i] = int64(i) / int64(keyRange)
		case 2:
			keys[i] = -(int64(i) / int64(keyRange))
		default:
			keys[i] = r
		}
	}
	return keys
}

// fnvForTest returns the 64-bit FNV-1a hash of the little-endian bytes of
// each number.
func fnvForTest(values []uint64) uint64 {
	h := uint64(0xcbf29ce484222325)
	var buf [8]byte
	for _, x := range values {
		binary.LittleEndian.PutUint64(buf[:], x)
		for _, b := range buf {
			h ^= uint64(b)
			h *= 0x100000001b3
		}
	}
	return h
}

// sortCasesHashForTest sorts the indices of each case with sort, in the
// order of the Rust program, and returns the hash of the hashes of the
// results.
func sortCasesHashForTest(sort func(v []int, key func(int) int64)) (int, uint64) {
	sizes := []int{2, 5, 17, 18, 20, 21, 25, 31, 32, 33, 40, 63, 64, 65, 100, 200, 513, 1000, 5000, 20000}
	ranges := []uint64{1, 2, 3, 10, 1000}
	var hashes []uint64
	for mode := range 4 {
		for _, n := range sizes {
			for _, keyRange := range ranges {
				for seed := uint64(1); seed < 3; seed++ {
					keys := sortCaseKeysForTest(n, keyRange, seed, mode)
					v := make([]int, n)
					for i := range v {
						v[i] = i
					}
					sort(v, func(i int) int64 { return keys[i] })
					result := make([]uint64, n)
					for i, x := range v {
						result[i] = uint64(x)
					}
					hashes = append(hashes, fnvForTest(result))
				}
			}
		}
	}
	return len(hashes), fnvForTest(hashes)
}

// TestMinimizeSortUnstableMatchesRust sorts 800 slices of indices, with
// many equal keys, and compares the orders with the orders that
// slice::sort_unstable_by_key of rustc 1.97.1 on x86_64 gives. The expected
// hash comes from a Rust program that sorts the same cases and hashes each
// result, with the same hash.
func TestMinimizeSortUnstableMatchesRust(t *testing.T) {
	t.Parallel()
	const (
		expectedCount = 800
		expectedHash  = 0x70c330cc88b2f86a
	)
	count, hash := sortCasesHashForTest(minimizeSortUnstableByKey)
	if count != expectedCount || hash != expectedHash {
		t.Errorf("expected %d cases with the hash %#016x, got %d cases with the hash %#016x", expectedCount, uint64(expectedHash), count, hash)
	}
	// A stable sort gives other orders, so the cases test the order of equal
	// elements.
	_, stableHash := sortCasesHashForTest(func(v []int, key func(int) int64) {
		slices.SortStableFunc(v, func(a, b int) int {
			ka, kb := key(a), key(b)
			switch {
			case ka < kb:
				return -1
			case ka > kb:
				return 1
			}
			return 0
		})
	})
	if stableHash == expectedHash {
		t.Error("expected the stable sort to give other orders than the unstable sort")
	}
}
