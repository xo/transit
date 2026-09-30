package cgrammar

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/xo/transit/grammars/cpp"
	"github.com/xo/transit/internal/abi"
)

func init() {
	goPackages = append(goPackages, goPackage{"cpp", cpp.Language})
}

// cppScannerInputs are inputs that reach each branch of the scanner of cpp:
// raw strings with and without a delimiter, a delimiter of 16 and of 17
// characters, a delimiter with a space, a backslash, a character that is not
// ASCII or a byte that is not UTF-8, a content with a part of the closing
// delimiter, and a raw string that the input ends in.
var cppScannerInputs = []string{
	`auto s = R"(hello)";`,
	`auto s = R"()";`,
	`auto s = R"x(a)x";`,
	`auto s = R"abc(he)ab)abc" )abc";`,
	`auto s = R"ab()a)ab)ab"; auto t = R"ab()ab";`,
	`auto s = R"""(hello)""";`,
	`auto s = R"0123456789abcdef(x)0123456789abcdef";`,
	`auto s = R"0123456789abcdefg(x)0123456789abcdefg";`,
	`auto s = R"a b(x)a b";`,
	"auto s = R\"a\tb(x)a\tb\";",
	`auto s = R"a\b(x)a\b";`,
	`auto s = R"é(x)é";`,
	"auto s = R\" (x) \";",
	"auto s = R\"\xff(x)\xff\";",
	"auto s = R\"x(\r\n)x\";\r\n",
	`auto s = u8R"x(a)x"; auto t = LR"(b)"; auto u = uR"y(c)y"; auto v = UR"(d)";`,
	`auto s = R"abc`,
	`auto s = R"abc(content ) abc`,
	`auto s = R"(unterminated`,
	`auto s = R"x(a)y";`,
	`auto s = R"x(a)x`,
	`auto s = R"x(a)x" "b" R"(c)";`,
	`auto s = R"x( }}} )x" }}} R"( ;`,
	`int f() { return R"(a)" + R"x(b)x"; }`,
	`#define S R"x(a)x"
auto s = S;`,
	`R"x(`,
	`R"`,
	`R`,
}

// TestGoPackageScannerCpp compares the Go scanner of cpp with its C scanner
// on each corpus input, on the error corpus of upstream and on
// cppScannerInputs.
func TestGoPackageScannerCpp(t *testing.T) {
	t.Parallel()
	g, examples := loadGoPackage(t, goPackage{"cpp", cpp.Language})
	var inputs [][]byte
	for _, e := range examples {
		inputs = append(inputs, e.Input)
	}
	for _, s := range cppScannerInputs {
		inputs = append(inputs, []byte(s))
	}
	n, calls := compareScanners(t, g, cpp.Language(), inputs)
	t.Logf("%d inputs, %d scanner calls", n, calls)
}

// TestGoPackageDeserializeCpp gives the Go scanner and the C scanner of cpp
// the same random bytes, and compares what Serialize writes after it. The C
// function asserts that the length is a multiple of 4, and it writes past
// the delimiter for more than 64 bytes, so the bytes that both get have such
// a length. Then the test gives the Go scanner random bytes of any length,
// which must not make it panic.
func TestGoPackageDeserializeCpp(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"cpp", cpp.Language})
	goCreate := tablesOf(cpp.Language()).ExternalScanner.Create
	cCreate := tablesOf(g.Language).ExternalScanner.Create
	r := rand.New(rand.NewPCG(1, 2))
	for i := range 2000 {
		goScanner, cScanner := goCreate(), cCreate()
		// Each scanner gets three buffers in a row, so that a buffer can
		// leave a part of the state of the one before it.
		for range 3 {
			buf := make([]byte, 4*r.IntN(17))
			for k := range buf {
				buf[k] = byte(r.Uint32())
			}
			goScanner.Deserialize(buf)
			cScanner.Deserialize(buf)
		}
		if want, got := serializeScanner(cScanner), serializeScanner(goScanner); !bytes.Equal(want, got) {
			t.Fatalf("run %d: C serializes %x, Go serializes %x", i, want, got)
		}
	}
	for range 2000 {
		s := goCreate()
		buf := make([]byte, r.IntN(abi.SerializationBufferSize+1))
		for k := range buf {
			buf[k] = byte(r.Uint32())
		}
		s.Deserialize(buf)
		serializeScanner(s)
	}
}

// serializeScanner returns the bytes that Serialize of a scanner writes.
func serializeScanner(s abi.Scanner) []byte {
	buf := make([]byte, abi.SerializationBufferSize)
	return buf[:s.Serialize(buf)]
}
