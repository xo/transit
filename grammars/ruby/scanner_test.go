package ruby

import (
	"context"
	"strings"
	"testing"

	"github.com/xo/transit"
)

// TestSerializeRoom parses a heredoc whose word fills the buffer of the
// state. The scanner of upstream writes 1 byte past the end of the buffer
// there. The Go scanner writes no state, and the parse ends with no panic
// (D81).
func TestSerializeRoom(t *testing.T) {
	t.Parallel()
	p := transit.NewParser()
	if err := p.SetLanguage(Language()); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{1017, 1018, 1019, 1020, 1021} {
		src := []byte("<<" + strings.Repeat("A", n) + "\nbody\n" + strings.Repeat("A", n) + "\n")
		if _, err := p.Parse(context.Background(), src, nil); err != nil {
			t.Fatalf("a heredoc word of %d bytes: %v", n, err)
		}
	}
}
