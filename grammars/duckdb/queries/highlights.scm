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
  "ABORT"
  "ACTION"
  "ADD"
  "AFTER"
  "ALIAS"
  "ALL"
  "ALTER"
  "ALWAYS"
  "ANALYSE"
  "ANALYZE"
  "AND"
  "ANY"
  "APPROX"
  "ARRAY"
  "AS"
  "ASC"
  "ASCENDING"
  "ASYMMETRIC"
  "AT"
  "ATTACH"
  "BEFORE"
  "BEGIN"
  "BETWEEN"
  "BIT"
  "BOTH"
  "BY"
  "CALL"
  "CASCADE"
  "CASE"
  "CAST"
  "CHAR"
  "CHARACTER"
  "CHECK"
  "CHECKPOINT"
  "COLLATE"
  "COLLATION"
  "COLUMN"
  "COMMENT"
  "COMMIT"
  "COMPRESSION"
  "CONFLICT"
  "CONNECT"
  "CONSTRAINT"
  "COPY"
  "CREATE"
  "CUBE"
  "CURRENT"
  "CYCLE"
  "DATA"
  "DATABASE"
  "DATABASES"
  "DEALLOCATE"
  "DEFAULT"
  "DELETE"
  "DESC"
  "DESCENDING"
  "DESCRIBE"
  "DESTROY"
  "DETACH"
  "DISCONNECT"
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
  "EXECUTE"
  "EXISTS"
  "EXPLAIN"
  "EXPORT"
  "EXPORT_STATE"
  "EXTENSION"
  "EXTENSIONS"
  "EXTERNAL"
  "FETCH"
  "FILTER"
  "FIRST"
  "FOLLOWING"
  "FOR"
  "FORCE"
  "FOREIGN"
  "FROM"
  "FULL"
  "FUNCTION"
  "GENERATED"
  "GLOB"
  "GLOBAL"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HAVING"
  "IF"
  "IGNORE"
  "ILIKE"
  "IMPORT"
  "IN"
  "INCLUDE"
  "INCREMENT"
  "INDEX"
  "INNER"
  "INSERT"
  "INSTALL"
  "INSTEAD"
  "INTERSECT"
  "INTERVAL"
  "INTO"
  "IS"
  "ISNULL"
  "JOIN"
  "KEY"
  "KEYS"
  "LAMBDA"
  "LAST"
  "LATERAL"
  "LEADING"
  "LIKE"
  "LIMIT"
  "LOAD"
  "LOCAL"
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
  "PRAGMA"
  "PRECEDING"
  "PRECISION"
  "PREFIX"
  "PREPARE"
  "PRESERVE"
  "PRIMARY"
  "PUBLIC"
  "QUALIFY"
  "RANGE"
  "READ"
  "RECURSIVE"
  "REFERENCES"
  "REFERENCING"
  "REGISTER"
  "RENAME"
  "REPEATABLE"
  "REPLACE"
  "REPOSITORY"
  "RESET"
  "RESOURCE"
  "RESOURCES"
  "RESPECT"
  "RESTART"
  "RESTRICT"
  "RETURNING"
  "ROLLBACK"
  "ROLLUP"
  "ROW"
  "ROWS"
  "SAMPLE"
  "SCHEMA"
  "SCHEMAS"
  "SECRET"
  "SECURE"
  "SELECT"
  "SEQUENCE"
  "SESSION"
  "SET"
  "SETOF"
  "SETS"
  "SHOW"
  "SIMILAR"
  "SIMILARITY"
  "SOME"
  "SORTED"
  "SOURCE"
  "START"
  "STATEMENT"
  "STORED"
  "SUMMARIZE"
  "SYMMETRIC"
  "TABLE"
  "TABLES"
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
  "TRANSACTION"
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
  "USE"
  "USING"
  "VACUUM"
  "VALUE"
  "VALUES"
  "VARIABLE"
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
  "WRITE"
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

(set_statement
  name: (identifier) @variable.builtin)

(reset_statement
  name: (identifier) @variable.builtin)

(pragma_statement
  name: (identifier) @function.builtin)

(pragma_value
  (identifier) @constant)

(show_statement
  name: (object_reference
    name: (identifier) @variable.builtin))

(use_statement
  name: (object_reference
    name: (identifier) @module))

(attach_statement
  alias: (identifier) @module)

(detach_statement
  name: (identifier) @module)

(connect_statement
  [
    database: (identifier)
    alias: (identifier)
  ] @module)

(checkpoint_statement
  database: (identifier) @module)

(export_statement
  database: (identifier) @module)

(copy_statement
  [
    source: (identifier)
    target: (identifier)
  ] @module)

(install_statement
  [
    extension: (identifier)
    repository: (identifier)
  ] @module)

(load_statement
  [
    extension: (identifier)
    repository: (identifier)
  ] @module)

(prepare_statement
  name: (identifier) @function)

(execute_statement
  name: (identifier) @function.call)

(deallocate_statement
  name: (identifier) @function)

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
