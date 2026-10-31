; The highlights of the PostgreSQL grammar. xo wrote this file with the
; grammar (D122). A keyword is an anonymous node with the name of the
; keyword in upper case. The patterns for the constants and the names come
; after the keywords, because the last pattern that captures a node counts,
; so a name gets the capture of its place, such as a function or a table.

(comment) @comment

[
  (string)
  (dollar_quoted_string)
] @string

(number) @number

(parameter) @variable.parameter

(identifier) @variable

(keyword) @keyword

[
  "ABSENT"
  "ALL"
  "AND"
  "ANY"
  "ARRAY"
  "AS"
  "ASC"
  "ASYMMETRIC"
  "AT"
  "BETWEEN"
  "BIT"
  "BOTH"
  "BREADTH"
  "BY"
  "CASE"
  "CAST"
  "CHAR"
  "CHARACTER"
  "COLLATE"
  "CONDITIONAL"
  "CROSS"
  "CUBE"
  "CURRENT"
  "CYCLE"
  "DAY"
  "DEFAULT"
  "DEPTH"
  "DESC"
  "DISTINCT"
  "DOCUMENT"
  "DOUBLE"
  "ELSE"
  "EMPTY"
  "ENCODING"
  "END"
  "ERROR"
  "ESCAPE"
  "EXCEPT"
  "EXCLUDE"
  "EXISTS"
  "FALSE"
  "FETCH"
  "FILTER"
  "FIRST"
  "FOLLOWING"
  "FOR"
  "FORMAT"
  "FROM"
  "FULL"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HAVING"
  "HOUR"
  "ILIKE"
  "IN"
  "INDENT"
  "INNER"
  "INTERSECT"
  "INTERVAL"
  "INTO"
  "IS"
  "ISNULL"
  "JOIN"
  "JSON"
  "KEEP"
  "KEY"
  "KEYS"
  "LAST"
  "LATERAL"
  "LEADING"
  "LEFT"
  "LIKE"
  "LIMIT"
  "LOCAL"
  "LOCKED"
  "MATERIALIZED"
  "MINUTE"
  "MONTH"
  "NATIONAL"
  "NATURAL"
  "NCHAR"
  "NEXT"
  "NFC"
  "NFD"
  "NFKC"
  "NFKD"
  "NO"
  "NORMALIZED"
  "NOT"
  "NOTNULL"
  "NOWAIT"
  "NULL"
  "NULLS"
  "OBJECT"
  "OF"
  "OFFSET"
  "OMIT"
  "ON"
  "ONLY"
  "OPERATOR"
  "OR"
  "ORDER"
  "ORDINALITY"
  "OTHERS"
  "OUTER"
  "OVER"
  "OVERLAPS"
  "PARTITION"
  "PASSING"
  "PLACING"
  "PRECEDING"
  "PRECISION"
  "QUOTES"
  "RANGE"
  "RECURSIVE"
  "REF"
  "REPEATABLE"
  "RETURNING"
  "RIGHT"
  "ROLLUP"
  "ROW"
  "ROWS"
  "SCALAR"
  "SEARCH"
  "SECOND"
  "SELECT"
  "SET"
  "SETS"
  "SHARE"
  "SIMILAR"
  "SKIP"
  "SOME"
  "STRING"
  "SYMMETRIC"
  "TABLE"
  "TABLESAMPLE"
  "TEMP"
  "TEMPORARY"
  "THEN"
  "TIES"
  "TIME"
  "TIMESTAMP"
  "TO"
  "TRAILING"
  "TRUE"
  "UNBOUNDED"
  "UNCONDITIONAL"
  "UNION"
  "UNIQUE"
  "UNKNOWN"
  "UNLOGGED"
  "UPDATE"
  "USING"
  "VALUE"
  "VALUES"
  "VARIADIC"
  "VARYING"
  "WHEN"
  "WHERE"
  "WINDOW"
  "WITH"
  "WITHIN"
  "WITHOUT"
  "WRAPPER"
  "YEAR"
  "ZONE"
] @keyword

(boolean
  _ @boolean)

(null
  _ @constant.builtin)

(type
  name: (identifier) @type.builtin)

(type
  schema: (identifier) @module)

(type
  [
    "ARRAY"
    "BIT"
    "CHAR"
    "CHARACTER"
    "DAY"
    "DOUBLE"
    "HOUR"
    "INTERVAL"
    "MINUTE"
    "MONTH"
    "NATIONAL"
    "NCHAR"
    "PRECISION"
    "SECOND"
    "TIME"
    "TIMESTAMP"
    "TO"
    "VARYING"
    "WITH"
    "WITHOUT"
    "YEAR"
    "ZONE"
  ] @type.builtin)

(typed_literal
  type: (type) @type.builtin)

(function_call
  name: (identifier) @function.call)

(function_call
  schema: (identifier) @module)

(object_reference
  database: (identifier) @module)

(object_reference
  schema: (identifier) @module)

(object_reference
  name: (identifier) @type)

(table_reference
  alias: (identifier) @variable)

(derived_table
  alias: (identifier) @variable)

(function_table
  alias: (identifier) @variable)

(parenthesized_join
  alias: (identifier) @variable)

(column_definition
  name: (identifier) @variable.member)

(cte
  name: (identifier) @type)

(window_definition
  name: (identifier) @type)

(over_clause
  window: (identifier) @type)

(column_reference
  database: (identifier) @module)

(column_reference
  schema: (identifier) @module)

(column_reference
  table: (identifier) @type)

(column_reference
  name: (identifier) @variable.member)

(all_columns) @punctuation.special

(all_columns
  schema: (identifier) @module)

(all_columns
  table: (identifier) @type)

(field_selection
  field: (identifier) @variable.member)

(aliased_expression
  alias: (identifier) @variable)

(named_argument
  name: (identifier) @variable.parameter)

(qualified_operator
  schema: (identifier) @module)

[
  (operator)
  "=>"
  ":="
] @operator

[
  "("
  ")"
  "["
  "]"
] @punctuation.bracket

[
  ";"
  ","
  "."
  ":"
  "::"
] @punctuation.delimiter

"*" @punctuation.special
