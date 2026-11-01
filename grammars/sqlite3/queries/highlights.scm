; The highlights of the SQLite grammar. xo wrote this file with the grammar
; (D126). A keyword is an anonymous node with the name of the keyword in
; upper case. The patterns for the constants and the names come after the
; keywords, because the last pattern that captures a node counts, so a name
; gets the capture of its place, such as a function or a table.

(comment) @comment

(string) @string

(blob) @string.special

(number) @number

(boolean) @boolean

(parameter) @variable.parameter

(identifier) @variable

[
  "ABORT"
  "ALL"
  "AND"
  "AS"
  "ASC"
  "BETWEEN"
  "BY"
  "CASE"
  "CAST"
  "COLLATE"
  "CURRENT"
  "DESC"
  "DISTINCT"
  "ELSE"
  "END"
  "ESCAPE"
  "EXCEPT"
  "EXCLUDE"
  "EXISTS"
  "FAIL"
  "FILTER"
  "FIRST"
  "FOLLOWING"
  "FROM"
  "GLOB"
  "GROUP"
  "GROUPS"
  "HAVING"
  "IGNORE"
  "IN"
  "INDEXED"
  "INTERSECT"
  "IS"
  "ISNULL"
  "JOIN"
  "LAST"
  "LIKE"
  "LIMIT"
  "MATCH"
  "MATERIALIZED"
  "NO"
  "NOT"
  "NOTNULL"
  "NULL"
  "NULLS"
  "OFFSET"
  "ON"
  "OR"
  "ORDER"
  "OTHERS"
  "OVER"
  "PARTITION"
  "PRECEDING"
  "RAISE"
  "RANGE"
  "RECURSIVE"
  "REGEXP"
  "ROLLBACK"
  "ROW"
  "ROWS"
  "SELECT"
  "THEN"
  "TIES"
  "UNBOUNDED"
  "UNION"
  "USING"
  "VALUES"
  "WHEN"
  "WHERE"
  "WINDOW"
  "WITH"
] @keyword

(null
  _ @constant.builtin)

(niladic_function) @function.builtin

(type
  name: (identifier) @type.builtin)

(function_call
  name: (identifier) @function.call)

(function_call
  schema: (identifier) @module)

(cast_expression
  "CAST" @function.builtin)

(raise_expression
  "RAISE" @keyword.exception)

(object_reference
  schema: (identifier) @module)

(object_reference
  name: (identifier) @type)

(table_reference
  alias: (identifier) @variable)

(table_reference
  index: (identifier) @label)

(derived_table
  alias: (identifier) @variable)

(function_table
  alias: (identifier) @variable)

(parenthesized_join
  alias: (identifier) @variable)

(cte
  name: (identifier) @type)

(window_definition
  name: (identifier) @type)

(window_specification
  name: (identifier) @type)

(over_clause
  window: (identifier) @type)

(column_reference
  schema: (identifier) @module)

(column_reference
  table: (identifier) @type)

(column_reference
  name: (identifier) @variable.member)

(all_columns) @punctuation.special

(all_columns
  table: (identifier) @type)

(aliased_expression
  alias: (identifier) @variable)

(column_list
  (identifier) @variable.member)

(collate_expression
  collation: (identifier) @variable.builtin)

(case_expression
  [
    "CASE"
    "WHEN"
    "THEN"
    "ELSE"
    "END"
  ] @keyword.conditional)

(when_clause
  [
    "WHEN"
    "THEN"
  ] @keyword.conditional)

(operator) @operator

[
  "("
  ")"
] @punctuation.bracket

[
  ","
  "."
  ";"
] @punctuation.delimiter
