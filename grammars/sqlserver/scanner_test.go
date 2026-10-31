package sqlserver

import (
	"testing"

	"github.com/xo/transit/internal/abi"
)

// FuzzDeserialize gives random bytes to Deserialize, and makes sure that
// Serialize after it writes no byte, as the C scanner does, because the
// scanner has no state. The test module compares the two scanners on
// random bytes too.
func FuzzDeserialize(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0})
	f.Add([]byte{7, 1})
	f.Fuzz(func(t *testing.T, b []byte) {
		s := newScanner()
		s.Deserialize(b)
		buf := make([]byte, abi.SerializationBufferSize)
		if n := s.Serialize(buf); n != 0 {
			t.Errorf("Serialize wrote %x after Deserialize of %x, and want no byte", buf[:n], b)
		}
	})
}
