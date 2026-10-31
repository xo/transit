; The highlights of the PL/pgSQL grammar. xo wrote this file with the
; grammar (D122). A keyword is an anonymous node with the name of the
; keyword in upper case. The SQL in a body is the node sql_expression or
; sql_statement, which queries/injections.scm gives to the grammar
; postgres, so this file does not color it. The patterns for the names come
; after the general patterns, because the last pattern that captures a node
; counts.

(comment) @comment

(string) @string

(number) @number

(parameter) @variable.parameter

(identifier) @variable

[
  "ABSOLUTE"
  "ALIAS"
  "ALL"
  "AND"
  "ARRAY"
  "ASSERT"
  "BACKWARD"
  "BEGIN"
  "BY"
  "CASE"
  "CHAIN"
  "CLOSE"
  "COLLATE"
  "COMMIT"
  "CONSTANT"
  "CONTINUE"
  "CURRENT"
  "CURSOR"
  "DEBUG"
  "DECLARE"
  "DEFAULT"
  "DIAGNOSTICS"
  "ELSE"
  "ELSEIF"
  "ELSIF"
  "END"
  "EXCEPTION"
  "EXECUTE"
  "EXIT"
  "FETCH"
  "FIRST"
  "FOR"
  "FOREACH"
  "FORWARD"
  "FROM"
  "GET"
  "IF"
  "IN"
  "INFO"
  "INTO"
  "IS"
  "LAST"
  "LOG"
  "LOOP"
  "MOVE"
  "NEXT"
  "NO"
  "NOT"
  "NOTICE"
  "NULL"
  "OPEN"
  "OR"
  "PERFORM"
  "PRIOR"
  "QUERY"
  "RAISE"
  "RELATIVE"
  "RETURN"
  "REVERSE"
  "ROLLBACK"
  "ROWTYPE"
  "SCROLL"
  "SLICE"
  "SQLSTATE"
  "STACKED"
  "STRICT"
  "THEN"
  "TYPE"
  "USING"
  "WARNING"
  "WHEN"
  "WHILE"
] @keyword

[
  ":="
  "="
  "=>"
  ".."
  "%"
  "-"
] @operator

[
  "("
  ")"
  "["
  "]"
  "<<"
  ">>"
] @punctuation.bracket

[
  ","
  "."
  ";"
] @punctuation.delimiter

(compiler_option
  "#" @keyword.directive
  name: (identifier) @keyword.directive
  value: (identifier) @constant)

(type
  (identifier) @type)

(label
  name: (identifier) @label)

(block
  end_label: (identifier) @label)

[
  (loop_statement
    end_label: (identifier) @label)
  (while_statement
    end_label: (identifier) @label)
  (for_statement
    end_label: (identifier) @label)
  (foreach_statement
    end_label: (identifier) @label)
  (exit_statement
    label: (identifier) @label)
]

(cursor_parameter
  name: (identifier) @variable.parameter)

(cursor_argument
  name: (identifier) @variable.parameter)

(exception_handler
  condition: (identifier) @constant)

(raise_statement
  condition: (identifier) @constant)

(diagnostics_item
  item: (identifier) @constant.builtin)

(raise_option
  name: (identifier) @property)
