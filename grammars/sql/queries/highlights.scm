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
  "AFTER"
  "AGGREGATE"
  "ALL"
  "ALTER"
  "ALWAYS"
  "ANALYZE"
  "AND"
  "ANY"
  "ARRAY"
  "AS"
  "ASC"
  "ASSERT"
  "ASYMMETRIC"
  "BEGIN"
  "BERNOULLI"
  "BETWEEN"
  "BLOCK"
  "BOTH"
  "BUCKET"
  "BY"
  "CACHE"
  "CALL"
  "CATALOG"
  "CHANGE"
  "CHECK"
  "CLEAR"
  "CLONE"
  "CLOSE"
  "COLLATE"
  "COLUMN"
  "COLUMNS"
  "COMMENT"
  "COMMIT"
  "CONDITION"
  "CONNECT"
  "CONNECTION"
  "CONSTRAINT"
  "COPY"
  "CORRESPONDING"
  "CREATE"
  "CUBE"
  "CURRENT"
  "CURSOR"
  "DATA"
  "DATABASE"
  "DECLARE"
  "DEFAULT"
  "DEFINE"
  "DELETE"
  "DESC"
  "DESCRIBE"
  "DIRECTORY"
  "DISTINCT"
  "DIV"
  "DROP"
  "EMPTY"
  "ENFORCED"
  "ESCAPE"
  "EXCLUDE"
  "EXECUTE"
  "EXISTS"
  "EXIT"
  "EXPLAIN"
  "EXPORT"
  "EXTEND"
  "FETCH"
  "FILTER"
  "FINAL"
  "FIRST"
  "FOLLOWING"
  "FOR"
  "FOREIGN"
  "FORMAT"
  "FOUND"
  "FROM"
  "FUNCTION"
  "GENERATED"
  "GET"
  "GRANT"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HAVING"
  "IDENTITY"
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
  "KEY"
  "LAST"
  "LATERAL"
  "LEADING"
  "LET"
  "LIKE"
  "LIMIT"
  "LIST"
  "LOAD"
  "LOCAL"
  "LOCATION"
  "LOCKED"
  "LS"
  "MATCH"
  "MATCHED"
  "MATCHES"
  "MATCH_CONDITION"
  "MATCH_RECOGNIZE"
  "MATERIALIZED"
  "MEASURES"
  "MERGE"
  "MODIFY"
  "MSCK"
  "NAME"
  "NEXT"
  "NO"
  "NOCYCLE"
  "NONE"
  "NOT"
  "NOWAIT"
  "NULLS"
  "OF"
  "OFFSET"
  "OMIT"
  "ON"
  "ONE"
  "ONLY"
  "OPEN"
  "OPTIMIZE"
  "OPTIONS"
  "OR"
  "ORDER"
  "ORDINALITY"
  "OTHERS"
  "OUT"
  "OVER"
  "OVERWRITE"
  "PARTITION"
  "PAST"
  "PATTERN"
  "PER"
  "PERCENT"
  "PIVOT"
  "PRECEDING"
  "PRIMARY"
  "PROCEDURE"
  "PUT"
  "QUALIFY"
  "RANGE"
  "RECURSIVE"
  "REFERENCES"
  "REFRESH"
  "REMOVE"
  "RENAME"
  "REPEATABLE"
  "REPLACE"
  "RESET"
  "RESULTSET"
  "RETURNS"
  "REVERSE"
  "REVOKE"
  "RM"
  "ROLE"
  "ROLES"
  "ROLLBACK"
  "ROLLUP"
  "ROW"
  "ROWS"
  "RUNNING"
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
  "SKIP"
  "SOME"
  "SOURCE"
  "SQLSTATE"
  "START"
  "STRICT"
  "STRUCT"
  "SUBSET"
  "SYMMETRIC"
  "SYSTEM"
  "TABLE"
  "TARGET"
  "TIES"
  "TIME"
  "TO"
  "TRAILING"
  "TRANSACTION"
  "TRUNCATE"
  "UNBOUNDED"
  "UNCACHE"
  "UNDROP"
  "UNIQUE"
  "UNKNOWN"
  "UNMATCHED"
  "UNNEST"
  "UNPIVOT"
  "UNSET"
  "UPDATE"
  "USE"
  "USING"
  "VACUUM"
  "VALUE"
  "VALUES"
  "VAR"
  "VARIABLE"
  "VIEW"
  "WAIT"
  "WAREHOUSE"
  "WHERE"
  "WINDOW"
  "WITH"
  "WITHIN"
  "WITHOUT"
  "WORK"
  "ZONE"
  "ZORDER"
]
@keyword

[
  "CASE"
  "WHEN"
  "THEN"
  "ELSE"
  "ELSEIF"
  "END"
  "IF"
] @keyword.conditional

[
  "WHILE"
  "REPEAT"
  "UNTIL"
  "BREAK"
  "CONTINUE"
  "LEAVE"
  "ITERATE"
] @keyword.repeat

[
  "RETURN"
] @keyword.return

[
  "EXCEPTION"
  "RAISE"
  "SIGNAL"
  "RESIGNAL"
  "HANDLER"
] @keyword.exception

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

(table_version) @string.special

(_
  label: (identifier) @label)

(_
  end_label: (identifier) @label)

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
  ":="
  "|>"
  "::"
  ":"
] @operator
