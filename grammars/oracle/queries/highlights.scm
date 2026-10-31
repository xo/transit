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
  "AFTER"
  "ALL"
  "ALLOW"
  "AND"
  "ANY"
  "APPLY"
  "ARRAY"
  "AS"
  "ASC"
  "ASCII"
  "AT"
  "AUTO"
  "AUTOMATIC"
  "BETWEEN"
  "BLOCK"
  "BOTH"
  "BREADTH"
  "BULK"
  "BY"
  "BYTE"
  "CASE"
  "CHAR"
  "CHARACTER"
  "CHECK"
  "COLLATE"
  "COLLECT"
  "COLUMNS"
  "COLUMN_VALUE"
  "CONDITIONAL"
  "CONNECT"
  "CONNECT_BY_ROOT"
  "CONSTRAINT"
  "CONVERSION"
  "COST"
  "COUNT"
  "CUBE"
  "CURRENT"
  "CURSOR"
  "CYCLE"
  "DANGLING"
  "DATE"
  "DAY"
  "DECIMAL"
  "DECREMENT"
  "DEFAULT"
  "DEFINE"
  "DENSE_RANK"
  "DEPTH"
  "DESC"
  "DIMENSION"
  "DISALLOW"
  "DISTINCT"
  "DOUBLE"
  "ELSE"
  "EMPTY"
  "END"
  "ERROR"
  "ESCAPE"
  "EXCEPT"
  "EXCLUDE"
  "EXISTS"
  "FALSE"
  "FETCH"
  "FINAL"
  "FIRST"
  "FLOAT"
  "FOLLOWING"
  "FOR"
  "FORMAT"
  "FROM"
  "GROUP"
  "GROUPING"
  "GROUPS"
  "HAVING"
  "HOUR"
  "IGNORE"
  "IN"
  "INCLUDE"
  "INCREMENT"
  "INFINITE"
  "INTEGER"
  "INTERSECT"
  "INTERVAL"
  "INTO"
  "IS"
  "ITERATE"
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
  "LOCAL"
  "LOCKED"
  "LONG"
  "MAIN"
  "MATCH"
  "MATCHES"
  "MATCH_RECOGNIZE"
  "MEASURES"
  "MEMBER"
  "MINUS"
  "MINUTE"
  "MISMATCH"
  "MODEL"
  "MONTH"
  "MULTISET"
  "NAN"
  "NATIONAL"
  "NAV"
  "NCHAR"
  "NESTED"
  "NESTED_TABLE_ID"
  "NEXT"
  "NO"
  "NOCYCLE"
  "NOT"
  "NOWAIT"
  "NULL"
  "NULLS"
  "NUMBER"
  "OBJECT"
  "OF"
  "OFFSET"
  "OMIT"
  "ON"
  "ONE"
  "ONLY"
  "OPTION"
  "OR"
  "ORDER"
  "ORDINALITY"
  "OTHERS"
  "OVER"
  "OVERFLOW"
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
  "PRECEDING"
  "PRECISION"
  "PRESENT"
  "PRETTY"
  "PRIOR"
  "RANGE"
  "RAW"
  "READ"
  "REF"
  "REFERENCE"
  "RESPECT"
  "RETURN"
  "RETURNING"
  "ROLLUP"
  "ROW"
  "ROWID"
  "ROWNUM"
  "ROWS"
  "ROWTYPE"
  "RULES"
  "RUNNING"
  "SAMPLE"
  "SCALAR"
  "SCALARS"
  "SCN"
  "SEARCH"
  "SECOND"
  "SEED"
  "SELECT"
  "SEQUENCE"
  "SEQUENTIAL"
  "SET"
  "SETS"
  "SHOW"
  "SIBLINGS"
  "SINGLE"
  "SKIP"
  "SMALLINT"
  "SOME"
  "START"
  "STRICT"
  "SUBMULTISET"
  "SUBPARTITION"
  "SUBSET"
  "SYSDATE"
  "TABLE"
  "THEN"
  "TIES"
  "TIME"
  "TIMESTAMP"
  "TO"
  "TRAILING"
  "TRUE"
  "TRUNCATE"
  "TYPE"
  "UID"
  "UNBOUNDED"
  "UNCONDITIONAL"
  "UNION"
  "UNIQUE"
  "UNMATCHED"
  "UNPIVOT"
  "UNTIL"
  "UPDATE"
  "UPDATED"
  "UPSERT"
  "USER"
  "USING"
  "VALUE"
  "VALUES"
  "VARCHAR"
  "VARCHAR2"
  "VARYING"
  "VERSIONS"
  "WAIT"
  "WHEN"
  "WHERE"
  "WINDOW"
  "WITH"
  "WITHIN"
  "WITHOUT"
  "WRAPPER"
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
