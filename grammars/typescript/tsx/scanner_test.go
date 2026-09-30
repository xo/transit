package tsx

import (
	"testing"

	"github.com/xo/transit/internal/abi"
)

// FuzzDeserialize gives random bytes to Deserialize, and makes sure that
// Serialize after it writes no byte, as the C scanner does. The test module
// compares the two scanners on random bytes too.
func FuzzDeserialize(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{1, 2, 3})
	f.Fuzz(func(t *testing.T, b []byte) {
		s := newScanner()
		s.Deserialize(b)
		if n := s.Serialize(make([]byte, abi.SerializationBufferSize)); n != 0 {
			t.Errorf("Serialize wrote %d bytes after Deserialize of %x", n, b)
		}
	})
}
