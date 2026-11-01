; The highlights of the SQLite grammar. xo wrote this file with the grammar
; (D126). A keyword is an anonymous node with the name of the keyword in
; upper case. The patterns for the constants and the names come after the
; keywords, because the last pattern that captures a node counts, so a name
; gets the capture of its place, such as a function or a table.

(comment) @comment

(string) @string

(blob) @string.special

(number) @number

(boolean) @boolean

(parameter) @variable.parameter

(identifier) @variable

[
  "ABORT"
  "ACTION"
  "ADD"
  "AFTER"
  "ALL"
  "ALTER"
  "ALWAYS"
  "ANALYZE"
  "AND"
  "AS"
  "ASC"
  "ATTACH"
  "AUTOINCREMENT"
  "BEFORE"
  "BEGIN"
  "BETWEEN"
  "BY"
  "CASCADE"
  "CASE"
  "CAST"
  "CHECK"
  "COLLATE"
  "COLUMN"
  "COMMIT"
  "CONFLICT"
  "CONSTRAINT"
  "CREATE"
  "CURRENT"
  "DATABASE"
  "DEFAULT"
  "DEFERRABLE"
  "DEFERRED"
  "DELETE"
  "DESC"
  "DETACH"
  "DISTINCT"
  "DO"
  "DROP"
  "EACH"
  "ELSE"
  "END"
  "ESCAPE"
  "EXCEPT"
  "EXCLUDE"
  "EXCLUSIVE"
  "EXISTS"
  "EXPLAIN"
  "FAIL"
  "FILTER"
  "FIRST"
  "FOLLOWING"
  "FOR"
  "FOREIGN"
  "FROM"
  "GENERATED"
  "GLOB"
  "GROUP"
  "GROUPS"
  "HAVING"
  "IF"
  "IGNORE"
  "IMMEDIATE"
  "IN"
  "INDEX"
  "INDEXED"
  "INITIALLY"
  "INNER"
  "INSERT"
  "INSTEAD"
  "INTERSECT"
  "INTO"
  "IS"
  "ISNULL"
  "JOIN"
  "KEY"
  "LAST"
  "LIKE"
  "LIMIT"
  "MATCH"
  "MATERIALIZED"
  "NO"
  "NOT"
  "NOTHING"
  "NOTNULL"
  "NULL"
  "NULLS"
  "OF"
  "OFFSET"
  "ON"
  "OR"
  "ORDER"
  "OTHERS"
  "OVER"
  "PARTITION"
  "PLAN"
  "PRAGMA"
  "PRECEDING"
  "PRIMARY"
  "QUERY"
  "RAISE"
  "RANGE"
  "RECURSIVE"
  "REFERENCES"
  "REGEXP"
  "REINDEX"
  "RELEASE"
  "RENAME"
  "REPLACE"
  "RESTRICT"
  "RETURNING"
  "ROLLBACK"
  "ROW"
  "ROWID"
  "ROWS"
  "SAVEPOINT"
  "SELECT"
  "SET"
  "STORED"
  "STRICT"
  "TABLE"
  "TEMP"
  "TEMPORARY"
  "THEN"
  "TIES"
  "TO"
  "TRANSACTION"
  "TRIGGER"
  "UNBOUNDED"
  "UNION"
  "UNIQUE"
  "UPDATE"
  "USING"
  "VACUUM"
  "VALUES"
  "VIEW"
  "VIRTUAL"
  "WHEN"
  "WHERE"
  "WINDOW"
  "WITH"
  "WITHOUT"
] @keyword

(null
  _ @constant.builtin)

(niladic_function) @function.builtin

(type
  name: (identifier) @type.builtin)

(function_call
  name: (identifier) @function.call)

(function_call
  schema: (identifier) @module)

(cast_expression
  "CAST" @function.builtin)

(raise_expression
  "RAISE" @keyword.exception)

(object_reference
  schema: (identifier) @module)

(object_reference
  name: (identifier) @type)

(table_reference
  alias: (identifier) @variable)

(table_reference
  index: (identifier) @label)

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
  schema: (identifier) @module)

(column_reference
  table: (identifier) @type)

(column_reference
  name: (identifier) @variable.member)

(all_columns) @punctuation.special

(all_columns
  table: (identifier) @type)

(aliased_expression
  alias: (identifier) @variable)

(column_list
  (identifier) @variable.member)

(column_definition
  name: (identifier) @variable.member)

(assignment
  left: (identifier) @variable.member)

(create_index_statement
  table: (identifier) @type)

(column_constraint
  name: (identifier) @label)

(table_constraint
  name: (identifier) @label)

(alter_table_statement
  column: (identifier) @variable.member)

(alter_table_statement
  new_name: (identifier) @variable.member)

(alter_table_statement
  constraint: (identifier) @label)

(column_constraint
  collation: (identifier) @variable.builtin)

(create_virtual_table_statement
  module: (identifier) @function)

(pragma_statement
  name: (object_reference
    name: (identifier) @variable.builtin))

(attach_statement
  schema: (identifier) @module)

(detach_statement
  schema: (identifier) @module)

(vacuum_statement
  schema: (identifier) @module)

(savepoint_statement
  name: (identifier) @label)

(release_statement
  name: (identifier) @label)

(rollback_statement
  savepoint: (identifier) @label)

(collate_expression
  collation: (identifier) @variable.builtin)

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

(operator) @operator

"=" @operator

[
  "("
  ")"
] @punctuation.bracket

[
  ","
  "."
  ";"
] @punctuation.delimiter
