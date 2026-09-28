# D26. The packages and modules of transit

Status: Decided, amended by D48.

Ken decided on 2026-09-29, answering questions 5, 8, 39 and 40:

| Path | Package | What it holds |
| --- | --- | --- |
| `github.com/xo/transit` | `transit` | the runtime |
| `generate` | `generate` | the generator |
| `generate/backend/c` | `c` | the C backend (D8) |
| `generate/backend/go` | `golang` | the Go backend. `go` is a keyword, so the package name differs from the folder |
| `inject` | `inject` | the injections (D27) |
| `cmd/transit` | `main` | the command (D41) |
| `chroma` | `chroma` | a module of its own (D32) |
| `grammars/<repository>` | the grammar | one Go module for each grammar repository |
| `test` | | the test module, with its own `go.mod` (D12) |

A grammar module holds one package for each grammar of its repository.
`docs/GRAMMAR.md` holds its layout.
