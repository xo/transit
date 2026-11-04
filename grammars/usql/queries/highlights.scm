; The highlights of the input of usql. The SQL of a statement and the text of
; a shell command come from the grammars that queries/injections.scm names.

(comment) @comment

(meta_command
  name: (command_name) @function.builtin)

(modifier) @keyword.modifier

(meta_command
  (word) @variable.parameter)

(option
  key: (word) @property)

(option
  value: (word) @string.special)

[
  (string)
  (dollar_string)
] @string

(quoted_identifier) @string.special

(shell_command) @embedded

(variable
  name: (variable_name) @variable)

(variable
  [
    ":"
    ":'"
    ":\""
    ":{?"
  ] @punctuation.special)

(variable
  [
    "'"
    "\""
    "}"
  ] @punctuation.special)

[
  "("
  ")"
] @punctuation.bracket

[
  "|"
  "`"
] @punctuation.special

"=" @operator

";" @punctuation.delimiter
