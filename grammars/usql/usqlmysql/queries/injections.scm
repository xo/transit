; Each SQL statement is parsed with the language sql. The host maps sql to the
; grammar of the dialect that the user selected (D102). The variables, the
; strings and the dollar quotes of the statement stay in the text of the
; layer.
((statement) @injection.content
  (#set! injection.language "sql")
  (#set! injection.include-children))

; The text of \!, of a pipe and of a command in backticks is a shell command.
((shell_command) @injection.content
  (#set! injection.language "bash"))
