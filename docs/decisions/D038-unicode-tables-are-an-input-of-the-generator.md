# D38. The generator uses Go's Unicode tables, and takes them as an input

Status: Decided.

Ken decided on 2026-09-29, answering questions 18 and 41, that the generator
uses the Unicode tables of the Go package `unicode` for a class such as
`\p{L}`.

On 2026-09-29, Go 1.27 has Unicode 17.0.0. The Rust crate `regex-syntax`
0.8.11, which the upstream generator uses, has Unicode 16.0.0. A grammar that
uses a Unicode class then gets other character ranges than upstream gives. The
set holds 51 such grammars.

So the Unicode tables are an input of the generator, as the ABI version is
(D17). The test of the C backend gives the generator a table at the Unicode
version of `regex-syntax`, so that its output matches the golden files byte
for byte (D8). Every other output, and the Go backend, uses the tables of
Go.
