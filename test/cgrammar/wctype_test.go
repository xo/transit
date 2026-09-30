package cgrammar

import (
	"testing"

	"github.com/xo/transit/internal/wctype"
)

// The tests of this package compare Go scanners with C scanners, so the
// character functions answer as the C library does (D46).
func init() {
	wctype.SetCLocale(true)
}

// goCharFuncs are the functions of internal/wctype, by the name of their C
// function, with the result as CCharFunc gives it.
var goCharFuncs = map[string]func(c int32) int32{
	"iswspace":  func(c int32) int32 { return b2i(wctype.Iswspace(c)) },
	"iswalpha":  func(c int32) int32 { return b2i(wctype.Iswalpha(c)) },
	"iswdigit":  func(c int32) int32 { return b2i(wctype.Iswdigit(c)) },
	"iswalnum":  func(c int32) int32 { return b2i(wctype.Iswalnum(c)) },
	"iswupper":  func(c int32) int32 { return b2i(wctype.Iswupper(c)) },
	"iswlower":  func(c int32) int32 { return b2i(wctype.Iswlower(c)) },
	"iswxdigit": func(c int32) int32 { return b2i(wctype.Iswxdigit(c)) },
	"towupper":  wctype.Towupper,
	"isdigit":   func(c int32) int32 { return b2i(wctype.Isdigit(c)) },
}

func b2i(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// TestCharFuncsMatchC measures each character function of the C library
// for every code point, as D39 asks. With SetCLocale, internal/wctype must
// answer as C for each one (D46). Without it, the package answers as the
// package unicode does, and the test lists how many code points differ from
// C for each function, and the first of them (D39).
func TestCharFuncsMatchC(t *testing.T) {
	// SetCLocale changes a package variable, so this test does not run in
	// parallel with the others, and it sets the C locale again at its end.
	defer wctype.SetCLocale(true)
	for _, f := range CCharFuncs {
		goFunc, ok := goCharFuncs[f.Name]
		if !ok {
			t.Fatalf("internal/wctype has no function for %s", f.Name)
		}
		wctype.SetCLocale(true)
		for c := int32(-1); c <= f.Max; c++ {
			if want, got := f.Call(c), goFunc(c); want != got {
				t.Errorf("%s(%#x) in the C locale is %d, and the package gives %d", f.Name, c, want, got)
				break
			}
		}
		wctype.SetCLocale(false)
		differ := 0
		first := int32(-2)
		for c := int32(-1); c <= f.Max; c++ {
			if f.Call(c) != goFunc(c) {
				if differ == 0 {
					first = c
				}
				differ++
			}
		}
		t.Logf("%s: %d code points differ from C with the package unicode (D39), the first is %#x", f.Name, differ, first)
	}
}
