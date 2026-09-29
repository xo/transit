// Package unicodetables holds the Unicode tables of the Rust crate
// regex-syntax 0.8.11, as Go data. It is a part of the port of regex-syntax
// (D59). The tables are at Unicode 16.0.0, and every output of the generator
// uses them (D60).
//
// The crate holds each table in a Rust file of src/unicode_tables, which
// ucd-generate made. The command test/cmd/regextables of the test module
// writes one Go file for each Rust file, with the same base name. Do not edit
// those Go files. Run the command again. Only this file is written by hand.
// LICENSE-UNICODE holds the license of the Unicode data.
//
// The Go files keep the order of every entry of the Rust files. The port of
// unicode.rs does a binary search on CaseFoldingSimple by the character, and
// on PropertyNames, PropertyValues, the values of each property, and each
// ByName table except AgeByName, by the name.
//
// Each Go name starts with the name of its table. For example, BY_NAME of
// general_category.rs is GeneralCategoryByName, and CASED_LETTER of that file
// is GeneralCategoryCasedLetter. An item with the name of its file, such as
// PERL_WORD of perl_word.rs, takes the name of the table alone, PerlWord. An
// underscore stays between two digits, so V1_1 of age.rs is AgeV1_1 and V11_0
// is AgeV11_0.
package unicodetables

// Range is a range of characters, from Start to End, with both ends in the
// range. It is (char, char) in Rust. The ranges of a class are sorted, and no
// two of them overlap.
type Range struct {
	Start, End rune
}

// Named is a class of characters with its name, such as the general category
// Cased_Letter. It is (&str, &[(char, char)]) in Rust.
type Named struct {
	Name   string
	Ranges []Range
}

// Fold is one entry of the simple case folding. It is (char, &[char]) in Rust.
// Folds holds each character that Rune folds with, other than Rune itself.
type Fold struct {
	Rune  rune
	Folds []rune
}

// Alias is a name in the normal form of UAX44-LM3, with the canonical name
// that it stands for, such as ahex for ASCII_Hex_Digit. It is (&str, &str) in
// Rust.
type Alias struct {
	Name      string
	Canonical string
}

// Property is a property with the values that it takes, such as Script with
// each script. It is (&str, &[(&str, &str)]) in Rust.
type Property struct {
	Name   string
	Values []Alias
}
