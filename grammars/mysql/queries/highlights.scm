; The highlights of the MySQL grammar. A keyword is an anonymous node with
; the name of the keyword in upper case. The patterns for the names come
; first, so a keyword that names a function or a type gets the capture of
; the name.

(comment) @comment

[
  (version_comment_start)
  (version_comment_end)
] @keyword.directive

(string) @string

(number) @number

(boolean) @boolean

(null) @constant.builtin

"UNKNOWN" @constant.builtin

(temporal_literal
  type: _ @type.builtin)

(type_name) @type.builtin

(type_name
  _ @type.builtin)

(function_call
  name: (identifier) @function.call)

(function_call
  schema: (identifier) @module)

(object_reference
  schema: (identifier) @module)

(object_reference
  name: (identifier) @type)

(table_reference
  schema: (identifier) @module)

(table_reference
  name: (identifier) @type)

(table_reference
  alias: (identifier) @variable)

(derived_table
  alias: (identifier) @variable)

(cte
  name: (identifier) @type)

(column_reference
  table: (identifier) @type)

(column_reference
  schema: (identifier) @module)

(column_reference
  name: (identifier) @variable.member)

(column_definition
  name: (identifier) @variable.member)

(aliased_expression
  alias: (identifier) @variable)

(routine_parameter
  name: (identifier) @variable.parameter)

(declare_variable
  name: (identifier) @variable)

(user_variable) @variable

(system_variable) @variable.builtin

(parameter) @variable.parameter

[
  (block
    label: (identifier) @label)
  (block
    end_label: (identifier) @label)
  (loop_statement
    label: (identifier) @label)
  (loop_statement
    end_label: (identifier) @label)
  (while_statement
    label: (identifier) @label)
  (while_statement
    end_label: (identifier) @label)
  (repeat_statement
    label: (identifier) @label)
  (repeat_statement
    end_label: (identifier) @label)
  (leave_statement
    label: (identifier) @label)
  (iterate_statement
    label: (identifier) @label)
]


[
  "IF"
  "ELSEIF"
  "ELSE"
  "THEN"
  "CASE"
  "WHEN"
] @keyword.conditional

[
  "LOOP"
  "WHILE"
  "REPEAT"
  "UNTIL"
  "ITERATE"
  "LEAVE"
] @keyword.repeat

[
  "RETURN"
  "RETURNS"
] @keyword.return

[
  "SIGNAL"
  "RESIGNAL"
  "HANDLER"
  "SQLEXCEPTION"
  "SQLWARNING"
] @keyword.exception

[
  "AND"
  "OR"
  "XOR"
  "NOT"
  "IN"
  "IS"
  "LIKE"
  "REGEXP"
  "RLIKE"
  "SOUNDS"
  "BETWEEN"
  "DIV"
  "MOD"
  "ESCAPE"
  "MEMBER"
  "OF"
  "COLLATE"
  "ANY"
  "SOME"
  "EXISTS"
] @keyword.operator

[
  "ACTION"
  "ADD"
  "ADMIN"
  "AFTER"
  "AGAINST"
  "ALGORITHM"
  "ALL"
  "ALTER"
  "ALWAYS"
  "ANALYZE"
  "ARRAY"
  "AS"
  "ASC"
  "AUTO_INCREMENT"
  "AVG_ROW_LENGTH"
  "BEFORE"
  "BEGIN"
  "BIGINT"
  "BINARY"
  "BIT"
  "BLOB"
  "BOOL"
  "BOOLEAN"
  "BOTH"
  "BTREE"
  "BY"
  "CALL"
  "CASCADE"
  "CASCADED"
  "CHAIN"
  "CHANGE"
  "CHAR"
  "CHARACTER"
  "CHARSET"
  "CHECK"
  "CHECKSUM"
  "CLIENT"
  "CLOSE"
  "CODE"
  "COLLATION"
  "COLUMN"
  "COLUMNS"
  "COLUMN_FORMAT"
  "COMMENT"
  "COMMIT"
  "COMMITTED"
  "COMPRESSION"
  "CONCURRENT"
  "CONDITION"
  "CONNECTION"
  "CONSISTENT"
  "CONSTRAINT"
  "CONTAINS"
  "CONTINUE"
  "CONVERT"
  "COUNT"
  "CREATE"
  "CROSS"
  "CURRENT"
  "CURRENT_USER"
  "CURSOR"
  "DATA"
  "DATABASE"
  "DATABASES"
  "DATE"
  "DATETIME"
  "DAY"
  "DAY_HOUR"
  "DAY_MICROSECOND"
  "DAY_MINUTE"
  "DAY_SECOND"
  "DEALLOCATE"
  "DEC"
  "DECIMAL"
  "DECLARE"
  "DEFAULT"
  "DEFINER"
  "DELAYED"
  "DELAY_KEY_WRITE"
  "DELETE"
  "DESC"
  "DESCRIBE"
  "DETERMINISTIC"
  "DIRECTORY"
  "DISABLE"
  "DISK"
  "DISTINCT"
  "DISTINCTROW"
  "DO"
  "DOUBLE"
  "DROP"
  "DUAL"
  "DUMPFILE"
  "DUPLICATE"
  "DYNAMIC"
  "EACH"
  "ENABLE"
  "ENCLOSED"
  "ENCRYPTION"
  "END"
  "ENFORCED"
  "ENGINE"
  "ENGINES"
  "ENUM"
  "ERRORS"
  "ESCAPED"
  "EVENT"
  "EVENTS"
  "EXCEPT"
  "EXECUTE"
  "EXIT"
  "EXPANSION"
  "EXPLAIN"
  "EXTENDED"
  "FETCH"
  "FIELDS"
  "FILE"
  "FIRST"
  "FIXED"
  "FLOAT"
  "FOLLOWING"
  "FOLLOWS"
  "FOR"
  "FORCE"
  "FOREIGN"
  "FORMAT"
  "FOUND"
  "FROM"
  "FULL"
  "FULLTEXT"
  "FUNCTION"
  "GENERATED"
  "GEOMETRY"
  "GEOMETRYCOLLECTION"
  "GLOBAL"
  "GRANT"
  "GRANTS"
  "GROUP"
  "HASH"
  "HAVING"
  "HIGH_PRIORITY"
  "HOSTS"
  "HOUR"
  "HOUR_MICROSECOND"
  "HOUR_MINUTE"
  "HOUR_SECOND"
  "IDENTIFIED"
  "IGNORE"
  "INDEX"
  "INDEXES"
  "INFILE"
  "INNER"
  "INOUT"
  "INSERT"
  "INSERT_METHOD"
  "INT"
  "INT1"
  "INT2"
  "INT3"
  "INT4"
  "INT8"
  "INTEGER"
  "INTERSECT"
  "INTERVAL"
  "INTO"
  "INVISIBLE"
  "INVOKER"
  "ISOLATION"
  "JOIN"
  "JSON"
  "KEY"
  "KEYS"
  "KEY_BLOCK_SIZE"
  "LANGUAGE"
  "LATERAL"
  "LEADING"
  "LEFT"
  "LESS"
  "LEVEL"
  "LIMIT"
  "LINEAR"
  "LINES"
  "LINESTRING"
  "LIST"
  "LOAD"
  "LOCAL"
  "LOCK"
  "LOCKED"
  "LOG"
  "LOGS"
  "LONGBLOB"
  "LONGTEXT"
  "LOW_PRIORITY"
  "MASTER"
  "MATCH"
  "MAXVALUE"
  "MAX_ROWS"
  "MEDIUMBLOB"
  "MEDIUMINT"
  "MEDIUMTEXT"
  "MEMORY"
  "MERGE"
  "MICROSECOND"
  "MINUTE"
  "MINUTE_MICROSECOND"
  "MINUTE_SECOND"
  "MIN_ROWS"
  "MODE"
  "MODIFIES"
  "MODIFY"
  "MONTH"
  "MULTILINESTRING"
  "MULTIPOINT"
  "MULTIPOLYGON"
  "MUTEX"
  "NAMES"
  "NATIONAL"
  "NATURAL"
  "NCHAR"
  "NEXT"
  "NO"
  "NOWAIT"
  "NUMERIC"
  "NVARCHAR"
  "OFFSET"
  "ON"
  "ONLY"
  "OPEN"
  "OPTION"
  "OPTIONALLY"
  "ORDER"
  "OUT"
  "OUTER"
  "OUTFILE"
  "OVER"
  "PACK_KEYS"
  "PARSER"
  "PARTIAL"
  "PARTITION"
  "PARTITIONS"
  "PASSWORD"
  "PERSIST"
  "PERSIST_ONLY"
  "PLUGINS"
  "POINT"
  "POLYGON"
  "PRECEDES"
  "PRECEDING"
  "PRECISION"
  "PREPARE"
  "PRIMARY"
  "PRIVILEGES"
  "PROCEDURE"
  "PROCESS"
  "PROCESSLIST"
  "PROFILES"
  "PROXY"
  "QUARTER"
  "QUERY"
  "QUICK"
  "RANGE"
  "READ"
  "READS"
  "REAL"
  "RECURSIVE"
  "REFERENCES"
  "RELEASE"
  "RELOAD"
  "RENAME"
  "REPEATABLE"
  "REPLACE"
  "REPLICA"
  "REPLICAS"
  "REPLICATION"
  "RESTRICT"
  "REVOKE"
  "RIGHT"
  "ROLE"
  "ROLLBACK"
  "ROLLUP"
  "ROUTINE"
  "ROW"
  "ROWS"
  "ROW_FORMAT"
  "SAVEPOINT"
  "SCHEMA"
  "SCHEMAS"
  "SECOND"
  "SECOND_MICROSECOND"
  "SECURITY"
  "SELECT"
  "SEPARATOR"
  "SERIAL"
  "SERIALIZABLE"
  "SESSION"
  "SET"
  "SHARE"
  "SHOW"
  "SHUTDOWN"
  "SIGNED"
  "SIMPLE"
  "SKIP"
  "SLAVE"
  "SMALLINT"
  "SNAPSHOT"
  "SPATIAL"
  "SQL"
  "SQLSTATE"
  "SQL_BIG_RESULT"
  "SQL_BUFFER_RESULT"
  "SQL_CALC_FOUND_ROWS"
  "SQL_NO_CACHE"
  "SQL_SMALL_RESULT"
  "SRID"
  "START"
  "STARTING"
  "STATS_AUTO_RECALC"
  "STATS_PERSISTENT"
  "STATS_SAMPLE_PAGES"
  "STATUS"
  "STORAGE"
  "STORED"
  "STRAIGHT_JOIN"
  "SUPER"
  "TABLE"
  "TABLES"
  "TABLESPACE"
  "TEMPORARY"
  "TEMPTABLE"
  "TERMINATED"
  "TEXT"
  "THAN"
  "TIME"
  "TIMESTAMP"
  "TINYBLOB"
  "TINYINT"
  "TINYTEXT"
  "TO"
  "TRAILING"
  "TRANSACTION"
  "TRIGGER"
  "TRIGGERS"
  "TRUNCATE"
  "UNBOUNDED"
  "UNCOMMITTED"
  "UNDEFINED"
  "UNDO"
  "UNION"
  "UNIQUE"
  "UNLOCK"
  "UNSIGNED"
  "UPDATE"
  "USAGE"
  "USE"
  "USER"
  "USING"
  "VALUE"
  "VALUES"
  "VARBINARY"
  "VARCHAR"
  "VARCHARACTER"
  "VARIABLES"
  "VARYING"
  "VIEW"
  "VIRTUAL"
  "VISIBLE"
  "WARNINGS"
  "WEEK"
  "WHERE"
  "WINDOW"
  "WITH"
  "WORK"
  "WRITE"
  "XML"
  "YEAR"
  "YEAR_MONTH"
  "ZEROFILL"
] @keyword

(all_columns
  "*" @punctuation.special)

(privilege_level
  "*" @punctuation.special)

[
  "+"
  "-"
  "*"
  "/"
  "%"
  "="
  "<=>"
  "!="
  "<>"
  "<"
  "<="
  ">"
  ">="
  "!"
  "~"
  "&"
  "|"
  "^"
  "<<"
  ">>"
  "&&"
  "||"
  ":="
  "->"
  "->>"
] @operator

[
  "("
  ")"
] @punctuation.bracket

[
  ";"
  ","
  "."
  ":"
] @punctuation.delimiter

"@" @punctuation.special
