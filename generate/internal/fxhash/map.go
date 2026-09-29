package fxhash

// This file ports the parts of src/map.rs of hashbrown 0.17.1, as the
// standard library of rustc 1.97.1 vendors it at
// library/vendor/hashbrown-0.17.1, that insert keys into a HashMap<u32, (),
// FxBuildHasher>. That is the map under a hash set of u32 values. The value
// () holds nothing, so the port leaves it out. FxBuildHasher holds nothing
// either, so the map has no field for it.

// hashMap is a hash map from u32 keys to (), with the hasher of lib.go.
//
// hashMap is HashMap<u32, (), FxBuildHasher>.
type hashMap struct {
	table rawTable
}

// makeHasher returns the function that hashes the key of an entry.
//
// makeHasher is make_hasher.
func makeHasher() func(uint32) uint64 {
	return makeHash
}

// equivalentKey returns the function that compares the key of an entry to
// k.
//
// equivalentKey is equivalent_key.
func equivalentKey(k uint32) func(uint32) bool {
	return func(x uint32) bool {
		return k == x
	}
}

// makeHash returns the hash of val.
//
// makeHash is make_hash with FxBuildHasher.
func makeHash(val uint32) uint64 {
	return hashOneU32(val)
}

// newHashMap returns an empty map, which does not allocate.
//
// newHashMap is HashMap::with_hasher.
func newHashMap() hashMap {
	return hashMap{table: newRawTable()}
}

// isEmpty reports whether the map has no entries.
//
// isEmpty is HashMap::is_empty.
func (m *hashMap) isEmpty() bool {
	return m.table.len() == 0
}

// reserve makes space for at least additional more entries.
//
// reserve is HashMap::reserve.
func (m *hashMap) reserve(additional int) {
	m.table.reserve(additional, makeHasher())
}

// insert inserts k. It reports whether the map held k before. The map then
// keeps its old key.
//
// insert is HashMap::insert. The Option<()> that it returns is the bool.
func (m *hashMap) insert(k uint32) bool {
	hash := makeHash(k)
	equivalent := equivalentKey(k)
	hasher := makeHasher()
	index, found := m.table.findOrFindInsertIndex(hash, equivalent, hasher)
	if found {
		return true
	}
	m.table.insertAtIndex(hash, index, k)
	return false
}

// extend inserts each key of keys, in order. sizeHint is the lower bound of
// the size hint of the Rust iterator that gives the keys.
//
// extend is Extend::extend of HashMap.
func (m *hashMap) extend(keys []uint32, sizeHint int) {
	// A key can be in the map already, or it can come more than once. If
	// the map is empty, reserve the lower bound of the hint. Otherwise,
	// reserve half of it, rounded up, so that the map resizes at most twice.
	var reserve int
	if m.isEmpty() {
		reserve = sizeHint
	} else {
		reserve = (sizeHint + 1) / 2
	}
	m.reserve(reserve)
	for _, k := range keys {
		m.insert(k)
	}
}
