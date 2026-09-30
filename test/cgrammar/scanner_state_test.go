package cgrammar

import (
	"bytes"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/internal/abi"
)

// compareScannerStateSequence gives the Go scanner of goLang and the C scanner of
// the grammar g the same n buffers from gen, one after the other, and fails
// the test when Serialize after Deserialize writes other bytes. The two
// scanners live through all the buffers, so a state that Deserialize keeps
// from the buffer before counts too. gen must give only buffers that the C
// function reads inside their length, because C does not test the length.
func compareScannerStateSequence(t *testing.T, g *Grammar, goLang *transit.Language, gen func(r *rand.Rand) []byte, n int) {
	t.Helper()
	goScanner := tablesOf(goLang).ExternalScanner.Create()
	cScanner := tablesOf(g.Language).ExternalScanner.Create()
	r := rand.New(rand.NewPCG(1, 2))
	differ := 0
	goOut := make([]byte, abi.SerializationBufferSize)
	cOut := make([]byte, abi.SerializationBufferSize)
	for i := range n {
		buf := gen(r)
		goScanner.Deserialize(slices.Clone(buf))
		cScanner.Deserialize(slices.Clone(buf))
		clear(goOut)
		clear(cOut)
		goN := goScanner.Serialize(goOut)
		cN := cScanner.Serialize(cOut)
		if !bytes.Equal(goOut[:goN], cOut[:cN]) {
			differ++
			if differ <= 3 {
				t.Errorf("buffer %d, %x: C serializes %x, and Go serializes %x", i, buf, cOut[:cN], goOut[:goN])
			}
		}
	}
	if differ > 0 {
		t.Errorf("%d of %d buffers differ", differ, n)
	}
}
