# D4. The upstream checkout is in the root, and git ignores it

Status: Decided.

Ken put a checkout of upstream tree-sitter in `tree-sitter/`, in the root of
this repository, on 2026-09-29, and asked that git ignore it. `.gitignore`
holds `/tree-sitter/`.

The checkout is the reference for the port. A person or an agent reads it,
runs `git log` and `git diff` in it, and builds the upstream tool from it to
compare output. Nobody edits a file in it.

Go does not see the Go files in the checkout. `go build ./...` skips a folder
that holds a `go.mod`, and the only Go files upstream are in
`crates/cli/src/templates`, which has one. If upstream adds a Go file outside
that folder, `./...` finds it. [`docs/UPSTREAM.md`](../UPSTREAM.md) holds the
command that finds such a file.

On 2026-09-29 the checkout was at commit
`dcdc8cc55e5dfedfc858080835f153999a29ec40`, from 2026-09-24. The latest
release tag was `v0.27.0`, from 2026-08-30, and `Cargo.toml` on `master` said
`0.28.0`.
