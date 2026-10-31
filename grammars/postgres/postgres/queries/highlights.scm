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
  "ACTION"
  "ADD"
  "AFTER"
  "AGGREGATE"
  "ALL"
  "ALTER"
  "ALWAYS"
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
  "ATTACH"
  "ATTRIBUTE"
  "AUTHORIZATION"
  "BACKWARD"
  "BEFORE"
  "BEGIN"
  "BETWEEN"
  "BINARY"
  "BIT"
  "BOTH"
  "BREADTH"
  "BY"
  "CACHE"
  "CALL"
  "CASCADE"
  "CASCADED"
  "CASE"
  "CAST"
  "CHAIN"
  "CHAR"
  "CHARACTER"
  "CHARACTERISTICS"
  "CHECK"
  "CHECKPOINT"
  "CLASS"
  "CLOSE"
  "CLUSTER"
  "COLLATE"
  "COLLATION"
  "COLUMN"
  "COMMENT"
  "COMMIT"
  "COMMITTED"
  "COMPRESSION"
  "CONCURRENTLY"
  "CONDITIONAL"
  "CONFIGURATION"
  "CONFLICT"
  "CONSTRAINT"
  "CONSTRAINTS"
  "CONTENT"
  "CONTINUE"
  "CONVERSION"
  "COPY"
  "CREATE"
  "CROSS"
  "CUBE"
  "CURRENT"
  "CURSOR"
  "CYCLE"
  "DATA"
  "DATABASE"
  "DAY"
  "DEALLOCATE"
  "DECLARE"
  "DEFAULT"
  "DEFERRABLE"
  "DEFERRED"
  "DELETE"
  "DELIMITERS"
  "DEPENDS"
  "DEPTH"
  "DESC"
  "DETACH"
  "DICTIONARY"
  "DISABLE"
  "DISCARD"
  "DISTINCT"
  "DO"
  "DOCUMENT"
  "DOMAIN"
  "DOUBLE"
  "DROP"
  "ELSE"
  "EMPTY"
  "ENABLE"
  "ENCODING"
  "END"
  "ENFORCED"
  "ENUM"
  "ERROR"
  "ESCAPE"
  "EVENT"
  "EXCEPT"
  "EXCLUDE"
  "EXCLUDING"
  "EXCLUSIVE"
  "EXECUTE"
  "EXISTS"
  "EXPLAIN"
  "EXPRESSION"
  "EXTENSION"
  "FALSE"
  "FAMILY"
  "FETCH"
  "FILTER"
  "FINALIZE"
  "FIRST"
  "FOLLOWING"
  "FOR"
  "FORCE"
  "FOREIGN"
  "FORMAT"
  "FORWARD"
  "FREEZE"
  "FROM"
  "FULL"
  "FUNCTION"
  "GENERATED"
  "GLOBAL"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HASH"
  "HAVING"
  "HOLD"
  "HOUR"
  "IDENTITY"
  "IF"
  "ILIKE"
  "IMMEDIATE"
  "IN"
  "INCLUDE"
  "INCLUDING"
  "INCREMENT"
  "INDENT"
  "INDEX"
  "INHERIT"
  "INHERITS"
  "INITIALLY"
  "INNER"
  "INOUT"
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
  "LABEL"
  "LANGUAGE"
  "LARGE"
  "LAST"
  "LATERAL"
  "LEADING"
  "LEFT"
  "LEVEL"
  "LIKE"
  "LIMIT"
  "LIST"
  "LISTEN"
  "LOAD"
  "LOCAL"
  "LOCK"
  "LOCKED"
  "LOGGED"
  "MATCH"
  "MATCHED"
  "MATERIALIZED"
  "MAXVALUE"
  "MERGE"
  "METHOD"
  "MINUTE"
  "MINVALUE"
  "MODE"
  "MONTH"
  "MOVE"
  "NAME"
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
  "OIDS"
  "OLD"
  "OMIT"
  "ON"
  "ONLY"
  "OPERATOR"
  "OPTION"
  "OPTIONS"
  "OR"
  "ORDER"
  "ORDINALITY"
  "OTHERS"
  "OUT"
  "OUTER"
  "OVER"
  "OVERLAPS"
  "OVERRIDING"
  "OWNED"
  "OWNER"
  "PARSER"
  "PARTIAL"
  "PARTITION"
  "PARTITIONS"
  "PASSING"
  "PERIOD"
  "PLACING"
  "PLANS"
  "POLICY"
  "PRECEDING"
  "PRECISION"
  "PREPARE"
  "PREPARED"
  "PRESERVE"
  "PRIMARY"
  "PRIOR"
  "PROCEDURAL"
  "PROCEDURE"
  "PROGRAM"
  "PUBLICATION"
  "QUOTES"
  "RANGE"
  "READ"
  "RECURSIVE"
  "REF"
  "REFERENCES"
  "REFRESH"
  "REINDEX"
  "RELATIVE"
  "RELEASE"
  "RENAME"
  "REPACK"
  "REPEATABLE"
  "REPLACE"
  "REPLICA"
  "RESET"
  "RESTART"
  "RESTRICT"
  "RETURNING"
  "RIGHT"
  "ROLE"
  "ROLLBACK"
  "ROLLUP"
  "ROUTINE"
  "ROW"
  "ROWS"
  "RULE"
  "SAVEPOINT"
  "SCALAR"
  "SCHEMA"
  "SCROLL"
  "SEARCH"
  "SECOND"
  "SECURITY"
  "SELECT"
  "SEQUENCE"
  "SEQUENCES"
  "SERIALIZABLE"
  "SERVER"
  "SESSION"
  "SET"
  "SETS"
  "SHARE"
  "SHOW"
  "SIMILAR"
  "SIMPLE"
  "SKIP"
  "SNAPSHOT"
  "SOME"
  "SOURCE"
  "SPLIT"
  "START"
  "STATISTICS"
  "STDIN"
  "STDOUT"
  "STORAGE"
  "STORED"
  "STRING"
  "SUBSCRIPTION"
  "SYMMETRIC"
  "SYSTEM"
  "TABLE"
  "TABLESAMPLE"
  "TABLESPACE"
  "TARGET"
  "TEMP"
  "TEMPLATE"
  "TEMPORARY"
  "TEXT"
  "THEN"
  "TIES"
  "TIME"
  "TIMESTAMP"
  "TO"
  "TRAILING"
  "TRANSACTION"
  "TRANSFORM"
  "TRIGGER"
  "TRUE"
  "TRUNCATE"
  "TYPE"
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
  "VALID"
  "VALIDATE"
  "VALUE"
  "VALUES"
  "VARIADIC"
  "VARYING"
  "VERBOSE"
  "VERSION"
  "VIEW"
  "VIRTUAL"
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

(column_constraint
  name: (identifier) @label)

(table_constraint
  name: (identifier) @label)

(table_constraint
  column: (identifier) @variable.member)

(table_constraint
  index: (identifier) @type)

(alter_column
  column: (identifier) @variable.member)

(drop_column
  column: (identifier) @variable.member)

(rename
  column: (identifier) @variable.member)

(rename
  constraint: (identifier) @label)

(alter_constraint
  constraint: (identifier) @label)

(drop_constraint
  constraint: (identifier) @label)

(alter_table_option
  constraint: (identifier) @label)

(alter_domain_statement
  constraint: (identifier) @label)

(create_index_statement
  name: (identifier) @type)

(create_schema_statement
  name: (identifier) @module)

(alter_schema_statement
  name: (identifier) @module)

(set_schema
  schema: (identifier) @module)

(create_extension_statement
  schema: (identifier) @module)

(create_database_statement
  name: (identifier) @module)

(alter_database_statement
  name: (identifier) @module)

(database_option
  name: (identifier) @property)

(option
  prefix: (identifier) @property)

(alter_type_statement
  attribute: (identifier) @variable.member)

(function_signature
  name: (object_reference
    name: (identifier) @function))

(function_parameter
  name: (identifier) @variable.parameter)

(operator_signature
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
