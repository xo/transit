package clickhouse

import (
	"bytes"
	"testing"

	"github.com/xo/transit/internal/abi"
)

// FuzzDeserialize gives random bytes to Deserialize, and makes sure that
// Serialize after it writes the state that the bytes hold, as the C scanner
// does: the bytes up to the end of the tag when they hold the whole tag,
// with the first byte as 1 or 0, and two zero bytes when they do not. The
// test module compares the two scanners on random bytes too.
func FuzzDeserialize(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{1})
	f.Add([]byte{1, 1, 'a', 0, 0, 0})
	f.Add([]byte{7, 2, 'a', 0, 0, 0, 'b', 0, 0, 0, 9})
	f.Add([]byte{1, 2, 'a', 0, 0, 0})
	f.Fuzz(func(t *testing.T, b []byte) {
		s := newScanner()
		s.Deserialize(b)
		buf := make([]byte, abi.SerializationBufferSize)
		n := s.Serialize(buf)
		want := []byte{0, 0}
		if len(b) >= 2 && len(b) >= 2+4*int(b[1]) {
			want = append([]byte{0}, b[1:2+4*int(b[1])]...)
			if b[0] != 0 {
				want[0] = 1
			}
		}
		if !bytes.Equal(buf[:n], want) {
			t.Errorf("Serialize wrote %x after Deserialize of %x, and want %x", buf[:n], b, want)
		}
	})
}
