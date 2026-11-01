; The highlights of the ClickHouse grammar. xo wrote this file with the
; grammar (D127). A keyword is an anonymous node with the name of the
; keyword in upper case. The patterns for the constants and the names come
; after the keywords, because the last pattern that captures a node counts,
; so a name gets the capture of its place, such as a function or a table.

(comment) @comment

(string) @string

(heredoc_string) @string

(hex_string) @string.special

(bit_string) @string.special

(number) @number

(boolean) @boolean

(null) @constant.builtin

(identifier) @variable

[
  "ALL"
  "ANALYZE"
  "AND"
  "ANTI"
  "ANY"
  "APPEND"
  "APPLY"
  "ARRAY"
  "AS"
  "ASC"
  "ASCENDING"
  "ASOF"
  "AST"
  "BETWEEN"
  "BINARY"
  "BOTH"
  "BY"
  "CASE"
  "CAST"
  "CHAR"
  "CHARACTER"
  "COLLATE"
  "COLUMNS"
  "COMPRESSION"
  "CROSS"
  "CUBE"
  "CURRENT"
  "DESC"
  "DESCENDING"
  "DISTINCT"
  "DIV"
  "DOUBLE"
  "ELSE"
  "END"
  "ESCAPE"
  "ESTIMATE"
  "EXCEPT"
  "EXISTS"
  "EXPLAIN"
  "FETCH"
  "FILL"
  "FILTER"
  "FINAL"
  "FIRST"
  "FOLLOWING"
  "FOR"
  "FORMAT"
  "FROM"
  "FULL"
  "GLOBAL"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HAVING"
  "IGNORE"
  "ILIKE"
  "IN"
  "INNER"
  "INTERPOLATE"
  "INTERSECT"
  "INTERVAL"
  "INTO"
  "IS"
  "JOIN"
  "LARGE"
  "LAST"
  "LEADING"
  "LEFT"
  "LEVEL"
  "LIKE"
  "LIMIT"
  "LOCAL"
  "MATERIALIZED"
  "MOD"
  "NATIONAL"
  "NATURAL"
  "NCHAR"
  "NEXT"
  "NOT"
  "NULLS"
  "OBJECT"
  "OFFSET"
  "ON"
  "ONLY"
  "OR"
  "ORDER"
  "OUTER"
  "OUTFILE"
  "OVER"
  "OVERRIDE"
  "PARTITION"
  "PASTE"
  "PIPELINE"
  "PLACING"
  "PLAN"
  "PRECEDING"
  "PRECISION"
  "PREWHERE"
  "QUALIFY"
  "QUERY"
  "RANGE"
  "RECURSIVE"
  "REGEXP"
  "REPLACE"
  "RESPECT"
  "RIGHT"
  "ROLLUP"
  "ROW"
  "ROWS"
  "SAMPLE"
  "SELECT"
  "SEMI"
  "SETS"
  "SETTINGS"
  "SIGNED"
  "SIMILAR"
  "SKIP"
  "STALENESS"
  "STDOUT"
  "STEP"
  "STRICT"
  "SYNTAX"
  "TABLE"
  "THEN"
  "TIES"
  "TO"
  "TOP"
  "TOTALS"
  "TRAILING"
  "TRANSACTION"
  "TREE"
  "TRUNCATE"
  "UNBOUNDED"
  "UNION"
  "UNKNOWN"
  "UNSIGNED"
  "USING"
  "VARYING"
  "WHEN"
  "WHERE"
  "WINDOW"
  "WITH"
] @keyword

(type
  name: (identifier) @type.builtin)

(typed_literal
  type: (type) @type.builtin)

(subcolumn_type
  type: (type
    name: (identifier) @type.builtin))

(interval_unit) @keyword

(query_parameter
  name: (identifier) @variable.parameter)

(function_call
  name: (identifier) @function.call)

(columns_expression
  "COLUMNS" @function.builtin)

(cast_expression
  "CAST" @function.builtin)

(object_reference
  database: (identifier) @module)

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

(cte
  name: (identifier) @type)

(window_definition
  name: (identifier) @type)

(window_specification
  name: (identifier) @type)

(over_clause
  window: (identifier) @type)

(column_reference
  database: (identifier) @module)

(column_reference
  table: (identifier) @type)

(column_reference
  name: (identifier) @variable.member)

(star_expression
  "*" @punctuation.special)

(star_expression
  table: (identifier) @type)

(star_expression
  database: (identifier) @module)

(columns_expression
  table: (identifier) @type)

(aliased_expression
  alias: (identifier) @variable)

(column_list
  (identifier) @variable.member)

(join
  (identifier) @variable.member)

(interpolate_item
  column: (identifier) @variable.member)

(field_definition
  name: (identifier) @property)

(field_selection
  field: (identifier) @property)

(path_prefix
  name: (identifier) @property)

(map_entry
  key: (_) @property)

(type_setting
  name: (identifier) @variable.parameter)

(enum_value
  name: (string) @constant)

(lambda_expression
  parameters: (column_reference
    name: (identifier) @variable.parameter))

(lambda_expression
  parameters: (tuple
    (column_reference
      name: (identifier) @variable.parameter)))

(setting
  name: (identifier) @variable.builtin)

(setting
  "=" @operator)

(type_setting
  "=" @operator)

(enum_value
  "=" @operator)

(format_clause
  format: (identifier) @type.builtin)

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

(ternary_expression
  [
    "?"
    ":"
  ] @keyword.conditional.ternary)

(operator) @operator

[
  "::"
  "->"
] @operator

[
  "("
  ")"
  "["
  "]"
  "{"
  "}"
] @punctuation.bracket

[
  ","
  "."
  ";"
] @punctuation.delimiter
