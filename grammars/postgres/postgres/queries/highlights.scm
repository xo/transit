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
  "ADMIN"
  "AFTER"
  "AGGREGATE"
  "ALL"
  "ALSO"
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
  "ASSIGNMENT"
  "ASYMMETRIC"
  "AT"
  "ATOMIC"
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
  "CALLED"
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
  "CONNECT"
  "CONNECTION"
  "CONSTRAINT"
  "CONSTRAINTS"
  "CONTENT"
  "CONTINUE"
  "CONVERSION"
  "COPY"
  "COST"
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
  "DEFINER"
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
  "EACH"
  "ELSE"
  "EMPTY"
  "ENABLE"
  "ENCODING"
  "ENCRYPTED"
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
  "EXTERNAL"
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
  "FUNCTIONS"
  "GENERATED"
  "GLOBAL"
  "GRANT"
  "GRANTED"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HANDLER"
  "HASH"
  "HAVING"
  "HOLD"
  "HOUR"
  "IDENTITY"
  "IF"
  "ILIKE"
  "IMMEDIATE"
  "IMMUTABLE"
  "IMPLICIT"
  "IMPORT"
  "IN"
  "INCLUDE"
  "INCLUDING"
  "INCREMENT"
  "INDENT"
  "INDEX"
  "INHERIT"
  "INHERITS"
  "INITIALLY"
  "INLINE"
  "INNER"
  "INOUT"
  "INPUT"
  "INSENSITIVE"
  "INSERT"
  "INSTEAD"
  "INTERSECT"
  "INTERVAL"
  "INTO"
  "INVOKER"
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
  "LEAKPROOF"
  "LEFT"
  "LEVEL"
  "LIKE"
  "LIMIT"
  "LIST"
  "LISTEN"
  "LOAD"
  "LOCAL"
  "LOCATION"
  "LOCK"
  "LOCKED"
  "LOGGED"
  "LSN"
  "MAINTAIN"
  "MAPPING"
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
  "OBJECTS"
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
  "PARALLEL"
  "PARAMETER"
  "PARSER"
  "PARTIAL"
  "PARTITION"
  "PARTITIONS"
  "PASSING"
  "PERIOD"
  "PERMISSIVE"
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
  "PRIVILEGES"
  "PROCEDURAL"
  "PROCEDURE"
  "PROCEDURES"
  "PROGRAM"
  "PUBLICATION"
  "QUOTES"
  "RANGE"
  "READ"
  "REASSIGN"
  "RECURSIVE"
  "REF"
  "REFERENCES"
  "REFERENCING"
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
  "RESTRICTIVE"
  "RETURN"
  "RETURNING"
  "RETURNS"
  "REVOKE"
  "RIGHT"
  "ROLE"
  "ROLLBACK"
  "ROLLUP"
  "ROUTINE"
  "ROUTINES"
  "ROW"
  "ROWS"
  "RULE"
  "SAVEPOINT"
  "SCALAR"
  "SCHEMA"
  "SCHEMAS"
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
  "SETOF"
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
  "SQL"
  "STABLE"
  "START"
  "STATEMENT"
  "STATISTICS"
  "STDIN"
  "STDOUT"
  "STORAGE"
  "STORED"
  "STRICT"
  "STRING"
  "SUBSCRIPTION"
  "SUPPORT"
  "SYMMETRIC"
  "SYSTEM"
  "TABLE"
  "TABLES"
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
  "TRUSTED"
  "TYPE"
  "TYPES"
  "UNBOUNDED"
  "UNCOMMITTED"
  "UNCONDITIONAL"
  "UNION"
  "UNIQUE"
  "UNKNOWN"
  "UNLISTEN"
  "UNLOGGED"
  "UNTIL"
  "UPDATE"
  "USAGE"
  "USER"
  "USING"
  "VACUUM"
  "VALID"
  "VALIDATE"
  "VALIDATOR"
  "VALUE"
  "VALUES"
  "VARIADIC"
  "VARYING"
  "VERBOSE"
  "VERSION"
  "VIEW"
  "VIRTUAL"
  "VOLATILE"
  "WAIT"
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

(definition
  name: (identifier) @property)

(generic_option
  name: (identifier) @property)

(role_option
  name: (identifier) @property)

(create_operator_statement
  schema: (identifier) @module)

(operator_class_item
  schema: (identifier) @module)

(create_trigger_statement
  name: (identifier) @label)

(alter_trigger_statement
  name: (identifier) @label)

(create_event_trigger_statement
  name: (identifier) @label)

(alter_event_trigger_statement
  name: (identifier) @label)

(create_rule_statement
  name: (identifier) @label)

(alter_rule_statement
  name: (identifier) @label)

(create_policy_statement
  name: (identifier) @label)

(alter_policy_statement
  name: (identifier) @label)

(trigger_event
  column: (identifier) @variable.member)

(event_filter
  variable: (identifier) @variable.builtin)

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
