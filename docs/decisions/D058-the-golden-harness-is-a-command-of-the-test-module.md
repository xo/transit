# D58. The golden harness is a command of the test module

Status: Decided.

Ken decided on 2026-09-29 that the golden harness is the command
`test/cmd/golden`, in the test module (D12). The harness is test
infrastructure, so the root module stays the runtime and the generator. It
needs no cgo, and it imports only the standard library (D15).

Run it from the test module:

    cd test && go run ./cmd/golden

Ken also decided that the first run of the harness covers the 68 test grammars
and the 15 fixture grammars. The rest of the set follows in a later change,
because a run over every grammar at both ABI versions takes hours.
