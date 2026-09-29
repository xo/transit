# D43. Each module is tagged on its own

Status: Decided, amended by D48 and D65.

Ken decided on 2026-09-29, answering question 33:

1. The root module is tagged `vX.Y.Z`.
2. Each grammar module is tagged `grammars/<repository>/vX.Y.Z`, as resvg
   tags `libresvg/<target>`. The chroma module is tagged `chroma/vX.Y.Z`.
3. A grammar module requires the version of the root module whose generator
   wrote it.
4. Each `go.mod` names the Go version of the root `go.mod`, which is `1.27.1`
   on 2026-09-29.
5. No module promises backward compatibility, as no `xo` project does.

Phase 4 makes the first grammar module and its first tag.
