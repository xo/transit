# D94. Remove on a zero match does nothing

Status: Decided, amends D89.

A `QueryMatch` that no call of `Matches` or `Captures` gave has no cursor.
The Rust binding cannot build such a match. Ken decided on 2026-10-01 that
`QueryMatch.Remove` on it returns at once and does nothing, so the zero value
is safe to use, as Go code expects.
