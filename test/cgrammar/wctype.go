package cgrammar

/*
#include <ctype.h>
#include <wctype.h>

static int w_iswspace(int c) { return iswspace((wint_t)c) != 0; }
static int w_iswalpha(int c) { return iswalpha((wint_t)c) != 0; }
static int w_iswdigit(int c) { return iswdigit((wint_t)c) != 0; }
static int w_iswalnum(int c) { return iswalnum((wint_t)c) != 0; }
static int w_iswupper(int c) { return iswupper((wint_t)c) != 0; }
static int w_iswlower(int c) { return iswlower((wint_t)c) != 0; }
static int w_iswxdigit(int c) { return iswxdigit((wint_t)c) != 0; }
static int w_towupper(int c) { return (int)towupper((wint_t)c); }
static int w_isdigit(int c) { return isdigit(c) != 0; }
*/
import "C"

// CCharFunc is a character function of the C library, in the C locale, with
// its result as an int32. A predicate gives 0 or 1.
type CCharFunc struct {
	Name string
	Call func(c int32) int32
	// Max is the largest character that the function takes. isdigit takes
	// only an unsigned char.
	Max int32
}

// CCharFuncs are the functions of the C library that the Go scanners call
// through the package internal/wctype. The Go process never calls setlocale,
// so they run in the C locale, as the scanners do in the upstream tool.
var CCharFuncs = []CCharFunc{
	{"iswspace", func(c int32) int32 { return int32(C.w_iswspace(C.int(c))) }, 0x10ffff},
	{"iswalpha", func(c int32) int32 { return int32(C.w_iswalpha(C.int(c))) }, 0x10ffff},
	{"iswdigit", func(c int32) int32 { return int32(C.w_iswdigit(C.int(c))) }, 0x10ffff},
	{"iswalnum", func(c int32) int32 { return int32(C.w_iswalnum(C.int(c))) }, 0x10ffff},
	{"iswupper", func(c int32) int32 { return int32(C.w_iswupper(C.int(c))) }, 0x10ffff},
	{"iswlower", func(c int32) int32 { return int32(C.w_iswlower(C.int(c))) }, 0x10ffff},
	{"iswxdigit", func(c int32) int32 { return int32(C.w_iswxdigit(C.int(c))) }, 0x10ffff},
	{"towupper", func(c int32) int32 { return int32(C.w_towupper(C.int(c))) }, 0x10ffff},
	{"isdigit", func(c int32) int32 { return int32(C.w_isdigit(C.int(c))) }, 0xff},
}
