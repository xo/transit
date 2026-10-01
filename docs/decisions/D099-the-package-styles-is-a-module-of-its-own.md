# D99. The package styles is a module of its own

Status: Decided, amends D26, D43 and D65.

D65 put the package `styles` in the root module, so a change of a style was a
release of the runtime. At Ken's request, the transit agent asked Gemini and
DeepSeek on 2026-10-01 where the package belongs: in the root module of
transit, in a module of its own in the transit repository, in a repository
of its own, or in rline. Both advised a module of its own in the transit
repository. A theme changes more often than the runtime. The styles key on
the capture names of the grammars of transit, so the list of capture names
and its test stay in the same repository. A pager or a code viewer can use
the styles without rline.

Ken decided on 2026-10-01, answering question 66:

1. The package `github.com/xo/transit/styles` is a module of its own, with
   its own `go.mod` in `styles/`, as each grammar module is (D26).
2. The module is tagged on its own, with tags such as `styles/v0.1.0` (D43).
   A change of a style does not release the root module.
3. The module imports only the Go standard library, as D65 says. It does not
   require the root module.
4. `gen.sh` writes its `replace` block, as it does for the other modules
   (D49).
5. The command that converts the styles of chroma and measures the capture
   names stays in the test module (D65).
