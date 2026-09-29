package transit

import (
	"math"
	"testing"
)

func TestLengthArithmetic(t *testing.T) {
	a := length{10, point{1, 4}}
	b := length{3, point{0, 3}}
	if got := a.add(b); got != (length{13, point{1, 7}}) {
		t.Errorf("add = %v", got)
	}
	if got := a.sub(b); got != (length{7, point{1, 4}}) {
		t.Errorf("sub = %v", got)
	}
	// the bytes of a sub do not go below zero, and the extent is subtracted
	if got := b.sub(a); got != (length{0, point{0, 0}}) {
		t.Errorf("sub of a longer length = %v", got)
	}
	if got := a.saturatingSub(b); got != a.sub(b) {
		t.Errorf("saturatingSub = %v", got)
	}
	if got := b.saturatingSub(a); got != lengthZero() {
		t.Errorf("saturatingSub of a longer length = %v", got)
	}
	if got := a.saturatingSub(a); got != lengthZero() {
		t.Errorf("saturatingSub of an equal length = %v", got)
	}
	if got := lengthMin(a, b); got != b {
		t.Errorf("lengthMin = %v", got)
	}
	if got := lengthMin(b, b); got != b {
		t.Errorf("lengthMin of equal lengths = %v", got)
	}
	if got := lengthMax.add(length{1, point{0, 1}}); got.bytes != 0 {
		t.Errorf("bytes of lengthMax plus 1 = %d, want 0 as in C", got.bytes)
	}
	if lengthMax.bytes != math.MaxUint32 {
		t.Errorf("lengthMax = %v", lengthMax)
	}
}

func TestLengthIsUndefined(t *testing.T) {
	if !lengthUndefined.isUndefined() {
		t.Error("lengthUndefined is not undefined")
	}
	for _, l := range []length{lengthZero(), {1, point{0, 1}}, {0, point{1, 0}}} {
		if l.isUndefined() {
			t.Errorf("%v is undefined", l)
		}
	}
}
