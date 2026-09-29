# D48. The chroma module is chromastyles

Status: Decided, amends D26, D32 and D43, superseded by D65.

Ken decided on 2026-09-29 that the package that matches capture names to
chroma token types is named `chromastyles`, to avoid a clash with the package
`chroma` of `github.com/alecthomas/chroma`. A consumer imports both, and two
packages with the same name need an alias.

Ken offered `cstyles` or `chromastyles`. `chromastyles` was chosen, because
usql already imports `github.com/alecthomas/chroma/v2/styles` under the name
`cstyles`, in its `styles` package.

The folder has the name of the package, so the module is
`github.com/xo/transit/chromastyles`, and it is tagged
`chromastyles/vX.Y.Z`, in place of the `chroma/vX.Y.Z` of D43. The rest of D32 holds.
