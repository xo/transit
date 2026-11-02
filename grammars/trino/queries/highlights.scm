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
  "AFTER"
  "ALL"
  "AND"
  "ANY"
  "ARRAY"
  "AS"
  "ASC"
  "ASYMMETRIC"
  "AT"
  "AUTO"
  "BEGIN"
  "BERNOULLI"
  "BETWEEN"
  "BOTH"
  "BY"
  "CALLED"
  "CASE"
  "CAST"
  "COLUMNS"
  "COMMENT"
  "CONDITIONAL"
  "CORRESPONDING"
  "COUNT"
  "CROSS"
  "CUBE"
  "CURRENT"
  "DAY"
  "DECLARE"
  "DEFAULT"
  "DEFINE"
  "DEFINER"
  "DESC"
  "DESCRIPTOR"
  "DETERMINISTIC"
  "DISTINCT"
  "DO"
  "ELSE"
  "ELSEIF"
  "EMPTY"
  "ENCODING"
  "END"
  "ERROR"
  "ESCAPE"
  "EXCEPT"
  "EXCLUDE"
  "EXISTS"
  "FETCH"
  "FILTER"
  "FINAL"
  "FIRST"
  "FOLLOWING"
  "FOR"
  "FORMAT"
  "FROM"
  "FULL"
  "FUNCTION"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HAVING"
  "HOUR"
  "IF"
  "IGNORE"
  "IN"
  "INITIAL"
  "INNER"
  "INPUT"
  "INTERSECT"
  "INTERVAL"
  "INVOKER"
  "IS"
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
  "LIKE"
  "LIMIT"
  "LOCAL"
  "LOOP"
  "MATCH"
  "MATCHES"
  "MATCH_RECOGNIZE"
  "MEASURES"
  "MINUTE"
  "MONTH"
  "NATURAL"
  "NEAREST"
  "NESTED"
  "NEXT"
  "NO"
  "NOT"
  "NULLS"
  "OBJECT"
  "OF"
  "OFFSET"
  "OMIT"
  "ON"
  "ONE"
  "ONLY"
  "OR"
  "ORDER"
  "ORDINALITY"
  "OTHERS"
  "OUTER"
  "OVER"
  "OVERFLOW"
  "PARTIAL"
  "PARTITION"
  "PASSING"
  "PAST"
  "PATH"
  "PATTERN"
  "PER"
  "PERMUTE"
  "PIVOT"
  "PLAN"
  "PRECEDING"
  "PRUNE"
  "QUOTES"
  "RANGE"
  "RECURSIVE"
  "REPEAT"
  "RESPECT"
  "RETURN"
  "RETURNING"
  "RETURNS"
  "RIGHT"
  "ROLLUP"
  "ROW"
  "ROWS"
  "RUNNING"
  "SCALAR"
  "SECOND"
  "SECURITY"
  "SEEK"
  "SELECT"
  "SESSION"
  "SET"
  "SETS"
  "SHOW"
  "SIMPLE"
  "SKIP"
  "SOME"
  "STRING"
  "SUBSET"
  "SYMMETRIC"
  "SYSTEM"
  "TABLE"
  "TABLESAMPLE"
  "THEN"
  "TIES"
  "TIME"
  "TIMESTAMP"
  "TO"
  "TRAILING"
  "TRUNCATE"
  "TRY_CAST"
  "UNBOUNDED"
  "UNCONDITIONAL"
  "UNION"
  "UNIQUE"
  "UNKNOWN"
  "UNMATCHED"
  "UNNEST"
  "UNTIL"
  "USING"
  "VALUE"
  "VALUES"
  "VERSION"
  "WHEN"
  "WHERE"
  "WHILE"
  "WINDOW"
  "WITH"
  "WITHIN"
  "WITHOUT"
  "WRAPPER"
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
