# D32. A chroma module maps capture names to chroma token types

Status: Decided, amends D14 and D15, amended by D48, superseded by D65.

Ken decided on 2026-09-29, answering questions 23 and 25, that transit
publishes a Go package that matches capture names, such as `keyword.function`,
to chroma token types. rline and usql then draw the same code in the same
colors.

The package is `github.com/xo/transit/chroma`, in the folder `chroma`, and it
has its own `go.mod`. A program that does not import it does not get chroma.
It requires only the standard library, the packages of transit and chroma.

The tests that draw captures with a chroma style live in this module.

This amends D14, which used chroma only in tests, and D15, which approved
chroma for tests only. Ken approves chroma for this module. The root module
still requires nothing (D11).
