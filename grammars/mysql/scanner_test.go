package mysql

import (
	"testing"

	"github.com/xo/transit/internal/abi"
)

// FuzzDeserialize gives random bytes to Deserialize, and makes sure that
// Serialize after it writes one byte: 1 when the first byte is not 0, and 0
// when it is 0 or when there are no bytes, as the C scanner does. The test
// module compares the two scanners on random bytes too.
func FuzzDeserialize(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0})
	f.Add([]byte{7, 1})
	f.Fuzz(func(t *testing.T, b []byte) {
		s := newScanner()
		s.Deserialize(b)
		buf := make([]byte, abi.SerializationBufferSize)
		n := s.Serialize(buf)
		var want byte
		if len(b) > 0 && b[0] != 0 {
			want = 1
		}
		if n != 1 || buf[0] != want {
			t.Errorf("Serialize wrote %x after Deserialize of %x, and want %x", buf[:n], b, []byte{want})
		}
	})
}
