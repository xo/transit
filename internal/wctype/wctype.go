// Package wctype holds the character functions of <wctype.h> and <ctype.h>
// that the Go scanners of the grammars call (D39, D46). A scanner calls the
// function of this package with the name of the C function, such as Iswspace
// for iswspace.
//
// Released code answers as the package unicode does (D39). The test module
// calls SetCLocale, and then each function answers as the GNU C library does
// in the C locale, where a function answers as for ASCII. A Go scanner and
// its C scanner then give the same tokens, and the trees of the two match
// (D46).
package wctype

import (
	"sync/atomic"
	"unicode"
)

// cLocale is true when the functions answer as the C library does.
var cLocale atomic.Bool

// SetCLocale makes the functions answer as the GNU C library does in the C
// locale when on is true, and as the package unicode does when it is false.
// Only the test module calls it (D46).
func SetCLocale(on bool) {
	cLocale.Store(on)
}

// Iswspace is iswspace.
func Iswspace(c int32) bool {
	if cLocale.Load() {
		return c == ' ' || c >= '\t' && c <= '\r'
	}
	return unicode.IsSpace(c)
}

// Iswalpha is iswalpha.
func Iswalpha(c int32) bool {
	if cLocale.Load() {
		return isASCIIUpper(c) || isASCIILower(c)
	}
	return unicode.IsLetter(c)
}

// Iswdigit is iswdigit.
func Iswdigit(c int32) bool {
	if cLocale.Load() {
		return isASCIIDigit(c)
	}
	return unicode.IsDigit(c)
}

// Iswalnum is iswalnum.
func Iswalnum(c int32) bool {
	if cLocale.Load() {
		return isASCIIUpper(c) || isASCIILower(c) || isASCIIDigit(c)
	}
	return unicode.IsLetter(c) || unicode.IsDigit(c)
}

// Iswupper is iswupper.
func Iswupper(c int32) bool {
	if cLocale.Load() {
		return isASCIIUpper(c)
	}
	return unicode.IsUpper(c)
}

// Iswlower is iswlower.
func Iswlower(c int32) bool {
	if cLocale.Load() {
		return isASCIILower(c)
	}
	return unicode.IsLower(c)
}

// Iswxdigit is iswxdigit. The C standard allows only the hexadecimal digits
// of ASCII in every locale, and the package unicode has no such class, so
// both forms answer the same.
func Iswxdigit(c int32) bool {
	return isASCIIDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// Towupper is towupper.
func Towupper(c int32) int32 {
	if cLocale.Load() {
		if isASCIILower(c) {
			return c - 'a' + 'A'
		}
		return c
	}
	return unicode.ToUpper(c)
}

// Isdigit is isdigit of <ctype.h>.
func Isdigit(c int32) bool {
	if cLocale.Load() {
		return isASCIIDigit(c)
	}
	return unicode.IsDigit(c)
}

func isASCIIUpper(c int32) bool { return c >= 'A' && c <= 'Z' }

func isASCIILower(c int32) bool { return c >= 'a' && c <= 'z' }

func isASCIIDigit(c int32) bool { return c >= '0' && c <= '9' }
