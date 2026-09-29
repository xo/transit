package unicodetables

import (
	"cmp"
	"slices"
	"testing"
	"unicode/utf8"
)

// byName holds each ByName table, by its Go name.
var byName = map[string][]Named{
	"AgeByName":                  AgeByName,
	"GeneralCategoryByName":      GeneralCategoryByName,
	"GraphemeClusterBreakByName": GraphemeClusterBreakByName,
	"PerlDecimalByName":          PerlDecimalByName,
	"PerlSpaceByName":            PerlSpaceByName,
	"PropertyBoolByName":         PropertyBoolByName,
	"ScriptByName":               ScriptByName,
	"ScriptExtensionByName":      ScriptExtensionByName,
	"SentenceBreakByName":        SentenceBreakByName,
	"WordBreakByName":            WordBreakByName,
}

// classes returns every class of the package by a name for a test message.
// The classes of the ByName tables cover every class of their files.
func classes() map[string][]Range {
	all := map[string][]Range{"PerlWord": PerlWord}
	for table, named := range byName {
		for _, n := range named {
			all[table+" "+n.Name] = n.Ranges
		}
	}
	return all
}

// TestRanges makes sure that the ranges of each class hold valid characters,
// that each range starts at or before its end, and that the ranges are sorted
// and do not overlap.
func TestRanges(t *testing.T) {
	for name, ranges := range classes() {
		if len(ranges) == 0 {
			t.Errorf("%s: expected ranges, got none", name)
		}
		for i, r := range ranges {
			if !utf8.ValidRune(r.Start) || !utf8.ValidRune(r.End) {
				t.Errorf("%s: range %d: expected Unicode scalar values, got: %U-%U", name, i, r.Start, r.End)
			}
			if r.Start > r.End {
				t.Errorf("%s: range %d: expected the start %U at or before the end %U", name, i, r.Start, r.End)
			}
			if i > 0 && r.Start <= ranges[i-1].End {
				t.Errorf("%s: range %d: expected the start %U after the end %U of range %d", name, i, r.Start, ranges[i-1].End, i-1)
			}
		}
	}
}

// strictlySorted reports whether the keys of s are in strictly increasing
// order, and returns the first index that is out of order.
func strictlySorted[E any, K cmp.Ordered](s []E, key func(E) K) (int, bool) {
	for i := 1; i < len(s); i++ {
		if key(s[i-1]) >= key(s[i]) {
			return i, false
		}
	}
	return 0, true
}

// TestSorted makes sure that each table that the port of unicode.rs searches
// is sorted, with no key twice. unicode.rs searches CASE_FOLDING_SIMPLE by the
// character, and PROPERTY_NAMES, PROPERTY_VALUES, the values of each property
// and each BY_NAME table other than the one of age.rs by the name. The table
// BY_NAME of age.rs is sorted too.
func TestSorted(t *testing.T) {
	for table, named := range byName {
		if i, ok := strictlySorted(named, func(n Named) string { return n.Name }); !ok {
			t.Errorf("%s: expected the names in order, got %q after %q", table, named[i].Name, named[i-1].Name)
		}
	}
	if i, ok := strictlySorted(CaseFoldingSimple, func(f Fold) rune { return f.Rune }); !ok {
		t.Errorf("CaseFoldingSimple: expected the characters in order, got %U after %U", CaseFoldingSimple[i].Rune, CaseFoldingSimple[i-1].Rune)
	}
	if i, ok := strictlySorted(PropertyNames, func(a Alias) string { return a.Name }); !ok {
		t.Errorf("PropertyNames: expected the names in order, got %q after %q", PropertyNames[i].Name, PropertyNames[i-1].Name)
	}
	if i, ok := strictlySorted(PropertyValues, func(p Property) string { return p.Name }); !ok {
		t.Errorf("PropertyValues: expected the names in order, got %q after %q", PropertyValues[i].Name, PropertyValues[i-1].Name)
	}
	for _, p := range PropertyValues {
		if i, ok := strictlySorted(p.Values, func(a Alias) string { return a.Name }); !ok {
			t.Errorf("PropertyValues %s: expected the names in order, got %q after %q", p.Name, p.Values[i].Name, p.Values[i-1].Name)
		}
	}
}

// TestCounts makes sure that the tables hold as many entries as the Rust files
// of regex-syntax 0.8.11.
func TestCounts(t *testing.T) {
	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{"AgeByName", len(AgeByName), 27},
		{"GeneralCategoryByName", len(GeneralCategoryByName), 37},
		{"GraphemeClusterBreakByName", len(GraphemeClusterBreakByName), 13},
		{"PerlDecimalByName", len(PerlDecimalByName), 1},
		{"PerlSpaceByName", len(PerlSpaceByName), 1},
		{"PropertyBoolByName", len(PropertyBoolByName), 65},
		{"ScriptByName", len(ScriptByName), 170},
		{"ScriptExtensionByName", len(ScriptExtensionByName), 170},
		{"SentenceBreakByName", len(SentenceBreakByName), 14},
		{"WordBreakByName", len(WordBreakByName), 18},
		{"CaseFoldingSimple", len(CaseFoldingSimple), 2938},
		{"PropertyNames", len(PropertyNames), 271},
		{"PropertyValues", len(PropertyValues), 7},
		{"PerlWord", len(PerlWord), 796},
		{"GeneralCategoryDecimalNumber", len(GeneralCategoryDecimalNumber), 71},
		{"PropertyValues General_Category", len(propertyValues(t, "General_Category")), 80},
	} {
		if c.got != c.want {
			t.Errorf("%s: expected %d entries, got: %d", c.name, c.want, c.got)
		}
	}
}

// class returns the class with a name in a ByName table, with a binary search
// as unicode.rs does it.
func class(t *testing.T, named []Named, name string) []Range {
	t.Helper()
	i, ok := slices.BinarySearchFunc(named, name, func(n Named, s string) int { return cmp.Compare(n.Name, s) })
	if !ok {
		t.Fatalf("expected the class %s, got none", name)
	}
	return named[i].Ranges
}

// propertyValues returns the values of the property with a name.
func propertyValues(t *testing.T, name string) []Alias {
	t.Helper()
	i, ok := slices.BinarySearchFunc(PropertyValues, name, func(p Property, s string) int { return cmp.Compare(p.Name, s) })
	if !ok {
		t.Fatalf("expected the property %s, got none", name)
	}
	return PropertyValues[i].Values
}

// contains reports whether the ranges hold r.
func contains(ranges []Range, r rune) bool {
	_, ok := slices.BinarySearchFunc(ranges, r, func(x Range, r rune) int {
		switch {
		case x.End < r:
			return -1
		case x.Start > r:
			return 1
		}
		return 0
	})
	return ok
}

// TestClasses makes sure that some classes hold the characters that Unicode
// 16.0.0 gives them.
func TestClasses(t *testing.T) {
	for _, c := range []struct {
		name   string
		ranges []Range
		in     []rune
		out    []rune
	}{
		{"Decimal_Number", class(t, GeneralCategoryByName, "Decimal_Number"), []rune{'0', '9', '٣'}, []rune{'a', '/', ':'}},
		{"PerlDecimalDecimalNumber", PerlDecimalDecimalNumber, []rune{'0', '9'}, []rune{'a'}},
		{"PerlWord", PerlWord, []rune{'_', 'a', 'Z', '0', 'é'}, []rune{' ', '-', '$'}},
		{"PerlSpaceWhiteSpace", PerlSpaceWhiteSpace, []rune{' ', '\t', '\n', 0x3000}, []rune{'a', 0x200B}},
		{"White_Space", class(t, PropertyBoolByName, "White_Space"), []rune{' ', '\t'}, []rune{'x'}},
		{"XID_Start", class(t, PropertyBoolByName, "XID_Start"), []rune{'a', 'Z', 'λ'}, []rune{'0', '_', '$'}},
		{"XID_Continue", class(t, PropertyBoolByName, "XID_Continue"), []rune{'a', '0', '_'}, []rune{'-', ' '}},
		{"Greek", class(t, ScriptByName, "Greek"), []rune{'α', 'Ω'}, []rune{'a'}},
		{"Latin", class(t, ScriptByName, "Latin"), []rune{'a', 'Z'}, []rune{'0', 'α'}},
		{"Cased_Letter", GeneralCategoryCasedLetter, []rune{'a', 'Z'}, []rune{'0'}},
		{"V1_1", AgeV1_1, []rune{'a'}, []rune{0x20AC}},
		{"V2_1", AgeV2_1, []rune{0x20AC}, []rune{'a'}},
		{"CR", class(t, GraphemeClusterBreakByName, "CR"), []rune{'\r'}, []rune{'\n'}},
		{"Double_Quote", class(t, WordBreakByName, "Double_Quote"), []rune{'"'}, []rune{'\''}},
		{"Single_Quote", class(t, WordBreakByName, "Single_Quote"), []rune{'\''}, []rune{'"'}},
	} {
		for _, r := range c.in {
			if !contains(c.ranges, r) {
				t.Errorf("%s: expected %U in the class", c.name, r)
			}
		}
		for _, r := range c.out {
			if contains(c.ranges, r) {
				t.Errorf("%s: expected %U not in the class", c.name, r)
			}
		}
	}
}

// TestCaseFolding makes sure that some characters fold as Unicode 16.0.0
// says.
func TestCaseFolding(t *testing.T) {
	for _, c := range []struct {
		r    rune
		want []rune
	}{
		{'A', []rune{'a'}},
		{'a', []rune{'A'}},
		{'K', []rune{'k', 0x212A}},
		{0x212A, []rune{'K', 'k'}},
		{'σ', []rune{'Σ', 'ς'}},
	} {
		i, ok := slices.BinarySearchFunc(CaseFoldingSimple, c.r, func(f Fold, r rune) int { return cmp.Compare(f.Rune, r) })
		if !ok {
			t.Errorf("%U: expected an entry, got none", c.r)
			continue
		}
		if got := CaseFoldingSimple[i].Folds; !slices.Equal(got, c.want) {
			t.Errorf("%U: expected %U, got: %U", c.r, c.want, got)
		}
	}
	for _, r := range []rune{'0', '_', ' '} {
		if _, ok := slices.BinarySearchFunc(CaseFoldingSimple, r, func(f Fold, r rune) int { return cmp.Compare(f.Rune, r) }); ok {
			t.Errorf("%U: expected no entry", r)
		}
	}
}

// TestAliases makes sure that some names map to the canonical names of
// Unicode.
func TestAliases(t *testing.T) {
	find := func(aliases []Alias, name string) string {
		i, ok := slices.BinarySearchFunc(aliases, name, func(a Alias, s string) int { return cmp.Compare(a.Name, s) })
		if !ok {
			return ""
		}
		return aliases[i].Canonical
	}
	for _, c := range []struct {
		table   string
		aliases []Alias
		name    string
		want    string
	}{
		{"PropertyNames", PropertyNames, "ahex", "ASCII_Hex_Digit"},
		{"PropertyNames", PropertyNames, "gc", "General_Category"},
		{"PropertyNames", PropertyNames, "xidstart", "XID_Start"},
		{"PropertyNames", PropertyNames, "sc", "Script"},
		{"General_Category", propertyValues(t, "General_Category"), "nd", "Decimal_Number"},
		{"General_Category", propertyValues(t, "General_Category"), "lc", "Cased_Letter"},
		{"Script", propertyValues(t, "Script"), "grek", "Greek"},
		{"Age", propertyValues(t, "Age"), "1.1", "V1_1"},
	} {
		if got := find(c.aliases, c.name); got != c.want {
			t.Errorf("%s %q: expected %q, got: %q", c.table, c.name, c.want, got)
		}
	}
}
