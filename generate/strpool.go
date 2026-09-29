package generate

import "maps"

// This file ports crates/generate/src/strpool.rs.

// StrID is the id of an interned string, a 1-based index into the string
// table of a pool.
//
// StrID is StrId.
type StrID uint32

// Index returns the 0-based index of the id.
//
// Index is StrId::index.
func (id StrID) Index() int {
	return int(id) - 1
}

// Raw returns the 1-based id, for packed encodings where 0 means none.
//
// Raw is StrId::raw.
func (id StrID) Raw() uint32 {
	return uint32(id)
}

// StrIDFromRaw is the inverse of Raw. It panics when raw is 0, which no pool
// hands out.
//
// StrIDFromRaw is StrId::from_raw.
func StrIDFromRaw(raw uint32) StrID {
	if raw == 0 {
		panic("generate: a StrID is never 0")
	}
	return StrID(raw)
}

// The ids that every pool starts with.
const (
	// EmptyStrID is the id of "".
	EmptyStrID StrID = 1
	// EndNameID is the id of "end".
	EndNameID StrID = 2
)

// strSpan is the byte range of one interned string.
//
// strSpan is StrSpan.
type strSpan struct {
	start uint32
	end   uint32
}

// StrPool is an append-only pool of strings. The zero StrPool is not ready to
// use. Make one with NewStrPool.
//
// StrPool is StrPool. Upstream finds a string with a hash table of ids. Go
// uses a map from the string to its id, which gives the same ids, because an
// id depends only on the order in which strings are interned.
type StrPool struct {
	// buf holds every interned string, in the order of interning.
	buf []byte
	// spans holds the byte range in buf of each string, by index.
	spans []strSpan
	// ids finds the id of an interned string.
	ids map[string]StrID
}

// NewStrPool returns a pool that holds "" and "end", with the ids EmptyStrID
// and EndNameID.
//
// NewStrPool is StrPool::default.
func NewStrPool() *StrPool {
	p := &StrPool{ids: map[string]StrID{}}
	if id := p.Intern(""); id != EmptyStrID {
		panic("generate: the empty string must have the first id")
	}
	if id := p.Intern("end"); id != EndNameID {
		panic("generate: \"end\" must have the second id")
	}
	return p
}

// Intern returns the id of a string, and adds the string to the pool when it
// is new.
//
// Intern is StrPool::intern.
func (p *StrPool) Intern(s string) StrID {
	if id, ok := p.ids[s]; ok {
		return id
	}
	// The pool is only for the generator. A pool of more than 4 GB of strings
	// is not a use that upstream supports.
	start := len(p.buf)
	end := start + len(s)
	if end > 1<<32-1 {
		panic("generate: the string pool is larger than 4 GB")
	}
	id := StrID(len(p.spans) + 1)
	p.buf = append(p.buf, s...)
	p.spans = append(p.spans, strSpan{start: uint32(start), end: uint32(end)})
	p.ids[s] = id
	return id
}

// Resolve returns the string of an id.
//
// Resolve is StrPool::resolve.
func (p *StrPool) Resolve(id StrID) string {
	span := p.spans[id.Index()]
	return string(p.buf[span.start:span.end])
}

// Clone returns a copy of the pool.
//
// Clone is the Clone of StrPool.
func (p *StrPool) Clone() *StrPool {
	c := &StrPool{
		buf:   append([]byte(nil), p.buf...),
		spans: append([]strSpan(nil), p.spans...),
		ids:   make(map[string]StrID, len(p.ids)),
	}
	maps.Copy(c.ids, p.ids)
	return c
}
