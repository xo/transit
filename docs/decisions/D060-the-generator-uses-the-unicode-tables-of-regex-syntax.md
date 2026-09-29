# D60. The generator uses the Unicode tables of regex-syntax

Status: Decided, amends D38 and D45.

Ken decided on 2026-09-29, answering question 57, that every output of the
generator uses the Unicode tables of the port of `regex-syntax` (D59). On
2026-09-29 those tables are at Unicode 16.0.0. The tables move to a new
Unicode version when upstream moves to a new version of `regex-syntax`.

D38 chose the tables of the Go package `unicode`, which is at Unicode 17.0.0
on 2026-09-29, and it made the tables an input of the generator. The port of
the translator showed a gap. The package `unicode` has no derived property,
such as `XID_Start`, `XID_Continue` or `Alphabetic`, and it has no script
extensions. Six patterns in the grammars on disk use `\p{XID_Start}` or
`\p{XID_Continue}`.

So the tables are no longer an input of the generator. The test of the C
backend and every other output use the same tables, and a grammar lexes each
character as upstream does.

D45 made a test build of each Go grammar, at the Unicode version of upstream,
so that the Go trees and the C trees match. A Go grammar and its C grammar now
use the same tables, so the test build is not needed, and the tests use the
grammar package itself.
