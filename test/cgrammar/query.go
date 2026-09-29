package cgrammar

/*
#include <stdlib.h>
#include "query.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"unsafe"

	"github.com/xo/transit"
)

// queryLoaded is true after LoadRuntime found the query functions.
var queryLoaded bool

// loadQuery finds the query functions of the C runtime in lib.
func loadQuery(lib unsafe.Pointer) error {
	if missing := C.cq_load(lib); missing != nil {
		return fmt.Errorf("finding %s in the C runtime", C.GoString(missing))
	}
	queryLoaded = true
	return nil
}

// CQuery is a query of the C runtime.
type CQuery struct {
	query  unsafe.Pointer
	source string
}

// NewCQuery compiles a query with the C runtime. It returns the text of the
// error, "error <offset> <kind>", when the query does not compile.
func (g *Grammar) NewCQuery(source string) (*CQuery, string, error) {
	if !queryLoaded {
		return nil, "", errNoRuntime
	}
	src := C.CString(source)
	defer C.free(unsafe.Pointer(src))
	var offset C.uint32_t
	var kind C.int
	query := C.cq_new(g.lang, src, C.uint32_t(len(source)), &offset, &kind)
	if query == nil {
		return nil, fmt.Sprintf("error %d %d\n", offset, kind), nil
	}
	return &CQuery{query: query, source: source}, "", nil
}

// Close frees the query.
func (q *CQuery) Close() {
	C.cq_delete(q.query)
}

// Describe returns the start, the end, rooted and non-local of each
// pattern, and then guaranteed at each byte of the text of the query.
func (q *CQuery) Describe() string {
	out := C.cq_describe(q.query, C.uint32_t(len(q.source)))
	defer C.free(unsafe.Pointer(out))
	return C.GoString(out)
}

// Run runs the query on the root of a tree of a session, and returns the
// text of each match or capture.
func (q *CQuery) Run(s *CSession, run QueryRun) string {
	root := C.rt_tree_root_node(s.tree.tree)
	captures := C.int(0)
	if run.Captures {
		captures = 1
	}
	out := C.cq_run(q.query, root, captures,
		C.uint32_t(run.StartByte), C.uint32_t(run.EndByte), C.uint32_t(run.MatchLimit), C.uint32_t(run.MaxStartDepth))
	defer C.free(unsafe.Pointer(out))
	return C.GoString(out)
}

// NewGoQuery compiles a query with the Go runtime. It returns the text of
// the error, as NewCQuery writes it, when the query does not compile.
func NewGoQuery(language *transit.Language, source string) (*transit.Query, string) {
	q, err := transit.NewQuery(language, source)
	if err != nil {
		if qerr, ok := errors.AsType[*transit.QueryError](err); ok {
			return nil, fmt.Sprintf("error %d %d\n", qerr.Offset, qerr.Kind)
		}
		return nil, "error " + err.Error()
	}
	return q, ""
}

// DescribeGoQuery returns the text of a Go query that Describe gives for a
// C query.
func DescribeGoQuery(q *transit.Query, source string) string {
	var b strings.Builder
	for i := range q.PatternCount() {
		fmt.Fprintf(&b, "pattern %d %d %d %d %d\n", i, q.StartByteForPattern(i), q.EndByteForPattern(i),
			boolInt(q.IsPatternRooted(i)), boolInt(q.IsPatternNonLocal(i)))
	}
	for i := range len(source) + 2 {
		fmt.Fprintf(&b, "%d", boolInt(q.IsPatternGuaranteedAtStep(i)))
	}
	b.WriteString("\n")
	return b.String()
}

// QueryRun holds the settings of a run of a query.
type QueryRun struct {
	// Captures is true for the captures of a run, and false for its matches.
	Captures   bool
	StartByte  uint32
	EndByte    uint32
	MatchLimit uint32
	// MaxStartDepth is the largest depth at which a match starts, or
	// math.MaxUint32 for none.
	MaxStartDepth uint32
}

// RunGoQuery runs a query on the root of a Go tree with the exported API of
// the Go runtime, and returns the text that Run gives for a C query.
func RunGoQuery(q *transit.Query, tree *transit.Tree, src []byte, run QueryRun) string {
	var b strings.Builder
	c := transit.NewQueryCursor()
	c.SetByteRange(int(run.StartByte), int(run.EndByte))
	c.SetMatchLimit(int(run.MatchLimit))
	if run.MaxStartDepth != math.MaxUint32 {
		c.SetMaxStartDepth(int(run.MaxStartDepth))
	}
	names := q.CaptureNames()
	capture := func(c transit.QueryCapture) {
		fmt.Fprintf(&b, " %s=%d@%d-%d", names[c.Index], c.Node.KindID(), c.Node.StartByte(), c.Node.EndByte())
	}
	ctx := context.Background()
	if run.Captures {
		for m, index := range c.Captures(ctx, q, tree.RootNode(), src) {
			fmt.Fprintf(&b, "capture %d %d:", m.PatternIndex, index)
			capture(m.Captures[index])
			b.WriteString("\n")
		}
	} else {
		for m := range c.Matches(ctx, q, tree.RootNode(), src) {
			fmt.Fprintf(&b, "match %d:", m.PatternIndex)
			for _, cap := range m.Captures {
				capture(cap)
			}
			b.WriteString("\n")
		}
	}
	fmt.Fprintf(&b, "exceeded %d\n", boolInt(c.DidExceedMatchLimit()))
	return b.String()
}

// boolInt returns 1 for true and 0 for false, as C prints a bool with %d.
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
