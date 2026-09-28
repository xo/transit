# D52. The concurrency guarantees follow upstream, in Go terms

Status: Decided.

Ken decided on 2026-09-29, answering question 51:

1. A `Language` and a `Query` are safe to share between goroutines.
2. A `Tree` is safe for reads from many goroutines. `Tree.Edit` changes it,
   so a caller that keeps a version calls `Tree.Copy` first, as upstream uses
   `ts_tree_copy`.
3. A `Parser`, a `QueryCursor`, a `TreeCursor` and a `LookaheadIterator`
   belong to one goroutine at a time.

The target API of phase 1 states each guarantee in the doc comment of the
type.
