; The injections of the PL/pgSQL grammar. xo wrote this file with the
; grammar (D122). Each expression and each SQL statement in a body goes to
; the grammar postgres. An expression alone is not a statement of SQL, so
; the grammar postgres gives an ERROR node around it, but it still colors
; its keywords, its names and its literals.

((sql_expression) @injection.content
  (#set! injection.language "postgres"))

((sql_statement) @injection.content
  (#set! injection.language "postgres"))
