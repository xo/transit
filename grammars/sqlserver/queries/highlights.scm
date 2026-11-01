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
  "ALL"
  "AND"
  "ANY"
  "APPLY"
  "ARRAY"
  "AS"
  "ASC"
  "AT"
  "BEGIN"
  "BETWEEN"
  "BOTH"
  "BREAK"
  "BROWSE"
  "BULK"
  "BY"
  "CALLER"
  "CASE"
  "CATCH"
  "CHECK"
  "CLOSE"
  "CLUSTERED"
  "COLLATE"
  "COMMIT"
  "COMMITTED"
  "CONSTRAINT"
  "CONTAINED"
  "CONTAINSTABLE"
  "CONTENT"
  "CONTINUE"
  "CONVERT"
  "COOKIE"
  "CUBE"
  "CURRENT"
  "CURSOR"
  "DATA_SOURCE"
  "DATEFORMAT"
  "DEADLOCK_PRIORITY"
  "DEALLOCATE"
  "DECLARE"
  "DEFAULT"
  "DELAY"
  "DELETE"
  "DESC"
  "DISTINCT"
  "DISTRIBUTED"
  "DOCUMENT"
  "DOUBLE"
  "ELSE"
  "END"
  "ESCAPE"
  "EXCEPT"
  "EXEC"
  "EXECUTE"
  "EXISTS"
  "FETCH"
  "FIRST"
  "FOLLOWING"
  "FOR"
  "FOR SYSTEM_TIME"
  "FREETEXTTABLE"
  "FROM"
  "GLOBAL"
  "GOTO"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HAVING"
  "HOLDLOCK"
  "IF"
  "IGNORE"
  "IN"
  "INDEX"
  "INSERT"
  "INTERSECT"
  "INTO"
  "IS"
  "ISOLATION"
  "JOIN"
  "JSON"
  "KEY"
  "LANGUAGE"
  "LAST"
  "LEADING"
  "LEVEL"
  "LIKE"
  "LOG"
  "LOGIN"
  "MARK"
  "MATCHED"
  "MERGE"
  "MODEL"
  "NATIONAL"
  "NEXT"
  "NEXT VALUE FOR"
  "NO"
  "NONCLUSTERED"
  "NONE"
  "NOT"
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
  "PERSISTED"
  "PIVOT"
  "PRECEDING"
  "PRECISION"
  "PRIMARY"
  "PRINT"
  "PRIOR"
  "RAISERROR"
  "RANGE"
  "READ"
  "READTEXT"
  "RECOMPILE"
  "RELATIVE"
  "REPEATABLE"
  "RESPECT"
  "RESULT"
  "RETURN"
  "RETURNING"
  "REVERT"
  "ROLLBACK"
  "ROLLUP"
  "ROW"
  "ROWGUIDCOL"
  "ROWS"
  "SAVE"
  "SELECT"
  "SEMANTICKEYPHRASETABLE"
  "SEMANTICSIMILARITYDETAILSTABLE"
  "SEMANTICSIMILARITYTABLE"
  "SERIALIZABLE"
  "SET"
  "SETERROR"
  "SETS"
  "SNAPSHOT"
  "SOME"
  "SOURCE"
  "SYSTEM"
  "TABLE"
  "TABLESAMPLE"
  "TARGET"
  "THEN"
  "THROW"
  "TIES"
  "TIME"
  "TO"
  "TOP"
  "TRAILING"
  "TRAN"
  "TRANSACTION"
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
  "WAITFOR"
  "WHEN"
  "WHERE"
  "WHILE"
  "WINDOW"
  "WITH"
  "WITHIN"
  "WORK"
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
  name: (identifier) @label)

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
