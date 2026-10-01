# D102. The shape of the usql grammar

Status: Decided, amended by D105.

Gemini and DeepSeek agreed on these points of the shape of the usql grammar
(D101). Ken accepted them on 2026-10-01, answering question 72.

1. The external scanner finds the end of a statement, the strings, the
   dollar quotes, the comments and the depth of the parentheses. It also
   ends the arguments of a meta command at the end of the line or at the
   next `\` outside quotes. The rules in `grammar.js` stay small.
2. A meta command is one node type, `meta_command`. It has a `name` field
   and argument children: a word, a quoted string, a variable, a list of
   `key=value` pairs in parentheses, a pipe and a shell command in
   backticks. The grammar has no node type for each command. usql maps the
   name and the position of an argument to the kind of the argument, such as
   a file name or a pattern (D6).
3. `\if`, `\elif`, `\else` and `\endif` are flat meta commands. The tree
   does not nest the text between them, so a block that is not finished
   while the user types does not break the tree. usql tracks the nesting.
4. A string or a dollar quote with no end is one token up to the end of the
   input, so the error stays in its own statement.
5. Each SQL statement is one node, and `injections.scm` injects it with the
   language `sql`. The host maps `sql` to the grammar of the dialect that the
   user selected.
6. The shell command after `\!`, after a `|` and in backticks is injected
   with the language `bash`.
7. A variable is a node with its sigil (`:`, `:'`, `:"` or `:{?`), its name,
   and its closing character, so each part can be highlighted.
