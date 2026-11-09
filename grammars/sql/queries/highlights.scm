; The highlights of the generic SQL grammar. xo wrote this file with the
; grammar (D143). A keyword is an anonymous node with the name of the
; keyword in upper case, and a word that the grammar reads as one token
; with other words, or as a word of a list of options, is the node keyword.
; The patterns for the constants and the names come after the keywords,
; because the last pattern that captures a node counts, so a name gets the
; capture of its place, such as a function or a table.

(comment) @comment

(string) @string

(number) @number

(boolean) @boolean

(null) @constant.builtin

(current_value) @function.builtin

(parameter) @variable.parameter

(stage_reference) @string.special.path

(identifier) @variable

(keyword) @keyword

(interval_unit) @keyword

(top_clause) @keyword

(operator) @operator

[
  "ADD"
  "ALL"
  "ALTER"
  "ALWAYS"
  "AND"
  "ANY"
  "APPLY"
  "ARRAY"
  "AS"
  "ASC"
  "ASYMMETRIC"
  "BEGIN"
  "BERNOULLI"
  "BETWEEN"
  "BLOCK"
  "BOTH"
  "BUCKET"
  "BY"
  "CALL"
  "CATALOG"
  "CHANGE"
  "CHECK"
  "CLONE"
  "COLLATE"
  "COLUMN"
  "COLUMNS"
  "COMMENT"
  "COMMIT"
  "CONSTRAINT"
  "COPY"
  "CREATE"
  "CUBE"
  "CURRENT"
  "DATABASE"
  "DEFAULT"
  "DELETE"
  "DESC"
  "DESCRIBE"
  "DISTINCT"
  "DROP"
  "ENFORCED"
  "ESCAPE"
  "EXCLUDE"
  "EXECUTE"
  "EXISTS"
  "EXPLAIN"
  "FETCH"
  "FILTER"
  "FIRST"
  "FOLLOWING"
  "FOR"
  "FOREIGN"
  "FORMAT"
  "FROM"
  "FULL"
  "FUNCTION"
  "GENERATED"
  "GRANT"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HAVING"
  "IDENTITY"
  "IF"
  "ILIKE"
  "IMMEDIATE"
  "IN"
  "INCLUDE"
  "INDEX"
  "INOUT"
  "INSERT"
  "INTERSECT"
  "INTERVAL"
  "INTO"
  "IS"
  "JOIN"
  "KEY"
  "LAST"
  "LATERAL"
  "LEADING"
  "LEFT"
  "LIKE"
  "LIMIT"
  "MATCHED"
  "MATCH_CONDITION"
  "MATERIALIZED"
  "MERGE"
  "MODIFY"
  "NAME"
  "NEXT"
  "NO"
  "NOT"
  "NULLS"
  "OF"
  "OFFSET"
  "ON"
  "ONLY"
  "OPTIONS"
  "OR"
  "ORDER"
  "ORDINALITY"
  "OTHERS"
  "OUT"
  "OUTER"
  "OVER"
  "OVERWRITE"
  "PARTITION"
  "PERCENT"
  "PIVOT"
  "PRECEDING"
  "PRIMARY"
  "PROCEDURE"
  "QUALIFY"
  "RANGE"
  "RECURSIVE"
  "REFERENCES"
  "REFRESH"
  "RENAME"
  "REPEATABLE"
  "REPLACE"
  "RESET"
  "RETURN"
  "RETURNS"
  "REVOKE"
  "RIGHT"
  "ROLE"
  "ROLES"
  "ROLLBACK"
  "ROLLUP"
  "ROW"
  "ROWS"
  "SAVEPOINT"
  "SCHEMA"
  "SECONDARY"
  "SEED"
  "SELECT"
  "SEQUENCE"
  "SESSION"
  "SET"
  "SETS"
  "SHOW"
  "SOME"
  "SOURCE"
  "START"
  "STRUCT"
  "SYMMETRIC"
  "SYSTEM"
  "SYSTEM_TIME"
  "TABLE"
  "TARGET"
  "TIES"
  "TIME"
  "TO"
  "TRAILING"
  "TRANSACTION"
  "TRUNCATE"
  "UNBOUNDED"
  "UNIQUE"
  "UNKNOWN"
  "UNNEST"
  "UNPIVOT"
  "UNSET"
  "UPDATE"
  "USE"
  "USING"
  "VALUE"
  "VALUES"
  "VAR"
  "VARIABLE"
  "VIEW"
  "WAREHOUSE"
  "WHERE"
  "WINDOW"
  "WITH"
  "WITHIN"
  "WITHOUT"
  "WORK"
  "ZONE"
]
@keyword

[
  "CASE"
  "WHEN"
  "THEN"
  "ELSE"
  "END"
] @keyword.conditional

(type
  name: (identifier) @type.builtin)

(type
  schema: (identifier) @module)

(typed_literal
  type: (type) @type.builtin)

(field_definition
  name: (identifier) @variable.member)

(function_call
  name: (identifier) @function.call)

(function_call
  [
    schema: (identifier)
    catalog: (identifier)
  ] @module)

(cast_expression
  name: (identifier) @function.builtin)

(object_reference
  [
    catalog: (identifier)
    schema: (identifier)
  ] @module)

(object_reference
  name: (identifier) @type)

(_
  alias: (identifier) @variable)

(_
  kind: (keyword) @keyword)

(call_statement
  procedure: (object_reference
    name: (identifier) @function.call))

(create_function_statement
  name: (object_reference
    name: (identifier) @function))

(parameter_declaration
  name: (identifier) @variable.parameter)

(column_definition
  name: (identifier) @variable.member)

(view_column
  name: (identifier) @variable.member)

(index_column
  name: (identifier) @variable.member)

(column_list
  (identifier) @variable.member)

(assignment
  column: (column_reference
    name: (identifier) @variable.member))

(named_argument
  name: (identifier) @variable.parameter)

(cte
  name: (identifier) @type)

(window_definition
  name: (identifier) @type)

(window_specification
  name: (identifier) @type)

(over_clause
  window: (identifier) @type)

(column_reference
  [
    catalog: (identifier)
    schema: (identifier)
  ] @module)

(column_reference
  table: (identifier) @type)

(column_reference
  name: (identifier) @variable.member)

(field_selection
  field: (identifier) @variable.member)

(json_access
  field: (identifier) @variable.member)

(star_expression) @punctuation.special

(star_expression
  schema: (identifier) @module)

(star_expression
  table: (identifier) @type)

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

[
  "="
  "=>"
  "->"
  "::"
  ":"
] @operator
