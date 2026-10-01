package sql

import (
	"bytes"
	"testing"

	"github.com/xo/transit/internal/abi"
)

// FuzzDeserialize gives random bytes to Deserialize, as docs/GRAMMAR.md
// asks, and makes sure that Deserialize does not panic, and that Serialize
// after it writes no more than the buffer holds and ends the tag with a 0
// byte. The test module compares the bytes with the C scanner.
func FuzzDeserialize(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{'$'})
	f.Add([]byte("$a$\x00"))
	f.Add([]byte("$a$"))
	f.Add([]byte{0, 0})
	f.Add(bytes.Repeat([]byte{'a'}, abi.SerializationBufferSize))
	f.Fuzz(func(t *testing.T, buf []byte) {
		s := newScanner()
		s.Deserialize(buf)
		out := make([]byte, abi.SerializationBufferSize)
		n := s.Serialize(out)
		switch {
		case n > len(out):
			t.Errorf("Serialize wrote %d bytes", n)
		case n > 0 && out[n-1] != 0:
			t.Errorf("Serialize wrote %q, which does not end with a 0 byte", out[:n])
		case n > 0 && s.startTag != nil:
			t.Errorf("Serialize wrote %d bytes, and the start tag %q stays", n, s.startTag)
		}
	})
}
