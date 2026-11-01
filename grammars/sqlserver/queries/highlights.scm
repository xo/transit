; The highlights of the SQL Server grammar. xo wrote this file with the
; grammar (D124). A keyword is an anonymous node with the name of the keyword
; in upper case. The patterns for the constants and the names come after the
; keywords, because the last pattern that captures a node counts, so a name
; gets the capture of its place, such as a function or a table.

(comment) @comment

(string) @string

[
  (number)
  (money)
  (binary)
] @number

(variable) @variable.parameter

(global_variable) @variable.builtin

(pseudocolumn) @variable.builtin

(identifier) @variable

(keyword) @keyword

[
  "ABSENT"
  "ABSOLUTE"
  "ACTION"
  "ADD"
  "AFTER"
  "AGGREGATE"
  "ALL"
  "ALL_SPARSE_COLUMNS"
  "ALTER"
  "ALWAYS"
  "AND"
  "ANY"
  "APPLICATION"
  "APPLY"
  "ARRAY"
  "AS"
  "ASC"
  "ASSEMBLY"
  "ASYMMETRIC"
  "AT"
  "AUDIT"
  "AUTHORIZATION"
  "AVAILABILITY"
  "BEFORE"
  "BEGIN"
  "BETWEEN"
  "BINDING"
  "BOTH"
  "BREAK"
  "BROKER"
  "BROWSE"
  "BULK"
  "BY"
  "CALLER"
  "CASCADE"
  "CASE"
  "CATALOG"
  "CATCH"
  "CERTIFICATE"
  "CHECK"
  "CLASSIFICATION"
  "CLASSIFIER"
  "CLOSE"
  "CLUSTERED"
  "COLLATE"
  "COLLECTION"
  "COLUMN"
  "COLUMNSTORE"
  "COLUMN_SET"
  "COMMIT"
  "COMMITTED"
  "CONFIGURATION"
  "CONNECTION"
  "CONSTRAINT"
  "CONTAINED"
  "CONTAINSTABLE"
  "CONTENT"
  "CONTINUE"
  "CONTRACT"
  "CONVERSATION"
  "CONVERT"
  "COOKIE"
  "COUNTER"
  "CREATE"
  "CREDENTIAL"
  "CRYPTOGRAPHIC"
  "CUBE"
  "CURRENT"
  "CURSOR"
  "DATA"
  "DATABASE"
  "DATA_SOURCE"
  "DATEFORMAT"
  "DEADLOCK_PRIORITY"
  "DEALLOCATE"
  "DECLARE"
  "DEFAULT"
  "DELAY"
  "DELETE"
  "DENY"
  "DESC"
  "DISTINCT"
  "DISTRIBUTED"
  "DOCUMENT"
  "DOUBLE"
  "DROP"
  "EDGE"
  "ELSE"
  "ENCRYPTED"
  "ENCRYPTION"
  "END"
  "ENDPOINT"
  "ESCAPE"
  "EVENT"
  "EXCEPT"
  "EXEC"
  "EXECUTE"
  "EXISTS"
  "EXTERNAL"
  "FETCH"
  "FILE"
  "FILESTREAM"
  "FILESTREAM_ON"
  "FILETABLE"
  "FILLFACTOR"
  "FIRST"
  "FOLLOWING"
  "FOR"
  "FOR SYSTEM_TIME"
  "FOREIGN"
  "FORMAT"
  "FREETEXTTABLE"
  "FROM"
  "FULLTEXT"
  "FUNCTION"
  "GENERATED"
  "GLOBAL"
  "GOTO"
  "GOVERNOR"
  "GRANT"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HASH"
  "HAVING"
  "HIDDEN"
  "HOLDLOCK"
  "IF"
  "IGNORE"
  "IN"
  "INCLUDE"
  "INDEX"
  "INSERT"
  "INTERSECT"
  "INTO"
  "IS"
  "ISOLATION"
  "JOB"
  "JOIN"
  "JSON"
  "KEY"
  "KEYS"
  "LANGUAGE"
  "LAST"
  "LEADING"
  "LEVEL"
  "LIBRARY"
  "LIKE"
  "LIST"
  "LOG"
  "LOGIN"
  "MARK"
  "MASKED"
  "MASTER"
  "MATCHED"
  "MERGE"
  "MESSAGE"
  "MODEL"
  "NAME"
  "NATIONAL"
  "NEXT"
  "NEXT VALUE FOR"
  "NO"
  "NOCHECK"
  "NODE"
  "NONCLUSTERED"
  "NONE"
  "NOT"
  "NOTIFICATION"
  "NOWAIT"
  "NULL"
  "NULLS"
  "OBJECT"
  "OF"
  "OFF"
  "OFFSET"
  "ON"
  "ONLY"
  "OPEN"
  "OPENDATASOURCE"
  "OPENQUERY"
  "OPENROWSET"
  "OPENXML"
  "OPTION"
  "OR"
  "ORDER"
  "OUT"
  "OUTPUT"
  "OVER"
  "PARTITION"
  "PERCENT"
  "PERIOD"
  "PERSISTED"
  "PIVOT"
  "POLICY"
  "POOL"
  "PRECEDING"
  "PRECISION"
  "PRIMARY"
  "PRINT"
  "PRIOR"
  "PRIORITY"
  "PROC"
  "PROCEDURE"
  "PROPERTY"
  "PROVIDER"
  "QUEUE"
  "RAISERROR"
  "RANGE"
  "READ"
  "READTEXT"
  "RECOMPILE"
  "REFERENCES"
  "RELATIVE"
  "REMOTE"
  "REPEATABLE"
  "REPLICATION"
  "RESOURCE"
  "RESPECT"
  "RESULT"
  "RETURN"
  "RETURNING"
  "REVERT"
  "REVOKE"
  "ROLE"
  "ROLLBACK"
  "ROLLUP"
  "ROUTE"
  "ROW"
  "ROWGUIDCOL"
  "ROWS"
  "RULE"
  "SAVE"
  "SCHEMA"
  "SCHEME"
  "SCOPED"
  "SEARCH"
  "SECONDARY"
  "SECURITY"
  "SELECT"
  "SELECTIVE"
  "SEMANTICKEYPHRASETABLE"
  "SEMANTICSIMILARITYDETAILSTABLE"
  "SEMANTICSIMILARITYTABLE"
  "SENSITIVITY"
  "SEQUENCE"
  "SEQUENCE_NUMBER"
  "SERIALIZABLE"
  "SERVER"
  "SERVICE"
  "SESSION"
  "SET"
  "SETERROR"
  "SETS"
  "SIGNATURE"
  "SNAPSHOT"
  "SOME"
  "SOURCE"
  "SPARSE"
  "SPATIAL"
  "SPECIFICATION"
  "START"
  "STATISTICS"
  "STOPLIST"
  "STREAM"
  "STREAMING"
  "SYMMETRIC"
  "SYNONYM"
  "SYSTEM"
  "TABLE"
  "TABLESAMPLE"
  "TARGET"
  "TEXTIMAGE_ON"
  "THEN"
  "THROW"
  "TIES"
  "TIME"
  "TO"
  "TOP"
  "TRAILING"
  "TRAN"
  "TRANSACTION"
  "TRANSACTION_ID"
  "TRIGGER"
  "TRUNCATE"
  "TRY"
  "TRY_CONVERT"
  "TYPE"
  "UNBOUNDED"
  "UNCOMMITTED"
  "UNDEFINED"
  "UNION"
  "UNIQUE"
  "UNPIVOT"
  "UPDATE"
  "UPDATETEXT"
  "USE"
  "USER"
  "USING"
  "VALUES"
  "VARYING"
  "VECTOR"
  "VIEW"
  "WAITFOR"
  "WHEN"
  "WHERE"
  "WHILE"
  "WINDOW"
  "WITH"
  "WITHIN"
  "WORK"
  "WORKLOAD"
  "WRAPPER"
  "WRITETEXT"
  "XML"
  "XMLNAMESPACES"
  "ZONE"
] @keyword

(go_statement
  "GO" @keyword.directive)

(null
  _ @constant.builtin)

(niladic_function) @function.builtin

(type
  name: (identifier) @type.builtin)

(type
  schema: (identifier) @module)

(type
  [
    "DOUBLE"
    "NATIONAL"
    "PRECISION"
    "VARYING"
  ] @type.builtin)

(function_call
  name: _ @function.call)

(function_call
  schema: (identifier) @module)

(function_call
  database: (identifier) @module)

(function_call
  type: (identifier) @type)

(convert_expression
  [
    "CONVERT"
    "TRY_CONVERT"
  ] @function.call)

(next_value_for
  sequence: (object_reference
    name: (identifier) @type))

(object_reference
  server: (identifier) @module)

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

(cte
  name: (identifier) @type)

(xmlnamespaces
  prefix: (identifier) @module)

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

(all_columns) @punctuation.special

(all_columns
  database: (identifier) @module)

(all_columns
  schema: (identifier) @module)

(all_columns
  table: (identifier) @type)

(field_expression
  field: (identifier) @variable.member)

(field_expression
  field: (identifier) @function.method.call
  (arguments))

(aliased_expression
  alias: (identifier) @variable)

(column_list
  (identifier) @variable.member)

(column_definition
  name: (identifier) @variable.member)

(into_clause
  filegroup: (identifier) @module)

(pivot_clause
  (identifier) @variable.member)

(unpivot_clause
  (identifier) @variable.member)

(collate_expression
  collation: (identifier) @variable.builtin)

(column_definition
  collation: (identifier) @variable.builtin)

(arguments
  model: (identifier) @type)

(if_statement
  [
    "IF"
    "ELSE"
  ] @keyword.conditional)

(while_statement
  "WHILE" @keyword.repeat)

(break_statement
  "BREAK" @keyword.repeat)

(continue_statement
  "CONTINUE" @keyword.repeat)

(return_statement
  "RETURN" @keyword.return)

(try_statement
  [
    "BEGIN"
    "TRY"
    "END"
  ] @keyword.exception)

(catch_block
  [
    "BEGIN"
    "CATCH"
    "END"
  ] @keyword.exception)

(throw_statement
  "THROW" @keyword.exception)

(raiserror_statement
  "RAISERROR" @keyword.exception)

(label) @label

(goto_statement
  label: (identifier) @label)

(execute_statement
  procedure: (object_reference
    name: (identifier) @function.call))

(execute_argument
  value: (identifier) @string)

(use_statement
  database: (identifier) @module)

(cursor_declaration
  name: (identifier) @variable)

[
  (open_statement
    cursor: (identifier) @variable)
  (fetch_statement
    cursor: (identifier) @variable)
  (close_statement
    cursor: (identifier) @variable)
  (deallocate_statement
    cursor: (identifier) @variable)
  (current_of_clause
    cursor: (identifier) @variable)
]

(table_constraint
  constraint: (identifier) @label)

(table_constraint
  column: (identifier) @variable.member)

[
  (create_index_statement
    name: (identifier) @label)
  (create_statistics_statement
    name: (identifier) @label)
  (alter_index_statement
    name: (identifier) @label)
  (update_statistics_statement
    statistics: (identifier) @label)
  (check_constraint_clause
    constraint: (identifier) @label)
]

[
  (create_table_statement
    filegroup: (identifier) @module)
  (create_index_statement
    filegroup: (identifier) @module)
  (create_statistics_statement
    filegroup: (identifier) @module)
  (table_index
    filegroup: (identifier) @module)
  (table_constraint
    filegroup: (identifier) @module)
  (column_definition
    filegroup: (identifier) @module)
]

[
  (create_table_statement
    column: (identifier) @variable.member)
  (create_index_statement
    column: (identifier) @variable.member)
  (create_statistics_statement
    column: (identifier) @variable.member)
  (table_index
    column: (identifier) @variable.member)
  (table_constraint
    column: (identifier) @variable.member)
  (column_definition
    column: (identifier) @variable.member)
  (create_fulltext_index_statement
    column: (identifier) @variable.member)
  (alter_column_clause
    name: (identifier) @variable.member)
  (drop_clause
    name: (identifier) @variable.member)
  (period_definition
    [
      start: (identifier)
      end: (identifier)
    ] @variable.member)
]

(create_schema_statement
  name: (identifier) @module)

[
  (create_schema_statement
    owner: (identifier) @variable)
  (grant_statement
    principal: (identifier) @variable)
  (deny_statement
    principal: (identifier) @variable)
  (revoke_statement
    principal: (identifier) @variable)
  (open_key_statement
    name: (identifier) @variable)
  (close_key_statement
    name: (identifier) @variable)
]

(table_index
  name: (identifier) @label)

(column_definition
  constraint: (identifier) @label)

(column_definition
  index: (identifier) @label)

(operator) @operator

[
  "="
  "::"
] @operator

[
  "("
  ")"
  "{"
  "}"
] @punctuation.bracket

[
  ","
  "."
  ";"
  ":"
] @punctuation.delimiter
