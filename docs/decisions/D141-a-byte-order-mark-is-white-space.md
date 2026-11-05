# D141. A byte order mark is white space in the usql grammar

Status: Decided.

The runtime skips U+FEFF, the byte order mark, only at byte 0 of the
input, as the lexer of upstream does. Anywhere else, the usql grammar read
it as the text of a statement. So a script that joins files that each
start with a byte order mark sent `U+FEFF create table ...` to the server,
which refused it. The same text could also split in two ways, by where a
parse starts. The usql peer found this with fuzz tests on 2026-11-05.

Ken decided on 2026-11-05, answering question 110:

1. In the usql grammar, U+FEFF is white space at each position outside a
   string, a quoted name, a dollar quote and a comment. No statement starts
   or ends with it, and no meta command starts with it. A meta command keeps
   the white space at the end of its line, so it can end with U+FEFF, as it
   can end with a space. Ken accepted this on 2026-11-05.
2. Inside a string, a quoted name, a dollar quote and a comment, U+FEFF is
   text, as before.
3. It needs no option, because no SQL product reads U+FEFF outside quotes
   as anything but an error.
4. The runtime does not change. It still skips U+FEFF only at byte 0, as
   upstream does.
