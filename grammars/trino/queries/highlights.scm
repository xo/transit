; The highlights of the Trino grammar. xo wrote this file with the grammar
; (D127). A keyword is an anonymous node with the name of the keyword in
; upper case. The patterns for the constants and the names come after the
; keywords, because the last pattern that captures a node counts, so a name
; gets the capture of its place, such as a function or a table.

(comment) @comment

(string) @string

(unicode_string) @string

(dollar_string) @string

(binary_string) @string.special

(number) @number

(boolean) @boolean

(null) @constant.builtin

(current_value) @function.builtin

(parameter) @variable.parameter

(identifier) @variable

[
  "ABSENT"
  "ADD"
  "ADMIN"
  "AFTER"
  "ALL"
  "ALTER"
  "ANALYZE"
  "AND"
  "ANY"
  "ARRAY"
  "AS"
  "ASC"
  "ASYMMETRIC"
  "AT"
  "AUTHORIZATION"
  "AUTO"
  "BEGIN"
  "BERNOULLI"
  "BETWEEN"
  "BOTH"
  "BRANCH"
  "BRANCHES"
  "BY"
  "CALL"
  "CALLED"
  "CASCADE"
  "CASE"
  "CAST"
  "CATALOG"
  "CATALOGS"
  "COLUMN"
  "COLUMNS"
  "COMMENT"
  "COMMIT"
  "COMMITTED"
  "CONDITIONAL"
  "CORRESPONDING"
  "COUNT"
  "CREATE"
  "CROSS"
  "CUBE"
  "CURRENT"
  "CURRENT_ROLE"
  "DATA"
  "DAY"
  "DEALLOCATE"
  "DECLARE"
  "DEFAULT"
  "DEFINE"
  "DEFINER"
  "DELETE"
  "DENY"
  "DESC"
  "DESCRIBE"
  "DESCRIPTOR"
  "DETERMINISTIC"
  "DISTINCT"
  "DISTRIBUTED"
  "DO"
  "DROP"
  "ELSE"
  "ELSEIF"
  "EMPTY"
  "ENCODING"
  "END"
  "ERROR"
  "ESCAPE"
  "EXCEPT"
  "EXCLUDE"
  "EXCLUDING"
  "EXECUTE"
  "EXISTS"
  "EXPLAIN"
  "FAIL"
  "FAST"
  "FETCH"
  "FILTER"
  "FINAL"
  "FIRST"
  "FOLLOWING"
  "FOR"
  "FORMAT"
  "FORWARD"
  "FROM"
  "FULL"
  "FUNCTION"
  "FUNCTIONS"
  "GRACE"
  "GRANT"
  "GRANTED"
  "GRANTS"
  "GRAPHVIZ"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HAVING"
  "HOUR"
  "IF"
  "IGNORE"
  "IMMEDIATE"
  "IN"
  "INCLUDING"
  "INITIAL"
  "INLINE"
  "INNER"
  "INPUT"
  "INSERT"
  "INTERSECT"
  "INTERVAL"
  "INTO"
  "INVOKER"
  "IO"
  "IS"
  "ISOLATION"
  "ITERATE"
  "JOIN"
  "JSON"
  "JSON_TABLE"
  "KEEP"
  "KEY"
  "KEYS"
  "LANGUAGE"
  "LAST"
  "LATERAL"
  "LEADING"
  "LEAVE"
  "LEFT"
  "LEVEL"
  "LIKE"
  "LIMIT"
  "LOCAL"
  "LOGICAL"
  "LOOP"
  "MATCH"
  "MATCHED"
  "MATCHES"
  "MATCH_RECOGNIZE"
  "MATERIALIZED"
  "MEASURES"
  "MERGE"
  "MINUTE"
  "MONTH"
  "NATURAL"
  "NEAREST"
  "NESTED"
  "NEXT"
  "NO"
  "NONE"
  "NOT"
  "NULLS"
  "OBJECT"
  "OF"
  "OFFSET"
  "OMIT"
  "ON"
  "ONE"
  "ONLY"
  "OPTION"
  "OR"
  "ORDER"
  "ORDINALITY"
  "OTHERS"
  "OUTER"
  "OUTPUT"
  "OVER"
  "OVERFLOW"
  "PARTIAL"
  "PARTITION"
  "PASSING"
  "PAST"
  "PATH"
  "PATTERN"
  "PER"
  "PERIOD"
  "PERMUTE"
  "PIVOT"
  "PLAN"
  "PRECEDING"
  "PREPARE"
  "PRIVILEGES"
  "PROPERTIES"
  "PRUNE"
  "QUOTES"
  "RANGE"
  "READ"
  "RECURSIVE"
  "REFRESH"
  "RENAME"
  "REPEAT"
  "REPEATABLE"
  "REPLACE"
  "RESET"
  "RESPECT"
  "RESTRICT"
  "RETURN"
  "RETURNING"
  "RETURNS"
  "REVOKE"
  "RIGHT"
  "ROLE"
  "ROLES"
  "ROLLBACK"
  "ROLLUP"
  "ROW"
  "ROWS"
  "RUNNING"
  "SCALAR"
  "SCHEMA"
  "SCHEMAS"
  "SECOND"
  "SECURITY"
  "SEEK"
  "SELECT"
  "SERIALIZABLE"
  "SESSION"
  "SET"
  "SETS"
  "SHOW"
  "SIMPLE"
  "SKIP"
  "SOME"
  "STALE"
  "START"
  "STATS"
  "STRING"
  "SUBSET"
  "SYMMETRIC"
  "SYSTEM"
  "TABLE"
  "TABLES"
  "TABLESAMPLE"
  "TEXT"
  "THEN"
  "TIES"
  "TIME"
  "TIMESTAMP"
  "TO"
  "TRAILING"
  "TRANSACTION"
  "TRUNCATE"
  "TRY_CAST"
  "TYPE"
  "UNBOUNDED"
  "UNCOMMITTED"
  "UNCONDITIONAL"
  "UNION"
  "UNIQUE"
  "UNKNOWN"
  "UNMATCHED"
  "UNNEST"
  "UNTIL"
  "UPDATE"
  "USE"
  "USER"
  "USING"
  "VALIDATE"
  "VALUE"
  "VALUES"
  "VERBOSE"
  "VERSION"
  "VIEW"
  "WHEN"
  "WHERE"
  "WHILE"
  "WINDOW"
  "WITH"
  "WITHIN"
  "WITHOUT"
  "WORK"
  "WRAPPER"
  "WRITE"
  "YEAR"
  "ZONE"
] @keyword

(type
  name: (identifier) @type.builtin)

(type
  schema: (identifier) @module)

(typed_literal
  type: (type) @type.builtin)

(function_call
  name: (identifier) @function.call)

(function_call
  schema: (identifier) @module)

(function_call
  catalog: (identifier) @module)

(table_function_call
  name: (identifier) @function.call)

(table_function_call
  schema: (identifier) @module)

(method_call
  name: (identifier) @function.method.call)

(json_function
  name: (identifier) @function.builtin)

(cast_expression
  [
    "CAST"
    "TRY_CAST"
  ] @function.builtin)

(object_reference
  catalog: (identifier) @module)

(object_reference
  schema: (identifier) @module)

(object_reference
  name: (identifier) @type)

(table_reference
  alias: (identifier) @variable)

(derived_table
  alias: (identifier) @variable)

(unnest_table
  alias: (identifier) @variable)

(function_table
  alias: (identifier) @variable)

(json_table
  alias: (identifier) @variable)

(nearest_table
  alias: (identifier) @variable)

(parenthesized_join
  alias: (identifier) @variable)

(match_recognize_table
  alias: (identifier) @variable)

(pivot_table
  alias: (identifier) @variable)

(table_argument
  alias: (identifier) @variable)

(schema_reference
  [
    catalog: (identifier)
    schema: (identifier)
  ] @module)

(call_statement
  procedure: (object_reference
    name: (identifier) @function.call))

(column_definition
  [
    parent: (identifier)
    name: (identifier)
  ] @variable.member)

(assignment
  column: (identifier) @variable.member)

(like_clause
  table: (object_reference
    name: (identifier) @type))

(principal
  name: (identifier) @variable)

(set_statement
  name: (object_reference
    name: (identifier) @variable.builtin))

(reset_statement
  name: (object_reference
    name: (identifier) @variable.builtin))

(cte
  name: (identifier) @type)

(window_definition
  name: (identifier) @type)

(window_specification
  name: (identifier) @type)

(over_clause
  window: (identifier) @type)

(column_reference
  catalog: (identifier) @module)

(column_reference
  schema: (identifier) @module)

(column_reference
  table: (identifier) @type)

(column_reference
  name: (identifier) @variable.member)

(star_expression) @punctuation.special

(star_expression
  [
    catalog: (identifier)
    schema: (identifier)
  ] @module)

(star_expression
  table: (identifier) @type)

(aliased_expression
  alias: (identifier) @variable)

(aliased_argument
  alias: (identifier) @property)

(column_list
  (identifier) @variable.member)

(field_definition
  name: (identifier) @property)

(field_selection
  field: (identifier) @property)

(field_selection
  "*" @punctuation.special)

(subscript_expression
  "*" @punctuation.special)

(named_argument
  name: (identifier) @variable.parameter)

(named_table_argument
  name: (identifier) @variable.parameter)

(descriptor_field
  name: (identifier) @variable.member)

(json_table_column
  name: (identifier) @variable.member)

(json_path
  name: (identifier) @variable)

(json_format_alias
  alias: (identifier) @variable.parameter)

(json_passing
  (aliased_expression
    alias: (identifier) @variable.parameter))

(session_property
  name: (object_reference
    name: (identifier) @variable.builtin))

(property
  name: (identifier) @variable.parameter)

(measure_definition
  name: (identifier) @variable)

(measure_reference
  measure: (identifier) @variable)

(variable_definition
  variable: (identifier) @variable)

(subset_definition
  [
    name: (identifier)
    variable: (identifier)
  ] @variable)

(after_match_clause
  variable: (identifier) @variable)

(pattern_variable) @variable

(pattern_quantifier) @operator

(pattern_anchor) @operator

(pattern_alternation
  "|" @operator)

(pattern_exclusion
  [
    "{-"
    "-}"
  ] @punctuation.bracket)

(function_definition
  name: (object_reference
    name: (identifier) @function))

(parameter_declaration
  name: (identifier) @variable.parameter)

(declare_statement
  variable: (identifier) @variable)

(assignment_statement
  variable: (identifier) @variable)

(loop_statement
  label: (identifier) @label)

(while_statement
  label: (identifier) @label)

(repeat_statement
  label: (identifier) @label)

(iterate_statement
  label: (identifier) @label)

(leave_statement
  label: (identifier) @label)

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

(if_statement
  [
    "IF"
    "THEN"
    "END"
  ] @keyword.conditional)

(elseif_clause
  [
    "ELSEIF"
    "THEN"
  ] @keyword.conditional)

(else_clause
  "ELSE" @keyword.conditional)

(case_statement
  [
    "CASE"
    "END"
  ] @keyword.conditional)

(case_statement_when
  [
    "WHEN"
    "THEN"
  ] @keyword.conditional)

(loop_statement
  [
    "LOOP"
    "END"
  ] @keyword.repeat)

(while_statement
  [
    "WHILE"
    "DO"
    "END"
  ] @keyword.repeat)

(repeat_statement
  [
    "REPEAT"
    "UNTIL"
    "END"
  ] @keyword.repeat)

(iterate_statement
  "ITERATE" @keyword.repeat)

(leave_statement
  "LEAVE" @keyword.repeat)

(return_statement
  "RETURN" @keyword.return)

(operator) @operator

[
  "->"
  "=>"
] @operator

(session_property
  "=" @operator)

(property
  "=" @operator)

(assignment
  "=" @operator)

(set_statement
  "=" @operator)

"@" @punctuation.special

(assignment_statement
  "=" @operator)

(subset_definition
  "=" @operator)

[
  "("
  ")"
  "["
  "]"
] @punctuation.bracket

[
  ","
  "."
  ";"
  ":"
] @punctuation.delimiter
