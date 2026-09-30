# D89. QueryMatch.Remove has a Go form

Status: Decided.

The upstream highlighter and its injection code call `QueryMatch::remove` of
the Rust binding, which calls `ts_query_cursor_remove_match`, so that no
other capture of a match comes again. The Go API had no form of it, and the
package `inject` and the highlight test of `internal/grammartest` imitated it
by the pattern and the captures of a match. The imitation is wrong when two
matches of one pattern have the same captures so far. Ken decided on
2026-09-30 that the Go API gets a form of `QueryMatch::remove`, which `inject`
and the highlight test call, and that the two upstream query tests that
skipped for want of it run. `docs/API.md` holds the form.
