; The highlights of the ClickHouse grammar. xo wrote this file with the
; grammar (D127). A keyword is an anonymous node with the name of the
; keyword in upper case. The patterns for the constants and the names come
; after the keywords, because the last pattern that captures a node counts,
; so a name gets the capture of its place, such as a function or a table.

(comment) @comment

(string) @string

(heredoc_string) @string

(hex_string) @string.special

(bit_string) @string.special

(number) @number

(boolean) @boolean

(null) @constant.builtin

(identifier) @variable

[
  "ADD"
  "ADMIN"
  "AFTER"
  "ALIAS"
  "ALL"
  "ALLOWED_LATENESS"
  "ALTER"
  "ANALYZE"
  "AND"
  "ANTI"
  "ANY"
  "APPEND"
  "APPLY"
  "ARRAY"
  "AS"
  "ASC"
  "ASCENDING"
  "ASOF"
  "ASSUME"
  "AST"
  "ASYNC"
  "ATTACH"
  "AUTHENTICATION"
  "AUTO_INCREMENT"
  "BACKUP"
  "BEGIN"
  "BETWEEN"
  "BIDIRECTIONAL"
  "BINARY"
  "BOTH"
  "BY"
  "CASE"
  "CAST"
  "CHANGEABLE_IN_READONLY"
  "CHAR"
  "CHARACTER"
  "CHECK"
  "CLEANUP"
  "CLEAR"
  "CLONE"
  "CLUSTER"
  "CN"
  "CODEC"
  "COLLATE"
  "COLLECTION"
  "COLUMN"
  "COLUMNS"
  "COMMENT"
  "COMMIT"
  "COMPRESSION"
  "CONST"
  "CONSTRAINT"
  "CREATE"
  "CROSS"
  "CUBE"
  "CURRENT"
  "DATABASE"
  "DATABASES"
  "DEDUPLICATE"
  "DEFAULT"
  "DEFINER"
  "DELAY"
  "DELETE"
  "DELETED"
  "DEPENDS"
  "DESC"
  "DESCENDING"
  "DESCRIBE"
  "DETACH"
  "DETACHED"
  "DICTIONARIES"
  "DICTIONARY"
  "DISK"
  "DISTINCT"
  "DIV"
  "DOUBLE"
  "DROP"
  "DRY"
  "ELSE"
  "EMPTY"
  "END"
  "ENGINE"
  "EPHEMERAL"
  "ESCAPE"
  "ESTIMATE"
  "EVENTS"
  "EVERY"
  "EXCEPT"
  "EXCHANGE"
  "EXECUTE"
  "EXISTS"
  "EXPLAIN"
  "EXPRESSION"
  "FETCH"
  "FILL"
  "FILTER"
  "FINAL"
  "FIRST"
  "FOLLOWING"
  "FOR"
  "FORCE"
  "FORGET"
  "FORMAT"
  "FREEZE"
  "FROM"
  "FULL"
  "FUNCTION"
  "GLOBAL"
  "GRANT"
  "GRANTEES"
  "GRANTS"
  "GRANULARITY"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HANDLER"
  "HAVING"
  "HIERARCHICAL"
  "HOST"
  "HYPOTHETICAL"
  "ID"
  "IDENTIFIED"
  "IF"
  "IGNORE"
  "ILIKE"
  "IMPLICIT"
  "IN"
  "INCREMENTAL"
  "INDEX"
  "INFILE"
  "INHERIT"
  "INJECTIVE"
  "INNER"
  "INSERT"
  "INTERPOLATE"
  "INTERSECT"
  "INTERVAL"
  "INTO"
  "INVOKER"
  "IP"
  "IPV4_PREFIX_BITS"
  "IPV6_PREFIX_BITS"
  "IS"
  "IS_OBJECT_ID"
  "JOIN"
  "KEY"
  "KEYED"
  "KILL"
  "LARGE"
  "LAST"
  "LAYOUT"
  "LEADING"
  "LEFT"
  "LEVEL"
  "LIFETIME"
  "LIKE"
  "LIMIT"
  "LIMITS"
  "LIVE"
  "LOCAL"
  "MASK"
  "MASKING"
  "MATERIALIZE"
  "MATERIALIZED"
  "MAX"
  "METHODS"
  "MIN"
  "MOD"
  "MODIFY"
  "MOVE"
  "MUTATION"
  "NAME"
  "NAMED"
  "NATIONAL"
  "NATURAL"
  "NCHAR"
  "NEW"
  "NEXT"
  "NO"
  "NONE"
  "NOT"
  "NULLS"
  "OBJECT"
  "OFFSET"
  "ON"
  "ONLY"
  "OPTIMIZE"
  "OPTION"
  "OR"
  "ORDER"
  "OUTER"
  "OUTFILE"
  "OVER"
  "OVERRIDABLE"
  "OVERRIDE"
  "PART"
  "PARTITION"
  "PARTITIONS"
  "PARTS"
  "PASTE"
  "PATCHES"
  "PERMANENTLY"
  "PERMISSIVE"
  "PIPELINE"
  "PLACING"
  "PLAN"
  "POLICY"
  "POPULATE"
  "PRECEDING"
  "PRECISION"
  "PREFIX"
  "PREWHERE"
  "PRIMARY"
  "PRIORITY"
  "PROFILE"
  "PROFILES"
  "PROJECTION"
  "PROTOCOL"
  "QUALIFY"
  "QUERY"
  "QUOTA"
  "RANDOMIZE"
  "RANDOMIZED"
  "RANGE"
  "READONLY"
  "REALM"
  "RECOMPRESS"
  "RECURSIVE"
  "REFRESH"
  "REGEXP"
  "REMOVE"
  "RENAME"
  "REPLACE"
  "REPLICATED"
  "RESET"
  "RESOURCE"
  "RESPECT"
  "RESTORE"
  "RESTRICTIVE"
  "REVOKE"
  "REWRITE"
  "RIGHT"
  "ROLE"
  "ROLLBACK"
  "ROLLUP"
  "ROW"
  "ROWS"
  "RUN"
  "SALT"
  "SAMPLE"
  "SAN"
  "SCHEME"
  "SECURITY"
  "SELECT"
  "SEMI"
  "SERVER"
  "SET"
  "SETS"
  "SETTING"
  "SETTINGS"
  "SHARED"
  "SHOW"
  "SIGNED"
  "SIMILAR"
  "SKIP"
  "SNAPSHOT"
  "SOURCE"
  "SQL"
  "STALENESS"
  "STATISTICS"
  "STDOUT"
  "STEP"
  "STRICT"
  "SYNC"
  "SYNTAX"
  "SYSTEM"
  "TABLE"
  "TABLES"
  "TEMPORARY"
  "TEST"
  "THEN"
  "TIES"
  "TIME"
  "TIMEOUT"
  "TO"
  "TOKEN"
  "TOP"
  "TOTALS"
  "TRACKING"
  "TRAILING"
  "TRANSACTION"
  "TREE"
  "TRUNCATE"
  "TTL"
  "TYPE"
  "UNBOUNDED"
  "UNDROP"
  "UNFREEZE"
  "UNION"
  "UNKNOWN"
  "UNSIGNED"
  "UNTIL"
  "UPDATE"
  "URL"
  "USE"
  "USER"
  "USING"
  "UUID"
  "VALID"
  "VALUES"
  "VARYING"
  "VIEW"
  "VOLUME"
  "WATCH"
  "WATERMARK"
  "WHEN"
  "WHERE"
  "WINDOW"
  "WITH"
  "WORKLOAD"
  "WRITABLE"
  "ZONE"
] @keyword

(type
  name: (identifier) @type.builtin)

(typed_literal
  type: (type) @type.builtin)

(subcolumn_type
  type: (type
    name: (identifier) @type.builtin))

(interval_unit) @keyword

(keyword) @keyword

(query_parameter
  name: (identifier) @variable.parameter)

(function_call
  name: (identifier) @function.call)

(columns_expression
  "COLUMNS" @function.builtin)

(cast_expression
  "CAST" @function.builtin)

(object_reference
  database: (identifier) @module)

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

(window_definition
  name: (identifier) @type)

(window_specification
  name: (identifier) @type)

(over_clause
  window: (identifier) @type)

(column_reference
  database: (identifier) @module)

(column_reference
  table: (identifier) @type)

(column_reference
  name: (identifier) @variable.member)

(star_expression
  "*" @punctuation.special)

(star_expression
  table: (identifier) @type)

(star_expression
  database: (identifier) @module)

(columns_expression
  table: (identifier) @type)

(aliased_expression
  alias: (identifier) @variable)

(column_list
  (identifier) @variable.member)

(join
  (identifier) @variable.member)

(interpolate_item
  column: (identifier) @variable.member)

(field_definition
  name: (identifier) @property)

(field_selection
  field: (identifier) @property)

(path_prefix
  name: (identifier) @property)

(map_entry
  key: (_) @property)

(type_setting
  name: (identifier) @variable.parameter)

(enum_value
  name: (string) @constant)

(lambda_expression
  parameters: (column_reference
    name: (identifier) @variable.parameter))

(lambda_expression
  parameters: (tuple
    (column_reference
      name: (identifier) @variable.parameter)))

(setting
  name: (identifier) @variable.builtin)

(setting
  "=" @operator)

(type_setting
  "=" @operator)

(enum_value
  "=" @operator)

(format_clause
  format: (identifier) @type.builtin)

(insert_data) @string.special

(insert_columns
  (column_reference
    name: (identifier) @variable.member))

(assignment
  column: (identifier) @variable.member)

(column_definition
  name: (identifier) @variable.member)

(index_definition
  name: (identifier) @type)

(projection_definition
  name: (identifier) @type)

(constraint_definition
  name: (identifier) @type)

(add_action
  name: (identifier) @type)

(modify_action
  name: (identifier) @variable.member)

(drop_action
  name: (identifier) @variable.member)

(rename_column_action
  [
    name: (identifier)
    new_name: (identifier)
  ] @variable.member)

(comment_column_action
  name: (identifier) @variable.member)

(alter_column_action
  name: (identifier) @variable.member)

(add_action
  after: (identifier) @variable.member)

(modify_action
  after: (identifier) @variable.member)

(engine_clause
  name: (identifier) @type.builtin)

(create_database_statement
  name: (identifier) @module)

(alter_named_collection_statement
  name: (identifier) @type)

(create_named_collection_statement
  name: (identifier) @type)

(create_index_statement
  name: (identifier) @type)

(create_function_statement
  name: (identifier) @function)

(truncate_statement
  database: (identifier) @module)

(on_cluster
  cluster: (identifier) @module)

(source_clause
  name: (identifier) @type.builtin)

(layout_clause
  name: (identifier) @type.builtin)

(dictionary_parameter
  name: (identifier) @variable.parameter)

(collection_item
  key: (identifier) @variable.parameter)

(collection_item
  "=" @operator)

(assignment
  "=" @operator)

(engine_clause
  "=" @operator)

(create_view_statement
  "=" @operator)

(profile_setting
  "=" @operator)

(quota_limit
  "=" @operator)

(set_statement
  "=" @operator)

(privilege
  (identifier) @keyword)

(resource_operation
  (identifier) @keyword)

(user_name
  "@" @punctuation.special)

(profile_setting
  name: (identifier) @variable.builtin)

(quota_limit
  name: (identifier) @variable.builtin)

(authentication
  method: (identifier) @type.builtin)

(use_statement
  database: (identifier) @module)

(create_workload_statement
  name: (identifier) @type)

(create_resource_statement
  name: (identifier) @type)

(create_handler_statement
  name: (identifier) @type)

(create_projection_statement
  name: (identifier) @type)

(backup_item
  alias: (object_reference
    name: (identifier) @type))

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

(ternary_expression
  [
    "?"
    ":"
  ] @keyword.conditional.ternary)

(operator) @operator

[
  "::"
  "->"
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
] @punctuation.delimiter
