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
  "ABORT"
  "ABSENT"
  "ABSOLUTE"
  "ACCESS"
  "ALL"
  "ANALYSE"
  "ANALYZE"
  "AND"
  "ANY"
  "ARRAY"
  "AS"
  "ASC"
  "ASENSITIVE"
  "ASYMMETRIC"
  "AT"
  "AUTHORIZATION"
  "BACKWARD"
  "BEGIN"
  "BETWEEN"
  "BINARY"
  "BIT"
  "BOTH"
  "BREADTH"
  "BY"
  "CALL"
  "CASCADE"
  "CASE"
  "CAST"
  "CHAIN"
  "CHAR"
  "CHARACTER"
  "CHARACTERISTICS"
  "CHECKPOINT"
  "CLOSE"
  "CLUSTER"
  "COLLATE"
  "COMMIT"
  "COMMITTED"
  "CONCURRENTLY"
  "CONDITIONAL"
  "CONFLICT"
  "CONSTRAINT"
  "CONSTRAINTS"
  "CONTENT"
  "CONTINUE"
  "COPY"
  "CROSS"
  "CUBE"
  "CURRENT"
  "CURSOR"
  "CYCLE"
  "DATABASE"
  "DAY"
  "DEALLOCATE"
  "DECLARE"
  "DEFAULT"
  "DEFERRABLE"
  "DEFERRED"
  "DELETE"
  "DELIMITERS"
  "DEPTH"
  "DESC"
  "DISCARD"
  "DISTINCT"
  "DO"
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
  "EXCLUSIVE"
  "EXECUTE"
  "EXISTS"
  "EXPLAIN"
  "FALSE"
  "FETCH"
  "FILTER"
  "FIRST"
  "FOLLOWING"
  "FOR"
  "FORMAT"
  "FORWARD"
  "FREEZE"
  "FROM"
  "FULL"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HAVING"
  "HOLD"
  "HOUR"
  "IDENTITY"
  "ILIKE"
  "IMMEDIATE"
  "IN"
  "INDENT"
  "INDEX"
  "INNER"
  "INSENSITIVE"
  "INSERT"
  "INTERSECT"
  "INTERVAL"
  "INTO"
  "IS"
  "ISNULL"
  "ISOLATION"
  "JOIN"
  "JSON"
  "KEEP"
  "KEY"
  "KEYS"
  "LANGUAGE"
  "LAST"
  "LATERAL"
  "LEADING"
  "LEFT"
  "LEVEL"
  "LIKE"
  "LIMIT"
  "LISTEN"
  "LOAD"
  "LOCAL"
  "LOCK"
  "LOCKED"
  "MATCHED"
  "MATERIALIZED"
  "MERGE"
  "MINUTE"
  "MODE"
  "MONTH"
  "MOVE"
  "NAMES"
  "NATIONAL"
  "NATURAL"
  "NCHAR"
  "NEW"
  "NEXT"
  "NFC"
  "NFD"
  "NFKC"
  "NFKD"
  "NO"
  "NORMALIZED"
  "NOT"
  "NOTHING"
  "NOTIFY"
  "NOTNULL"
  "NOWAIT"
  "NULL"
  "NULLS"
  "OBJECT"
  "OF"
  "OFFSET"
  "OLD"
  "OMIT"
  "ON"
  "ONLY"
  "OPERATOR"
  "OPTION"
  "OR"
  "ORDER"
  "ORDINALITY"
  "OTHERS"
  "OUTER"
  "OVER"
  "OVERLAPS"
  "OVERRIDING"
  "PARTITION"
  "PASSING"
  "PLACING"
  "PLANS"
  "PRECEDING"
  "PRECISION"
  "PREPARE"
  "PREPARED"
  "PRIOR"
  "PROGRAM"
  "QUOTES"
  "RANGE"
  "READ"
  "RECURSIVE"
  "REF"
  "REINDEX"
  "RELATIVE"
  "RELEASE"
  "REPACK"
  "REPEATABLE"
  "RESET"
  "RESTART"
  "RESTRICT"
  "RETURNING"
  "RIGHT"
  "ROLE"
  "ROLLBACK"
  "ROLLUP"
  "ROW"
  "ROWS"
  "SAVEPOINT"
  "SCALAR"
  "SCHEMA"
  "SCROLL"
  "SEARCH"
  "SECOND"
  "SELECT"
  "SEQUENCES"
  "SERIALIZABLE"
  "SESSION"
  "SET"
  "SETS"
  "SHARE"
  "SHOW"
  "SIMILAR"
  "SKIP"
  "SNAPSHOT"
  "SOME"
  "SOURCE"
  "START"
  "STDIN"
  "STDOUT"
  "STRING"
  "SYMMETRIC"
  "SYSTEM"
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
  "TRANSACTION"
  "TRUE"
  "TRUNCATE"
  "UNBOUNDED"
  "UNCOMMITTED"
  "UNCONDITIONAL"
  "UNION"
  "UNIQUE"
  "UNKNOWN"
  "UNLISTEN"
  "UNLOGGED"
  "UPDATE"
  "USER"
  "USING"
  "VACUUM"
  "VALUE"
  "VALUES"
  "VARIADIC"
  "VARYING"
  "VERBOSE"
  "WHEN"
  "WHERE"
  "WINDOW"
  "WITH"
  "WITHIN"
  "WITHOUT"
  "WORK"
  "WRAPPER"
  "WRITE"
  "XML"
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

(column_list
  (identifier) @variable.member)

(assignment
  column: (identifier) @variable.member)

(assignment
  field: (identifier) @variable.member)

(index_element
  column: (identifier) @variable.member)

(insert_statement
  alias: (identifier) @variable)

(returning_clause
  alias: (identifier) @variable)

(setting_name
  (identifier) @variable.builtin)

(option
  name: (identifier) @property)

(copy_option
  name: (identifier) @property)

(prepare_statement
  name: (identifier) @function)

(execute_statement
  name: (identifier) @function)

(deallocate_statement
  name: (identifier) @function)

(savepoint_statement
  name: (identifier) @label)

(release_statement
  name: (identifier) @label)

(rollback_statement
  savepoint: (identifier) @label)

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
