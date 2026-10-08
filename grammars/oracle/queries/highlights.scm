; xo wrote this file, because the repository of the grammar has no highlight
; query (D113). It uses only the capture names of styles/captures.txt.
;
; Each identifier is a variable, and a later pattern gives an identifier of
; a known role its own color. When two patterns capture one node, the last
; one counts, as the highlighter of upstream keeps it (D80).

(identifier) @variable

; Comments

[
  (comment_sl)
  (comment_ml)
] @comment

; Literals

(literal_string) @string

(number) @number

(float) @number.float

(kw_null) @constant.builtin

[
  (kw_true)
  (kw_false)
] @boolean

; Types

[
  (kw_anydata)
  (kw_anydataset)
  (kw_anytype)
  (kw_bfile)
  (kw_binary_double)
  (kw_binary_float)
  (kw_binary_integer)
  (kw_blob)
  (kw_boolean)
  (kw_char)
  (kw_clob)
  (kw_date)
  (kw_dec)
  (kw_decimal)
  (kw_double)
  (kw_float)
  (kw_int)
  (kw_integer)
  (kw_json_array_t)
  (kw_json_element_t)
  (kw_json_key_list)
  (kw_json_object_t)
  (kw_json_scalar_t)
  (kw_long)
  (kw_natural)
  (kw_naturaln)
  (kw_nchar)
  (kw_nclob)
  (kw_number)
  (kw_numeric)
  (kw_nvarchar2)
  (kw_pls_integer)
  (kw_positive)
  (kw_positiven)
  (kw_raw)
  (kw_real)
  (kw_rowid)
  (kw_sdo_geometry)
  (kw_sdo_georaster)
  (kw_sdo_topo_geometry)
  (kw_signtype)
  (kw_simple_double)
  (kw_simple_float)
  (kw_simple_integer)
  (kw_smallint)
  (kw_string)
  (kw_timestamp)
  (kw_uritype)
  (kw_urowid)
  (kw_varchar)
  (kw_varchar2)
  (kw_xmltype)
] @type.builtin

[
  (kw_datatype_rowtype)
  (kw_datatype_type)
] @type.qualifier

(datatype
  (referenced_element
    ref_name: (identifier) @type))

; Keywords

[
  (kw_and)
  (kw_or)
  (kw_not)
  (kw_in)
  (kw_like)
  (kw_between)
  (kw_is)
] @keyword.operator

[
  (kw_if)
  (kw_then)
  (kw_else)
  (kw_elsif)
  (kw_case)
  (kw_when)
] @keyword.conditional

[
  (kw_loop)
  (kw_while)
  (kw_for)
  (kw_exit)
  (kw_continue)
] @keyword.repeat

(kw_return) @keyword.return

[
  (kw_function)
  (kw_procedure)
] @keyword.function

[
  (kw_exception)
  (kw_raise)
] @keyword.exception

[
  (kw_access)
  (kw_accessible)
  (kw_add)
  (kw_after)
  (kw_agent)
  (kw_aggregate)
  (kw_all)
  (kw_alter)
  (kw_analytic)
  (kw_analyze)
  (kw_any)
  (kw_apply)
  (kw_array)
  (kw_as)
  (kw_asc)
  (kw_associate)
  (kw_attribute)
  (kw_audit)
  (kw_authid)
  (kw_badfile)
  (kw_batch)
  (kw_before)
  (kw_begin)
  (kw_block)
  (kw_body)
  (kw_breadth)
  (kw_bulk)
  (kw_by)
  (kw_byte)
  (kw_c)
  (kw_cascade)
  (kw_character)
  (kw_charsetfrom)
  (kw_charsetid)
  (kw_check)
  (kw_clone)
  (kw_close)
  (kw_cluster)
  (kw_collation)
  (kw_collect)
  (kw_comment)
  (kw_commit)
  (kw_commtted)
  (kw_compile)
  (kw_compound)
  (kw_connect)
  (kw_constant)
  (kw_constraint)
  (kw_constructor)
  (kw_container)
  (kw_containers)
  (kw_context)
  (kw_convert)
  (kw_create)
  (kw_cross)
  (kw_crossedition)
  (kw_current_user)
  (kw_cursor)
  (kw_cycle)
  (kw_data)
  (kw_database)
  (kw_day)
  (kw_db_role_change)
  (kw_ddl)
  (kw_debug)
  (kw_declare)
  (kw_default)
  (kw_definer)
  (kw_delete)
  (kw_deleting)
  (kw_depth)
  (kw_desc)
  (kw_deterministic)
  (kw_directory)
  (kw_disable)
  (kw_disassociate)
  (kw_discardfile)
  (kw_distinct)
  (kw_drop)
  (kw_duration)
  (kw_each)
  (kw_editionable)
  (kw_element)
  (kw_enable)
  (kw_end)
  (kw_errors)
  (kw_exceptions)
  (kw_execute)
  (kw_exists)
  (kw_extend)
  (kw_external)
  (kw_fact)
  (kw_fetch)
  (kw_filter)
  (kw_final)
  (kw_first)
  (kw_follows)
  (kw_force)
  (kw_forward)
  (kw_found)
  (kw_from)
  (kw_full)
  (kw_goto)
  (kw_grant)
  (kw_group)
  (kw_hash)
  (kw_having)
  (kw_hierarchies)
  (kw_immediate)
  (kw_immutable)
  (kw_including)
  (kw_index)
  (kw_indicator)
  (kw_indices)
  (kw_inner)
  (kw_insert)
  (kw_inserting)
  (kw_instantiable)
  (kw_instead)
  (kw_intersect)
  (kw_interval)
  (kw_into)
  (kw_invalidate)
  (kw_isolation)
  (kw_isopen)
  (kw_java)
  (kw_join)
  (kw_language)
  (kw_last)
  (kw_lateral)
  (kw_left)
  (kw_length)
  (kw_level)
  (kw_library)
  (kw_limit)
  (kw_local)
  (kw_location)
  (kw_lock)
  (kw_log)
  (kw_logfile)
  (kw_logoff)
  (kw_logon)
  (kw_map)
  (kw_matched)
  (kw_maxlen)
  (kw_measure)
  (kw_measures)
  (kw_member)
  (kw_merge)
  (kw_metadata)
  (kw_minus)
  (kw_mode)
  (kw_modify)
  (kw_month)
  (kw_mutable)
  (kw_name)
  (kw_nested)
  (kw_new)
  (kw_next)
  (kw_noaudit)
  (kw_nocopy)
  (kw_nocycle)
  (kw_none)
  (kw_noneditionable)
  (kw_notfound)
  (kw_nowait)
  (kw_nulls)
  (kw_object)
  (kw_of)
  (kw_offset)
  (kw_oid)
  (kw_old)
  (kw_on)
  (kw_only)
  (kw_open)
  (kw_option)
  (kw_order)
  (kw_others)
  (kw_out)
  (kw_outer)
  (kw_overriding)
  (kw_package)
  (kw_pairs)
  (kw_parallel_enable)
  (kw_parameters)
  (kw_parent)
  (kw_partition)
  (kw_percent)
  (kw_persistable)
  (kw_pipe)
  (kw_pipelined)
  (kw_pluggable)
  (kw_precedes)
  (kw_precision)
  (kw_prior)
  (kw_range)
  (kw_read)
  (kw_record)
  (kw_reference)
  (kw_referencing)
  (kw_reject)
  (kw_relies_on)
  (kw_rename)
  (kw_repeat)
  (kw_replace)
  (kw_reset)
  (kw_result)
  (kw_result_cache)
  (kw_returning)
  (kw_reuse)
  (kw_reverse)
  (kw_revoke)
  (kw_right)
  (kw_rollback)
  (kw_row)
  (kw_rowcount)
  (kw_rows)
  (kw_sample)
  (kw_savepoint)
  (kw_schema)
  (kw_search)
  (kw_second)
  (kw_seed)
  (kw_segment)
  (kw_select)
  (kw_self)
  (kw_serializable)
  (kw_servererror)
  (kw_set)
  (kw_settings)
  (kw_shards)
  (kw_shutdown)
  (kw_siblings)
  (kw_specification)
  (kw_start)
  (kw_startup)
  (kw_statement)
  (kw_static)
  (kw_statistics)
  (kw_struct)
  (kw_subpartition)
  (kw_substitutable)
  (kw_subtype)
  (kw_suspend)
  (kw_sys)
  (kw_table)
  (kw_tdo)
  (kw_ties)
  (kw_time)
  (kw_to)
  (kw_transaction)
  (kw_trigger)
  (kw_trim)
  (kw_truncate)
  (kw_type)
  (kw_under)
  (kw_union)
  (kw_unique)
  (kw_unlimited)
  (kw_unplug)
  (kw_update)
  (kw_updating)
  (kw_use)
  (kw_using)
  (kw_using_nls_comp)
  (kw_validate)
  (kw_value)
  (kw_values)
  (kw_varray)
  (kw_varying)
  (kw_view)
  (kw_wait)
  (kw_where)
  (kw_with)
  (kw_work)
  (kw_write)
  (kw_year)
  (kw_zone)
] @keyword

(kw_count) @function.builtin

; Names

(referenced_element
  schema_name: (identifier) @namespace)

(referenced_element
  remote_name: (identifier) @namespace)

(table_list_element
  (referenced_element
    ref_name: (identifier) @type))

(join_clause
  (referenced_element
    ref_name: (identifier) @type))

(ref_call
  (referenced_element
    ref_name: (identifier) @function.call))

[
  (alter_function obj_name: (identifier) @function)
  (alter_procedure obj_name: (identifier) @function)
  (create_function fnc_name: (identifier) @function)
  (create_procedure prc_name: (identifier) @function)
  (element_spec_function_spec fnc_name: (identifier) @function)
  (element_spec_procedure_spec prc_name: (identifier) @function)
  (func_decl_in_type fnc_name: (identifier) @function)
  (function_declaration fnc_name: (identifier) @function)
  (function_definition fnc_name: (identifier) @function)
  (proc_decl_in_type prc_name: (identifier) @function)
  (procedure_declaration prc_name: (identifier) @function)
  (procedure_definition prc_name: (identifier) @function)
]

[
  (alter_function schema_name: (identifier) @namespace)
  (alter_library schema_name: (identifier) @namespace)
  (alter_package schema_name: (identifier) @namespace)
  (alter_procedure schema_name: (identifier) @namespace)
  (alter_trigger schema_name: (identifier) @namespace)
  (alter_type schema_name: (identifier) @namespace)
  (create_function schema_name: (identifier) @namespace)
  (create_package schema_name: (identifier) @namespace)
  (create_package_body schema_name: (identifier) @namespace)
  (create_procedure schema_name: (identifier) @namespace)
  (create_trigger schema_name: (identifier) @namespace)
  (create_type schema_name: (identifier) @namespace)
  (create_type_body schema_name: (identifier) @namespace)
  (alter_library obj_name: (identifier) @module)
  (alter_package obj_name: (identifier) @module)
  (create_package package_name: (identifier) @module)
  (create_package_body package_name: (identifier) @module)
  (library_name library: (identifier) @module)
]

[
  (alter_type type_name: (identifier) @type)
  (create_type type_name: (identifier) @type)
  (create_type_body type_name: (identifier) @type)
  (type_definition_collection type_collection_name: (identifier) @type)
  (type_definition_record type_rec_name: (identifier) @type)
  (type_definition_sub subtype_name: (identifier) @type)
]

[
  (alter_trigger trigger_name: (identifier) @function)
  (create_trigger trigger_name: (identifier) @function)
]

(field_definition
  (identifier) @field)

[
  (parameter_name (identifier) @variable.parameter)
  (parameter_declaration_element (identifier) @variable.parameter)
  (cursor_declaration_parameter (identifier) @variable.parameter)
  (compiler_parameter_clause compile_parameter_name: (identifier) @variable.parameter)
]

(host_variable
  (identifier) @variable.parameter)

(indicator_variable
  (identifier) @variable.parameter)

(label
  (identifier) @label)

(java_declaration
  name: (literal_string) @string)

; Operators

[
  "!="
  "%"
  "*"
  "**"
  "+"
  "-"
  "/"
  ":="
  "<"
  "<="
  "<>"
  "="
  "=>"
  ">"
  ">="
  "^="
  "||"
  "~="
  ".."
  (kw_asterisk)
] @operator

; Punctuation

[
  "("
  ")"
] @punctuation.bracket

[
  "<<"
  ">>"
  "@"
] @punctuation.special

[
  ","
  ";"
  "."
  ":"
] @punctuation.delimiter
