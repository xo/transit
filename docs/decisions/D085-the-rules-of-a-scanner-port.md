# D85. The rules of a scanner port

Status: Decided, amended by D96.

The ports of the scanners of the fixture grammars in phase 4 raised these
questions. Ken decided on 2026-09-30:

1. `Deserialize` of a Go scanner stops at the end of its buffer. A byte past
   the end reads as 0, and bytes after the state that it reads are ignored.
   A C `assert` of a scanner has no Go form. No buffer can make a Go scanner
   panic. This differs from C only for a buffer that the runtime never
   writes, where C reads past the end or stops on the assert.
2. A C `char` of a scanner is signed, as on x86-64, where the golden files
   are made and the scanners are compared with C. So a byte above 0x7f is
   negative when a scanner widens it. upstream gives other results on a
   platform where `char` is unsigned, such as Linux on arm64.
3. The C header that the scanners of two grammars of one repository share,
   such as `common/scanner.h`, becomes the package `internal/scan` of the
   module.
4. A forward `goto` to a label whose code runs to the end of the function
   becomes a method that holds the code after the label, and the `goto`
   becomes a return of its call. `UPSTREAM.md` holds the rule.
5. A typed constant of a scanner needs no `String` method.
6. A port leaves out an assignment of C that nothing reads, when a linter
   reports it, and a comment says so.
