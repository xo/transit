package plpgsql

import (
	"testing"

	"github.com/xo/transit/internal/abi"
)

// FuzzDeserialize gives random bytes to Deserialize, and makes sure that
// Serialize after it writes no bytes, because the scanner keeps no state.
// The test module compares the Go scanner with the C scanner on random
// bytes too.
func FuzzDeserialize(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{1, 2, 3})
	f.Fuzz(func(t *testing.T, b []byte) {
		s := newScanner()
		s.Deserialize(b)
		buf := make([]byte, abi.SerializationBufferSize)
		if n := s.Serialize(buf); n != 0 {
			t.Errorf("Serialize wrote %d bytes after Deserialize of %x, and want 0", n, b)
		}
	})
}
