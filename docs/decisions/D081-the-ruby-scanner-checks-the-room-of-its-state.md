# D81. The ruby scanner checks the room of its state

Status: Decided.

The external scanner of tree-sitter-ruby at `v0.23.1` counts 2 bytes for the
4 bytes of the header of each heredoc when it tests the room in the buffer of
its state. A heredoc word of 1,019 bytes makes it write 1,025 bytes into the
buffer of 1,024 bytes. In C this writes past the end of the buffer. In the Go
port it panics, so a Ruby text can stop a program that parses it.

Ken decided on 2026-09-30 that the Go scanner of ruby counts the room of each
field it writes, and writes no state (it returns 0) when the state does not
fit, as other scanners do. This is a deliberate difference from upstream
(hard rule 6). It changes only a state that C cannot write safely.
