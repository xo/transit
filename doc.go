// Package transit is a pure Go port of tree-sitter, the parser generator and
// incremental parsing library.
//
// This package holds the runtime: the parser, the tree, the query engine and
// the lookahead iterator. Phase 3 ports it from lib/src of upstream, one C
// file at a time. Today it holds all of the runtime.
// See docs/PLAN.md for the plan, and docs/decisions for the decisions that
// shape it.
package transit
