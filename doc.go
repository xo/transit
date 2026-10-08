// Package transit is a pure Go port of tree-sitter, the parser generator and
// incremental parsing library.
//
// This package holds the runtime: the parser, the tree, the query engine and
// the lookahead iterator. It ports lib/src of upstream, and each Go file
// ports one C file. See docs/PLAN.md for the plan, and docs/decisions for
// the decisions that shape it.
//
// This module requires no other module, so the examples that parse a text
// are in the grammar package github.com/xo/transit/grammars/json. They show
// a parse, an edit, a walk with a cursor, a query and a completion with
// StatesAt.
package transit
