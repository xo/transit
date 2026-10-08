; xo wrote this file, because the repository of the grammar has no highlight
; query (D113). It uses only the capture names of styles/captures.txt.
;
; The grammar gives most names and literals as anonymous nodes, such as
; "table" and "column", and it hides the literals of a constant in tokens
; that a query cannot capture. A predicate on the text of a constant chooses
; its color. No two patterns capture the same node.

; Keywords

[
  "ADD"
  "AGGREGATE"
  "ALL"
  "ALLOW"
  "ALTER"
  "APPLY"
  "AS"
  "ASC"
  "AUTHORIZE"
  "BATCH"
  "BEGIN"
  "BY"
  "CALLED"
  "CLUSTERING"
  "COMPACT"
  "CONTAINS"
  "CREATE"
  "DELETE"
  "DESC"
  "DESCRIBE"
  "DISTINCT"
  "DROP"
  "DURABLE_WRITES"
  "ENTRIES"
  "EXECUTE"
  "EXISTS"
  "FILTERING"
  "FINALFUNC"
  "FROM"
  "FULL"
  "FUNCTION"
  "FUNCTIONS"
  "GRANT"
  "IF"
  "INDEX"
  "INITCOND"
  "INPUT"
  "INSERT"
  "INTO"
  "IS"
  "JSON"
  "KEY"
  "KEYS"
  "KEYSPACE"
  "KEYSPACES"
  "LANGUAGE"
  "LIMIT"
  "LOGGED"
  "LOGIN"
  "MATERIALIZED"
  "MODIFY"
  "NORECURSIVE"
  "NOSUPERUSER"
  "OF"
  "ON"
  "OPTIONS"
  "ORDER"
  "PASSWORD"
  "PERMISSIONS"
  "PRIMARY"
  "RENAME"
  "REPLACE"
  "REPLICATION"
  "RETURNS"
  "REVOKE"
  "ROLE"
  "ROLES"
  "SELECT"
  "SFUNC"
  "STORAGE"
  "STYPE"
  "SUPERUSER"
  "TABLE"
  "TO"
  "TRIGGER"
  "TRUNCATE"
  "TTL"
  "TYPE"
  "UNLOGGED"
  "UPDATE"
  "USE"
  "USER"
  "USING"
  "VALUES"
  "VIEW"
  "WHERE"
  "WITH"
] @keyword

[
  "AND"
  "OR"
  "NOT"
  "IN"
] @keyword.operator

; A word that is a type in a data type and a keyword elsewhere

(begin_batch
  "COUNTER" @keyword)

(list_permissions
  "LIST" @keyword)

(list_roles
  "LIST" @keyword)

(update_assignments
  "SET" @keyword)

(using_timestamp_spec
  "TIMESTAMP" @keyword)

(using_ttl_timestamp
  "TIMESTAMP" @keyword)

; Types

(data_type_name
  [
    "ASCII"
    "BIGINT"
    "BLOB"
    "BOOLEAN"
    "COUNTER"
    "DATE"
    "DECIMAL"
    "DOUBLE"
    "FLOAT"
    "FROZEN"
    "INET"
    "INT"
    "LIST"
    "MAP"
    "SET"
    "SMALLINT"
    "TEXT"
    "TIME"
    "TIMESTAMP"
    "TIMEUUID"
    "TINYINT"
    "TUPLE"
    "UUID"
    "VARCHAR"
    "VARINT"
  ] @type.builtin)

(data_type_name
  (object_name) @type)

(data_type_definition
  [
    "<"
    ">"
  ] @punctuation.bracket)

; Literals

"NULL" @constant.builtin

((constant) @boolean
  (#match? @boolean "^(?i:true|false)$"))

((constant) @string
  (#match? @string "^'"))

((constant) @string.special
  (#match? @string.special "^\\$\\$"))

; A number, a hexadecimal blob or a UUID.
((constant) @number
  (#match? @number "^(-?[0-9]|[0-9a-fA-F]{8}-)"))

[
  "code_block"
  "password"
] @string

"login" @boolean

(role_with_option
  "user" @boolean)

[
  "limit_value"
  "time"
  "ttl"
] @number

(indexed_column
  "index" @number)

((assignment_element
  "assignment_operand" @number)
  (#match? @number "^-?[0-9]"))

((assignment_element
  "assignment_operand" @variable)
  (#match? @variable "^[^-0-9]"))

(option_hash_item
  "key" @string.special.key)

(replication_list_item
  "key" @string.special.key)

"hash_key" @string.special.key

((option_hash_item
  "value" @string)
  (#match? @string "^'"))

((option_hash_item
  "value" @number)
  (#match? @number "^-?[0-9]"))

((option_hash_item
  "value" @boolean)
  (#match? @boolean "^(?i:true|false)$"))

((replication_list_item
  "value" @string)
  (#match? @string "^'"))

((replication_list_item
  "value" @number)
  (#match? @number "^-?[0-9]"))

(durable_writes
  "value" @boolean)

; Names

"keyspace" @namespace

[
  "table"
  "type"
  "materialized_view"
] @type

[
  "column"
  "partition_key"
  "primary_key"
  "entry"
  "full"
] @field

(index_keys_spec
  "key" @field)

(alter_table_drop_columns
  (object_name) @field)

(assignment_element
  (object_name) @field)

(clustering_key_list
  (object_name) @field)

(column_not_null
  (object_name) @field)

(indexed_column
  (object_name) @field)

(order_spec
  (object_name) @field)

(partition_key_list
  (object_name) @field)

(function_args
  (object_name) @variable)

(table_option_name
  (object_name) @property)

((table_option_value) @string
  (#match? @string "^'"))

((table_option_value) @number
  (#match? @number "^-?[0-9]"))

((table_option_value) @constant
  (#match? @constant "^[^-0-9']"))

(trigger_class) @string

[
  "function"
  "aggregate"
  "sfunc"
  "finalfunc"
] @function

"function_name" @function.call

(user_name
  "user" @variable)

[
  "alias"
  "role"
  "trigger"
  "language"
] @variable

(index_name
  "index" @variable)

(short_index_name
  "index" @variable)

(delete_column_item
  "index" @variable)

(bind_marker) @variable.parameter

; Operators

[
  "="
  "<>"
  "<="
  ">="
  "+"
  "-"
  "*"
] @operator

(relation_element
  [
    "<"
    ">"
  ] @operator)

; Punctuation

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
  ";"
  "."
  ":"
] @punctuation.delimiter
