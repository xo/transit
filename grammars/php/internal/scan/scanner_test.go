package scan

import (
	"testing"

	"github.com/xo/transit/internal/abi"
)

// FuzzDeserialize gives random bytes to Deserialize, as docs/GRAMMAR.md
// asks, and makes sure that Deserialize does not panic, and that Serialize
// after it writes no more than the buffer holds. It calls Deserialize twice,
// because the scanner keeps the heredocs of the first call. The test module
// compares the bytes with the C scanner.
func FuzzDeserialize(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{1})
	f.Add([]byte{1, 1, 3, 0, 0})
	f.Add([]byte{1, 0, 1, 0, 0, 0, 'A', 0, 0, 0})
	f.Add([]byte{1, 0, 2, 0, 0, 0, 'A', 0, 0, 0})
	f.Add([]byte{2, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0x40})
	f.Add([]byte{1, 0, 0xff, 0xff, 0xff, 0xff})
	f.Fuzz(func(t *testing.T, buf []byte) {
		s := New()
		s.Deserialize(buf)
		s.Deserialize(buf)
		out := make([]byte, abi.SerializationBufferSize)
		if n := s.Serialize(out); n > len(out) {
			t.Errorf("Serialize wrote %d bytes", n)
		}
	})
}
