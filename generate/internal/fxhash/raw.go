package fxhash

// This file ports src/raw.rs of hashbrown 0.17.1, as the standard library of
// rustc 1.97.1 vendors it at library/vendor/hashbrown-0.17.1, on x86_64. The
// port covers what a hash set of u32 values needs to insert values, to grow
// and to iterate: the probe sequence, the number of buckets for a capacity,
// the search for a value or a free bucket, the resize, and the iterators.
//
// The value of a bucket is a (u32, ()), which has the size 4 and the
// alignment 4. hashbrown keeps the values and the control bytes in one
// allocation, and RawTableInner moves the values as bytes, because it does
// not know their type. The port keeps the values in a slice of uint32 in
// rawTableInner, because it knows the type.
//
// The port leaves out removal, and the rehash in place, which reuses the
// buckets of deleted values. A table that only inserts has no deleted
// values, so reserveRehashInner never rehashes in place. It also leaves out
// the allocator and the fallible paths, because Go panics when memory runs
// out, as a Rust program with an infallible table aborts.

// h1 returns the primary hash, which selects the first bucket to probe.
//
// h1 is h1.
func h1(hash uint64) int {
	return int(hash)
}

// probeSeq is a probe sequence of triangular numbers. The table has a size
// that is a power of two, so the sequence visits each group exactly once.
//
// A triangular probe jumps one group more each time. It jumps 1 group first,
// which continues the linear scan, then 2 groups, which skips 1 group, then
// 3 groups, which skips 2 groups, and so on.
//
// A proof that the probe visits every group of the table is at
// https://fgiesen.wordpress.com/2015/02/22/triangular-numbers-mod-2n/.
//
// probeSeq is ProbeSeq.
type probeSeq struct {
	pos    int
	stride int
}

// moveNext moves to the next group of the sequence.
//
// moveNext is ProbeSeq::move_next.
func (p *probeSeq) moveNext(bucketMask int) {
	p.stride += groupWidth
	p.pos += p.stride
	p.pos &= bucketMask
}

// capacityToBuckets returns the number of buckets that hold cap items, with
// the maximum load factor. It returns false if the number overflows.
//
// capacityToBuckets is capacity_to_buckets.
func capacityToBuckets(capacity int, layout tableLayout) (int, bool) {
	// A small table needs at least 1 empty bucket, so that a lookup ends
	// when an element is not in the table.
	if capacity < 15 {
		// A small layout, such as { size: 1, ctrl_align: 16 } with groups of
		// 16 bytes, wastes bytes to pad the buckets to ctrl_align. The
		// number of bytes of the buckets must be at least ctrl_align, so a
		// small item gets a larger minimum capacity. This is brittle: with
		// groups of 32 bytes, it selects 3 for every size.
		var minCap int
		switch {
		case groupWidth == 16 && layout.size <= 1:
			minCap = 14
		case groupWidth == 16 && layout.size <= 3, groupWidth == 8 && layout.size <= 1:
			minCap = 7
		default:
			minCap = 3
		}
		capacity = max(minCap, capacity)
		// A table of 2 buckets can hold only 1 element. The table skips to
		// 4 buckets, which hold 3 elements.
		switch {
		case capacity < 4:
			return 4, true
		case capacity < 8:
			return 8, true
		default:
			return 16, true
		}
	}

	// Otherwise, 1/8 of the buckets must be empty (a load of 87.5%).
	if capacity > maxInt/8 {
		return 0, false
	}
	adjustedCap := capacity * 8 / 7

	// The check above catches an overflow. nextPowerOfTwo cleans up an error
	// of rounding from the division, and it cannot overflow, because of the
	// division.
	return nextPowerOfTwo(adjustedCap), true
}

// maxInt is the largest int. Go has no usize, so the checks for an overflow
// use the largest int in place of usize::MAX. A table of u32 values never
// comes near either of them.
const maxInt = int(^uint(0) >> 1)

// nextPowerOfTwo returns the smallest power of two that is at least n.
//
// nextPowerOfTwo is usize::next_power_of_two.
func nextPowerOfTwo(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

// bucketMaskToCapacity returns the maximum capacity for a bucket mask, with
// the maximum load factor.
//
// bucketMaskToCapacity is bucket_mask_to_capacity.
func bucketMaskToCapacity(bucketMask int) int {
	if bucketMask < 8 {
		// A table of 1, 2, 4 or 8 buckets keeps one bucket empty. The bucket
		// mask is one less than the number of buckets.
		return bucketMask
	}
	// A larger table keeps 12.5% of the buckets empty.
	return ((bucketMask + 1) / 8) * 7
}

// tableLayout holds the size of a value and the alignment of the control
// bytes.
//
// tableLayout is TableLayout.
type tableLayout struct {
	size      int
	ctrlAlign int
}

// layoutU32 is the layout of a bucket of a set of u32 values, whose value is
// a (u32, ()).
//
// layoutU32 is TableLayout::new::<(u32, ())>().
var layoutU32 = tableLayout{size: 4, ctrlAlign: groupWidth}

// rawTable is a hash table of u32 values.
//
// rawTable is RawTable<(u32, ())>.
type rawTable struct {
	table rawTableInner
}

// rawTableInner is the part of a table that does not depend on the type of
// its values.
//
// rawTableInner is RawTableInner. The values of the buckets, which hashbrown
// keeps before the control bytes, are in data.
type rawTableInner struct {
	// bucketMask is the number of buckets less one. The number of buckets
	// is a power of two.
	bucketMask int
	// ctrl holds the control bytes: one for each bucket, then groupWidth
	// more, which repeat the first ones, so that a group can be loaded at
	// any bucket.
	ctrl []tag
	// growthLeft is the number of items that the table can take before it
	// must grow.
	growthLeft int
	// items is the number of items in the table.
	items int
	// data holds the value of each bucket.
	data []uint32
}

// newRawTable returns an empty table, which does not allocate.
//
// newRawTable is RawTable::new.
func newRawTable() rawTable {
	return rawTable{table: newRawTableInner()}
}

// reserve makes sure that the table can take additional more items without
// a new allocation.
//
// reserve is RawTable::reserve.
func (t *rawTable) reserve(additional int, hasher func(uint32) uint64) {
	if additional > t.table.growthLeft {
		t.reserveRehash(additional, hasher)
	}
}

// reserveRehash is the slow path of reserve.
//
// reserveRehash is RawTable::reserve_rehash.
func (t *rawTable) reserveRehash(additional int, hasher func(uint32) uint64) {
	t.table.reserveRehashInner(additional, func(table *rawTableInner, index int) uint64 {
		return hasher(table.data[index])
	}, layoutU32)
}

// findOrFindInsertIndex searches for a value that eq accepts, with the hash
// hash. If the table holds it, findOrFindInsertIndex returns its index and
// true. Otherwise, it returns the index of the bucket where the value goes,
// and false. It can resize the table first, to make space for one more item.
//
// findOrFindInsertIndex is RawTable::find_or_find_insert_index. The index of
// a bucket takes the place of the Bucket.
func (t *rawTable) findOrFindInsertIndex(hash uint64, eq func(uint32) bool, hasher func(uint32) uint64) (int, bool) {
	t.reserve(1, hasher)

	return t.table.findOrFindInsertIndexInner(hash, func(index int) bool {
		return eq(t.table.data[index])
	})
}

// insertAtIndex inserts value in the bucket at index, with the tag of hash.
// index must come from findOrFindInsertIndex, with no change to the table
// after it.
//
// insertAtIndex is RawTable::insert_at_index.
func (t *rawTable) insertAtIndex(hash uint64, index int, value uint32) {
	t.insertTaggedAtIndex(tagFull(hash), index, value)
}

// insertTaggedAtIndex inserts value in the bucket at index, with the tag tg.
// index must come from findOrFindInsertIndex, with no change to the table
// after it.
//
// insertTaggedAtIndex is RawTable::insert_tagged_at_index.
func (t *rawTable) insertTaggedAtIndex(tg tag, index int, value uint32) {
	oldCtrl := t.table.ctrl[index]
	t.table.recordItemInsertAt(index, oldCtrl, tg)

	t.table.data[index] = value
}

// len returns the number of items in the table.
//
// len is RawTable::len.
func (t *rawTable) len() int {
	return t.table.items
}

// iter returns an iterator over the values of the table.
//
// iter is RawTable::iter.
func (t *rawTable) iter() rawIter {
	return t.table.iter()
}

// newRawTableInner returns an empty table with no buckets.
//
// newRawTableInner is RawTableInner::new, and RawTableInner::NEW.
func newRawTableInner() rawTableInner {
	return rawTableInner{
		ctrl: staticEmpty(),
	}
}

// newUninitializedInner returns a table with buckets buckets, whose control
// bytes the caller must fill.
//
// newUninitializedInner is RawTableInner::new_uninitialized.
func newUninitializedInner(buckets int) rawTableInner {
	return rawTableInner{
		bucketMask: buckets - 1,
		ctrl:       make([]tag, buckets+groupWidth),
		growthLeft: bucketMaskToCapacity(buckets - 1),
		data:       make([]uint32, buckets),
	}
}

// fallibleWithCapacity returns an empty table that can take capacity items.
// It panics, as the infallible table of upstream does, when the number of
// buckets overflows.
//
// fallibleWithCapacity is RawTableInner::fallible_with_capacity, with
// Fallibility::Infallible.
func fallibleWithCapacity(layout tableLayout, capacity int) rawTableInner {
	if capacity == 0 {
		return newRawTableInner()
	}
	buckets, ok := capacityToBuckets(capacity, layout)
	if !ok {
		panic("Hash table capacity overflow")
	}
	result := newUninitializedInner(buckets)
	fillEmpty(result.ctrl)
	return result
}

// fillEmpty sets every tag of ctrl to tagEmpty.
//
// fillEmpty is TagSliceExt::fill_empty.
func fillEmpty(ctrl []tag) {
	for i := range ctrl {
		ctrl[i] = tagEmpty
	}
}

// fixInsertIndex fixes an index that findInsertIndexInGroup returned.
//
// In a table smaller than a group, the trailing control bytes of a group can
// match as empty, even though they do not belong to a bucket. The index of
// such a byte, masked, can be the index of a full bucket. The table has at
// least one free bucket, so fixInsertIndex then searches the first group,
// which is aligned and holds every bucket, for a free one.
//
// fixInsertIndex is RawTableInner::fix_insert_index.
func (r *rawTableInner) fixInsertIndex(index int) int {
	if r.isBucketFull(index) {
		index, _ = groupLoad(r.ctrl, 0).matchEmptyOrDeleted().lowestSetBit()
	}
	return index
}

// findInsertIndexInGroup returns the index of the first empty or deleted
// bucket in the group at the position of probe, and false if it has none.
// The index can be the index of a full bucket in a table smaller than a
// group. fixInsertIndex fixes that.
//
// findInsertIndexInGroup is RawTableInner::find_insert_index_in_group.
func (r *rawTableInner) findInsertIndexInGroup(g group, probe probeSeq) (int, bool) {
	bit, ok := g.matchEmptyOrDeleted().lowestSetBit()
	if !ok {
		return 0, false
	}
	return (probe.pos + bit) & r.bucketMask, true
}

// findOrFindInsertIndexInner searches for a bucket that eq accepts, with the
// hash hash. If it finds one, it returns its index and true. Otherwise, it
// returns the index of the first empty or deleted bucket of the probe
// sequence, and false. The table must have at least one empty bucket.
//
// findOrFindInsertIndexInner is RawTableInner::find_or_find_insert_index_inner.
func (r *rawTableInner) findOrFindInsertIndexInner(hash uint64, eq func(int) bool) (int, bool) {
	insertIndex, haveInsert := 0, false

	tagHash := tagFull(hash)
	probe := r.probeSeq(hash)

	for {
		g := groupLoad(r.ctrl, probe.pos)

		for it := g.matchTag(tagHash).iter(); ; {
			bit, ok := it.next()
			if !ok {
				break
			}
			index := (probe.pos + bit) & r.bucketMask
			if eq(index) {
				return index, true
			}
		}

		// The table can have deleted buckets, so the probe keeps searching
		// for the value after it finds a free bucket, until the group has an
		// empty bucket.
		if !haveInsert {
			insertIndex, haveInsert = r.findInsertIndexInGroup(g, probe)
		}

		if haveInsert && g.matchEmpty().anyBitSet() {
			return r.fixInsertIndex(insertIndex), false
		}

		probe.moveNext(r.bucketMask)
	}
}

// prepareInsertIndex finds a free bucket for hash and sets its control
// byte. It returns the index of the bucket and its old control byte.
//
// prepareInsertIndex is RawTableInner::prepare_insert_index.
func (r *rawTableInner) prepareInsertIndex(hash uint64) (int, tag) {
	index := r.findInsertIndex(hash)
	oldCtrl := r.ctrl[index]
	r.setCtrlHash(index, hash)
	return index, oldCtrl
}

// findInsertIndex returns the index of the first empty or deleted bucket of
// the probe sequence of hash. The table must have at least one empty bucket.
//
// findInsertIndex is RawTableInner::find_insert_index.
func (r *rawTableInner) findInsertIndex(hash uint64) int {
	probe := r.probeSeq(hash)
	for {
		g := groupLoad(r.ctrl, probe.pos)

		if index, ok := r.findInsertIndexInGroup(g, probe); ok {
			return r.fixInsertIndex(index)
		}
		probe.moveNext(r.bucketMask)
	}
}

// iter returns an iterator over the values of the table.
//
// iter is RawTableInner::iter.
func (r *rawTableInner) iter() rawIter {
	return rawIter{
		iter:  newRawIterRange(r, 0, r.numBuckets()),
		items: r.items,
	}
}

// probeSeq returns the start of the probe sequence of hash.
//
// probeSeq is RawTableInner::probe_seq.
func (r *rawTableInner) probeSeq(hash uint64) probeSeq {
	return probeSeq{
		pos:    h1(hash) & r.bucketMask,
		stride: 0,
	}
}

// recordItemInsertAt sets the control byte of the bucket at index to
// newCtrl, and counts the new item.
//
// recordItemInsertAt is RawTableInner::record_item_insert_at.
func (r *rawTableInner) recordItemInsertAt(index int, oldCtrl, newCtrl tag) {
	if oldCtrl.specialIsEmpty() {
		r.growthLeft--
	}
	r.setCtrl(index, newCtrl)
	r.items++
}

// setCtrlHash sets the control byte of the bucket at index to the tag of
// hash.
//
// setCtrlHash is RawTableInner::set_ctrl_hash.
func (r *rawTableInner) setCtrlHash(index int, hash uint64) {
	r.setCtrl(index, tagFull(hash))
}

// setCtrl sets a control byte, and the copy of it at the end of the control
// bytes when it has one. It does not change the values, items or
// growthLeft.
//
// setCtrl is RawTableInner::set_ctrl.
func (r *rawTableInner) setCtrl(index int, ctrl tag) {
	// Copy the first groupWidth control bytes to the end of the control
	// bytes, with no branch. If the table is smaller than a group, index2 is
	// groupWidth + index. Otherwise:
	//
	//   - If index >= groupWidth, then index2 is index.
	//   - Otherwise, index2 is bucketMask + 1 + index.
	//
	// A load never reads the last copied byte, because the first index of an
	// unaligned load is masked. The byte is written all the same, because
	// that makes setCtrl simpler.
	//
	// If the table has fewer buckets than groupWidth, the copies are at the
	// end of the trailing group. For example, with 2 buckets and a group of
	// 4 bytes, the control bytes are:
	//
	//	    Real    |             Copies
	//	---------------------------------------------
	//	| [A] | [B] | [Tag::EMPTY] | [EMPTY] | [A] | [B] |
	//	---------------------------------------------
	//
	// The number of buckets is a power of two, so this is
	// (index - groupWidth) % numBuckets + groupWidth, with a subtraction
	// that wraps. The & of a negative int in Go gives the same bits.
	index2 := ((index - groupWidth) & r.bucketMask) + groupWidth

	r.ctrl[index] = ctrl
	r.ctrl[index2] = ctrl
}

// numBuckets returns the number of buckets.
//
// numBuckets is RawTableInner::num_buckets.
func (r *rawTableInner) numBuckets() int {
	return r.bucketMask + 1
}

// isBucketFull reports whether the bucket at index is full.
//
// isBucketFull is RawTableInner::is_bucket_full.
func (r *rawTableInner) isBucketFull(index int) bool {
	return r.ctrl[index].isFull()
}

// prepareResize returns a new empty table that can take capacity items.
//
// prepareResize is RawTableInner::prepare_resize. Upstream frees the table
// with a scope guard when the resize fails. Here the garbage collector frees
// it.
func (r *rawTableInner) prepareResize(layout tableLayout, capacity int) rawTableInner {
	return fallibleWithCapacity(layout, capacity)
}

// reserveRehashInner makes space for additional more items. hasher returns
// the hash of the value at an index of the table.
//
// reserveRehashInner is RawTableInner::reserve_rehash_inner. Upstream
// rehashes in place when the new number of items is at most half of the
// full capacity, to reuse the buckets of deleted values. A table that only
// inserts always has growthLeft equal to the full capacity less items, so
// the caller comes here only when items plus additional is more than the
// full capacity, and that branch never runs.
func (r *rawTableInner) reserveRehashInner(additional int, hasher func(*rawTableInner, int) uint64, layout tableLayout) {
	if additional > maxInt-r.items {
		panic("Hash table capacity overflow")
	}
	newItems := r.items + additional
	fullCapacity := bucketMaskToCapacity(r.bucketMask)
	if newItems <= fullCapacity/2 {
		panic("fxhash: rehash in place is not ported, because a table that only inserts never needs it")
	}
	r.resizeInner(max(newItems, fullCapacity+1), hasher, layout)
}

// fullBucketsIndicesOf returns an iterator over the indexes of the full
// buckets of the table.
//
// fullBucketsIndicesOf is RawTableInner::full_buckets_indices.
func (r *rawTableInner) fullBucketsIndicesOf() fullBucketsIndices {
	return fullBucketsIndices{
		currentGroup:    groupLoad(r.ctrl, 0).matchFull().iter(),
		groupFirstIndex: 0,
		ctrl:            r.ctrl,
		pos:             0,
		items:           r.items,
	}
}

// resizeInner moves the items to a new table that can take capacity items.
// It inserts them in the order of the full buckets of the old table. hasher
// returns the hash of the value at an index of the old table.
//
// resizeInner is RawTableInner::resize_inner.
func (r *rawTableInner) resizeInner(capacity int, hasher func(*rawTableInner, int) uint64, layout tableLayout) {
	newTable := r.prepareResize(layout, capacity)

	for it := r.fullBucketsIndicesOf(); ; {
		fullByteIndex, ok := it.next()
		if !ok {
			break
		}
		hash := hasher(r, fullByteIndex)

		newIndex, _ := newTable.prepareInsertIndex(hash)

		newTable.data[newIndex] = r.data[fullByteIndex]
	}

	newTable.growthLeft -= r.items
	newTable.items = r.items

	*r = newTable
}

// rawIterRange iterates over the full buckets of a range of a table.
//
// rawIterRange is RawIterRange. The index data of the first bucket of the
// current group takes the place of the Bucket, and the index nextCtrl of the
// next group takes the place of the pointer.
type rawIterRange struct {
	currentGroup bitMaskIter
	table        *rawTableInner
	data         int
	nextCtrl     int
	end          int
}

// newRawIterRange returns an iterator over the n buckets of table from the
// index start. start must be a multiple of groupWidth.
//
// newRawIterRange is RawIterRange::new.
func newRawIterRange(table *rawTableInner, start, n int) rawIterRange {
	return rawIterRange{
		currentGroup: groupLoad(table.ctrl, start).matchFull().iter(),
		table:        table,
		data:         start,
		nextCtrl:     start + groupWidth,
		end:          start + n,
	}
}

// nextImpl returns the index of the next full bucket. When checkPtrRange is
// true, it returns false at the end of the range. When it is false, the
// caller must know that a next full bucket exists.
//
// nextImpl is RawIterRange::next_impl.
func (it *rawIterRange) nextImpl(checkPtrRange bool) (int, bool) {
	for {
		if index, ok := it.currentGroup.next(); ok {
			return it.data + index, true
		}

		if checkPtrRange && it.nextCtrl >= it.end {
			return 0, false
		}

		it.currentGroup = groupLoad(it.table.ctrl, it.nextCtrl).matchFull().iter()
		it.data += groupWidth
		it.nextCtrl += groupWidth
	}
}

// rawIter iterates over the values of a table. It counts the items, so it
// stops after the last full bucket.
//
// rawIter is RawIter.
type rawIter struct {
	iter  rawIterRange
	items int
}

// next returns the next value, and false at the end.
//
// next is Iterator::next of RawIter.
func (it *rawIter) next() (uint32, bool) {
	if it.items == 0 {
		return 0, false
	}

	index, _ := it.iter.nextImpl(false)

	it.items--

	return it.iter.table.data[index], true
}

// fullBucketsIndices iterates over the indexes of the full buckets of a
// table.
//
// fullBucketsIndices is FullBucketsIndices. The index pos of the current
// group in ctrl takes the place of the pointer ctrl.
type fullBucketsIndices struct {
	currentGroup    bitMaskIter
	groupFirstIndex int
	ctrl            []tag
	pos             int
	items           int
}

// nextImpl returns the index of the next full bucket. The caller must know
// that it exists.
//
// nextImpl is FullBucketsIndices::next_impl.
func (it *fullBucketsIndices) nextImpl() int {
	for {
		if index, ok := it.currentGroup.next(); ok {
			return it.groupFirstIndex + index
		}

		it.pos += groupWidth

		it.currentGroup = groupLoad(it.ctrl, it.pos).matchFull().iter()
		it.groupFirstIndex += groupWidth
	}
}

// next returns the index of the next full bucket, and false at the end.
//
// next is Iterator::next of FullBucketsIndices.
func (it *fullBucketsIndices) next() (int, bool) {
	if it.items == 0 {
		return 0, false
	}

	nxt := it.nextImpl()

	it.items--

	return nxt, true
}
