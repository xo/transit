; The highlights of the SQL Server grammar. xo wrote this file with the
; grammar (D124). A keyword is an anonymous node with the name of the keyword
; in upper case. The patterns for the constants and the names come after the
; keywords, because the last pattern that captures a node counts, so a name
; gets the capture of its place, such as a function or a table.

(comment) @comment

(string) @string

[
  (number)
  (money)
  (binary)
] @number

(variable) @variable.parameter

(global_variable) @variable.builtin

(pseudocolumn) @variable.builtin

(identifier) @variable

(keyword) @keyword

[
  "ABSENT"
  "ALL"
  "AND"
  "ANY"
  "APPLY"
  "ARRAY"
  "AS"
  "ASC"
  "AT"
  "BETWEEN"
  "BOTH"
  "BROWSE"
  "BULK"
  "BY"
  "CASE"
  "COLLATE"
  "CONTAINED"
  "CONTAINSTABLE"
  "CONTENT"
  "CONVERT"
  "CUBE"
  "CURRENT"
  "DEFAULT"
  "DESC"
  "DISTINCT"
  "DOCUMENT"
  "DOUBLE"
  "ELSE"
  "END"
  "ESCAPE"
  "EXCEPT"
  "EXISTS"
  "FETCH"
  "FIRST"
  "FOLLOWING"
  "FOR"
  "FOR SYSTEM_TIME"
  "FREETEXTTABLE"
  "FROM"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HAVING"
  "IGNORE"
  "IN"
  "INTERSECT"
  "INTO"
  "IS"
  "JOIN"
  "JSON"
  "LEADING"
  "LIKE"
  "MODEL"
  "NATIONAL"
  "NEXT"
  "NEXT VALUE FOR"
  "NOT"
  "NULL"
  "NULLS"
  "OF"
  "OFFSET"
  "ON"
  "ONLY"
  "OPENDATASOURCE"
  "OPENQUERY"
  "OPENROWSET"
  "OPENXML"
  "OPTION"
  "OR"
  "ORDER"
  "OVER"
  "PARTITION"
  "PERCENT"
  "PIVOT"
  "PRECEDING"
  "PRECISION"
  "RANGE"
  "REPEATABLE"
  "RESPECT"
  "RETURNING"
  "ROLLUP"
  "ROW"
  "ROWS"
  "SELECT"
  "SEMANTICKEYPHRASETABLE"
  "SEMANTICSIMILARITYDETAILSTABLE"
  "SEMANTICSIMILARITYTABLE"
  "SETS"
  "SOME"
  "SYSTEM"
  "TABLESAMPLE"
  "THEN"
  "TIES"
  "TIME"
  "TO"
  "TOP"
  "TRAILING"
  "TRY_CONVERT"
  "UNBOUNDED"
  "UNION"
  "UNPIVOT"
  "USE"
  "USING"
  "VALUES"
  "VARYING"
  "WHEN"
  "WHERE"
  "WINDOW"
  "WITH"
  "WITHIN"
  "WRAPPER"
  "XML"
  "XMLNAMESPACES"
  "ZONE"
] @keyword

(go_statement
  "GO" @keyword.directive)

(null
  _ @constant.builtin)

(niladic_function) @function.builtin

(type
  name: (identifier) @type.builtin)

(type
  schema: (identifier) @module)

(type
  [
    "DOUBLE"
    "NATIONAL"
    "PRECISION"
    "VARYING"
  ] @type.builtin)

(function_call
  name: _ @function.call)

(function_call
  schema: (identifier) @module)

(function_call
  database: (identifier) @module)

(function_call
  type: (identifier) @type)

(convert_expression
  [
    "CONVERT"
    "TRY_CONVERT"
  ] @function.call)

(next_value_for
  sequence: (object_reference
    name: (identifier) @type))

(object_reference
  server: (identifier) @module)

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

(cte
  name: (identifier) @type)

(xmlnamespaces
  prefix: (identifier) @module)

(window_definition
  name: (identifier) @type)

(window_specification
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
  database: (identifier) @module)

(all_columns
  schema: (identifier) @module)

(all_columns
  table: (identifier) @type)

(field_expression
  field: (identifier) @variable.member)

(field_expression
  field: (identifier) @function.method.call
  (arguments))

(aliased_expression
  alias: (identifier) @variable)

(column_list
  (identifier) @variable.member)

(column_definition
  name: (identifier) @variable.member)

(into_clause
  filegroup: (identifier) @module)

(pivot_clause
  (identifier) @variable.member)

(unpivot_clause
  (identifier) @variable.member)

(collate_expression
  collation: (identifier) @variable.builtin)

(column_definition
  collation: (identifier) @variable.builtin)

(arguments
  model: (identifier) @type)

(operator) @operator

[
  "="
  "::"
] @operator

[
  "("
  ")"
  "{"
  "}"
] @punctuation.bracket

[
  ","
  "."
  ";"
  ":"
] @punctuation.delimiter
