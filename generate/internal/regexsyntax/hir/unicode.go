package hir

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/xo/transit/generate/internal/regexsyntax/unicodetables"
)

// This file ports src/unicode.rs: the lookup of Unicode classes and of the
// simple case folding in the Unicode tables. The file is in package hir,
// because unicode.rs and the module hir use each other. The tables are in the
// package unicodetables. Each crate::unicode_tables::x::NAME of upstream is
// the Go name that the package unicodetables gives it, such as
// unicodetables.GeneralCategoryByName for general_category::BY_NAME.
//
// The port follows the default features of the crate, which turn on every
// table. So only the code for a feature that is on is here, and the code for
// a feature that is off is not. With every feature on, some functions of
// upstream return a Result that is always Ok. The port drops that error from
// newSimpleCaseFolder, perlWord, perlSpace, perlDigit, canonicalGencat,
// canonicalScript, canonicalProp and propertyValues.
//
// The port leaves out is_word_character and UnicodeWordError, which only
// try_is_word_character of lib.rs uses. The port leaves out lib.rs.
//
// Upstream calls the error of this file Error. Package hir has an Error, so
// here it is unicodeError.

// unicodeError is an error of a lookup in the Unicode tables. The translator
// turns it into an *Error.
//
// unicodeError is Error.
type unicodeError uint8

// The kinds of unicodeError, in the order of upstream.
const (
	// errPropertyNotFound is PropertyNotFound.
	errPropertyNotFound unicodeError = iota
	// errPropertyValueNotFound is PropertyValueNotFound.
	errPropertyValueNotFound
	// errPerlClassNotFound is PerlClassNotFound. The port never returns it,
	// because the table of each Perl class is always there.
	errPerlClassNotFound
)

// Error returns the name of the error.
//
// Error is the derived Debug of Error. Upstream does not implement Display
// for it, because the translator always turns it into another error.
func (e unicodeError) Error() string {
	switch e {
	case errPropertyNotFound:
		return "PropertyNotFound"
	case errPropertyValueNotFound:
		return "PropertyValueNotFound"
	case errPerlClassNotFound:
		return "PerlClassNotFound"
	}
	return ""
}

// CaseFoldError is the error of a Unicode case fold, when the tables for
// case folding are not available. The port always has the tables, so it
// never returns this error.
//
// CaseFoldError is CaseFoldError.
type CaseFoldError struct{}

// Error returns the text of the error.
//
// Error is the Display of CaseFoldError.
func (CaseFoldError) Error() string {
	return "Unicode-aware case folding is not available (probably because the unicode-case feature is not enabled)"
}

// simpleCaseFolder walks the table of simple case folding with a state.
//
// After newSimpleCaseFolder, a caller calls mapping with characters in
// strictly increasing order. For example, to call it on b and then on a is
// not allowed, and mapping panics. The order lets the lookups use the
// structure of the table, and so they are fast.
//
// simpleCaseFolder is SimpleCaseFolder.
type simpleCaseFolder struct {
	// table is the table of simple case folding. It is sorted by the
	// character. The value of each entry is the class of characters that
	// case fold with the character, without the character itself.
	table []unicodetables.Fold
	// last is the last character of a lookup, when hasLast is true.
	last    rune
	hasLast bool
	// next is the index of the entry of table with the smallest character
	// k such that k > k0, where k0 is the character of the most recent
	// lookup. k0 is not always in the table.
	next int
}

// newSimpleCaseFolder returns a new simple case folder.
//
// newSimpleCaseFolder is SimpleCaseFolder::new, with the feature
// unicode-case on.
func newSimpleCaseFolder() *simpleCaseFolder {
	return &simpleCaseFolder{
		table:   unicodetables.CaseFoldingSimple,
		hasLast: false,
		next:    0,
	}
}

// mapping returns the characters that case fold with c, without c itself.
// If c has no entry in the table, it returns an empty slice.
//
// mapping panics if c is not after the character of the call before it. A
// caller must call it with characters in strictly increasing order.
//
// mapping is SimpleCaseFolder::mapping.
func (f *simpleCaseFolder) mapping(c rune) []rune {
	if f.hasLast {
		if f.last >= c {
			panic(fmt.Sprintf("got codepoint U+%X which occurs before last codepoint U+%X", uint32(c), uint32(f.last)))
		}
	}
	f.last, f.hasLast = c, true
	if f.next >= len(f.table) {
		return nil
	}
	e := f.table[f.next]
	if e.Rune == c {
		f.next++
		return e.Folds
	}
	i, found := f.get(c)
	if !found {
		f.next = i
		return nil
	}
	// The lookups go in order, so the entry must be after the entry that
	// the code took for the next one. If it is not, the caller goes out of
	// order, or the entry was at next and the code found it above.
	if i <= f.next {
		panic("assertion failed: i > self.next")
	}
	f.next = i + 1
	return f.table[i].Folds
}

// overlaps reports whether a character in the range from start to end, with
// both ends in the range, has an entry in the table. When it returns true,
// at least one character in the range case folds with another character.
// When it returns false, each character in the range case folds only with
// itself.
//
// A caller can call it before it walks the characters of a range, to skip
// the walk when no lookup can find anything.
//
// overlaps panics if end is before start.
//
// overlaps is SimpleCaseFolder::overlaps.
func (f *simpleCaseFolder) overlaps(start, end rune) bool {
	if start > end {
		panic("assertion failed: start <= end")
	}
	_, found := slices.BinarySearchFunc(f.table, 0, func(e unicodetables.Fold, _ int) int {
		switch {
		case start <= e.Rune && e.Rune <= end:
			return 0
		case e.Rune > end:
			return 1
		}
		return -1
	})
	return found
}

// get returns the index of c in the table, and true. If c is not in the
// table, it returns an index i such that the character of table[i-1] is
// before c and the character of table[i] is after c, and false.
//
// get is SimpleCaseFolder::get.
func (f *simpleCaseFolder) get(c rune) (int, bool) {
	return slices.BinarySearchFunc(f.table, c, func(e unicodetables.Fold, c rune) int {
		return cmp.Compare(e.Rune, c)
	})
}

// classQueryKind is the kind of a classQuery.
type classQueryKind uint8

// The kinds of classQuery, in the order of the enum ClassQuery.
const (
	queryOneLetter classQueryKind = iota
	queryBinary
	queryByValue
)

// classQuery is a query for a class that Unicode defines. It names a
// property, or a property and a value. A property name alone is usually a
// binary property (UTS#44, Table 8). By a special exception (UTS#18, Section
// 1.2), the general categories, which are an enumeration, and the scripts,
// which are a catalog, work as though each of their values is a binary
// property.
//
// The query normalizes the names and the values, and makes them canonical.
// So GC, gc, GeneralCategory and general_category are the same.
//
// classQuery is ClassQuery, an enum with data upstream. The fields that a
// kind uses are:
//
//   - queryOneLetter: letter, a binary property with a name of one letter.
//   - queryBinary: name, a binary property. By the exception above, it can
//     be a general category or a script.
//   - queryByValue: propertyName and propertyValue, the class of all the
//     characters with the value propertyValue of the property propertyName.
type classQuery struct {
	kind          classQueryKind
	letter        rune
	name          string
	propertyName  string
	propertyValue string
}

// canonicalize returns the canonical form of the query.
//
// canonicalize is ClassQuery::canonicalize.
func (q classQuery) canonicalize() (canonicalClassQuery, error) {
	switch q.kind {
	case queryOneLetter:
		return q.canonicalBinary(string(q.letter))
	case queryBinary:
		return q.canonicalBinary(q.name)
	}
	propertyName := symbolicNameNormalize(q.propertyName)
	propertyValue := symbolicNameNormalize(q.propertyValue)

	canonName, ok := canonicalProp(propertyName)
	if !ok {
		return canonicalClassQuery{}, errPropertyNotFound
	}
	switch canonName {
	case "General_Category":
		canon, ok := canonicalGencat(propertyValue)
		if !ok {
			return canonicalClassQuery{}, errPropertyValueNotFound
		}
		return canonicalClassQuery{kind: canonicalGeneralCategory, name: canon}, nil
	case "Script":
		canon, ok := canonicalScript(propertyValue)
		if !ok {
			return canonicalClassQuery{}, errPropertyValueNotFound
		}
		return canonicalClassQuery{kind: canonicalScriptKind, name: canon}, nil
	}
	vals, ok := propertyValues(canonName)
	if !ok {
		return canonicalClassQuery{}, errPropertyValueNotFound
	}
	canonVal, ok := canonicalValue(vals, propertyValue)
	if !ok {
		return canonicalClassQuery{}, errPropertyValueNotFound
	}
	return canonicalClassQuery{
		kind:          canonicalByValue,
		propertyName:  canonName,
		propertyValue: canonVal,
	}, nil
}

// canonicalBinary returns the canonical form of a query for the binary
// property name.
//
// canonicalBinary is ClassQuery::canonical_binary.
func (q classQuery) canonicalBinary(name string) (canonicalClassQuery, error) {
	norm := symbolicNameNormalize(name)

	// This is a special case. cf is the general category Format, but cf is
	// also an abbreviation of the property Case_Folding. The code takes it
	// for the general category. The port does not support Case_Folding. If
	// it does in the future, a user must spell the name in full.
	//
	// And sc is the general category Currency_Symbol, but sc is also the
	// abbreviation of the property Script. So the code does not call
	// canonicalProp for it either. canonicalProp makes it Script, which is
	// wrong.
	//
	// Another case: lc is an abbreviation of the general category
	// Cased_Letter, and also of the property Lowercase_Mapping. The port does
	// not support Lowercase_Mapping, so as with cf above, lc is
	// Cased_Letter.
	if norm != "cf" && norm != "sc" && norm != "lc" {
		canon, ok := canonicalProp(norm)
		if ok {
			return canonicalClassQuery{kind: canonicalBinaryKind, name: canon}, nil
		}
	}
	canon, ok := canonicalGencat(norm)
	if ok {
		return canonicalClassQuery{kind: canonicalGeneralCategory, name: canon}, nil
	}
	canon, ok = canonicalScript(norm)
	if ok {
		return canonicalClassQuery{kind: canonicalScriptKind, name: canon}, nil
	}
	return canonicalClassQuery{}, errPropertyNotFound
}

// canonicalClassQueryKind is the kind of a canonicalClassQuery.
type canonicalClassQueryKind uint8

// The kinds of canonicalClassQuery, in the order of the enum
// CanonicalClassQuery. The names of Binary and Script have the suffix Kind,
// because the functions canonicalBinary and canonicalScript have the names
// without it.
const (
	canonicalBinaryKind canonicalClassQueryKind = iota
	canonicalGeneralCategory
	canonicalScriptKind
	canonicalByValue
)

// canonicalClassQuery is a classQuery with canonical names and values. It
// also tells a binary property from a general category or a script, which a
// classQuery takes as a binary property.
//
// canonicalClassQuery is CanonicalClassQuery, an enum with data upstream.
// The fields that a kind uses are:
//
//   - canonicalBinaryKind: name, the canonical name of a binary property.
//   - canonicalGeneralCategory: name, the canonical name of a general
//     category.
//   - canonicalScriptKind: name, the canonical name of a script.
//   - canonicalByValue: propertyName and propertyValue, a property and its
//     value, both canonical. The property is never General_Category or
//     Script, because the kinds above hold those two cases.
type canonicalClassQuery struct {
	kind          canonicalClassQueryKind
	name          string
	propertyName  string
	propertyValue string
}

// unicodeClass returns the Unicode class of the query.
//
// unicodeClass is class.
func unicodeClass(query classQuery) (*ClassUnicode, error) {
	canon, err := query.canonicalize()
	if err != nil {
		return nil, err
	}
	switch canon.kind {
	case canonicalBinaryKind:
		return boolProperty(canon.name)
	case canonicalGeneralCategory:
		return gencat(canon.name)
	case canonicalScriptKind:
		return script(canon.name)
	}
	switch canon.propertyName {
	case "Age":
		class := EmptyClassUnicode()
		sets, err := ages(canon.propertyValue)
		if err != nil {
			return nil, err
		}
		for _, set := range sets {
			class.Union(hirClass(set))
		}
		return class, nil
	case "Script_Extensions":
		return scriptExtension(canon.propertyValue)
	case "Grapheme_Cluster_Break":
		return gcb(canon.propertyValue)
	case "Sentence_Break":
		return sb(canon.propertyValue)
	case "Word_Break":
		return wb(canon.propertyValue)
	}
	// Upstream asks what else it must support.
	return nil, errPropertyNotFound
}

// perlWord returns the Unicode class of \w.
//
// perlWord is perl_word, with the feature unicode-perl on.
func perlWord() *ClassUnicode {
	return hirClass(unicodetables.PerlWord)
}

// perlSpace returns the Unicode class of \s.
//
// perlSpace is perl_space, with the feature unicode-bool on.
func perlSpace() *ClassUnicode {
	return hirClass(unicodetables.PropertyBoolWhiteSpace)
}

// perlDigit returns the Unicode class of \d.
//
// perlDigit is perl_digit, with the feature unicode-gencat on.
func perlDigit() *ClassUnicode {
	return hirClass(unicodetables.GeneralCategoryDecimalNumber)
}

// hirClass returns a class of the ranges of a table.
//
// hirClass is hir_class.
func hirClass(ranges []unicodetables.Range) *ClassUnicode {
	hirRanges := make([]ClassUnicodeRange, 0, len(ranges))
	for _, r := range ranges {
		hirRanges = append(hirRanges, NewClassUnicodeRange(r.Start, r.End))
	}
	return NewClassUnicode(hirRanges)
}

// canonicalGencat returns the canonical name of a general category, and
// true. The value must be normalized. If no general category has the name, it
// returns false.
//
// canonicalGencat is canonical_gencat.
func canonicalGencat(normalizedValue string) (string, bool) {
	switch normalizedValue {
	case "any":
		return "Any", true
	case "assigned":
		return "Assigned", true
	case "ascii":
		return "ASCII", true
	}
	gencats, ok := propertyValues("General_Category")
	if !ok {
		panic("called `Option::unwrap()` on a `None` value")
	}
	return canonicalValue(gencats, normalizedValue)
}

// canonicalScript returns the canonical name of a script, and true. The
// value must be normalized. If no script has the name, it returns false.
//
// canonicalScript is canonical_script.
func canonicalScript(normalizedValue string) (string, bool) {
	scripts, ok := propertyValues("Script")
	if !ok {
		panic("called `Option::unwrap()` on a `None` value")
	}
	return canonicalValue(scripts, normalizedValue)
}

// canonicalProp returns the canonical name of a property, and true. If no
// property has the name, it returns false.
//
// The name must be normalized by UAX44-LM3, as symbolicNameNormalize does.
//
// canonicalProp is canonical_prop, with the features on.
func canonicalProp(normalizedName string) (string, bool) {
	i, found := slices.BinarySearchFunc(unicodetables.PropertyNames, normalizedName, func(a unicodetables.Alias, name string) int {
		return strings.Compare(a.Name, name)
	})
	if !found {
		return "", false
	}
	return unicodetables.PropertyNames[i].Canonical, true
}

// canonicalValue returns the canonical value of a property, and true. vals
// are the values of the property, as propertyValues returns them. If no value
// has the name, it returns false.
//
// The value must be normalized by UAX44-LM3, as symbolicNameNormalize does.
//
// canonicalValue is canonical_value.
func canonicalValue(vals []unicodetables.Alias, normalizedValue string) (string, bool) {
	i, found := slices.BinarySearchFunc(vals, normalizedValue, func(a unicodetables.Alias, name string) int {
		return strings.Compare(a.Name, name)
	})
	if !found {
		return "", false
	}
	return vals[i].Canonical, true
}

// propertyValues returns the values of the property with the canonical name,
// and true. If no property has the name, it returns false.
//
// propertyValues is property_values, with the features on.
func propertyValues(canonicalPropertyName string) ([]unicodetables.Alias, bool) {
	i, found := slices.BinarySearchFunc(unicodetables.PropertyValues, canonicalPropertyName, func(p unicodetables.Property, name string) int {
		return strings.Compare(p.Name, name)
	})
	if !found {
		return nil, false
	}
	return unicodetables.PropertyValues[i].Values, true
}

// propertySet returns the ranges of the class with the canonical name in
// nameMap, and true. If nameMap has no class with the name, it returns false.
//
// propertySet is property_set.
func propertySet(nameMap []unicodetables.Named, canonical string) ([]unicodetables.Range, bool) {
	i, found := slices.BinarySearchFunc(nameMap, canonical, func(n unicodetables.Named, name string) int {
		return strings.Compare(n.Name, name)
	})
	if !found {
		return nil, false
	}
	return nameMap[i].Ranges, true
}

// ages returns the sets of characters of each version of Unicode up to the
// version canonicalAge, in the order of the versions. Each set holds the
// characters that its version added.
//
// If the version is not valid, ages returns an error.
//
// ages is ages, which returns an iterator.
func ages(canonicalAge string) ([][]unicodetables.Range, error) {
	// age is one version of Unicode, with the characters that it added.
	type age struct {
		name   string
		ranges []unicodetables.Range
	}
	agesTable := []age{
		{"V1_1", unicodetables.AgeV1_1},
		{"V2_0", unicodetables.AgeV2_0},
		{"V2_1", unicodetables.AgeV2_1},
		{"V3_0", unicodetables.AgeV3_0},
		{"V3_1", unicodetables.AgeV3_1},
		{"V3_2", unicodetables.AgeV3_2},
		{"V4_0", unicodetables.AgeV4_0},
		{"V4_1", unicodetables.AgeV4_1},
		{"V5_0", unicodetables.AgeV5_0},
		{"V5_1", unicodetables.AgeV5_1},
		{"V5_2", unicodetables.AgeV5_2},
		{"V6_0", unicodetables.AgeV6_0},
		{"V6_1", unicodetables.AgeV6_1},
		{"V6_2", unicodetables.AgeV6_2},
		{"V6_3", unicodetables.AgeV6_3},
		{"V7_0", unicodetables.AgeV7_0},
		{"V8_0", unicodetables.AgeV8_0},
		{"V9_0", unicodetables.AgeV9_0},
		{"V10_0", unicodetables.AgeV10_0},
		{"V11_0", unicodetables.AgeV11_0},
		{"V12_0", unicodetables.AgeV12_0},
		{"V12_1", unicodetables.AgeV12_1},
		{"V13_0", unicodetables.AgeV13_0},
		{"V14_0", unicodetables.AgeV14_0},
		{"V15_0", unicodetables.AgeV15_0},
		{"V15_1", unicodetables.AgeV15_1},
		{"V16_0", unicodetables.AgeV16_0},
	}
	if len(agesTable) != len(unicodetables.AgeByName) {
		panic("ages are out of sync")
	}

	pos := slices.IndexFunc(agesTable, func(a age) bool {
		return a.name == canonicalAge
	})
	if pos < 0 {
		return nil, errPropertyValueNotFound
	}
	sets := make([][]unicodetables.Range, 0, pos+1)
	for _, a := range agesTable[:pos+1] {
		sets = append(sets, a.ranges)
	}
	return sets, nil
}

// gencat returns the class of a general category. The caller makes the name
// canonical.
//
// If the tables have no general category with the name, gencat returns an
// error.
//
// gencat is gencat, with the feature unicode-gencat on.
func gencat(canonicalName string) (*ClassUnicode, error) {
	imp := func(name string) (*ClassUnicode, error) {
		switch name {
		case "ASCII":
			return hirClass([]unicodetables.Range{{Start: 0, End: 0x7F}}), nil
		case "Any":
			return hirClass([]unicodetables.Range{{Start: 0, End: maxRune}}), nil
		case "Assigned":
			cls, err := gencat("Unassigned")
			if err != nil {
				return nil, err
			}
			cls.Negate()
			return cls, nil
		}
		set, ok := propertySet(unicodetables.GeneralCategoryByName, name)
		if !ok {
			return nil, errPropertyValueNotFound
		}
		return hirClass(set), nil
	}

	if canonicalName == "Decimal_Number" {
		return perlDigit(), nil
	}
	return imp(canonicalName)
}

// script returns the class of a script. The caller makes the name canonical.
//
// If the tables have no script with the name, script returns an error.
//
// script is script, with the feature unicode-script on.
func script(canonicalName string) (*ClassUnicode, error) {
	set, ok := propertySet(unicodetables.ScriptByName, canonicalName)
	if !ok {
		return nil, errPropertyValueNotFound
	}
	return hirClass(set), nil
}

// scriptExtension returns the class of a script extension. The caller makes
// the name canonical.
//
// If the tables have no script extension with the name, scriptExtension
// returns an error.
//
// scriptExtension is script_extension, with the feature unicode-script on.
func scriptExtension(canonicalName string) (*ClassUnicode, error) {
	set, ok := propertySet(unicodetables.ScriptExtensionByName, canonicalName)
	if !ok {
		return nil, errPropertyValueNotFound
	}
	return hirClass(set), nil
}

// boolProperty returns the class of a boolean property. The caller makes the
// name canonical.
//
// If the tables have no boolean property with the name, boolProperty returns
// an error.
//
// boolProperty is bool_property, with the feature unicode-bool on.
func boolProperty(canonicalName string) (*ClassUnicode, error) {
	imp := func(name string) (*ClassUnicode, error) {
		set, ok := propertySet(unicodetables.PropertyBoolByName, name)
		if !ok {
			return nil, errPropertyNotFound
		}
		return hirClass(set), nil
	}

	switch canonicalName {
	case "Decimal_Number":
		return perlDigit(), nil
	case "White_Space":
		return perlSpace(), nil
	}
	return imp(canonicalName)
}

// gcb returns the class of a value of the property Grapheme_Cluster_Break.
// The caller makes the name canonical.
//
// If the tables have no value with the name, gcb returns an error.
//
// gcb is gcb, with the feature unicode-segment on.
func gcb(canonicalName string) (*ClassUnicode, error) {
	set, ok := propertySet(unicodetables.GraphemeClusterBreakByName, canonicalName)
	if !ok {
		return nil, errPropertyValueNotFound
	}
	return hirClass(set), nil
}

// wb returns the class of a value of the property Word_Break. The caller
// makes the name canonical.
//
// If the tables have no value with the name, wb returns an error.
//
// wb is wb, with the feature unicode-segment on.
func wb(canonicalName string) (*ClassUnicode, error) {
	set, ok := propertySet(unicodetables.WordBreakByName, canonicalName)
	if !ok {
		return nil, errPropertyValueNotFound
	}
	return hirClass(set), nil
}

// sb returns the class of a value of the property Sentence_Break. The caller
// makes the name canonical.
//
// If the tables have no value with the name, sb returns an error.
//
// sb is sb, with the feature unicode-segment on.
func sb(canonicalName string) (*ClassUnicode, error) {
	set, ok := propertySet(unicodetables.SentenceBreakByName, canonicalName)
	if !ok {
		return nil, errPropertyValueNotFound
	}
	return hirClass(set), nil
}

// symbolicNameNormalize returns the name normalized by UAX44-LM3, as
// symbolicNameNormalizeBytes does.
//
// symbolicNameNormalize is symbolic_name_normalize.
func symbolicNameNormalize(x string) string {
	tmp := []byte(x)
	n := len(symbolicNameNormalizeBytes(tmp))
	tmp = tmp[:n]
	// The result is always valid UTF-8, because symbolicNameNormalizeBytes
	// keeps only ASCII bytes.
	return string(tmp)
}

// symbolicNameNormalizeBytes normalizes a symbolic name in place, by
// UAX44-LM3, and returns the start of slice that holds the result. A
// symbolic name is usually the name of a property, or an alias of the value
// of a property. It is not for the string value of a property.
//
// The result is valid UTF-8 for any slice.
//
// See https://unicode.org/reports/tr44/#UAX44-LM3.
//
// symbolicNameNormalizeBytes is symbolic_name_normalize_bytes.
func symbolicNameNormalizeBytes(slice []byte) []byte {
	// Upstream found no place in the standard that gives a structure to the
	// names and aliases of properties, as it does for the names of
	// characters. The code assumes that they are ASCII, and drops each byte
	// that is not ASCII.
	start := 0
	startsWithIs := false
	if len(slice) >= 2 {
		// Ignore a prefix "is".
		p := string(slice[0:2])
		startsWithIs = p == "is" || p == "IS" || p == "iS" || p == "Is"
		if startsWithIs {
			start = 2
		}
	}
	nextWrite := 0
	for i := start; i < len(slice); i++ {
		// The result must be valid UTF-8, so it holds only ASCII bytes. The
		// code drops each byte that is not ASCII.
		b := slice[i]
		switch {
		case b == ' ' || b == '_' || b == '-':
			continue
		case 'A' <= b && b <= 'Z':
			slice[nextWrite] = b + ('a' - 'A')
			nextWrite++
		case b <= 0x7F:
			slice[nextWrite] = b
			nextWrite++
		}
	}
	// This is a special case. ISO_Comment has the abbreviation isc. The code
	// ignores a prefix "is", so isc becomes c, an alias of ISO_Comment. But
	// c is an alias of the general category Other.
	if startsWithIs && nextWrite == 1 && slice[0] == 'c' {
		slice[0] = 'i'
		slice[1] = 's'
		slice[2] = 'c'
		nextWrite = 3
	}
	return slice[:nextWrite]
}
