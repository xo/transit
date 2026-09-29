package transit

// This file ports lib/src/ts_assert.h.

// assert is ts_assert. Upstream builds the runtime without NDEBUG, so the C
// macro calls assert of the C library, which aborts when the condition is
// false. The Go form panics, which D44 counts as a fault. The caller
// computes the condition before the call, so any side effect of it happens
// as in C.
func assert(cond bool) {
	if !cond {
		panic("transit: assertion failed")
	}
}
