; The highlights of the DuckDB grammar. xo wrote this file with the grammar
; (D126). A keyword is an anonymous node with the name of the keyword in
; upper case. The patterns for the constants and the names come after the
; keywords, because the last pattern that captures a node counts, so a name
; gets the capture of its place, such as a function or a table.

(comment) @comment

(string) @string

(dollar_quoted_string) @string

(blob) @string.special

(bit_string) @string.special

(number) @number

(boolean) @boolean

(null) @constant.builtin

(parameter) @variable.parameter

(positional_reference) @variable.parameter

(identifier) @variable

[
  "ACTION"
  "ADD"
  "AFTER"
  "ALIAS"
  "ALL"
  "ALTER"
  "ALWAYS"
  "AND"
  "ANY"
  "APPROX"
  "ARRAY"
  "AS"
  "ASC"
  "ASCENDING"
  "ASYMMETRIC"
  "AT"
  "BEFORE"
  "BETWEEN"
  "BIT"
  "BOTH"
  "BY"
  "CASCADE"
  "CASE"
  "CAST"
  "CHAR"
  "CHARACTER"
  "CHECK"
  "COLLATE"
  "COLLATION"
  "COLUMN"
  "COMMIT"
  "COMPRESSION"
  "CONFLICT"
  "CONSTRAINT"
  "CREATE"
  "CUBE"
  "CURRENT"
  "CYCLE"
  "DATA"
  "DATABASE"
  "DEFAULT"
  "DELETE"
  "DESC"
  "DESCENDING"
  "DISTANCE"
  "DISTINCT"
  "DO"
  "DOUBLE"
  "DROP"
  "EACH"
  "ELSE"
  "END"
  "ENUM"
  "ERROR"
  "ESCAPE"
  "EXACT"
  "EXCEPT"
  "EXCLUDE"
  "EXISTS"
  "EXPORT_STATE"
  "FETCH"
  "FILTER"
  "FIRST"
  "FOLLOWING"
  "FOR"
  "FOREIGN"
  "FROM"
  "FUNCTION"
  "GENERATED"
  "GLOB"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HAVING"
  "IF"
  "IGNORE"
  "ILIKE"
  "IN"
  "INCLUDE"
  "INCREMENT"
  "INDEX"
  "INNER"
  "INSERT"
  "INSTEAD"
  "INTERSECT"
  "INTERVAL"
  "INTO"
  "IS"
  "ISNULL"
  "JOIN"
  "KEY"
  "LAMBDA"
  "LAST"
  "LATERAL"
  "LEADING"
  "LIKE"
  "LIMIT"
  "MACRO"
  "MAP"
  "MATCHED"
  "MATERIALIZED"
  "MAXVALUE"
  "MERGE"
  "MINVALUE"
  "NAME"
  "NATIONAL"
  "NCHAR"
  "NEAREST"
  "NEW"
  "NEXT"
  "NO"
  "NOT"
  "NOTHING"
  "NOTNULL"
  "NULLS"
  "OF"
  "OFFSET"
  "OLD"
  "ON"
  "ONLY"
  "OR"
  "ORDER"
  "ORDINALITY"
  "OTHERS"
  "OVER"
  "OWNED"
  "PARTITION"
  "PARTITIONED"
  "PERCENT"
  "PERSISTENT"
  "PIVOT"
  "PIVOT_LONGER"
  "PIVOT_WIDER"
  "PLACING"
  "POSITION"
  "PRECEDING"
  "PRECISION"
  "PRESERVE"
  "PRIMARY"
  "QUALIFY"
  "RANGE"
  "RECURSIVE"
  "REFERENCES"
  "REFERENCING"
  "RENAME"
  "REPEATABLE"
  "REPLACE"
  "RESET"
  "RESPECT"
  "RESTART"
  "RESTRICT"
  "RETURNING"
  "ROLLUP"
  "ROW"
  "ROWS"
  "SAMPLE"
  "SCHEMA"
  "SECRET"
  "SECURE"
  "SELECT"
  "SEQUENCE"
  "SET"
  "SETOF"
  "SETS"
  "SIMILAR"
  "SIMILARITY"
  "SOME"
  "SORTED"
  "SOURCE"
  "START"
  "STATEMENT"
  "STORED"
  "SYMMETRIC"
  "TABLE"
  "TABLESAMPLE"
  "TARGET"
  "TEMP"
  "TEMPORARY"
  "THEN"
  "TIES"
  "TIME"
  "TIMESTAMP"
  "TO"
  "TRAILING"
  "TRIGGER"
  "TRUNCATE"
  "TRY_CAST"
  "TYPE"
  "UNBOUNDED"
  "UNION"
  "UNIQUE"
  "UNKNOWN"
  "UNPIVOT"
  "UPDATE"
  "USING"
  "VALUE"
  "VALUES"
  "VARYING"
  "VERSION"
  "VIEW"
  "VIRTUAL"
  "WHEN"
  "WHERE"
  "WINDOW"
  "WITH"
  "WITHIN"
  "WITHOUT"
  "ZONE"
] @keyword

(type
  name: (identifier) @type.builtin)

(type
  schema: (identifier) @module)

(typed_literal
  type: (type) @type.builtin)

(interval_unit) @keyword

(function_call
  name: (identifier) @function.call)

(function_call
  schema: (identifier) @module)

(method_call
  name: (identifier) @function.method.call)

(columns_expression
  "*COLUMNS" @function.builtin)

(cast_expression
  [
    "CAST"
    "TRY_CAST"
  ] @function.builtin)

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

(pivot_table
  alias: (identifier) @variable)

(unpivot_table
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
  schema: (identifier) @module)

(column_reference
  table: (identifier) @type)

(column_reference
  name: (identifier) @variable.member)

(star_expression) @punctuation.special

(star_expression
  table: (identifier) @type)

(aliased_expression
  alias: (identifier) @variable)

(column_list
  (identifier) @variable.member)

(column_definition
  parent: (identifier) @variable.member)

(column_definition
  name: (identifier) @variable.member)

(macro_parameter
  name: (identifier) @variable.parameter)

(option
  name: (identifier) @variable.parameter)

(create_trigger_statement
  transition: (identifier) @variable)

(merge_when_clause
  "*" @punctuation.special)

(assignment
  "=" @operator)

(option
  "=" @operator)

(struct_field
  name: (identifier) @property)

(field_definition
  name: (identifier) @property)

(field_selection
  field: (identifier) @property)

(named_argument
  name: (identifier) @variable.parameter)

(lambda_expression
  parameter: (identifier) @variable.parameter)

(collate_expression
  collation: (object_reference
    name: (identifier) @variable.builtin))

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
  "::"
  "->"
  ":="
  "=>"
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
  ":"
] @punctuation.delimiter
