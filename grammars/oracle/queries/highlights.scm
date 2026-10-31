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
  "ACCESSIBLE"
  "ADD"
  "ADMIN"
  "ADMINISTER"
  "AFTER"
  "AGGREGATE"
  "ALL"
  "ALLOW"
  "ALTER"
  "ALWAYS"
  "ANALYTIC"
  "ANALYZE"
  "AND"
  "ANNOTATIONS"
  "ANY"
  "APPLY"
  "ARCHIVAL"
  "ARCHIVE"
  "ARE"
  "ARRAY"
  "AS"
  "ASC"
  "ASCII"
  "ASSOCIATE"
  "AT"
  "ATTRIBUTE"
  "ATTRIBUTES"
  "AUDIT"
  "AUTHID"
  "AUTHORIZATION"
  "AUTO"
  "AUTOMATIC"
  "BATCH"
  "BEFORE"
  "BEGIN"
  "BEQUEATH"
  "BETWEEN"
  "BIGFILE"
  "BITMAP"
  "BLOCK"
  "BLOCKCHAIN"
  "BODY"
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
  "CHOOSE"
  "CLASS"
  "CLOSE"
  "CLUSTER"
  "CLUSTERING"
  "COLLATE"
  "COLLATION"
  "COLLECT"
  "COLUMN"
  "COLUMNS"
  "COLUMN_VALUE"
  "COMMENT"
  "COMMIT"
  "COMMITTED"
  "COMPILE"
  "COMPOUND"
  "COMPRESS"
  "COMPUTE"
  "CONDITIONAL"
  "CONNECT"
  "CONNECT_BY_ROOT"
  "CONSISTENT"
  "CONSTANT"
  "CONSTRAINT"
  "CONSTRAINTS"
  "CONSTRUCTOR"
  "CONTAINER"
  "CONTAINERS_DEFAULT"
  "CONTAINER_MAP"
  "CONTEXT"
  "CONTINUE"
  "CONTROLFILE"
  "CONVERSION"
  "CORRUPT_XID"
  "CORRUPT_XID_ALL"
  "COST"
  "COUNT"
  "CREATE"
  "CROSSEDITION"
  "CUBE"
  "CURRENT"
  "CURRENT_USER"
  "CURSOR"
  "CYCLE"
  "DANGLING"
  "DATABASE"
  "DATE"
  "DAY"
  "DBA_RECYCLEBIN"
  "DECIMAL"
  "DECLARE"
  "DECREMENT"
  "DEFAULT"
  "DEFERRABLE"
  "DEFERRED"
  "DEFINE"
  "DEFINER"
  "DEFINITION"
  "DELEGATE"
  "DELETE"
  "DEMAND"
  "DENSE_RANK"
  "DEPTH"
  "DESC"
  "DESTINATION"
  "DETERMINISTIC"
  "DICTIONARY"
  "DIMENSION"
  "DIRECTORY"
  "DISABLE"
  "DISALLOW"
  "DISASSOCIATE"
  "DISKGROUP"
  "DISPLAY"
  "DISTINCT"
  "DOMAIN"
  "DOUBLE"
  "DROP"
  "DUALITY"
  "DUPLICATED"
  "EACH"
  "EDGE"
  "EDITION"
  "EDITIONABLE"
  "EDITIONING"
  "ELSE"
  "ELSIF"
  "EMPTY"
  "ENABLE"
  "ENCRYPT"
  "END"
  "ENUM"
  "ENV"
  "ERROR"
  "ERRORS"
  "ESCAPE"
  "EXCEPT"
  "EXCEPTION"
  "EXCEPTIONS"
  "EXCLUDE"
  "EXCLUSIVE"
  "EXECUTE"
  "EXISTS"
  "EXIT"
  "EXPLAIN"
  "EXTERNAL"
  "FALSE"
  "FETCH"
  "FILESTORE"
  "FINAL"
  "FIRST"
  "FLASHBACK"
  "FLEXIBLE"
  "FLOAT"
  "FOLLOWING"
  "FOLLOWS"
  "FOR"
  "FORALL"
  "FORCE"
  "FOREIGN"
  "FOREVER"
  "FORMAT"
  "FORWARD"
  "FROM"
  "FUNCTION"
  "FUNCTIONS"
  "GENERATED"
  "GLOBAL"
  "GOTO"
  "GRANT"
  "GRAPH"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HASH"
  "HAVING"
  "HIERARCHY"
  "HOUR"
  "ID"
  "IDENTIFIED"
  "IDENTIFIER"
  "IDENTITY"
  "IF"
  "IGNORE"
  "IMMEDIATE"
  "IMMUTABLE"
  "IN"
  "INCLUDE"
  "INCREMENT"
  "INDEX"
  "INDEXES"
  "INDEXTYPE"
  "INDEXTYPES"
  "INDICATOR"
  "INDICES"
  "INFINITE"
  "INITIALLY"
  "INITRANS"
  "INMEMORY"
  "INSERT"
  "INSTANTIABLE"
  "INSTEAD"
  "INTEGER"
  "INTERLEAVED"
  "INTERSECT"
  "INTERVAL"
  "INTO"
  "INVISIBLE"
  "IS"
  "ISOLATION"
  "ITERATE"
  "JAVA"
  "JOIN"
  "JSON"
  "JSON_TABLE"
  "KEEP"
  "KEY"
  "KEYS"
  "LABEL"
  "LANGUAGE"
  "LAST"
  "LATERAL"
  "LAX"
  "LEADING"
  "LESS"
  "LEVEL"
  "LIBRARY"
  "LIKE"
  "LIKE2"
  "LIKE4"
  "LIKEC"
  "LIMIT"
  "LINEAR"
  "LINK"
  "LIST"
  "LOB"
  "LOCAL"
  "LOCK"
  "LOCKDOWN"
  "LOCKED"
  "LOG"
  "LOGGING"
  "LOGICAL"
  "LONG"
  "LOOP"
  "MAIN"
  "MANAGED"
  "MANAGEMENT"
  "MAP"
  "MATCH"
  "MATCHED"
  "MATCHES"
  "MATCH_RECOGNIZE"
  "MATERIALIZED"
  "MAXVALUE"
  "MEASURES"
  "MEMBER"
  "MERGE"
  "MICROSECONDS"
  "MILLISECONDS"
  "MINING"
  "MINUS"
  "MINUTE"
  "MISMATCH"
  "MLE"
  "MODE"
  "MODEL"
  "MODIFY"
  "MODULE"
  "MONTH"
  "MULTISET"
  "MULTIVALUE"
  "MUTABLE"
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
  "NOCHECK"
  "NOCOMPRESS"
  "NOCOPY"
  "NOCYCLE"
  "NODELETE"
  "NOFORCE"
  "NOINSERT"
  "NOLOGGING"
  "NONE"
  "NONEDITIONABLE"
  "NORELY"
  "NOSORT"
  "NOT"
  "NOUPDATE"
  "NOVALIDATE"
  "NOWAIT"
  "NULL"
  "NULLS"
  "NUMBER"
  "OBJECT"
  "OF"
  "OFFSET"
  "OID"
  "OLD"
  "OMIT"
  "ON"
  "ONE"
  "ONLINE"
  "ONLY"
  "OPEN"
  "OPERATOR"
  "OPTION"
  "OPTIONS"
  "OR"
  "ORDER"
  "ORDINALITY"
  "OTHERS"
  "OUT"
  "OUTLINE"
  "OVER"
  "OVERFLOW"
  "OVERRIDING"
  "PACKAGE"
  "PACKAGES"
  "PAIRS"
  "PARALLEL_ENABLE"
  "PARENT"
  "PARTITION"
  "PARTITIONS"
  "PASSING"
  "PAST"
  "PATH"
  "PATTERN"
  "PCTFREE"
  "PER"
  "PERCENT"
  "PERIOD"
  "PERMUTE"
  "PERSISTABLE"
  "PFILE"
  "PIPE"
  "PIPELINED"
  "PIVOT"
  "PLAN"
  "PLUGGABLE"
  "PMEM"
  "POINT"
  "POLICY"
  "POLYMORPHIC"
  "PRAGMA"
  "PREBUILT"
  "PRECEDES"
  "PRECEDING"
  "PRECISION"
  "PRESENT"
  "PRESERVE"
  "PRETTY"
  "PRIMARY"
  "PRIOR"
  "PRIVATE"
  "PROCEDURE"
  "PROFILE"
  "PROPERTIES"
  "PROPERTY"
  "PUBLIC"
  "PURGE"
  "RAISE"
  "RANGE"
  "RAW"
  "READ"
  "RECORD"
  "RECYCLEBIN"
  "REDUCED"
  "REF"
  "REFERENCE"
  "REFERENCES"
  "REFERENCING"
  "REJECT"
  "RELATIONAL"
  "RELIES_ON"
  "RELY"
  "RENAME"
  "REPEAT"
  "REPLACE"
  "REPLICATION"
  "RESERVABLE"
  "RESET"
  "RESETLOGS"
  "RESOLVE"
  "RESOURCE"
  "RESPECT"
  "RESTORE"
  "RESULT"
  "RESULT_CACHE"
  "RETURN"
  "RETURNING"
  "REUSE"
  "REVERSE"
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
  "SALT"
  "SAMPLE"
  "SAVE"
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
  "SELECTIVITY"
  "SELF"
  "SEQUENCE"
  "SEQUENTIAL"
  "SERIALIZABLE"
  "SESSION"
  "SET"
  "SETS"
  "SHARDED"
  "SHARE"
  "SHARED"
  "SHARING"
  "SHOW"
  "SIBLINGS"
  "SINGLE"
  "SKIP"
  "SMALLFILE"
  "SMALLINT"
  "SOME"
  "SORT"
  "SOURCE"
  "SPFILE"
  "SQL"
  "SQL_MACRO"
  "STANDBY"
  "START"
  "STATEMENT"
  "STATEMENT_ID"
  "STATIC"
  "STATISTICS"
  "STORAGE"
  "STORE"
  "STRICT"
  "SUBMULTISET"
  "SUBPARTITION"
  "SUBPARTITIONS"
  "SUBSET"
  "SUBTYPE"
  "SUCCESSFUL"
  "SUPPLEMENTAL"
  "SYNONYM"
  "SYSDATE"
  "SYSTEM"
  "TABLE"
  "TABLES"
  "TABLESPACE"
  "TEMPLATE"
  "TEMPORARY"
  "THAN"
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
  "TYPES"
  "UID"
  "UNBOUNDED"
  "UNCONDITIONAL"
  "UNDER"
  "UNDO"
  "UNION"
  "UNIQUE"
  "UNLIMITED"
  "UNMATCHED"
  "UNPIVOT"
  "UNTIL"
  "UNUSED"
  "UPDATE"
  "UPDATED"
  "UPSERT"
  "USE"
  "USECASE"
  "USER"
  "USING"
  "VALIDATE"
  "VALUE"
  "VALUES"
  "VARCHAR"
  "VARCHAR2"
  "VARRAY"
  "VARYING"
  "VECTOR"
  "VERSIONS"
  "VERTEX"
  "VIEW"
  "VIRTUAL"
  "VISIBILITY"
  "VISIBLE"
  "WAIT"
  "WHEN"
  "WHENEVER"
  "WHERE"
  "WHILE"
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
  "ZONEMAP"
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
  column: (identifier) @variable.member)

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

(column_definition
  name: (identifier) @variable.member)

(column_definition
  collation: (identifier) @variable.builtin)

(column_constraint
  name: (identifier) @label)

(table_constraint
  name: (identifier) @label)

(period_definition
  name: (identifier) @variable.member)

(partition_definition
  name: (identifier) @type)

(annotation
  name: (identifier) @attribute)

(create_index_statement
  alias: (identifier) @variable)

(modify_clause
  constraint: (identifier) @label)

(modify_clause
  partition: (identifier) @type)

(drop_clause
  column: (identifier) @variable.member)

(drop_clause
  constraint: (identifier) @label)

(drop_clause
  partition: (identifier) @type)

(drop_clause
  period: (identifier) @variable.member)

(rename_clause
  name: (identifier) @variable.member)

(rename_clause
  new_name: (identifier) @variable.member)

(set_unused_clause
  column: (identifier) @variable.member)

(type_attribute
  name: (identifier) @variable.member)

(type_method
  name: (identifier) @function)

(parameter_declaration
  name: (identifier) @variable.parameter)

(create_domain_statement
  column: (identifier) @variable.member)

(enum_item
  name: (identifier) @constant)

(graph_element_table
  alias: (identifier) @variable)

(graph_element_table
  label: (identifier) @label)

(graph_element_table
  reference: (identifier) @type)

(create_schema_statement
  schema: (identifier) @module)

(create_procedure_statement
  name: (object_reference
    name: (identifier) @function))

(create_function_statement
  name: (object_reference
    name: (identifier) @function))

(create_package_statement
  name: (object_reference
    name: (identifier) @module))

(create_trigger_statement
  name: (object_reference
    name: (identifier) @function))

(create_trigger_statement
  schema: (identifier) @module)

(create_trigger_statement
  column: (identifier) @variable.member)

(create_procedure_statement
  end: (identifier) @function)

(create_function_statement
  end: (identifier) @function)

(procedure_declaration
  end: (identifier) @function)

(function_declaration
  end: (identifier) @function)

(compound_trigger
  end: (identifier) @function)

(create_package_statement
  end: (identifier) @module)

(block
  end: (identifier) @label)

(loop_statement
  end: (identifier) @label)

(case_statement
  end: (identifier) @label)

(procedure_declaration
  name: (identifier) @function)

(function_declaration
  name: (identifier) @function)

(procedure_call
  (column_reference
    name: (identifier) @function.call))

(label
  name: (identifier) @label)

(exit_statement
  label: (identifier) @label)

(continue_statement
  label: (identifier) @label)

(goto_statement
  label: (identifier) @label)

(variable_declaration
  name: (identifier) @variable)

(cursor_declaration
  name: (identifier) @variable)

(exception_declaration
  name: (identifier) @constant)

(exception_handler
  exception: (identifier) @constant)

(raise_statement
  exception: (identifier) @constant)

(exception_handler
  package: (identifier) @module)

(raise_statement
  package: (identifier) @module)

(type_definition
  name: (identifier) @type.definition)

(subtype_definition
  name: (identifier) @type.definition)

(field_definition
  name: (identifier) @variable.member)

(pragma
  name: (identifier) @keyword.directive)

(iterator
  name: (identifier) @variable)

(forall_statement
  index: (identifier) @variable)

(referencing_alias
  alias: (identifier) @variable)

(cursor_attribute
  attribute: (identifier) @attribute.builtin)

(inquiry_directive) @constant.macro

[
  "$IF"
  "$THEN"
  "$ELSIF"
  "$ELSE"
  "$END"
  "$ERROR"
] @keyword.directive

[
  (operator)
  "="
  "=>"
  "%"
  ":="
  ".."
] @operator

[
  "<<"
  ">>"
] @punctuation.special

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
