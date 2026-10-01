package usqlplain

import (
	"bytes"
	"testing"

	"github.com/xo/transit/internal/abi"
)

// FuzzDeserialize gives random bytes to Deserialize, and makes sure that
// Serialize after it writes the bytes back when there are 6 of them, and
// the state of a new scanner when there are not, as the C scanner does. The
// test module compares the two scanners on random bytes too.
func FuzzDeserialize(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{1, 2, 3, 4, 5, 6})
	f.Add([]byte{1, 2, 3})
	f.Fuzz(func(t *testing.T, b []byte) {
		s := newScanner()
		s.Deserialize(b)
		buf := make([]byte, abi.SerializationBufferSize)
		n := s.Serialize(buf)
		want := make([]byte, 6)
		if len(b) == 6 {
			want = b
		}
		if !bytes.Equal(buf[:n], want) {
			t.Errorf("Serialize wrote %x after Deserialize of %x, and want %x", buf[:n], b, want)
		}
	})
}
