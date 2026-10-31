; The highlights of the Oracle grammar. xo wrote this file with the grammar
; (D124). A keyword is an anonymous node with the name of the keyword in
; upper case. The patterns for the constants and the names come after the
; keywords, because the last pattern that captures a node counts, so a name
; gets the capture of its place, such as a function or a table.

(comment) @comment

(string) @string

(number) @number

(bind_variable) @variable.parameter

(identifier) @variable

(keyword) @keyword

[
  "A"
  "ABSENT"
  "ACCESS"
  "ADMIN"
  "AFTER"
  "ALL"
  "ALLOW"
  "ALTER"
  "ANALYZE"
  "AND"
  "ANY"
  "APPLY"
  "ARCHIVAL"
  "ARRAY"
  "AS"
  "ASC"
  "ASCII"
  "AT"
  "AUDIT"
  "AUTO"
  "AUTOMATIC"
  "BATCH"
  "BEFORE"
  "BETWEEN"
  "BLOCK"
  "BOTH"
  "BREADTH"
  "BULK"
  "BY"
  "BYTE"
  "CALL"
  "CASCADE"
  "CASE"
  "CHAR"
  "CHARACTER"
  "CHECK"
  "CLUSTER"
  "COLLATE"
  "COLLECT"
  "COLUMN"
  "COLUMNS"
  "COLUMN_VALUE"
  "COMMENT"
  "COMMIT"
  "COMMITTED"
  "CONDITIONAL"
  "CONNECT"
  "CONNECT_BY_ROOT"
  "CONSTRAINT"
  "CONSTRAINTS"
  "CONTAINER"
  "CONVERSION"
  "CORRUPT_XID"
  "CORRUPT_XID_ALL"
  "COST"
  "COUNT"
  "CREATE"
  "CUBE"
  "CURRENT"
  "CURSOR"
  "CYCLE"
  "DANGLING"
  "DATABASE"
  "DATE"
  "DAY"
  "DBA_RECYCLEBIN"
  "DECIMAL"
  "DECREMENT"
  "DEFAULT"
  "DEFERRED"
  "DEFINE"
  "DELEGATE"
  "DELETE"
  "DENSE_RANK"
  "DEPTH"
  "DESC"
  "DIMENSION"
  "DIRECTORY"
  "DISABLE"
  "DISALLOW"
  "DISTINCT"
  "DOUBLE"
  "DROP"
  "EDITION"
  "ELSE"
  "EMPTY"
  "ENABLE"
  "END"
  "ERROR"
  "ERRORS"
  "ESCAPE"
  "EXCEPT"
  "EXCLUDE"
  "EXCLUSIVE"
  "EXISTS"
  "EXPLAIN"
  "FALSE"
  "FETCH"
  "FINAL"
  "FIRST"
  "FLASHBACK"
  "FLOAT"
  "FOLLOWING"
  "FOR"
  "FORCE"
  "FOREVER"
  "FORMAT"
  "FROM"
  "FUNCTION"
  "GRANT"
  "GRAPH"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HAVING"
  "HIERARCHY"
  "HOUR"
  "IDENTIFIED"
  "IGNORE"
  "IMMEDIATE"
  "IN"
  "INCLUDE"
  "INCREMENT"
  "INDEX"
  "INDEXTYPE"
  "INDICATOR"
  "INFINITE"
  "INSERT"
  "INTEGER"
  "INTERSECT"
  "INTERVAL"
  "INTO"
  "IS"
  "ISOLATION"
  "ITERATE"
  "JAVA"
  "JOIN"
  "JSON"
  "JSON_TABLE"
  "KEEP"
  "KEYS"
  "LAST"
  "LATERAL"
  "LAX"
  "LEADING"
  "LEVEL"
  "LIKE"
  "LIKE2"
  "LIKE4"
  "LIKEC"
  "LIMIT"
  "LOCAL"
  "LOCK"
  "LOCKED"
  "LOG"
  "LONG"
  "MAIN"
  "MATCH"
  "MATCHED"
  "MATCHES"
  "MATCH_RECOGNIZE"
  "MATERIALIZED"
  "MEASURES"
  "MEMBER"
  "MERGE"
  "MICROSECONDS"
  "MILLISECONDS"
  "MINING"
  "MINUS"
  "MINUTE"
  "MISMATCH"
  "MODE"
  "MODEL"
  "MONTH"
  "MULTISET"
  "NAME"
  "NAN"
  "NATIONAL"
  "NAV"
  "NCHAR"
  "NESTED"
  "NESTED_TABLE_ID"
  "NEW"
  "NEXT"
  "NO"
  "NOAUDIT"
  "NOCYCLE"
  "NONE"
  "NOT"
  "NOWAIT"
  "NULL"
  "NULLS"
  "NUMBER"
  "OBJECT"
  "OF"
  "OFFLINE"
  "OFFSET"
  "OLD"
  "OMIT"
  "ON"
  "ONE"
  "ONLINE"
  "ONLY"
  "OPERATOR"
  "OPTION"
  "OR"
  "ORDER"
  "ORDINALITY"
  "OTHERS"
  "OVER"
  "OVERFLOW"
  "PACKAGE"
  "PARTITION"
  "PASSING"
  "PAST"
  "PATH"
  "PATTERN"
  "PER"
  "PERCENT"
  "PERIOD"
  "PERMUTE"
  "PIVOT"
  "PLAN"
  "PLUGGABLE"
  "POINT"
  "POLICY"
  "PRECEDING"
  "PRECISION"
  "PRESENT"
  "PRESERVE"
  "PRETTY"
  "PRIOR"
  "PROCEDURE"
  "PROFILE"
  "PROPERTY"
  "PUBLIC"
  "PURGE"
  "RANGE"
  "RAW"
  "READ"
  "RECYCLEBIN"
  "REF"
  "REFERENCE"
  "REJECT"
  "RENAME"
  "RESET"
  "RESETLOGS"
  "RESOURCE"
  "RESPECT"
  "RESTORE"
  "RETURN"
  "RETURNING"
  "REUSE"
  "REVOKE"
  "ROLE"
  "ROLLBACK"
  "ROLLUP"
  "ROW"
  "ROWID"
  "ROWNUM"
  "ROWS"
  "ROWTYPE"
  "RULES"
  "RUNNING"
  "SAMPLE"
  "SAVEPOINT"
  "SCALAR"
  "SCALARS"
  "SCHEMA"
  "SCN"
  "SEARCH"
  "SECOND"
  "SECONDS"
  "SEED"
  "SEGMENT"
  "SELECT"
  "SEQUENCE"
  "SEQUENTIAL"
  "SERIALIZABLE"
  "SESSION"
  "SET"
  "SETS"
  "SHARE"
  "SHOW"
  "SIBLINGS"
  "SINGLE"
  "SKIP"
  "SMALLINT"
  "SOME"
  "SOURCE"
  "SQL"
  "STANDBY"
  "START"
  "STATEMENT_ID"
  "STORAGE"
  "STRICT"
  "SUBMULTISET"
  "SUBPARTITION"
  "SUBSET"
  "SUCCESSFUL"
  "SYNONYM"
  "SYSDATE"
  "SYSTEM"
  "TABLE"
  "TABLESPACE"
  "THEN"
  "TIES"
  "TIME"
  "TIMESTAMP"
  "TO"
  "TRAILING"
  "TRANSACTION"
  "TRANSLATION"
  "TRIGGER"
  "TRIGGERS"
  "TRUE"
  "TRUNCATE"
  "TYPE"
  "UID"
  "UNBOUNDED"
  "UNCONDITIONAL"
  "UNION"
  "UNIQUE"
  "UNLIMITED"
  "UNMATCHED"
  "UNPIVOT"
  "UNTIL"
  "UPDATE"
  "UPDATED"
  "UPSERT"
  "USE"
  "USER"
  "USING"
  "VALIDATE"
  "VALUE"
  "VALUES"
  "VARCHAR"
  "VARCHAR2"
  "VARYING"
  "VERSIONS"
  "VIEW"
  "VISIBILITY"
  "WAIT"
  "WHEN"
  "WHENEVER"
  "WHERE"
  "WINDOW"
  "WITH"
  "WITHIN"
  "WITHOUT"
  "WORK"
  "WRAPPER"
  "WRITE"
  "XML"
  "XMLTABLE"
  "YEAR"
  "ZONE"
] @keyword

(boolean
  _ @boolean)

(null
  _ @constant.builtin)

(pseudocolumn
  _ @variable.builtin)

(type
  name: (identifier) @type.builtin)

(type
  schema: (identifier) @module)

(type
  [
    "CHAR"
    "CHARACTER"
    "DATE"
    "DECIMAL"
    "DOUBLE"
    "FLOAT"
    "INTEGER"
    "INTERVAL"
    "LOCAL"
    "LONG"
    "NATIONAL"
    "NCHAR"
    "NUMBER"
    "PRECISION"
    "RAW"
    "ROWID"
    "ROWTYPE"
    "SMALLINT"
    "TIME"
    "TIMESTAMP"
    "TYPE"
    "VARCHAR"
    "VARCHAR2"
    "VARYING"
    "WITH"
    "ZONE"
  ] @type.builtin)

(typed_literal
  type: _ @type.builtin)

(interval_qualifier
  [
    "DAY"
    "HOUR"
    "MINUTE"
    "MONTH"
    "SECOND"
    "TO"
    "YEAR"
  ] @type.builtin)

(function_call
  name: (identifier) @function.call)

(function_call
  package: (identifier) @module)

(function_call
  schema: (identifier) @module)

(object_reference
  schema: (identifier) @module)

(object_reference
  name: (identifier) @type)

(database_link
  (identifier) @module)

(table_reference
  alias: (identifier) @variable)

(derived_table
  alias: (identifier) @variable)

(collection_table
  alias: (identifier) @variable)

(parenthesized_join
  alias: (identifier) @variable)

(json_table
  alias: (identifier) @variable)

(xml_table
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
  schema: (identifier) @module)

(column_reference
  table: (identifier) @type)

(column_reference
  name: (identifier) @variable.member)

(outer_join) @operator

(all_columns) @punctuation.special

(all_columns
  schema: (identifier) @module)

(all_columns
  table: (identifier) @type)

(field_expression
  field: (identifier) @variable.member)

(aliased_expression
  alias: (identifier) @variable)

(measure
  alias: (identifier) @variable)

(named_argument
  name: (identifier) @variable.parameter)

(column_list
  (identifier) @variable.member)

(json_column
  name: (identifier) @variable.member)

(xml_column
  name: (identifier) @variable.member)

(cell_reference
  measure: (identifier) @variable.member)

(pattern_definition
  variable: (identifier) @label)

(skip_clause
  variable: (identifier) @label)

(subset
  name: (identifier) @label)

(row_pattern
  (identifier) @label)

(subquery_restriction
  constraint: (identifier) @label)

(search_clause
  column: (identifier) @variable.member)

(cycle_clause
  mark: (identifier) @variable.member)

(reference_model
  name: (identifier) @type)

(model_clause
  name: (identifier) @type)

(flashback_clause
  period: (identifier) @variable.member)

(partition_extension
  name: (identifier) @type)

(insert_into_clause
  alias: (identifier) @variable)

(update_statement
  alias: (identifier) @variable)

(delete_statement
  alias: (identifier) @variable)

(merge_statement
  alias: (identifier) @variable)

(assignment
  left: (identifier) @variable)

(where_clause
  cursor: (identifier) @variable)

(rollback_statement
  savepoint: (identifier) @label)

(savepoint_statement
  name: (identifier) @label)

(set_transaction_statement
  segment: (identifier) @type)

(parameter
  name: (identifier) @variable.builtin)

(purge_statement
  name: (identifier) @type)

(flashback_table_statement
  new_name: (identifier) @type)

(flashback_table_statement
  restore_point: (identifier) @label)

(flashback_database_statement
  name: (identifier) @type)

(flashback_database_statement
  restore_point: (identifier) @label)

(rename_statement
  name: (identifier) @type)

(rename_statement
  new_name: (identifier) @type)

(privilege
  (identifier) @keyword)

[
  (operator)
  "="
  "=>"
  "%"
] @operator

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
  "@"
  "/"
] @punctuation.delimiter
