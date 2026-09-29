package transit

import "testing"

func TestAssert(t *testing.T) {
	assert(true)
	defer func() {
		if recover() == nil {
			t.Error("assert(false) did not panic")
		}
	}()
	assert(false)
}
