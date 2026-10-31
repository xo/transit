; The injections of the PostgreSQL grammar. xo wrote this file with the
; grammar (D122). The body of a function, a procedure or a DO block in
; dollar quotes is in the language that LANGUAGE names: PL/pgSQL goes to the
; grammar plpgsql, and SQL goes to this grammar. The code of DO without
; LANGUAGE is PL/pgSQL. A body in single quotes gets no injection, because
; its text holds the quotes and each quote in it is doubled.

((create_function_statement
  language: (_) @_language
  body: (function_body
    (dollar_quoted_string
      (string_content) @injection.content)))
  (#match? @_language "^(?i)['\"]?plpgsql['\"]?$")
  (#set! injection.language "plpgsql"))

((create_function_statement
  body: (function_body
    (dollar_quoted_string
      (string_content) @injection.content))
  language: (_) @_language)
  (#match? @_language "^(?i)['\"]?plpgsql['\"]?$")
  (#set! injection.language "plpgsql"))

((create_function_statement
  language: (_) @_language
  body: (function_body
    (dollar_quoted_string
      (string_content) @injection.content)))
  (#match? @_language "^(?i)['\"]?sql['\"]?$")
  (#set! injection.language "postgres"))

((create_function_statement
  body: (function_body
    (dollar_quoted_string
      (string_content) @injection.content))
  language: (_) @_language)
  (#match? @_language "^(?i)['\"]?sql['\"]?$")
  (#set! injection.language "postgres"))

((do_statement
  body: (function_body
    (dollar_quoted_string
      (string_content) @injection.content))
  !language)
  (#set! injection.language "plpgsql"))

((do_statement
  body: (function_body
    (dollar_quoted_string
      (string_content) @injection.content))
  language: (_) @_language)
  (#match? @_language "^(?i)['\"]?plpgsql['\"]?$")
  (#set! injection.language "plpgsql"))

((do_statement
  language: (_) @_language
  body: (function_body
    (dollar_quoted_string
      (string_content) @injection.content)))
  (#match? @_language "^(?i)['\"]?plpgsql['\"]?$")
  (#set! injection.language "plpgsql"))
