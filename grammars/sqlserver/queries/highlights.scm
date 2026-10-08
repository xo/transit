; xo wrote this file, because the repository of the grammar has no highlight
; query (D113). It uses only the capture names of styles/captures.txt.
;
; The grammar hides some keywords and the dots of a name in tokens that a
; query cannot capture, such as FROM, OVER and ORDER BY. A pattern captures
; the node that holds such a token, and the patterns of its children color
; the rest of it. No two patterns capture the same node.

; Nodes that hold a keyword that the grammar hides

[
  (go_statement)
  (query_specification)
  (over_clause)
  (partition_by_clause)
  (order_by_clause)
  (collation_)
  (row_or_range_clause)
  (window_frame_extent)
] @keyword

; Nodes that hold the hidden dots of a name

[
  (full_table_name)
  (func_proc_name_server_database_schema)
  (func_proc_name_database_schema)
  (func_proc_name_schema)
  (partition_function)
  (udt_elem)
  (hierarchyid_static_method)
] @punctuation.delimiter

; A node that holds the hidden "=" of an alias, as in "SELECT n = id"

(expression_elem
  leftAlias: (column_alias)) @operator

; Keywords

[
  (AS)
  (as)
  (AT_KEYWORD)
  (LOGIN)
  (NONE)
  (OUTPUT)
  (RECOMPILE)
  (RESULT_SETS)
  (UNDEFINED)
  (USER)
  (WITH)
  (all_)
  (asc_)
  (default)
  (desc_)
  (distinct_)
  (dollar_partition_)
  (execute)
  (ignore_nulls_)
  (keyword)
  (over_)
  (respect_nulls_)
  (select)
  (window_frame_following)
  (window_frame_preceding)
  (within_group_)
] @keyword

; Functions that the grammar knows by name

[
  (avg_)
  (binary_checksum_)
  (checksum_)
  (checksum_agg_)
  (count_)
  (count_big_)
  (cume_dist_)
  (dense_rank_)
  (first_value_)
  (get_descendant_)
  (get_reparented_value_)
  (getancestor_)
  (getlevel_)
  (getroot_)
  (is_descendant_of_)
  (lag_)
  (last_value_)
  (lead_)
  (left_)
  (max_)
  (min_)
  (ntile_)
  (parse_)
  (percent_rank_)
  (percentile_cont_)
  (percentile_disc_)
  (rank_)
  (right_)
  (row_number_)
  (stdev_)
  (stdevp_)
  (sum_)
  (tostring_)
  (var_)
  (varp_)
] @function.builtin

(hierachyid_) @type.builtin

; Literals

(null_) @constant.builtin

(string_lit) @string

((constant) @string
  (#match? @string "^N?'"))

((constant) @number
  (#match? @number "^-?[0-9]"))

[
  (integer)
  (decimal_)
  (binary)
  (money_)
] @number

[
  (float_)
  (real_)
] @number.float

; Variables and names

(LOCAL_ID_) @variable

(execute_statement_arg_named) @variable.parameter

(full_table_name
  [
    server: (id_)
    database: (id_)
    schema: (id_)
  ] @namespace)

(full_table_name
  table: (id_) @type)

(full_column_name
  (id_) @field)

(column_alias
  (id_) @variable)

(collation_
  collation_name: (id_) @constant)

(execute_body
  linkedServer: (id_) @namespace)

(execute_parameter
  (id_) @variable)

(func_proc_name_server_database_schema
  [
    server: (id_)
    database: (id_)
    schema: (id_)
  ] @namespace)

(func_proc_name_server_database_schema
  procedure: (id_) @function.call)

(func_proc_name_database_schema
  [
    database: (id_)
    schema: (id_)
  ] @namespace)

(func_proc_name_database_schema
  procedure: (id_) @function.call)

(func_proc_name_schema
  schema: (id_) @namespace)

(func_proc_name_schema
  procedure: (id_) @function.call)

(partition_function
  database: (id_) @namespace)

(partition_function
  func_name: (id_) @function.call)

(udt_elem
  udt_column_name: (id_) @field)

(udt_elem
  non_static_attr: (id_) @function.method.call)

(hierarchyid_static_method
  (id_) @variable)

; Operators

[
  "+"
  "-"
  "="
  "+="
  "-="
  "*="
  "/="
  "%="
  "&="
  "^="
  "|="
  "::"
  (PLUS)
  (assignment_operator)
  (asterisk)
] @operator

; Punctuation

[
  "("
  ")"
] @punctuation.bracket

[
  ","
  ";"
] @punctuation.delimiter
