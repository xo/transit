package transit

import "math"

// This file ports lib/src/length.h.

// length is Length: a length of text in bytes, and as a point.
type length struct {
	bytes  uint32
	extent point
}

// lengthUndefined is LENGTH_UNDEFINED.
var lengthUndefined = length{0, point{0, 1}}

// lengthMax is LENGTH_MAX.
var lengthMax = length{math.MaxUint32, point{math.MaxUint32, math.MaxUint32}}

// isUndefined is length_is_undefined.
func (l length) isUndefined() bool {
	return l.bytes == 0 && l.extent.column != 0
}

// lengthMin is length_min.
func lengthMin(len1, len2 length) length {
	if len1.bytes < len2.bytes {
		return len1
	}
	return len2
}

// add is length_add.
func (l length) add(len2 length) length {
	var result length
	result.bytes = l.bytes + len2.bytes
	result.extent = l.extent.add(len2.extent)
	return result
}

// sub is length_sub.
func (l length) sub(len2 length) length {
	var result length
	if l.bytes >= len2.bytes {
		result.bytes = l.bytes - len2.bytes
	}
	result.extent = l.extent.sub(len2.extent)
	return result
}

// lengthZero is length_zero.
func lengthZero() length {
	return length{0, point{0, 0}}
}

// saturatingSub is length_saturating_sub.
func (l length) saturatingSub(len2 length) length {
	if l.bytes > len2.bytes {
		return l.sub(len2)
	}
	return lengthZero()
}
