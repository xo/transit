package cgrammar

import (
	"bytes"
	"encoding/binary"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/xo/transit/grammars/bash"
	"github.com/xo/transit/internal/abi"
)

func init() {
	goPackages = append(goPackages, goPackage{"bash", bash.Language})
}

// bashScannerInputs are inputs that reach the branches of the scanner of
// bash, part by part of the function scan: the concatenations, the
// expansions, the empty values, the heredocs, the test operators, the bare
// dollars, the variable names and the file descriptors, the regular
// expressions, the extended globs, the expansion words and the brace
// expansions. Some of them are not valid bash, so that the scanner runs in
// error recovery too.
var bashScannerInputs = []string{
	// concat
	"echo a`b` c\n",
	"echo a`b`c\n",
	"echo a`b\n",
	"echo \"a\"\\\"b\n",
	"echo a\\\\b a\\'b a\\\"b\n",
	"echo a\\ b\n",
	"echo a\\",
	"echo ${a }\n",
	"echo ${a:-b}c\"d\"'e'$f\n",
	"echo x[1] a]b\n",
	// immediate double hash and the expansion operators
	"echo ${a##b} ${a##} ${a##*/} ${#a}\n",
	"echo ${a#b} ${!a} ${a=b} ${#} ${!} ${=} ${#  } ${!#=}\n",
	"echo ${!a[@]} ${!a*} ${#a[@]}\n",
	// empty value
	"a= ;b=&c=\nlocal d=\nexport e=\n",
	"a=",
	// heredocs
	"cat <<EOF\nhello $x\nEOF\n",
	"cat <<EOF\nhello ${x} $(y) $z \\$ \\\\\nEOF\n",
	"cat <<-EOF\n\thello\n\t$x\n\tEOF\n",
	"cat <<-EOF\n  hello\n    EOF\necho\n",
	"cat <<'EOF'\n$x\nEOF\n",
	"cat <<\"EOF\"\n$x ${y}\nEOF\n",
	"cat <<\\EOF\n$x\nEOF\n",
	"cat <<EOF\n$x\n",
	"cat <<EOF\nabc",
	"cat <<EOF",
	"cat <<A <<B\na\nA\nb\nB\n",
	"cat <<A; cat <<B\na $x\nA\nb $y\nB\n",
	"cat <<EOF\n$\nEOF\n",
	"cat <<EOF\n$x\n$y\nEOF\n",
	"cat <<EOF\nEOFX\n EOF\nx\nEOF\n",
	"cat <<EOF | grep a\nabc\nEOF\necho done\n",
	"x=$(cat <<EOF\nhello\nEOF\n)\n",
	"cat <<< \"here string\"\n",
	"cat <<= x\n",
	"cat <<é\nx\né\n",
	"cat <<EOF \nx\nEOF\n",
	"cat <<''\nx\n\n",
	"cat <<\"\"\nx\n",
	"cat <<EOF\r\nhello\r\nEOF\r\n",
	"cat <<EOF\na\\\nb\nEOF\n",
	"cat <<\"a b\"\nx\na b\n",
	"cat <<'a\nb'\nx\n",
	"cat <<EOF\n\\",
	"f() {\ncat <<EOF\n${a}b\nEOF\n}\n",
	"cat <<EOF\n$(echo <<INNER\ninner\nINNER\n)\nEOF\n",
	"cat <<" + strings.Repeat("a", 1100) + "\nx\n" + strings.Repeat("a", 1100) + "\n",
	"cat <<a\\",
	"cat <<a\\\x00b\n",
	"cat <<EOF\n\x00\nEOF\n",
	"cat <<EOF\n$x\x00y\nEOF\n",
	"cat <<EOF\nabc\n  x\nEOF\n",
	"cat <<EOF\n$x a\\\nEOF\n",
	"cat <<EOF\n$x\\\n  EOF\n$y\nEOF\n",
	// test operators
	"[[ -f a ]]\n[ -n \"$x\" ]\n[[ a -eq b ]]\ntest -z a\n",
	"[[ -f\\\na ]]\n",
	"[[ -f \\\r\n a ]]\n",
	"[[ \\\n -f a ]]\n",
	"[[ \\x ]]\n",
	"[[ \\",
	"[[\n -f a ]]\n",
	"[[ -a ]]\n[[ - a ]]\n[[ -ab- ]]\n",
	"[[ $a == -x ]]\n[[ -1 ]]\n",
	"[[ ${a:-b} -gt 1 ]]\n",
	"[[ -f $ ]]\n[ $ ]\n",
	"echo ${a:-b -c}\n",
	"echo ${a -x }\n[ -f }\n{ [ -z }\n",
	"[[ -f }\n{ test -f }\necho ${a[ -f }]}\n",
	// bare dollar
	"echo $\necho $ a\necho \"$\"\necho x$\n",
	"echo $",
	// variable names and file descriptors
	"a=b\na+=b\na[1]=b\na+b\n",
	"echo ${a:-b} ${a%b} ${a/b/c} ${a-b} ${a?} ${a?b} ${a@Q} ${a+b} ${a:b}\n",
	"ls 2>&1 10>f 1<g 3<<EOF\nx\nEOF\n",
	"a\\\nb=c\n",
	"a\\\r\nb=c\n",
	"\\",
	"a=1 \\",
	"a=1 \\\r\nb=2\n",
	"echo a; \\",
	"${\\",
	"$((\\",
	"for \\",
	"a[\\",
	"${\\\r\na}\n",
	"for \\\r\n a in b; do :; done\n",
	"$((\\\r\n1))\n",
	"x\n\\\r\ny\n",
	"echo \"a $\"\nx=$ \necho ${a}$ b\n",
	"echo ${*} ${@} ${?} ${-} ${0} ${_} ${a+=b}\n",
	"_=a\n*=a\n@\n?x\n",
	"for a in b; do :; done\n",
	"f() { :; }\nf:a() { :; }\n",
	"a:b\n",
	"a#b\n1#2\na@b\n",
	"${a-}\n${a#}\n",
	"echo ${a%%b} ${a,,} ${a^^}\n",
	"echo $((a + b)) $((1 << 2))\n",
	"declare -A a=([x]=1)\n",
	"echo ${a\\\n}\n",
	"echo ${a:-\\\\b}\n",
	// regular expressions
	"[[ $a =~ ^(a|b)$ ]]\n",
	"[[ $a =~ 'quoted' ]]\n",
	"[[ $a =~ [[:space:]] ]]\n",
	"[[ $a =~ a{1,2}\\{ ]]\n",
	"[[ $a =~ 9999$ ]]\n",
	"[[ $a =~ $(b) ]]\n",
	"[[ $a =~ \\. ]]\n",
	"[[ $a =~ a\\ b ]]\n",
	"[[ $a =~ ( a b ) ]]\n",
	"[[ $a =~ a'b c'd ]]\n",
	"[[ $a =~ \"x\" ]]\n",
	"[[ $a =~ a$b ]]\n",
	"[[ $a =~ $ ]]\n",
	"[[ $a =~ abc ]]\n",
	"[[ $a =~ ",
	"echo ${a/b\\/c/d} ${a/$b/c} ${a/'b'/c} ${a//[a-z]/x} ${a/#b/c} ${a/%b/c}\n",
	"echo ${a/b c/d} ${a/\\[/x} ${a/$(b)/c} ${a/}\n",
	"echo ${a/b",
	"[[ $a =~ a] ]]\necho ${a/b]/c}\n",
	"[[ ( a =~ b) ]]\n[[ a =~ b) ]]\n",
	// extended globs and case items
	"case a in *) ;; esac\n",
	"case a in -) ;; esac\n",
	"case a in a|b) ;; esac\n",
	"case $a in @(a|b)) ;; esac\n",
	"case a in *${b}*) ;; esac\n",
	"case a in *$(b)*) ;; esac\n",
	"case a in e) ;; esac\n",
	"case a in es) ;; esac\n",
	"case a in esac\n",
	"case a in -a) ;; -a-b) ;; esac\n",
	"case a in [ab]) ;; ?\"x\") ;; esac\n",
	"case a in \\ ) ;; esac\n",
	"case a in x|y|z) ;; *.txt|*.md) ;; esac\n",
	"[[ $a == !(b) ]]\n[[ a == +(x) ]]\n[[ $a == *.txt ]]\n[[ $a == $b* ]]\n",
	"[[ a == ?\"x\" ]]\n[[ a == \\ b ]]\n[[ a == [ab] ]]\n[[ a == @(x|y)z ]]\n",
	"[[ a == *(a|(b|c)) ]]\n[[ a == ab{c,d} ]]\n[[ a == a\\\"b ]]\n",
	"[[ a == -x- ]]\n[[ a == -x) ]]\n[[ a == x$ ]]\n",
	"[[ a != .*/ ]]\n[[ a == * ]]\n",
	"[[ a == *(",
	"case a in x-y) ;; esac\n[[ a == x-y ]]\n[[ a == x-y) ]]\n[[ a == x-\\ ]]\n[[ a == *-a.b ]]\n",
	"[[ a == *{a,b} ]]\n[[ a == *a] ]]\ncase a in *a}) ;; esac\n[[ a == @(a}) ]]\n",
	"case a in *(a|{b}|[c])) ;; esac\n[[ a == ?(a]) ]]\n",
	"[[ a == *a{b,c} ]]\ncase a in a*{b}) ;; esac\n",
	// expansion words
	"echo ${a:-b c} ${a:-$b} ${a:-(x)} ${a:-(x $y)} ${a:-'x'} ${a:-\"x\"} ${a:- }\n",
	"echo ${a:-(} ${a:-()} ${a:-( x )} ${a:-x(y)}\n",
	"echo ${a:-(x}\n",
	"echo ${a:-b",
	"echo ${a:-$}\n",
	"echo ${a:-(x$ y)} ${a:-(x $)}\n",
	"echo ${a:-x$(y)} ${a:-x${y}} ${a:-x$'y'}\n",
	// brace expansions
	"echo {1..5} {a..b} {1..} {1.5} {..} {01..10}\n",
	"echo {1..2",
	"echo {²..3}\n",
	"for i in {1..3}; do echo $i; done\n",
	// errors
	"$((\n",
	"${\n",
	"\"$(\n",
	"<<\n",
	"if then fi\n",
	"echo \"${a[\"b\"]}\"\n",
	"echo $'a\\'b'\n",
	"a=(1 2 3)\necho ${a[@]:1:2}\n",
	" echo\u0085a\n",
	"echo a\xffb\n",
}

// TestGoPackageScannerBash compares the Go scanner of bash with its C
// scanner on each corpus input, on the error corpus of upstream and on
// bashScannerInputs.
func TestGoPackageScannerBash(t *testing.T) {
	t.Parallel()
	g, examples := loadGoPackage(t, goPackage{"bash", bash.Language})
	var inputs [][]byte
	for _, e := range examples {
		inputs = append(inputs, e.Input)
	}
	for _, s := range bashScannerInputs {
		inputs = append(inputs, []byte(s))
	}
	n, calls := compareScanners(t, g, bash.Language(), inputs)
	t.Logf("%d inputs, %d scanner calls", n, calls)
}

// bashState returns random bytes in the form that serialize of the scanner
// of bash writes: four bytes, and for each heredoc three bytes, the size of
// the delimiter as a uint32 and the delimiter. A byte of a bool can be any
// byte. The C function reads past the end of a buffer of another form, and
// it asserts that it read the whole buffer, so only this form can go to it.
func bashState(r *rand.Rand) []byte {
	buf := []byte{byte(r.Uint32()), byte(r.Uint32()), byte(r.Uint32())}
	count := r.IntN(6)
	buf = append(buf, byte(count))
	for range count {
		buf = append(buf, byte(r.Uint32()), byte(r.Uint32()), byte(r.Uint32()))
		delimiter := make([]byte, r.IntN(20))
		for k := range delimiter {
			delimiter[k] = byte(r.Uint32())
		}
		buf = binary.LittleEndian.AppendUint32(buf, uint32(len(delimiter)))
		buf = append(buf, delimiter...)
	}
	return buf
}

// TestGoPackageDeserializeBash gives the Go scanner and the C scanner of
// bash the same random states, and compares what Serialize writes after
// them. Each scanner gets a few states in a row, and some of them are
// empty, because the C function keeps the heredocs that a state does not
// name, and an empty state resets the heredocs but keeps their number. Then
// the test gives the Go scanner random bytes of any length, which must not
// make it panic.
func TestGoPackageDeserializeBash(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"bash", bash.Language})
	goCreate := tablesOf(bash.Language()).ExternalScanner.Create
	cCreate := tablesOf(g.Language).ExternalScanner.Create
	r := rand.New(rand.NewPCG(3, 4))
	for i := range 5000 {
		goScanner, cScanner := goCreate(), cCreate()
		for range 1 + r.IntN(4) {
			var buf []byte
			if r.IntN(4) != 0 {
				buf = bashState(r)
			}
			goScanner.Deserialize(buf)
			cScanner.Deserialize(buf)
		}
		if want, got := serializeScanner(cScanner), serializeScanner(goScanner); !bytes.Equal(want, got) {
			t.Fatalf("run %d: C serializes %x, Go serializes %x", i, want, got)
		}
	}
	for range 5000 {
		s := goCreate()
		var buf []byte
		if r.IntN(2) == 0 {
			buf = make([]byte, r.IntN(abi.SerializationBufferSize+1))
			for k := range buf {
				buf[k] = byte(r.Uint32())
			}
		} else {
			// a state that stops in the middle
			buf = bashState(r)
			buf = buf[:r.IntN(len(buf)+1)]
		}
		s.Deserialize(buf)
		serializeScanner(s)
	}
}
