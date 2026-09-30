# D87. The test module runs with a timeout of an hour

Status: Decided.

With the 15 grammar modules of the fixture grammars, the package
`test/cgrammar` takes about 15 minutes under `-race`, more than the default
timeout of 10 minutes of `go test`. Ken decided on 2026-09-30 that the
command of the test module in `AGENTS.md` and in CI passes `-timeout 60m`.
Every comparison with C keeps its full size.
