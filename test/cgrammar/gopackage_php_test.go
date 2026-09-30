package cgrammar

import (
	"encoding/binary"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/xo/transit/grammars/php/php"
	"github.com/xo/transit/grammars/php/phponly"
	"github.com/xo/transit/internal/abi"
)

func init() {
	goPackages = append(goPackages,
		goPackage{"php", php.Language},
		goPackage{"php_only", phponly.Language},
	)
}

// phpPackages are the two grammar packages of tree-sitter-php, which share
// the scanner of the package internal/scanner.
var phpPackages = []goPackage{
	{"php", php.Language},
	{"php_only", phponly.Language},
}

// phpScannerInputs are inputs that reach each branch of the scanner of
// tree-sitter-php. The test gives each one to the grammar php after "<?php"
// and as it is, and to the grammar php_only in the same two forms.
var phpScannerInputs = []string{
	// strings, with each case of scan_encapsed_part_string
	`echo "abc $a->b $a->1 $a- $a[0] {$a} \{ \\ \$ \n \x41 \xg \u{41} \7 \8 $1 $ { - [";`,
	`echo "$a->b->c $a[1][2] ${a} $$a";`,
	`echo "$a-> $a->! $a->";`,
	`echo "unterminated $a`,
	"echo \"line\none\r\ntwo\";",
	`echo "\"";`,
	// execution strings
	"echo `ls $a -l \\` \\x $a->b $a[0] {$a} \" \n`;",
	"echo `unterminated",
	// heredocs, with the end tag in each position
	"echo <<<EOT\nabc $a->b $a[0] {$a} \\\\ \\$ \\n \" `\nEOT;\n",
	"echo <<<\"EOT\"\n  abc\n  EOT;\n",
	"f(<<<EOT\nabc\nEOT, 1);\nf(<<<EOT\nabc\nEOT);\n",
	"echo <<<EOT\nEOTX\nEOT x\n\tEOT\t;\n",
	"echo <<<EOT\nabc\n\tEOT  ;\n",
	"echo <<<EOT\nabc $a\nEOT\n;",
	"echo <<<EOT\r\nabc\r\nEOT;\r\n",
	"echo <<<EOT\nabc",
	"echo <<<A\n{$a(<<<B\nb\nB)}\nA;\n",
	"echo <<< EOT\nabc\nEOT;\n",
	"echo <<<\nabc\n",
	"echo <<<" + strings.Repeat("L", 300) + "\nabc\n" + strings.Repeat("L", 300) + ";\n",
	// nowdocs
	"echo <<<'EOT'\nabc $a {$b} \\n\nEOT;\n",
	"echo <<<'EOT'\n  abc\n  EOT;\n",
	"f(<<<'EOT'\nabc\nEOT, 1);\n",
	"echo <<<'EOT'\nEOTX\nEOT  ;\n",
	"echo <<<'EOT'\nabc",
	"echo <<<'EOT'\n",
	// the automatic semicolon, the end of the file and the comments of
	// scan_whitespace
	"echo 1 ?>text<?php echo 2 ?>",
	"echo 1 ?",
	"$a = 1 // comment\n?>",
	"$a = 1 / 2 ?>",
	"echo 1;\n// comment",
	"echo 1;",
	"",
	// names and whitespace outside ASCII
	"echo <<<\u00e9t\u00e9\nabc\n\u00e9t\u00e9;\n",
	"echo \"$\u0660 $\u00e9 $_ $\u00a0\";",
	"echo <<<EOT\nabc\n\u00a0EOT\u00a0;\n",
	"echo\u00a01;\u2028",
	"echo \"\xff\xfe $\xff\";",
	// error recovery
	"echo \"abc\" \"def;\n} } { <<<EOT\n",
	"class { function (<<<EOT\n$a\nEOT; ) }",
}

// TestGoPackagePHPScannerMatchesC compares the Go scanner of php and of
// php_only with the C scanner, on every corpus input, and on the inputs of
// phpScannerInputs.
func TestGoPackagePHPScannerMatchesC(t *testing.T) {
	t.Parallel()
	for _, gp := range phpPackages {
		t.Run(gp.name, func(t *testing.T) {
			t.Parallel()
			g, examples := loadGoPackage(t, gp)
			var inputs [][]byte
			for _, e := range examples {
				inputs = append(inputs, e.Input)
			}
			for _, s := range phpScannerInputs {
				inputs = append(inputs, []byte("<?php\n"+s), []byte(s))
			}
			n, calls := compareScanners(t, g, gp.language(), inputs)
			if calls == 0 {
				t.Errorf("the scanner of %s was not called on %d inputs", gp.name, n)
			}
			t.Logf("%d inputs, %d calls", n, calls)
		})
	}
}

// randomPHPState returns a random state of the scanner of tree-sitter-php
// in the form that serialize writes. It has random bytes where deserialize
// reads a bool, and random characters in each word. deserialize of C does
// not test the length of the buffer, and it asserts that it reads the whole
// buffer, so the state has the length that the C function reads.
func randomPHPState(r *rand.Rand) []byte {
	if r.IntN(20) == 0 {
		return nil
	}
	buf := []byte{0}
	count := r.IntN(8)
	for range count {
		n := r.IntN(10)
		if r.IntN(10) == 0 {
			n = 200 + r.IntN(100)
		}
		if len(buf)+5+4*n > abi.SerializationBufferSize {
			break
		}
		buf[0]++
		var allowed byte
		if r.IntN(2) == 0 {
			allowed = byte(r.UintN(256))
		}
		buf = append(buf, allowed)
		buf = binary.LittleEndian.AppendUint32(buf, uint32(n))
		for range n {
			buf = binary.LittleEndian.AppendUint32(buf, r.Uint32())
		}
	}
	return buf
}

// TestGoPackagePHPScannerStatesMatchC gives random states to Deserialize of
// the Go scanner and of the C scanner, and compares what Serialize writes
// after it.
func TestGoPackagePHPScannerStatesMatchC(t *testing.T) {
	t.Parallel()
	for _, gp := range phpPackages {
		t.Run(gp.name, func(t *testing.T) {
			t.Parallel()
			g, _ := loadGoPackage(t, gp)
			compareScannerStateSequence(t, g, gp.language(), randomPHPState, 5000)
		})
	}
}
