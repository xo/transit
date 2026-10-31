/**
 * @file The grammar of the SQL of PostgreSQL 19
 * @license MIT
 */

// xo writes this grammar from the reference manual of PostgreSQL 19 (D122).
// It does not come from the Bison grammar of PostgreSQL, and it does not
// copy its keyword classes.
//
// The grammar is for a highlighter and for completion, so it accepts more
// than the server does where that keeps the rules small. A keyword is a
// token in any case, and it is an anonymous node with the name of the
// keyword in upper case, such as "SELECT". The token identifier is the word
// of the grammar, so a keyword that PostgreSQL does not reserve is an
// identifier in each place where the grammar does not expect the keyword.
// The reserved words of PostgreSQL are in the reserved set global, so they
// are not identifiers, but for a label after AS or after a dot, where
// PostgreSQL accepts every keyword. A function name can be a keyword that
// PostgreSQL reserves only for column names, such as left.
//
// The external scanner in src/scanner.c reads the comments, which nest, and
// the dollar-quoted strings. The variables of psql belong to the usql
// grammar, which hands this grammar a placeholder of the same length in
// place of each variable (D101). A statement ends at a semicolon or at the
// end of the input.

/// <reference types="tree-sitter-cli/dsl" />
// @ts-check

// The precedences of the operators, from the lowest to the highest, as the
// section "Operator Precedence" of the manual lists them.
const PREC = {
  or: 1,
  and: 2,
  not: 3,
  is: 4,
  compare: 5,
  like: 6,
  other: 7,
  add: 8,
  multiply: 9,
  exponent: 10,
  at: 11,
  collate: 12,
  unary: 13,
  subscript: 14,
  cast: 15,
  field: 16,
};

// The words that PostgreSQL 19 reserves, from appendix C of the manual. A
// word is in the list only when the grammar uses it as a keyword, because a
// reserved word must be a token of the grammar. A word that PostgreSQL
// reserves but accepts as a function name or a type name is in
// TYPE_FUNC_NAME_WORDS.
const RESERVED_WORDS = [
  'ALL', 'ANALYSE', 'ANALYZE', 'AND', 'ANY', 'ARRAY', 'AS', 'ASC', 'ASYMMETRIC',
  'BOTH', 'CASE', 'CAST', 'CHECK', 'COLLATE', 'COLUMN', 'CONSTRAINT', 'CREATE',
  'CURRENT_CATALOG',
  'CURRENT_DATE', 'CURRENT_ROLE', 'CURRENT_TIME', 'CURRENT_TIMESTAMP',
  'CURRENT_USER', 'DEFAULT', 'DEFERRABLE', 'DESC', 'DISTINCT', 'DO',
  'ELSE', 'END', 'EXCEPT', 'FALSE', 'FETCH', 'FOR', 'FOREIGN', 'FROM', 'GRANT', 'GROUP',
  'HAVING', 'IN', 'INITIALLY', 'INTERSECT', 'INTO', 'LATERAL', 'LEADING', 'LIMIT',
  'LOCALTIME', 'LOCALTIMESTAMP', 'NOT', 'NULL', 'OFFSET', 'ON', 'ONLY', 'OR',
  'ORDER', 'PLACING', 'PRIMARY', 'REFERENCES', 'RETURNING', 'SELECT', 'SESSION_USER', 'SOME',
  'SYMMETRIC', 'SYSTEM_USER', 'TABLE', 'THEN', 'TO', 'TRAILING', 'TRUE',
  'UNION', 'UNIQUE', 'USER', 'USING', 'VARIADIC', 'WHEN', 'WHERE', 'WINDOW', 'WITH',
];

const TYPE_FUNC_NAME_WORDS = [
  'AUTHORIZATION', 'BINARY', 'COLLATION', 'CONCURRENTLY', 'CROSS', 'CURRENT_SCHEMA', 'FREEZE',
  'FULL', 'ILIKE', 'INNER', 'IS', 'ISNULL', 'JOIN', 'LEFT', 'LIKE', 'NATURAL',
  'NOTNULL', 'OUTER', 'OVERLAPS', 'RIGHT', 'SIMILAR', 'TABLESAMPLE', 'VERBOSE',
];

/**
 * Returns a keyword: a token that matches word in any case, as an
 * anonymous node with the name word.
 *
 * @param {string} word
 */
function kw(word) {
  return alias(keyword(word), word);
}

/**
 * Returns the token of a keyword, which matches word in any case, with no
 * alias.
 *
 * @param {string} word
 */
function keyword(word) {
  let pattern = '';
  for (const c of word) {
    const lower = c.toLowerCase();
    const upper = c.toUpperCase();
    pattern += lower === upper ? c : `[${lower}${upper}]`;
  }
  return new RegExp(pattern);
}

/**
 * Returns the tokens of the keywords words, as the node identifier.
 *
 * @param {GrammarSymbols<string>} $
 * @param {...string} words
 */
function keywordName($, ...words) {
  const tokens = words.map(keyword);
  return alias(tokens.length === 1 ? tokens[0] : choice(...tokens), $.identifier);
}

/**
 * Returns a sequence of keywords.
 *
 * @param {...string} words
 */
function kws(...words) {
  return seq(...words.map(kw));
}

/**
 * Returns one or more of rule, with a comma between two of them.
 *
 * @param {RuleOrLiteral} rule
 */
function commaSep1(rule) {
  return seq(rule, repeat(seq(',', rule)));
}

/**
 * Returns rule in parentheses.
 *
 * @param {RuleOrLiteral} rule
 */
function parens(rule) {
  return seq('(', rule, ')');
}

module.exports = grammar({
  name: 'postgres',

  extras: $ => [
    /\s/,
    $.comment,
  ],

  // The order of the external tokens is the order of enum TokenType in
  // src/scanner.c.
  externals: $ => [
    $.comment,
    $._dollar_quote_start,
    $.string_content,
    $._dollar_quote_end,
  ],

  word: $ => $.identifier,

  // The reserved set global holds the words that PostgreSQL reserves. The
  // set function_name leaves out the words that PostgreSQL accepts as a
  // function name, and the set label is empty, because PostgreSQL accepts
  // every keyword as a label after AS and after a dot. A state takes the
  // last set of its items, so the sets go from the strictest to the least
  // strict.
  reserved: {
    global: _ => [...RESERVED_WORDS, ...TYPE_FUNC_NAME_WORDS].map(keyword),
    function_name: _ => RESERVED_WORDS.map(keyword),
    label: _ => [],
  },

  // Each hidden rule that an expression can start with is a column of each
  // state where an expression starts. Such a state with more than 64
  // columns takes the large form, so these rules are inlined. The rule
  // _operand is inlined so that the precedence of the operator before it
  // decides where its expression ends, as in a = 1 AND b.
  inline: $ => [
    $._name,
    $._label,
    $._literal,
    $._literal_type,
    $._function_name,
    $._primary_expression,
    $._operand,
  ],

  conflicts: $ => [
    // A parenthesized query can be the left side of a set operation, or a
    // subquery in an expression or in FROM.
    [$.parenthesized_query, $.subquery],
    [$.join],
    // The first list of CREATE AGGREGATE can be the types of the arguments
    // or, in the old form, the definitions.
    [$.definition, $.type],
    // The action of a rule in parentheses can be a list of statements or a
    // parenthesized query.
    [$.select_statement, $.parenthesized_query],
  ],

  rules: {
    source_file: $ => seq(
      repeat(choice(seq($._statement, ';'), ';')),
      optional($._statement),
    ),

    _statement: $ => choice(
      $.select_statement,
      $.insert_statement,
      $.update_statement,
      $.delete_statement,
      $.merge_statement,
      $.copy_statement,
      $.truncate_statement,
      $.lock_statement,
      $.begin_statement,
      $.commit_statement,
      $.rollback_statement,
      $.savepoint_statement,
      $.release_statement,
      $.prepare_transaction_statement,
      $.prepare_statement,
      $.execute_statement,
      $.deallocate_statement,
      $.declare_statement,
      $.fetch_statement,
      $.close_statement,
      $.call_statement,
      $.do_statement,
      $.explain_statement,
      $.set_statement,
      $.show_statement,
      $.reset_statement,
      $.listen_statement,
      $.notify_statement,
      $.unlisten_statement,
      $.vacuum_statement,
      $.analyze_statement,
      $.cluster_statement,
      $.repack_statement,
      $.reindex_statement,
      $.checkpoint_statement,
      $.discard_statement,
      $.load_statement,
      $.create_table_statement,
      $.create_table_as_statement,
      $.alter_table_statement,
      $.create_index_statement,
      $.alter_index_statement,
      $.create_view_statement,
      $.create_materialized_view_statement,
      $.refresh_materialized_view_statement,
      $.alter_view_statement,
      $.create_sequence_statement,
      $.alter_sequence_statement,
      $.create_schema_statement,
      $.alter_schema_statement,
      $.create_database_statement,
      $.alter_database_statement,
      $.create_type_statement,
      $.alter_type_statement,
      $.create_domain_statement,
      $.alter_domain_statement,
      $.create_extension_statement,
      $.alter_extension_statement,
      $.drop_statement,
      $.comment_statement,
      $.security_label_statement,
      $.create_function_statement,
      $.alter_function_statement,
      $.create_aggregate_statement,
      $.create_operator_statement,
      $.create_operator_class_statement,
      $.create_operator_family_statement,
      $.alter_operator_statement,
      $.create_trigger_statement,
      $.alter_trigger_statement,
      $.create_event_trigger_statement,
      $.alter_event_trigger_statement,
      $.create_rule_statement,
      $.alter_rule_statement,
      $.create_policy_statement,
      $.alter_policy_statement,
      $.create_role_statement,
      $.alter_role_statement,
      $.grant_statement,
      $.revoke_statement,
      $.alter_default_privileges_statement,
      $.reassign_owned_statement,
      $.drop_owned_statement,
      $.create_publication_statement,
      $.alter_publication_statement,
      $.create_subscription_statement,
      $.alter_subscription_statement,
      $.create_foreign_data_wrapper_statement,
      $.alter_foreign_data_wrapper_statement,
      $.create_server_statement,
      $.alter_server_statement,
      $.create_user_mapping_statement,
      $.alter_user_mapping_statement,
      $.import_foreign_schema_statement,
      $.create_tablespace_statement,
      $.create_collation_statement,
      $.create_conversion_statement,
      $.create_text_search_statement,
      $.create_statistics_statement,
      $.create_cast_statement,
      $.create_language_statement,
      $.create_transform_statement,
      $.create_access_method_statement,
      $.alter_object_statement,
      $.alter_system_statement,
      $.wait_statement,
    ),

    // The statements that change rows: INSERT, UPDATE, DELETE and MERGE. A
    // WITH clause can come before each of them, and each of them can be the
    // query of a common table expression.
    _modifying_statement: $ => choice(
      $.insert_statement,
      $.update_statement,
      $.delete_statement,
      $.merge_statement,
    ),

    // The statements that EXPLAIN and PREPARE take.
    _explainable_statement: $ => choice(
      $.select_statement,
      $._modifying_statement,
      $.declare_statement,
      $.execute_statement,
      $.create_table_as_statement,
      $.create_materialized_view_statement,
      $.refresh_materialized_view_statement,
    ),

    // Queries

    select_statement: $ => $._query,

    // Data manipulation

    // The target of INSERT can have an alias only after AS, because VALUES
    // can come after the name.
    insert_statement: $ => seq(
      optional($.with_clause),
      kw('INSERT'),
      kw('INTO'),
      field('table', $.object_reference),
      optional(seq(kw('AS'), field('alias', $._label))),
      optional($._insert_columns),
      optional($._overriding),
      choice(
        kws('DEFAULT', 'VALUES'),
        field('query', $._query),
      ),
      optional($.on_conflict_clause),
      optional($.returning_clause),
    ),

    // A column of INSERT can be a field or an element of a column.
    _insert_columns: $ => alias($._insert_column_list, $.column_list),

    _insert_column_list: $ => parens(commaSep1($._assignment_target)),

    _overriding: _ => seq(
      kw('OVERRIDING'),
      choice(kw('SYSTEM'), kw('USER')),
      kw('VALUE'),
    ),

    on_conflict_clause: $ => seq(
      kw('ON'),
      kw('CONFLICT'),
      optional($.conflict_target),
      kw('DO'),
      choice(
        kw('NOTHING'),
        seq(kw('UPDATE'), $.set_clause, optional($.where_clause)),
        seq(
          kw('SELECT'),
          optional(seq(kw('FOR'), $._lock_strength)),
          optional($.where_clause),
        ),
      ),
    ),

    conflict_target: $ => choice(
      seq(parens(commaSep1($.index_element)), optional($.where_clause)),
      seq(kw('ON'), kw('CONSTRAINT'), field('constraint', $._name)),
    ),

    // A column or an expression of an index, with its collation and its
    // operator class, which can take parameters.
    index_element: $ => seq(
      choice(
        field('column', $._name),
        field('expression', choice($.function_call, $.parenthesized_expression)),
      ),
      optional(seq(kw('COLLATE'), field('collation', $.object_reference))),
      optional(seq(field('operator_class', $.object_reference), optional($.option_list))),
      optional(choice(kw('ASC'), kw('DESC'))),
      optional(seq(kw('NULLS'), choice(kw('FIRST'), kw('LAST')))),
    ),

    // RETURNING can name the old row and the new row with WITH.
    returning_clause: $ => seq(
      kw('RETURNING'),
      optional(seq(
        kw('WITH'),
        parens(commaSep1(seq(
          choice(kw('OLD'), kw('NEW')),
          kw('AS'),
          field('alias', $._name),
        ))),
      )),
      commaSep1($._select_item),
    ),

    update_statement: $ => seq(
      optional($.with_clause),
      kw('UPDATE'),
      field('table', $.table_reference),
      $.set_clause,
      optional($.from_clause),
      optional($.where_clause),
      optional($.returning_clause),
    ),

    set_clause: $ => seq(kw('SET'), commaSep1($.assignment)),

    // A list of columns takes a list of values, in which DEFAULT can stand
    // for a value, or a subquery.
    assignment: $ => choice(
      seq(
        $._assignment_target,
        '=',
        field('value', choice($._expression, kw('DEFAULT'))),
      ),
      seq(
        parens(commaSep1($._assignment_target)),
        '=',
        field('value', choice(
          seq(optional(kw('ROW')), $.value_list),
          $.subquery,
        )),
      ),
    ),

    // A column, or a field or an element of a column.
    _assignment_target: $ => seq(
      field('column', $._name),
      repeat(choice(
        seq('.', field('field', $._label)),
        seq(
          '[',
          optional(field('index', $._expression)),
          optional(seq(':', optional(field('upper', $._expression)))),
          ']',
        ),
      )),
    ),

    delete_statement: $ => seq(
      optional($.with_clause),
      kw('DELETE'),
      kw('FROM'),
      field('table', $.table_reference),
      optional(seq(kw('USING'), commaSep1($._table_reference))),
      optional($.where_clause),
      optional($.returning_clause),
    ),

    merge_statement: $ => seq(
      optional($.with_clause),
      kw('MERGE'),
      kw('INTO'),
      field('table', $.table_reference),
      kw('USING'),
      field('source', $._table_reference),
      kw('ON'),
      field('condition', $._expression),
      repeat1($.merge_when_clause),
      optional($.returning_clause),
    ),

    merge_when_clause: $ => seq(
      kw('WHEN'),
      choice(
        kw('MATCHED'),
        seq(
          kw('NOT'),
          kw('MATCHED'),
          optional(seq(kw('BY'), choice(kw('SOURCE'), kw('TARGET')))),
        ),
      ),
      optional(seq(kw('AND'), field('condition', $._expression))),
      kw('THEN'),
      choice(
        $.merge_insert,
        $.merge_update,
        $.merge_delete,
        kws('DO', 'NOTHING'),
      ),
    ),

    merge_insert: $ => seq(
      kw('INSERT'),
      optional($._insert_columns),
      optional($._overriding),
      choice(
        seq(kw('VALUES'), $.value_list),
        kws('DEFAULT', 'VALUES'),
      ),
    ),

    merge_update: $ => seq(kw('UPDATE'), $.set_clause),

    merge_delete: _ => kw('DELETE'),

    // COPY has the options of PostgreSQL 9.0 and later in parentheses, and
    // the older options with no parentheses.
    copy_statement: $ => seq(
      kw('COPY'),
      optional(kw('BINARY')),
      choice(
        seq(field('table', $.object_reference), optional($.column_list)),
        parens(field('query', choice($._query, $._modifying_statement))),
      ),
      choice(kw('FROM'), kw('TO')),
      choice(
        seq(optional(kw('PROGRAM')), field('file', $.string)),
        kw('STDIN'),
        kw('STDOUT'),
      ),
      optional(seq(optional(kw('USING')), kw('DELIMITERS'), $.string)),
      optional(kw('WITH')),
      optional(choice(
        $.option_list,
        seq($.copy_option, repeat(seq(optional(','), $.copy_option))),
      )),
      optional($.where_clause),
    ),

    // An option of COPY in the old form, such as CSV, DELIMITER ',' or
    // FORCE QUOTE *. The words of such an option are not keywords.
    copy_option: $ => prec.right(seq(
      field('name', $._label),
      optional(kw('AS')),
      optional(field('value', choice($.string, '*'))),
    )),

    truncate_statement: $ => seq(
      kw('TRUNCATE'),
      optional(kw('TABLE')),
      commaSep1($._relation),
      optional(seq(choice(kw('RESTART'), kw('CONTINUE')), kw('IDENTITY'))),
      optional($._drop_behavior),
    ),

    _drop_behavior: _ => choice(kw('CASCADE'), kw('RESTRICT')),

    // A table, with ONLY or * for its descendant tables.
    _relation: $ => seq(
      optional(kw('ONLY')),
      field('table', $.object_reference),
      optional('*'),
    ),

    lock_statement: $ => seq(
      kw('LOCK'),
      optional(kw('TABLE')),
      commaSep1($._relation),
      optional(seq(kw('IN'), $.lock_mode, kw('MODE'))),
      optional(kw('NOWAIT')),
    ),

    lock_mode: _ => choice(
      kws('ACCESS', 'SHARE'),
      kws('ROW', 'SHARE'),
      kws('ROW', 'EXCLUSIVE'),
      kws('SHARE', 'UPDATE', 'EXCLUSIVE'),
      kw('SHARE'),
      kws('SHARE', 'ROW', 'EXCLUSIVE'),
      kw('EXCLUSIVE'),
      kws('ACCESS', 'EXCLUSIVE'),
    ),

    // Transactions

    begin_statement: $ => seq(
      choice(
        seq(kw('BEGIN'), optional($._transaction)),
        kws('START', 'TRANSACTION'),
      ),
      optional($._transaction_modes),
    ),

    _transaction: _ => choice(kw('WORK'), kw('TRANSACTION')),

    _transaction_modes: $ => seq(
      $.transaction_mode,
      repeat(seq(optional(','), $.transaction_mode)),
    ),

    transaction_mode: _ => choice(
      seq(
        kws('ISOLATION', 'LEVEL'),
        choice(
          kw('SERIALIZABLE'),
          kws('REPEATABLE', 'READ'),
          kws('READ', 'COMMITTED'),
          kws('READ', 'UNCOMMITTED'),
        ),
      ),
      kws('READ', 'WRITE'),
      kws('READ', 'ONLY'),
      seq(optional(kw('NOT')), kw('DEFERRABLE')),
    ),

    _chain: _ => seq(kw('AND'), optional(kw('NO')), kw('CHAIN')),

    commit_statement: $ => choice(
      seq(choice(kw('COMMIT'), kw('END')), optional($._transaction), optional($._chain)),
      seq(kws('COMMIT', 'PREPARED'), field('id', $.string)),
    ),

    rollback_statement: $ => choice(
      seq(
        choice(kw('ROLLBACK'), kw('ABORT')),
        optional($._transaction),
        optional(choice(
          $._chain,
          seq(kw('TO'), optional(kw('SAVEPOINT')), field('savepoint', $._name)),
        )),
      ),
      seq(kws('ROLLBACK', 'PREPARED'), field('id', $.string)),
    ),

    savepoint_statement: $ => seq(kw('SAVEPOINT'), field('name', $._name)),

    release_statement: $ => seq(
      kw('RELEASE'),
      optional(kw('SAVEPOINT')),
      field('name', $._name),
    ),

    prepare_transaction_statement: $ => seq(
      kws('PREPARE', 'TRANSACTION'),
      field('id', $.string),
    ),

    // Prepared statements and cursors

    prepare_statement: $ => seq(
      kw('PREPARE'),
      field('name', $._name),
      optional(parens(commaSep1($.type))),
      kw('AS'),
      field('statement', $._explainable_statement),
    ),

    execute_statement: $ => seq(
      kw('EXECUTE'),
      field('name', $._name),
      optional($.arguments),
    ),

    deallocate_statement: $ => seq(
      kw('DEALLOCATE'),
      optional(kw('PREPARE')),
      choice(field('name', $._name), kw('ALL')),
    ),

    declare_statement: $ => seq(
      kw('DECLARE'),
      field('name', $._name),
      repeat(choice(
        kw('BINARY'),
        kw('ASENSITIVE'),
        kw('INSENSITIVE'),
        seq(optional(kw('NO')), kw('SCROLL')),
      )),
      kw('CURSOR'),
      optional(seq(choice(kw('WITH'), kw('WITHOUT')), kw('HOLD'))),
      kw('FOR'),
      field('query', $._query),
    ),

    // FETCH and MOVE take the same direction.
    fetch_statement: $ => seq(
      choice(kw('FETCH'), kw('MOVE')),
      optional($.fetch_direction),
      optional(choice(kw('FROM'), kw('IN'))),
      field('cursor', $._name),
    ),

    fetch_direction: $ => choice(
      kw('NEXT'),
      kw('PRIOR'),
      kw('FIRST'),
      kw('LAST'),
      seq(choice(kw('ABSOLUTE'), kw('RELATIVE')), $._signed_number),
      $._signed_number,
      kw('ALL'),
      seq(choice(kw('FORWARD'), kw('BACKWARD')), optional(choice($._signed_number, kw('ALL')))),
    ),

    _signed_number: $ => seq(optional(alias($._sign, $.operator)), $.number),

    close_statement: $ => seq(
      kw('CLOSE'),
      choice(field('cursor', $._name), kw('ALL')),
    ),

    call_statement: $ => seq(kw('CALL'), $.function_call),

    // The code of DO is a string in the language that LANGUAGE names, which
    // is PL/pgSQL if LANGUAGE is not there.
    do_statement: $ => choice(
      seq(kw('DO'), field('code', $._code), optional($._language)),
      seq(kw('DO'), $._language, field('code', $._code)),
    ),

    _code: $ => choice($.string, $.dollar_quoted_string),

    _language: $ => seq(kw('LANGUAGE'), field('language', choice($._name, $.string))),

    // Utility statements

    explain_statement: $ => seq(
      kw('EXPLAIN'),
      optional(choice(
        $.option_list,
        seq(choice(kw('ANALYZE'), kw('ANALYSE')), optional(kw('VERBOSE'))),
        kw('VERBOSE'),
      )),
      field('statement', $._explainable_statement),
    ),

    // The options of EXPLAIN, VACUUM, ANALYZE, CLUSTER, REINDEX, CHECKPOINT
    // and COPY, such as (FORMAT json, ANALYZE). A name and a value can be
    // any word.
    // The storage parameters of a table or an index and the options of
    // CREATE TYPE take the same rule, with = between a name and its value.
    // The name of a storage parameter can have a prefix, as in
    // toast.autovacuum_enabled, and a value can be a qualified name, such
    // as the name of a function.
    option_list: $ => parens(commaSep1($.option)),

    option: $ => seq(
      optional(seq(field('prefix', $._label), '.')),
      field('name', $._label),
      optional('='),
      optional(field('value', choice(
        $._option_value,
        $.operator,
        $.qualified_operator,
        '*',
        parens(commaSep1($._option_value)),
      ))),
    ),

    _option_value: $ => choice(
      $.string,
      $._signed_number,
      seq($._label, optional(seq('.', $._label))),
    ),

    // SET has a form for each of the settings that the standard names, such
    // as SET TIME ZONE, and a form for any setting. ROLE is not a keyword,
    // so that role can also name a setting.
    set_statement: $ => seq(
      kw('SET'),
      optional(choice(kw('SESSION'), kw('LOCAL'), kws('LOCAL', 'SESSION'))),
      choice(
        seq(
          field('name', $.setting_name),
          choice(kw('TO'), '='),
          choice(kw('DEFAULT'), commaSep1(field('value', $._option_value))),
        ),
        seq(field('name', $.setting_name), kws('FROM', 'CURRENT')),
        seq(
          kws('TIME', 'ZONE'),
          field('value', choice($._option_value, $.typed_literal, kw('LOCAL'), kw('DEFAULT'))),
        ),
        seq(kw('SCHEMA'), field('value', $.string)),
        seq(kw('NAMES'), optional(field('value', choice($.string, kw('DEFAULT'))))),
        seq(alias($.identifier, 'ROLE'), field('value', choice($._name, $.string))),
        seq(kw('AUTHORIZATION'), field('value', choice($._name, $.string, kw('DEFAULT')))),
        seq(kws('XML', 'OPTION'), choice(kw('DOCUMENT'), kw('CONTENT'))),
        seq(kw('TRANSACTION'), choice(
          seq(kw('SNAPSHOT'), field('value', $.string)),
          $._transaction_modes,
        )),
        seq(kws('CHARACTERISTICS', 'AS', 'TRANSACTION'), $._transaction_modes),
        seq(
          kw('CONSTRAINTS'),
          choice(kw('ALL'), commaSep1(field('constraint', $.object_reference))),
          choice(kw('DEFERRED'), kw('IMMEDIATE')),
        ),
      ),
    ),

    // The name of a setting can have a prefix, as in plpgsql.check.
    setting_name: $ => seq($._name, repeat(seq('.', $._label))),

    show_statement: $ => seq(
      kw('SHOW'),
      choice(field('name', $.setting_name), kw('ALL'), $._special_setting),
    ),

    reset_statement: $ => seq(
      kw('RESET'),
      choice(field('name', $.setting_name), kw('ALL'), $._special_setting),
    ),

    _special_setting: _ => choice(
      kws('TIME', 'ZONE'),
      kws('TRANSACTION', 'ISOLATION', 'LEVEL'),
      kws('SESSION', 'AUTHORIZATION'),
    ),

    listen_statement: $ => seq(kw('LISTEN'), field('channel', $._name)),

    notify_statement: $ => seq(
      kw('NOTIFY'),
      field('channel', $._name),
      optional(seq(',', field('payload', $.string))),
    ),

    unlisten_statement: $ => seq(
      kw('UNLISTEN'),
      choice(field('channel', $._name), '*'),
    ),

    vacuum_statement: $ => seq(
      kw('VACUUM'),
      optional($.option_list),
      repeat(choice(kw('FULL'), kw('FREEZE'), kw('VERBOSE'), kw('ANALYZE'), kw('ANALYSE'))),
      optional(commaSep1($._vacuum_relation)),
    ),

    _vacuum_relation: $ => seq($._relation, optional($.column_list)),

    analyze_statement: $ => seq(
      choice(kw('ANALYZE'), kw('ANALYSE')),
      optional(choice($.option_list, kw('VERBOSE'))),
      optional(commaSep1($._vacuum_relation)),
    ),

    // CLUSTER index ON table is the form of PostgreSQL 8.2 and earlier.
    cluster_statement: $ => seq(
      kw('CLUSTER'),
      optional(choice($.option_list, kw('VERBOSE'))),
      optional(choice(
        seq(
          field('table', $.object_reference),
          optional(seq(kw('USING'), field('index', $._name))),
        ),
        seq(field('index', $._name), kw('ON'), field('table', $.object_reference)),
      )),
    ),

    repack_statement: $ => seq(
      kw('REPACK'),
      optional($.option_list),
      optional(seq(field('table', $.object_reference), optional($.column_list))),
      optional(seq(kws('USING', 'INDEX'), optional(field('index', $._name)))),
    ),

    reindex_statement: $ => seq(
      kw('REINDEX'),
      optional($.option_list),
      choice(kw('INDEX'), kw('TABLE'), kw('SCHEMA'), kw('DATABASE'), kw('SYSTEM')),
      optional(kw('CONCURRENTLY')),
      optional(field('name', $.object_reference)),
    ),

    checkpoint_statement: $ => seq(kw('CHECKPOINT'), optional($.option_list)),

    discard_statement: _ => seq(
      kw('DISCARD'),
      choice(kw('ALL'), kw('PLANS'), kw('SEQUENCES'), kw('TEMPORARY'), kw('TEMP')),
    ),

    load_statement: $ => seq(kw('LOAD'), field('file', $.string)),

    // Tables

    create_table_statement: $ => seq(
      kw('CREATE'),
      optional($._temporary),
      kw('TABLE'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      choice(
        seq(
          parens(optional(commaSep1($._table_element))),
          optional(seq(kw('INHERITS'), parens(commaSep1($.object_reference)))),
        ),
        seq(
          kw('OF'),
          field('type', $.object_reference),
          optional($._typed_table_elements),
        ),
        seq(
          kws('PARTITION', 'OF'),
          field('parent', $.object_reference),
          optional($._typed_table_elements),
          $.partition_bound,
        ),
      ),
      optional($.partition_by_clause),
      repeat($._table_option),
    ),

    // GLOBAL and LOCAL are noise words before TEMPORARY. FOREIGN makes a
    // foreign table, which takes SERVER and OPTIONS. The grammar also
    // accepts FOREIGN in CREATE TABLE AS and in the other statements that
    // take TEMPORARY.
    _temporary: _ => choice(
      seq(optional(choice(kw('GLOBAL'), kw('LOCAL'))), choice(kw('TEMPORARY'), kw('TEMP'))),
      kw('UNLOGGED'),
      kw('FOREIGN'),
    ),

    _if_not_exists: _ => kws('IF', 'NOT', 'EXISTS'),

    _if_exists: _ => kws('IF', 'EXISTS'),

    _table_element: $ => choice(
      $.column_definition,
      $.table_constraint,
      $.like_clause,
    ),

    // A typed table and a partition name their columns with no type.
    _typed_table_elements: $ => parens(commaSep1(choice(
      $.table_constraint,
      alias($._typed_column, $.column_definition),
    ))),

    _typed_column: $ => seq(
      field('name', $._name),
      optional(kws('WITH', 'OPTIONS')),
      repeat($._column_qualifier),
    ),

    // INCLUDING and EXCLUDING take a word, such as DEFAULTS or ALL, which
    // is not a keyword.
    like_clause: $ => seq(
      kw('LIKE'),
      field('table', $.object_reference),
      repeat(seq(choice(kw('INCLUDING'), kw('EXCLUDING')), $._label)),
    ),

    // The column definition of a table, of a composite type and of a
    // function that returns record.
    column_definition: $ => seq(
      field('name', $._name),
      field('type', $.type),
      repeat($._column_qualifier),
    ),

    _column_qualifier: $ => choice(
      seq(kw('COLLATE'), field('collation', $.object_reference)),
      seq(kw('COMPRESSION'), field('compression', $._label)),
      seq(kw('STORAGE'), field('storage', $._label)),
      $.options_clause,
      $.column_constraint,
      $._constraint_attribute,
    ),

    // The expression after DEFAULT ends before NOT, LIKE, IN and BETWEEN,
    // and before each operator whose precedence is lower than that of
    // LIKE, so that NOT NULL can come after it. PostgreSQL also limits this
    // expression.
    column_constraint: $ => seq(
      optional(seq(kw('CONSTRAINT'), field('name', $._name))),
      choice(
        kws('NOT', 'NULL'),
        kw('NULL'),
        $._check,
        prec.left(PREC.like, seq(kw('DEFAULT'), field('default', $._expression))),
        seq(
          kw('GENERATED'),
          choice(kw('ALWAYS'), kws('BY', 'DEFAULT')),
          kw('AS'),
          choice(
            seq(
              kw('IDENTITY'),
              optional(parens(repeat1($.sequence_option))),
            ),
            seq(
              field('expression', $.parenthesized_expression),
              optional(choice(kw('STORED'), kw('VIRTUAL'))),
            ),
          ),
        ),
        seq(kw('UNIQUE'), optional($._nulls_distinct), repeat($._index_parameter)),
        seq(kws('PRIMARY', 'KEY'), repeat($._index_parameter)),
        $.references_clause,
      ),
    ),

    _check: $ => seq(kw('CHECK'), field('check', $.parenthesized_expression)),

    _nulls_distinct: _ => seq(kw('NULLS'), optional(kw('NOT')), kw('DISTINCT')),

    // The attributes of a constraint come after it, and PostgreSQL also
    // accepts them as a constraint of their own.
    _constraint_attribute: _ => choice(
      kw('DEFERRABLE'),
      kws('NOT', 'DEFERRABLE'),
      seq(kw('INITIALLY'), choice(kw('DEFERRED'), kw('IMMEDIATE'))),
      kw('ENFORCED'),
      kws('NOT', 'ENFORCED'),
      kws('NOT', 'VALID'),
      kws('NO', 'INHERIT'),
    ),

    _index_parameter: $ => choice(
      seq(kw('INCLUDE'), $.column_list),
      seq(kw('WITH'), $.option_list),
      seq(kws('USING', 'INDEX', 'TABLESPACE'), field('tablespace', $._name)),
    ),

    table_constraint: $ => prec.right(seq(
      optional(seq(kw('CONSTRAINT'), field('name', $._name))),
      choice(
        $._check,
        seq(kws('NOT', 'NULL'), optional(field('column', $._name))),
        seq(
          choice(seq(kw('UNIQUE'), optional($._nulls_distinct)), kws('PRIMARY', 'KEY')),
          choice(
            seq($._key_columns, repeat($._index_parameter)),
            seq(kws('USING', 'INDEX'), field('index', $._name)),
          ),
        ),
        seq(
          kw('EXCLUDE'),
          optional(seq(kw('USING'), field('method', $._name))),
          parens(commaSep1($.exclude_element)),
          repeat($._index_parameter),
          optional(seq(kw('WHERE'), field('predicate', $.parenthesized_expression))),
        ),
        seq(kws('FOREIGN', 'KEY'), $._key_columns, $.references_clause),
      ),
      repeat($._constraint_attribute),
    )),

    // The last column of a key can take WITHOUT OVERLAPS, and the last
    // column of a foreign key can take PERIOD.
    _key_columns: $ => alias($._key_column_list, $.column_list),

    _key_column_list: $ => parens(commaSep1(seq(
      optional(kw('PERIOD')),
      $._name,
      optional(kws('WITHOUT', 'OVERLAPS')),
    ))),

    exclude_element: $ => seq(
      $.index_element,
      kw('WITH'),
      field('operator', choice($._operator, $.qualified_operator)),
    ),

    references_clause: $ => prec.right(seq(
      kw('REFERENCES'),
      field('table', $.object_reference),
      optional($._key_columns),
      repeat(choice(
        seq(kw('MATCH'), choice(kw('FULL'), kw('PARTIAL'), kw('SIMPLE'))),
        seq(kw('ON'), choice(kw('DELETE'), kw('UPDATE')), $.referential_action),
      )),
    )),

    referential_action: $ => choice(
      kws('NO', 'ACTION'),
      kw('RESTRICT'),
      kw('CASCADE'),
      seq(kw('SET'), choice(kw('NULL'), kw('DEFAULT')), optional($.column_list)),
    ),

    partition_by_clause: $ => seq(
      kws('PARTITION', 'BY'),
      field('strategy', choice(kw('RANGE'), kw('LIST'), kw('HASH'))),
      parens(commaSep1($.index_element)),
    ),

    // MINVALUE and MAXVALUE in a bound of a range are names, which the
    // grammar reads as columns.
    partition_bound: $ => choice(
      seq(
        kws('FOR', 'VALUES'),
        choice(
          seq(kw('IN'), $.list),
          seq(kw('FROM'), $.list, kw('TO'), $.list),
          seq(kw('WITH'), $.option_list),
        ),
      ),
      kw('DEFAULT'),
    ),

    // The options after the elements of a table, which CREATE TABLE AS and
    // CREATE MATERIALIZED VIEW also take.
    _table_option: $ => choice(
      seq(kw('USING'), field('method', $._name)),
      seq(kw('WITH'), $.option_list),
      kws('WITHOUT', 'OIDS'),
      seq(
        kws('ON', 'COMMIT'),
        choice(kws('PRESERVE', 'ROWS'), kws('DELETE', 'ROWS'), kw('DROP')),
      ),
      seq(kw('TABLESPACE'), field('tablespace', $._name)),
      seq(kw('SERVER'), field('server', $._name)),
      $.options_clause,
    ),

    create_table_as_statement: $ => seq(
      kw('CREATE'),
      optional($._temporary),
      kw('TABLE'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      optional($.column_list),
      repeat($._table_option),
      kw('AS'),
      field('query', choice($._query, $.execute_statement)),
      optional($._with_data),
    ),

    _with_data: _ => seq(kw('WITH'), optional(kw('NO')), kw('DATA')),

    // ALTER TABLE takes a list of actions. PostgreSQL takes RENAME, SET
    // SCHEMA, ATTACH PARTITION and DETACH PARTITION only alone, but the
    // grammar accepts them in the list, so that each action is in the same
    // parse states.
    alter_table_statement: $ => seq(
      kw('ALTER'),
      optional(kw('FOREIGN')),
      kw('TABLE'),
      $._alter_target,
      commaSep1($._alter_table_action),
    ),

    // ALL IN TABLESPACE names each table, index or materialized view in a
    // tablespace.
    _alter_target: $ => choice(
      seq(optional($._if_exists), $._relation),
      seq(
        kws('ALL', 'IN', 'TABLESPACE'),
        field('tablespace', $._name),
        optional(seq(kws('OWNED', 'BY'), commaSep1($._role))),
      ),
    ),

    _alter_table_action: $ => choice(
      $.add_column,
      $.drop_column,
      $.alter_column,
      $.add_constraint,
      $.alter_constraint,
      $.drop_constraint,
      $.rename,
      $.set_schema,
      $.owner_to,
      $.alter_table_option,
    ),

    add_column: $ => seq(
      kw('ADD'),
      optional(kw('COLUMN')),
      optional($._if_not_exists),
      $.column_definition,
    ),

    drop_column: $ => seq(
      kw('DROP'),
      optional(kw('COLUMN')),
      optional($._if_exists),
      field('column', $._name),
      optional($._drop_behavior),
    ),

    alter_column: $ => seq(
      kw('ALTER'),
      optional(kw('COLUMN')),
      // An index names its column by its number.
      field('column', choice($._name, $.number)),
      choice(
        seq(
          optional(kws('SET', 'DATA')),
          kw('TYPE'),
          field('type', $.type),
          optional(seq(kw('COLLATE'), field('collation', $.object_reference))),
          optional(seq(kw('USING'), field('using', $._expression))),
        ),
        $._alter_column_action,
        seq(
          kw('ADD'),
          kw('GENERATED'),
          choice(kw('ALWAYS'), kws('BY', 'DEFAULT')),
          kws('AS', 'IDENTITY'),
          optional(parens(repeat1($.sequence_option))),
        ),
        repeat1(choice(
          seq(kws('SET', 'GENERATED'), choice(kw('ALWAYS'), kws('BY', 'DEFAULT'))),
          seq(kw('SET'), $.sequence_option),
          seq(kw('RESTART'), optional(seq(optional(kw('WITH')), $._signed_number))),
        )),
        seq(kws('DROP', 'IDENTITY'), optional($._if_exists)),
        seq(kws('SET', 'EXPRESSION', 'AS'), field('expression', $.parenthesized_expression)),
        seq(kws('DROP', 'EXPRESSION'), optional($._if_exists)),
        seq(kws('SET', 'STATISTICS'), choice($._signed_number, kw('DEFAULT'))),
        seq(kw('SET'), $.option_list),
        seq(kw('RESET'), $.option_list),
        seq(kws('SET', 'STORAGE'), field('storage', $._label)),
        seq(kws('SET', 'COMPRESSION'), field('compression', $._label)),
        $.options_clause,
      ),
    ),

    // The actions on a default and on NOT NULL, which ALTER DOMAIN also
    // takes.
    _alter_column_action: $ => choice(
      seq(kws('SET', 'DEFAULT'), field('default', $._expression)),
      kws('DROP', 'DEFAULT'),
      kws('SET', 'NOT', 'NULL'),
      kws('DROP', 'NOT', 'NULL'),
    ),

    add_constraint: $ => seq(kw('ADD'), $.table_constraint),

    alter_constraint: $ => seq(
      kws('ALTER', 'CONSTRAINT'),
      field('constraint', $._name),
      repeat1(choice($._constraint_attribute, kw('INHERIT'))),
    ),

    drop_constraint: $ => seq(
      kws('DROP', 'CONSTRAINT'),
      optional($._if_exists),
      field('constraint', $._name),
      optional($._drop_behavior),
    ),

    // RENAME names the column, the constraint or the object itself.
    rename: $ => seq(
      kw('RENAME'),
      optional(choice(
        seq(optional(kw('COLUMN')), field('column', $._name)),
        seq(kw('CONSTRAINT'), field('constraint', $._name)),
      )),
      kw('TO'),
      field('new_name', $._name),
    ),

    set_schema: $ => seq(kws('SET', 'SCHEMA'), field('schema', $._name)),

    owner_to: $ => seq(kws('OWNER', 'TO'), field('owner', $._role)),

    // A role: a name, or one of the keywords of the current role.
    _role: $ => choice(
      $._name,
      keywordName($, 'CURRENT_ROLE', 'CURRENT_USER', 'SESSION_USER'),
    ),

    // The other actions of ALTER TABLE, ALTER INDEX and ALTER VIEW.
    alter_table_option: $ => choice(
      seq(kws('VALIDATE', 'CONSTRAINT'), field('constraint', $._name)),
      seq(
        choice(
          seq(kw('ENABLE'), optional(choice(kw('REPLICA'), kw('ALWAYS')))),
          kw('DISABLE'),
        ),
        choice(
          seq(kw('TRIGGER'), choice(field('trigger', $._name), kw('ALL'), kw('USER'))),
          seq(kw('RULE'), field('rule', $._name)),
        ),
      ),
      seq(
        choice(kw('ENABLE'), kw('DISABLE'), kw('FORCE'), kws('NO', 'FORCE')),
        kws('ROW', 'LEVEL', 'SECURITY'),
      ),
      seq(kws('CLUSTER', 'ON'), field('index', $._name)),
      kws('SET', 'WITHOUT', 'CLUSTER'),
      kws('SET', 'WITHOUT', 'OIDS'),
      seq(kws('SET', 'ACCESS', 'METHOD'), choice(field('method', $._name), kw('DEFAULT'))),
      seq(kws('SET', 'TABLESPACE'), field('tablespace', $._name), optional(kw('NOWAIT'))),
      kws('SET', 'LOGGED'),
      kws('SET', 'UNLOGGED'),
      seq(kw('SET'), $.option_list),
      seq(kw('RESET'), $.option_list),
      seq(optional(kw('NO')), kw('INHERIT'), field('parent', $.object_reference)),
      seq(kw('OF'), field('type', $.object_reference)),
      kws('NOT', 'OF'),
      seq(
        kws('REPLICA', 'IDENTITY'),
        choice(
          kw('DEFAULT'),
          kw('FULL'),
          kw('NOTHING'),
          seq(kws('USING', 'INDEX'), field('index', $._name)),
        ),
      ),
      seq(kws('ATTACH', 'PARTITION'), field('partition', $.object_reference), optional($.partition_bound)),
      seq(
        kws('DETACH', 'PARTITION'),
        field('partition', $.object_reference),
        optional(choice(kw('CONCURRENTLY'), kw('FINALIZE'))),
      ),
      seq(
        kws('MERGE', 'PARTITIONS'),
        parens(commaSep1(field('partition', $.object_reference))),
        kw('INTO'),
        field('partition', $.object_reference),
      ),
      seq(
        kws('SPLIT', 'PARTITION'),
        field('partition', $.object_reference),
        kw('INTO'),
        parens(commaSep1($.partition_definition)),
      ),
      seq(optional(kw('NO')), kws('DEPENDS', 'ON', 'EXTENSION'), field('extension', $._name)),
      $.options_clause,
    ),

    partition_definition: $ => seq(
      kw('PARTITION'),
      field('name', $.object_reference),
      $.partition_bound,
    ),

    // Indexes

    create_index_statement: $ => seq(
      kw('CREATE'),
      optional(kw('UNIQUE')),
      kw('INDEX'),
      optional(kw('CONCURRENTLY')),
      optional(seq(optional($._if_not_exists), field('name', $._name))),
      kw('ON'),
      optional(kw('ONLY')),
      field('table', $.object_reference),
      optional(seq(kw('USING'), field('method', $._name))),
      parens(commaSep1($.index_element)),
      repeat(choice(
        seq(kw('INCLUDE'), $.column_list),
        $._nulls_distinct,
        seq(kw('WITH'), $.option_list),
        seq(kw('TABLESPACE'), field('tablespace', $._name)),
      )),
      optional($.where_clause),
    ),

    alter_index_statement: $ => seq(
      kw('ALTER'),
      kw('INDEX'),
      $._alter_target,
      commaSep1($._alter_table_action),
    ),

    // Views

    create_view_statement: $ => seq(
      kw('CREATE'),
      optional(kws('OR', 'REPLACE')),
      optional($._temporary),
      optional(kw('RECURSIVE')),
      kw('VIEW'),
      field('name', $.object_reference),
      optional($.column_list),
      optional(seq(kw('WITH'), $.option_list)),
      kw('AS'),
      field('query', $._query),
      optional(seq(
        kw('WITH'),
        optional(choice(kw('CASCADED'), kw('LOCAL'))),
        kws('CHECK', 'OPTION'),
      )),
    ),

    create_materialized_view_statement: $ => seq(
      kw('CREATE'),
      kws('MATERIALIZED', 'VIEW'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      optional($.column_list),
      repeat($._table_option),
      kw('AS'),
      field('query', $._query),
      optional($._with_data),
    ),

    refresh_materialized_view_statement: $ => seq(
      kws('REFRESH', 'MATERIALIZED', 'VIEW'),
      optional(kw('CONCURRENTLY')),
      field('name', $.object_reference),
      optional($._with_data),
    ),

    // ALTER VIEW and ALTER MATERIALIZED VIEW.
    alter_view_statement: $ => seq(
      kw('ALTER'),
      optional(kw('MATERIALIZED')),
      kw('VIEW'),
      $._alter_target,
      commaSep1($._alter_table_action),
    ),

    // Sequences

    create_sequence_statement: $ => seq(
      kw('CREATE'),
      optional($._temporary),
      kw('SEQUENCE'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      repeat($.sequence_option),
    ),

    // OWNED BY NONE takes NONE as a name.
    sequence_option: $ => choice(
      seq(kw('AS'), field('type', $.type)),
      seq(kw('INCREMENT'), optional(kw('BY')), $._signed_number),
      seq(choice(kw('MINVALUE'), kw('MAXVALUE')), $._signed_number),
      seq(kw('NO'), choice(kw('MINVALUE'), kw('MAXVALUE'), kw('CYCLE'))),
      seq(kw('START'), optional(kw('WITH')), $._signed_number),
      seq(kw('RESTART'), optional(seq(optional(kw('WITH')), $._signed_number))),
      seq(kw('CACHE'), $._signed_number),
      kw('CYCLE'),
      seq(kws('OWNED', 'BY'), field('owner', $.object_reference)),
      seq(kws('SEQUENCE', 'NAME'), field('name', $.object_reference)),
      kw('LOGGED'),
      kw('UNLOGGED'),
    ),

    alter_sequence_statement: $ => seq(
      kw('ALTER'),
      kw('SEQUENCE'),
      optional($._if_exists),
      field('name', $.object_reference),
      choice(
        repeat1($.sequence_option),
        commaSep1($._alter_table_action),
      ),
    ),

    // Schemas and databases

    // CREATE SCHEMA can create the objects of the schema in the same
    // statement, but the grammar leaves that out. Each statement in it
    // would have parse states of its own, because CREATE can come after
    // it, and that costs about 50 large states.
    create_schema_statement: $ => seq(
      kw('CREATE'),
      kw('SCHEMA'),
      optional($._if_not_exists),
      choice(
        seq(field('name', $._name), optional(seq(kw('AUTHORIZATION'), field('owner', $._role)))),
        seq(kw('AUTHORIZATION'), field('owner', $._role)),
      ),
    ),

    alter_schema_statement: $ => seq(
      kw('ALTER'),
      kw('SCHEMA'),
      field('name', $._name),
      choice($.rename, $.owner_to),
    ),

    create_database_statement: $ => seq(
      kw('CREATE'),
      kw('DATABASE'),
      field('name', $._name),
      optional(kw('WITH')),
      repeat($.database_option),
    ),

    // An option of a database, such as OWNER = name or CONNECTION LIMIT 10.
    // The name of an option is not a keyword.
    database_option: $ => seq(
      field('name', $._label),
      optional(kw('LIMIT')),
      optional('='),
      field('value', $._option_value),
    ),

    alter_database_statement: $ => seq(
      kw('ALTER'),
      kw('DATABASE'),
      field('name', $._name),
      choice(
        seq(optional(kw('WITH')), repeat1($.database_option)),
        $.rename,
        $.owner_to,
        seq(kws('SET', 'TABLESPACE'), field('tablespace', $._name)),
        kws('REFRESH', 'COLLATION', 'VERSION'),
        $._set_parameter,
      ),
    ),

    // SET and RESET of a setting for a database, a role or a function.
    _set_parameter: $ => choice(
      seq(
        kw('SET'),
        field('setting', $.setting_name),
        choice(
          seq(
            choice(kw('TO'), '='),
            choice(kw('DEFAULT'), commaSep1(field('value', $._option_value))),
          ),
          kws('FROM', 'CURRENT'),
        ),
      ),
      seq(kw('RESET'), choice(field('setting', $.setting_name), kw('ALL'))),
    ),

    // Types and domains

    // A composite type, an enum, a range, a base type, or a shell type
    // with no definition.
    create_type_statement: $ => seq(
      kw('CREATE'),
      kw('TYPE'),
      field('name', $.object_reference),
      optional(choice(
        seq(kw('AS'), parens(optional(commaSep1($.column_definition)))),
        seq(kws('AS', 'ENUM'), parens(optional(commaSep1($.string)))),
        seq(kws('AS', 'RANGE'), $.option_list),
        $.option_list,
      )),
    ),

    alter_type_statement: $ => seq(
      kw('ALTER'),
      kw('TYPE'),
      field('name', $.object_reference),
      commaSep1(choice(
        $.rename,
        $.set_schema,
        $.owner_to,
        seq(kw('SET'), $.option_list),
        seq(
          kws('ADD', 'VALUE'),
          optional($._if_not_exists),
          field('value', $.string),
          optional(seq(choice(kw('BEFORE'), kw('AFTER')), field('neighbor', $.string))),
        ),
        seq(kws('RENAME', 'VALUE'), field('value', $.string), kw('TO'), field('new_value', $.string)),
        seq(
          kws('RENAME', 'ATTRIBUTE'),
          field('attribute', $._name),
          kw('TO'),
          field('new_name', $._name),
          optional($._drop_behavior),
        ),
        seq(
          kws('ADD', 'ATTRIBUTE'),
          field('attribute', $._name),
          field('type', $.type),
          optional(seq(kw('COLLATE'), field('collation', $.object_reference))),
          optional($._drop_behavior),
        ),
        seq(
          kws('DROP', 'ATTRIBUTE'),
          optional($._if_exists),
          field('attribute', $._name),
          optional($._drop_behavior),
        ),
        seq(
          kws('ALTER', 'ATTRIBUTE'),
          field('attribute', $._name),
          optional(kws('SET', 'DATA')),
          kw('TYPE'),
          field('type', $.type),
          optional(seq(kw('COLLATE'), field('collation', $.object_reference))),
          optional($._drop_behavior),
        ),
      )),
    ),

    create_domain_statement: $ => seq(
      kw('CREATE'),
      kw('DOMAIN'),
      field('name', $.object_reference),
      optional(kw('AS')),
      field('type', $.type),
      repeat($._column_qualifier),
    ),

    alter_domain_statement: $ => seq(
      kw('ALTER'),
      kw('DOMAIN'),
      field('name', $.object_reference),
      choice(
        $._alter_column_action,
        $.add_constraint,
        $.drop_constraint,
        seq(kws('VALIDATE', 'CONSTRAINT'), field('constraint', $._name)),
        $.rename,
        $.set_schema,
        $.owner_to,
      ),
    ),

    // Extensions

    create_extension_statement: $ => seq(
      kw('CREATE'),
      kw('EXTENSION'),
      optional($._if_not_exists),
      field('name', $._name),
      optional(kw('WITH')),
      repeat(choice(
        seq(kw('SCHEMA'), field('schema', $._name)),
        seq(kw('VERSION'), field('version', choice($._name, $.string))),
        kw('CASCADE'),
      )),
    ),

    alter_extension_statement: $ => seq(
      kw('ALTER'),
      kw('EXTENSION'),
      field('name', $._name),
      choice(
        seq(kw('UPDATE'), optional(seq(kw('TO'), field('version', choice($._name, $.string))))),
        $.set_schema,
        seq(choice(kw('ADD'), kw('DROP')), $._object),
      ),
    ),

    // Objects of any kind

    // DROP takes the objects of each kind, with IF EXISTS and with a list
    // of names. COMMENT, SECURITY LABEL and ALTER EXTENSION take one
    // object, but the grammar accepts the same forms for each of them.
    drop_statement: $ => seq(
      kw('DROP'),
      $._object,
      optional($._drop_behavior),
      optional(seq(optional(kw('WITH')), $.option_list)),
    ),

    _object: $ => choice(
      seq(
        $._object_kind,
        optional($._if_exists),
        commaSep1(field('name', $.object_reference)),
      ),
      seq(
        choice(kw('FUNCTION'), kw('PROCEDURE'), kw('ROUTINE'), kw('AGGREGATE')),
        optional($._if_exists),
        commaSep1($.function_signature),
      ),
      seq(
        kw('OPERATOR'),
        optional($._if_exists),
        commaSep1($.operator_signature),
      ),
      seq(
        kw('OPERATOR'),
        choice(kw('CLASS'), kw('FAMILY')),
        optional($._if_exists),
        field('name', $.object_reference),
        kw('USING'),
        field('method', $._name),
      ),
      seq(
        kw('CAST'),
        optional($._if_exists),
        parens(seq(field('source', $.type), kw('AS'), field('target', $.type))),
      ),
      seq(
        kws('TRANSFORM'),
        optional($._if_exists),
        kw('FOR'),
        field('type', $.type),
        kw('LANGUAGE'),
        field('language', $._name),
      ),
      seq(kws('LARGE', 'OBJECT'), field('oid', $.number)),
      seq(kws('USER', 'MAPPING'), optional($._if_exists), $._user_mapping),
      seq(kw('COLUMN'), field('column', $.column_reference)),
      seq(
        choice(kw('CONSTRAINT'), kw('POLICY'), kw('RULE'), kw('TRIGGER')),
        optional($._if_exists),
        field('name', $._name),
        kw('ON'),
        optional(kw('DOMAIN')),
        field('table', $.object_reference),
      ),
    ),

    _object_kind: _ => choice(
      kws('ACCESS', 'METHOD'),
      kw('COLLATION'),
      kw('CONVERSION'),
      kw('DATABASE'),
      kw('DOMAIN'),
      kw('EXTENSION'),
      kws('EVENT', 'TRIGGER'),
      kws('FOREIGN', 'DATA', 'WRAPPER'),
      kws('FOREIGN', 'TABLE'),
      seq(kw('INDEX'), optional(kw('CONCURRENTLY'))),
      kws('MATERIALIZED', 'VIEW'),
      seq(optional(kw('PROCEDURAL')), kw('LANGUAGE')),
      kw('PUBLICATION'),
      kw('ROLE'),
      kw('SCHEMA'),
      kw('SEQUENCE'),
      kw('SERVER'),
      kw('STATISTICS'),
      kw('SUBSCRIPTION'),
      kw('TABLE'),
      kw('TABLESPACE'),
      seq(
        kws('TEXT', 'SEARCH'),
        choice(kw('CONFIGURATION'), kw('DICTIONARY'), kw('PARSER'), kw('TEMPLATE')),
      ),
      kw('TYPE'),
      kw('USER'),
      kw('GROUP'),
      kw('VIEW'),
    ),

    // A function, a procedure or an aggregate, with the types of its
    // arguments. An aggregate can take * or ORDER BY.
    function_signature: $ => prec.right(seq(
      field('name', $.object_reference),
      optional($.function_parameters),
    )),

    function_parameters: $ => parens(optional(choice(
      '*',
      seq(
        commaSep1($.function_parameter),
        optional(seq(kws('ORDER', 'BY'), commaSep1($.function_parameter))),
      ),
      seq(kws('ORDER', 'BY'), commaSep1($.function_parameter)),
    ))),

    // A parameter of CREATE FUNCTION can have a default value. The mode
    // can also come after the name.
    function_parameter: $ => seq(
      optional($._parameter_mode),
      optional(seq(field('name', $._name), optional($._parameter_mode))),
      $._function_type,
      optional(seq(choice(kw('DEFAULT'), '='), field('default', $._expression))),
    ),

    _parameter_mode: _ => choice(kw('IN'), kw('OUT'), kw('INOUT'), kw('VARIADIC')),

    // An operator with the types of its left and right operands, where
    // NONE stands for no operand.
    operator_signature: $ => seq(
      optional(seq(field('schema', $._name), '.')),
      field('name', $._operator),
      parens(seq(field('left', $.type), ',', field('right', $.type))),
    ),

    comment_statement: $ => seq(
      kws('COMMENT', 'ON'),
      $._object,
      kw('IS'),
      field('comment', choice($.string, $.null)),
    ),

    security_label_statement: $ => seq(
      kws('SECURITY', 'LABEL'),
      optional(seq(kw('FOR'), field('provider', choice($._name, $.string)))),
      kw('ON'),
      $._object,
      kw('IS'),
      field('label', choice($.string, $.null)),
    ),

    // Functions, procedures, aggregates and operators

    // CREATE FUNCTION and CREATE PROCEDURE. The body is a string in the
    // language that LANGUAGE names, or a body in SQL after RETURN or BEGIN
    // ATOMIC. The body and the language are children of the statement, so
    // that an injection can find both.
    create_function_statement: $ => seq(
      kw('CREATE'),
      optional(kws('OR', 'REPLACE')),
      choice(kw('FUNCTION'), kw('PROCEDURE')),
      $.function_signature,
      optional(seq(
        kw('RETURNS'),
        choice(
          $._function_type,
          seq(kw('TABLE'), $.column_definitions),
        ),
      )),
      repeat($._function_option),
      optional($._sql_body),
    ),

    // A type of an argument or of a result can be SETOF a type, or the
    // type of a column with %TYPE.
    _function_type: $ => seq(
      optional(kw('SETOF')),
      field('type', $.type),
      optional(seq('%', kw('TYPE'))),
    ),

    // The options of a function, which ALTER FUNCTION also takes.
    _function_option: $ => choice(
      $._language,
      seq(kw('TRANSFORM'), commaSep1(seq(kws('FOR', 'TYPE'), field('transform', $.type)))),
      kw('WINDOW'),
      kw('IMMUTABLE'),
      kw('STABLE'),
      kw('VOLATILE'),
      seq(optional(kw('NOT')), kw('LEAKPROOF')),
      kws('CALLED', 'ON', 'NULL', 'INPUT'),
      kws('RETURNS', 'NULL', 'ON', 'NULL', 'INPUT'),
      kw('STRICT'),
      seq(optional(kw('EXTERNAL')), kw('SECURITY'), choice(kw('INVOKER'), kw('DEFINER'))),
      seq(kw('PARALLEL'), field('parallel', $._label)),
      seq(kw('COST'), field('cost', $.number)),
      seq(kw('ROWS'), field('rows', $.number)),
      seq(kw('SUPPORT'), field('support', $.object_reference)),
      $._set_parameter,
      seq(
        kw('AS'),
        field('body', $.function_body),
        optional(seq(',', field('symbol', $.string))),
      ),
    ),

    // The body of a function in a string. The function names its language
    // with LANGUAGE.
    function_body: $ => choice($.string, $.dollar_quoted_string),

    // A body in SQL: RETURN and an expression, or BEGIN ATOMIC and a list
    // of statements, each with a semicolon after it. The grammar takes the
    // queries and the statements that change rows in the list, because END
    // is also a statement. PostgreSQL takes the body among the options, but
    // the grammar takes it only after them. An expression that the options
    // can follow has parse states of its own, and that costs about 700
    // parse states and 22 large states.
    _sql_body: $ => choice(
      seq(kw('RETURN'), field('return', $._expression)),
      $.atomic_block,
    ),

    atomic_block: $ => seq(
      kws('BEGIN', 'ATOMIC'),
      repeat(seq(optional(choice($.select_statement, $._modifying_statement)), ';')),
      kw('END'),
    ),

    // ALTER FUNCTION, ALTER PROCEDURE, ALTER ROUTINE and ALTER AGGREGATE.
    alter_function_statement: $ => seq(
      kw('ALTER'),
      choice(kw('FUNCTION'), kw('PROCEDURE'), kw('ROUTINE'), kw('AGGREGATE')),
      $.function_signature,
      repeat1(choice(
        $._function_option,
        kw('RESTRICT'),
        $.rename,
        $.owner_to,
        $.set_schema,
        $._depends_on_extension,
      )),
    ),

    // CREATE AGGREGATE takes the types of its arguments and a list of
    // definitions. The old form has no types, so the first list can be the
    // types or the definitions.
    create_aggregate_statement: $ => seq(
      kw('CREATE'),
      optional(kws('OR', 'REPLACE')),
      kw('AGGREGATE'),
      field('name', $.object_reference),
      optional($.function_parameters),
      $.definition_list,
    ),

    // The definitions of CREATE AGGREGATE, CREATE OPERATOR, CREATE
    // COLLATION and CREATE TEXT SEARCH, such as (SFUNC = int4pl, STYPE =
    // int4). A value can be a type or an operator.
    definition_list: $ => parens(commaSep1($.definition)),

    definition: $ => seq(
      field('name', $._label),
      optional(seq(
        '=',
        field('value', choice(
          $.string,
          $._signed_number,
          $.boolean,
          kw('DEFAULT'),
          $.type,
          $._operator,
          $.qualified_operator,
        )),
      )),
    ),

    create_operator_statement: $ => seq(
      kw('CREATE'),
      kw('OPERATOR'),
      optional(seq(field('schema', $._name), '.')),
      field('name', $._operator),
      $.definition_list,
    ),

    // CREATE OPERATOR CLASS lists the operators, the functions and the
    // storage type of the class.
    create_operator_class_statement: $ => seq(
      kw('CREATE'),
      kws('OPERATOR', 'CLASS'),
      field('name', $.object_reference),
      optional(kw('DEFAULT')),
      kws('FOR', 'TYPE'),
      field('type', $.type),
      kw('USING'),
      field('method', $._name),
      optional(seq(kw('FAMILY'), field('family', $.object_reference))),
      kw('AS'),
      commaSep1($.operator_class_item),
    ),

    // An operator or a function of an operator class or of an operator
    // family, with its strategy number or its support number.
    operator_class_item: $ => prec.right(choice(
      // DROP of an item of an operator family names no operator.
      seq(
        kw('OPERATOR'),
        field('number', $.number),
        optional(seq(
          optional(seq(field('schema', $._name), '.')),
          field('operator', $._operator),
        )),
        optional(parens(commaSep1($.type))),
        optional(choice(
          kws('FOR', 'SEARCH'),
          seq(kws('FOR', 'ORDER', 'BY'), field('sort_family', $.object_reference)),
        )),
      ),
      seq(
        kw('FUNCTION'),
        field('number', $.number),
        optional(parens(commaSep1($.type))),
        optional($.function_signature),
      ),
      seq(kw('STORAGE'), field('type', $.type)),
    )),

    create_operator_family_statement: $ => seq(
      kw('CREATE'),
      kws('OPERATOR', 'FAMILY'),
      field('name', $.object_reference),
      kw('USING'),
      field('method', $._name),
    ),

    // ALTER OPERATOR, ALTER OPERATOR CLASS and ALTER OPERATOR FAMILY.
    alter_operator_statement: $ => seq(
      kw('ALTER'),
      choice(
        seq(kw('OPERATOR'), $.operator_signature),
        seq(
          kw('OPERATOR'),
          choice(kw('CLASS'), kw('FAMILY')),
          field('name', $.object_reference),
          kw('USING'),
          field('method', $._name),
        ),
      ),
      $._alter_actions,
    ),

    _depends_on_extension: $ => seq(
      optional(kw('NO')),
      kws('DEPENDS', 'ON', 'EXTENSION'),
      field('extension', $._name),
    ),

    // The actions of the statements that alter an object other than a
    // table or a function. The grammar accepts each action for each kind
    // of object, so that the actions are in the same parse states. The
    // actions come one after the other, with no commas. ALTER FUNCTION has
    // actions of its own, because the options of a function in this list
    // cost 20 large states.
    _alter_actions: $ => repeat1($._alter_action),

    _alter_action: $ => choice(
      $.rename,
      $.owner_to,
      $.set_schema,
      $._depends_on_extension,
      $.definition_list,
      seq(kw('SET'), $.option_list),
      seq(kw('RESET'), $.option_list),
      seq(kws('SET', 'STATISTICS'), choice($._signed_number, kw('DEFAULT'))),
      kws('REFRESH', 'VERSION'),
      seq(kw('ENABLE'), optional(choice(kw('REPLICA'), kw('ALWAYS')))),
      kw('DISABLE'),
      // The roles and the expressions of a policy.
      seq(kw('TO'), commaSep1($._role)),
      seq(kw('USING'), field('using', $.parenthesized_expression)),
      seq(kws('WITH', 'CHECK'), field('check', $.parenthesized_expression)),
      seq(kw('WITH'), $.option_list),
      // The items of an operator family.
      seq(choice(kw('ADD'), kw('DROP')), commaSep1($.operator_class_item)),
      // The tables and the schemas of a publication.
      seq(choice(kw('ADD'), kw('SET'), kw('DROP')), $._publication_objects),
      // The publications and the connection of a subscription.
      seq(kw('CONNECTION'), field('connection', $.string)),
      seq(
        choice(kw('ADD'), kw('SET'), kw('DROP')),
        kw('PUBLICATION'),
        commaSep1(field('publication', $._name)),
      ),
      seq(kw('REFRESH'), choice(kw('PUBLICATION'), kw('SEQUENCES'))),
      seq(kw('SKIP'), $.option_list),
      seq(kw('SERVER'), field('server', $._name)),
      // The handler, the validator, the version and the options of a
      // foreign-data wrapper or a server.
      $._wrapper_option,
      seq(kw('VERSION'), field('version', $.string)),
      // The mappings of a text search configuration.
      seq(
        choice(kw('ADD'), kw('ALTER')),
        kw('MAPPING'),
        optional(seq(kw('FOR'), commaSep1($._name))),
        choice(
          seq(kw('WITH'), commaSep1(field('dictionary', $.object_reference))),
          seq(
            kw('REPLACE'),
            field('dictionary', $.object_reference),
            kw('WITH'),
            field('dictionary', $.object_reference),
          ),
        ),
      ),
      seq(kws('DROP', 'MAPPING'), optional($._if_exists), kw('FOR'), commaSep1($._name)),
    ),

    // Triggers, rules and policies

    create_trigger_statement: $ => seq(
      kw('CREATE'),
      optional(kws('OR', 'REPLACE')),
      optional(kw('CONSTRAINT')),
      kw('TRIGGER'),
      field('name', $._name),
      choice(kw('BEFORE'), kw('AFTER'), kws('INSTEAD', 'OF')),
      $.trigger_event,
      repeat(seq(kw('OR'), $.trigger_event)),
      kw('ON'),
      field('table', $.object_reference),
      repeat(choice(
        seq(kw('FROM'), field('referenced_table', $.object_reference)),
        $._constraint_attribute,
        seq(
          kw('REFERENCING'),
          repeat1(seq(
            choice(kw('OLD'), kw('NEW')),
            kw('TABLE'),
            optional(kw('AS')),
            field('transition', $._name),
          )),
        ),
        seq(kw('FOR'), optional(kw('EACH')), choice(kw('ROW'), kw('STATEMENT'))),
        seq(kw('WHEN'), field('condition', $.parenthesized_expression)),
      )),
      $._execute_function,
    ),

    trigger_event: $ => choice(
      kw('INSERT'),
      seq(kw('UPDATE'), optional(seq(kw('OF'), commaSep1(field('column', $._name))))),
      kw('DELETE'),
      kw('TRUNCATE'),
    ),

    _execute_function: $ => seq(
      kw('EXECUTE'),
      choice(kw('FUNCTION'), kw('PROCEDURE')),
      field('function', $.function_call),
    ),

    alter_trigger_statement: $ => seq(
      kw('ALTER'),
      kw('TRIGGER'),
      field('name', $._name),
      kw('ON'),
      optional(kw('ONLY')),
      field('table', $.object_reference),
      $._alter_actions,
    ),

    // The filter of an event trigger, such as WHEN TAG IN ('CREATE TABLE').
    create_event_trigger_statement: $ => seq(
      kw('CREATE'),
      kws('EVENT', 'TRIGGER'),
      field('name', $._name),
      kw('ON'),
      field('event', $._name),
      optional(seq(
        kw('WHEN'),
        $.event_filter,
        repeat(seq(kw('AND'), $.event_filter)),
      )),
      $._execute_function,
    ),

    event_filter: $ => seq(
      field('variable', $._name),
      kw('IN'),
      parens(commaSep1($.string)),
    ),

    alter_event_trigger_statement: $ => seq(
      kw('ALTER'),
      kws('EVENT', 'TRIGGER'),
      field('name', $._name),
      $._alter_actions,
    ),

    // The action of a rule is NOTHING, a statement, or statements in
    // parentheses with a semicolon between two of them.
    create_rule_statement: $ => seq(
      kw('CREATE'),
      optional(kws('OR', 'REPLACE')),
      kw('RULE'),
      field('name', $._name),
      kws('AS', 'ON'),
      field('event', choice(kw('SELECT'), kw('INSERT'), kw('UPDATE'), kw('DELETE'))),
      kw('TO'),
      field('table', $.object_reference),
      optional($.where_clause),
      kw('DO'),
      optional(choice(kw('ALSO'), kw('INSTEAD'))),
      choice(
        kw('NOTHING'),
        $._rule_action,
        prec.dynamic(1, parens(seq(
          optional($._rule_action),
          repeat(seq(';', optional($._rule_action))),
        ))),
      ),
    ),

    _rule_action: $ => choice(
      $.select_statement,
      $._modifying_statement,
      $.notify_statement,
    ),

    alter_rule_statement: $ => seq(
      kw('ALTER'),
      kw('RULE'),
      field('name', $._name),
      kw('ON'),
      field('table', $.object_reference),
      $._alter_actions,
    ),

    create_policy_statement: $ => seq(
      kw('CREATE'),
      kw('POLICY'),
      field('name', $._name),
      kw('ON'),
      field('table', $.object_reference),
      optional(seq(kw('AS'), choice(kw('PERMISSIVE'), kw('RESTRICTIVE')))),
      optional(seq(
        kw('FOR'),
        choice(kw('ALL'), kw('SELECT'), kw('INSERT'), kw('UPDATE'), kw('DELETE')),
      )),
      optional(seq(kw('TO'), commaSep1($._role))),
      optional(seq(kw('USING'), field('using', $.parenthesized_expression))),
      optional(seq(kws('WITH', 'CHECK'), field('check', $.parenthesized_expression))),
    ),

    alter_policy_statement: $ => seq(
      kw('ALTER'),
      kw('POLICY'),
      field('name', $._name),
      kw('ON'),
      field('table', $.object_reference),
      $._alter_actions,
    ),

    // Roles and privileges

    // CREATE ROLE, CREATE USER and CREATE GROUP.
    create_role_statement: $ => seq(
      kw('CREATE'),
      choice(kw('ROLE'), kw('USER'), kw('GROUP')),
      field('name', $._name),
      optional(kw('WITH')),
      repeat($.role_option),
    ),

    // An option of a role, such as LOGIN, NOSUPERUSER or PASSWORD 'x'. The
    // name of an option is not a keyword, but for the options that take
    // more than one word.
    role_option: $ => choice(
      seq(field('name', $._label), optional(field('value', choice($.string, $._signed_number, $.null)))),
      seq(kws('CONNECTION', 'LIMIT'), field('value', $._signed_number)),
      seq(kw('ENCRYPTED'), field('name', $._label), field('value', $.string)),
      seq(kws('VALID', 'UNTIL'), field('value', $.string)),
      seq(
        choice(
          seq(kw('IN'), choice(kw('ROLE'), kw('GROUP'))),
          kw('ROLE'),
          kw('ADMIN'),
          kw('USER'),
        ),
        commaSep1($._role),
      ),
    ),

    // ALTER ROLE, ALTER USER and ALTER GROUP. ALL names each role for SET
    // and RESET.
    alter_role_statement: $ => seq(
      kw('ALTER'),
      choice(kw('ROLE'), kw('USER'), kw('GROUP')),
      choice(field('name', $._role), kw('ALL')),
      choice(
        seq(optional(kw('WITH')), repeat1($.role_option)),
        $.rename,
        seq(optional(seq(kws('IN', 'DATABASE'), field('database', $._name))), $._set_parameter),
        seq(choice(kw('ADD'), kw('DROP')), kw('USER'), commaSep1($._role)),
      ),
    ),

    // GRANT gives privileges on objects, or the membership of roles. The
    // grammar takes both forms in one rule, because a privilege and a role
    // can both be a name.
    grant_statement: $ => seq(
      kw('GRANT'),
      $._privileges,
      optional(seq(kw('ON'), $._privilege_target)),
      kw('TO'),
      commaSep1($._grantee),
      optional(seq(
        kw('WITH'),
        commaSep1(seq(field('option', $._label), choice(kw('OPTION'), $.boolean))),
      )),
      optional($._granted_by),
    ),

    revoke_statement: $ => seq(
      kw('REVOKE'),
      optional(seq(
        choice(kw('GRANT'), kw('ADMIN'), kw('INHERIT'), kw('SET')),
        kws('OPTION', 'FOR'),
      )),
      $._privileges,
      optional(seq(kw('ON'), $._privilege_target)),
      kw('FROM'),
      commaSep1($._grantee),
      optional($._granted_by),
      optional($._drop_behavior),
    ),

    _privileges: $ => choice(
      seq(kw('ALL'), optional(kw('PRIVILEGES')), optional($.column_list)),
      commaSep1($.privilege),
    ),

    // A privilege, which can name columns, or a role.
    privilege: $ => seq(
      choice(
        kw('SELECT'),
        kw('INSERT'),
        kw('UPDATE'),
        kw('DELETE'),
        kw('TRUNCATE'),
        kw('REFERENCES'),
        kw('TRIGGER'),
        kw('MAINTAIN'),
        kw('CREATE'),
        kw('CONNECT'),
        kw('TEMPORARY'),
        kw('TEMP'),
        kw('EXECUTE'),
        kw('USAGE'),
        kw('SET'),
        kws('ALTER', 'SYSTEM'),
        field('role', $._name),
      ),
      optional($.column_list),
    ),

    // The objects of GRANT and REVOKE. ALTER DEFAULT PRIVILEGES names a
    // kind of object, such as TABLES.
    _privilege_target: $ => choice(
      seq(optional(kw('TABLE')), commaSep1(field('name', $.object_reference))),
      seq(
        kw('ALL'),
        $._privilege_kinds,
        kws('IN', 'SCHEMA'),
        commaSep1(field('schema', $._name)),
      ),
      $._privilege_kinds,
      seq(
        choice(
          kw('SEQUENCE'),
          kw('DATABASE'),
          kw('DOMAIN'),
          kws('FOREIGN', 'DATA', 'WRAPPER'),
          kws('FOREIGN', 'SERVER'),
          kw('LANGUAGE'),
          kw('PARAMETER'),
          kw('SCHEMA'),
          kw('TABLESPACE'),
          kw('TYPE'),
        ),
        commaSep1(field('name', $.object_reference)),
      ),
      seq(
        choice(kw('FUNCTION'), kw('PROCEDURE'), kw('ROUTINE')),
        commaSep1($.function_signature),
      ),
      seq(kws('LARGE', 'OBJECT'), commaSep1(field('oid', $.number))),
    ),

    _privilege_kinds: _ => choice(
      kw('TABLES'),
      kw('SEQUENCES'),
      kw('FUNCTIONS'),
      kw('PROCEDURES'),
      kw('ROUTINES'),
      kw('TYPES'),
      kw('SCHEMAS'),
      kws('LARGE', 'OBJECTS'),
    ),

    _grantee: $ => seq(optional(kw('GROUP')), field('grantee', $._role)),

    _granted_by: $ => seq(kws('GRANTED', 'BY'), field('grantor', $._role)),

    alter_default_privileges_statement: $ => seq(
      kws('ALTER', 'DEFAULT', 'PRIVILEGES'),
      repeat(choice(
        seq(kw('FOR'), choice(kw('ROLE'), kw('USER')), commaSep1(field('role', $._role))),
        seq(kws('IN', 'SCHEMA'), commaSep1(field('schema', $._name))),
      )),
      choice($.grant_statement, $.revoke_statement),
    ),

    reassign_owned_statement: $ => seq(
      kws('REASSIGN', 'OWNED', 'BY'),
      commaSep1($._role),
      kw('TO'),
      field('owner', $._role),
    ),

    drop_owned_statement: $ => seq(
      kws('DROP', 'OWNED', 'BY'),
      commaSep1($._role),
      optional($._drop_behavior),
    ),

    // Replication

    create_publication_statement: $ => seq(
      kw('CREATE'),
      kw('PUBLICATION'),
      field('name', $._name),
      optional(seq(kw('FOR'), $._publication_objects)),
      optional(seq(kw('WITH'), $.option_list)),
    ),

    // The tables and the schemas of a publication. A name after a comma
    // continues the list of the keyword before it, but the grammar reads it
    // as a table.
    _publication_objects: $ => seq(
      $._publication_object,
      repeat(seq(',', choice(
        $._publication_object,
        $.publication_table,
        field('schema', keywordName($, 'CURRENT_SCHEMA')),
      ))),
    ),

    // ALL TABLES can leave out tables with EXCEPT.
    _publication_object: $ => choice(
      seq(kw('TABLE'), $.publication_table),
      seq(
        kws('ALL', 'TABLES'),
        optional(seq(
          kw('EXCEPT'),
          parens(commaSep1(choice($._publication_object, $.publication_table))),
        )),
      ),
      kws('ALL', 'SEQUENCES'),
      seq(
        kws('TABLES', 'IN', 'SCHEMA'),
        field('schema', choice($._name, keywordName($, 'CURRENT_SCHEMA'))),
      ),
    ),

    publication_table: $ => prec.right(seq(
      optional(kw('ONLY')),
      field('table', $.object_reference),
      optional('*'),
      optional($.column_list),
      optional(seq(kw('WHERE'), field('condition', $.parenthesized_expression))),
    )),

    alter_publication_statement: $ => seq(
      kw('ALTER'),
      kw('PUBLICATION'),
      field('name', $._name),
      $._alter_actions,
    ),

    create_subscription_statement: $ => seq(
      kw('CREATE'),
      kw('SUBSCRIPTION'),
      field('name', $._name),
      choice(
        seq(kw('CONNECTION'), field('connection', $.string)),
        seq(kw('SERVER'), field('server', $._name)),
      ),
      kw('PUBLICATION'),
      commaSep1(field('publication', $._name)),
      optional(seq(kw('WITH'), $.option_list)),
    ),

    alter_subscription_statement: $ => seq(
      kw('ALTER'),
      kw('SUBSCRIPTION'),
      field('name', $._name),
      $._alter_actions,
    ),

    // Foreign data

    create_foreign_data_wrapper_statement: $ => seq(
      kw('CREATE'),
      kws('FOREIGN', 'DATA', 'WRAPPER'),
      field('name', $._name),
      repeat($._wrapper_option),
    ),

    _wrapper_option: $ => choice(
      seq(kw('CONNECTION'), field('connection', $.object_reference)),
      kws('NO', 'CONNECTION'),
      seq(kw('HANDLER'), field('handler', $.object_reference)),
      kws('NO', 'HANDLER'),
      seq(kw('VALIDATOR'), field('validator', $.object_reference)),
      kws('NO', 'VALIDATOR'),
      $.options_clause,
    ),

    // The options of a foreign-data wrapper, a server, a user mapping, a
    // foreign table and its columns. ALTER can add, set or drop each one.
    options_clause: $ => seq(kw('OPTIONS'), parens(commaSep1($.generic_option))),

    generic_option: $ => seq(
      optional(choice(kw('ADD'), kw('SET'), kw('DROP'))),
      field('name', $._label),
      optional(field('value', $.string)),
    ),

    alter_foreign_data_wrapper_statement: $ => seq(
      kw('ALTER'),
      kws('FOREIGN', 'DATA', 'WRAPPER'),
      field('name', $._name),
      $._alter_actions,
    ),

    create_server_statement: $ => seq(
      kw('CREATE'),
      kw('SERVER'),
      optional($._if_not_exists),
      field('name', $._name),
      optional(seq(kw('TYPE'), field('type', $.string))),
      optional(seq(kw('VERSION'), field('version', $.string))),
      kws('FOREIGN', 'DATA', 'WRAPPER'),
      field('wrapper', $._name),
      optional($.options_clause),
    ),

    alter_server_statement: $ => seq(
      kw('ALTER'),
      kw('SERVER'),
      field('name', $._name),
      $._alter_actions,
    ),

    create_user_mapping_statement: $ => seq(
      kw('CREATE'),
      kws('USER', 'MAPPING'),
      optional($._if_not_exists),
      $._user_mapping,
      optional($.options_clause),
    ),

    alter_user_mapping_statement: $ => seq(
      kw('ALTER'),
      kws('USER', 'MAPPING'),
      $._user_mapping,
      $.options_clause,
    ),

    _user_mapping: $ => seq(
      kw('FOR'),
      field('user', choice($._role, keywordName($, 'USER'))),
      kw('SERVER'),
      field('server', $._name),
    ),

    import_foreign_schema_statement: $ => seq(
      kws('IMPORT', 'FOREIGN', 'SCHEMA'),
      field('remote_schema', $._name),
      optional(seq(
        choice(kws('LIMIT', 'TO'), kw('EXCEPT')),
        parens(commaSep1(field('table', $._name))),
      )),
      kws('FROM', 'SERVER'),
      field('server', $._name),
      kw('INTO'),
      field('schema', $._name),
      optional($.options_clause),
    ),

    // Other objects

    create_tablespace_statement: $ => seq(
      kw('CREATE'),
      kw('TABLESPACE'),
      field('name', $._name),
      optional(seq(kw('OWNER'), field('owner', $._role))),
      kw('LOCATION'),
      field('location', $.string),
      optional(seq(kw('WITH'), $.option_list)),
    ),

    create_collation_statement: $ => seq(
      kw('CREATE'),
      kw('COLLATION'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      choice(
        $.definition_list,
        seq(kw('FROM'), field('source', $.object_reference)),
      ),
    ),

    create_conversion_statement: $ => seq(
      kw('CREATE'),
      optional(kw('DEFAULT')),
      kw('CONVERSION'),
      field('name', $.object_reference),
      kw('FOR'),
      field('source', $.string),
      kw('TO'),
      field('target', $.string),
      kw('FROM'),
      field('function', $.object_reference),
    ),

    create_text_search_statement: $ => seq(
      kw('CREATE'),
      kws('TEXT', 'SEARCH'),
      choice(kw('CONFIGURATION'), kw('DICTIONARY'), kw('PARSER'), kw('TEMPLATE')),
      field('name', $.object_reference),
      $.definition_list,
    ),

    // The name of extended statistics can be left out.
    create_statistics_statement: $ => seq(
      kw('CREATE'),
      kw('STATISTICS'),
      optional(seq(optional($._if_not_exists), field('name', $.object_reference))),
      optional(parens(commaSep1(field('kind', $._name)))),
      kw('ON'),
      commaSep1($._expression),
      kw('FROM'),
      commaSep1($._table_reference),
    ),

    create_cast_statement: $ => seq(
      kw('CREATE'),
      kw('CAST'),
      parens(seq(field('source', $.type), kw('AS'), field('target', $.type))),
      choice(
        seq(kws('WITH', 'FUNCTION'), $.function_signature),
        kws('WITHOUT', 'FUNCTION'),
        kws('WITH', 'INOUT'),
      ),
      optional(seq(kw('AS'), choice(kw('ASSIGNMENT'), kw('IMPLICIT')))),
    ),

    create_language_statement: $ => seq(
      kw('CREATE'),
      optional(kws('OR', 'REPLACE')),
      optional(kw('TRUSTED')),
      optional(kw('PROCEDURAL')),
      kw('LANGUAGE'),
      field('name', $._name),
      optional(seq(
        kw('HANDLER'),
        field('handler', $.object_reference),
        optional(seq(kw('INLINE'), field('inline', $.object_reference))),
        optional(seq(kw('VALIDATOR'), field('validator', $.object_reference))),
      )),
    ),

    create_transform_statement: $ => seq(
      kw('CREATE'),
      optional(kws('OR', 'REPLACE')),
      kws('TRANSFORM', 'FOR'),
      field('type', $.type),
      kw('LANGUAGE'),
      field('language', $._name),
      parens(commaSep1(seq(
        choice(kw('FROM'), kw('TO')),
        kws('SQL', 'WITH', 'FUNCTION'),
        $.function_signature,
      ))),
    ),

    create_access_method_statement: $ => seq(
      kw('CREATE'),
      kws('ACCESS', 'METHOD'),
      field('name', $._name),
      kw('TYPE'),
      choice(kw('TABLE'), kw('INDEX')),
      kw('HANDLER'),
      field('handler', $.object_reference),
    ),

    // ALTER of the other kinds of object, which take the actions of
    // _alter_actions.
    alter_object_statement: $ => seq(
      kw('ALTER'),
      choice(
        seq(
          choice(
            kw('COLLATION'),
            kw('CONVERSION'),
            seq(optional(kw('PROCEDURAL')), kw('LANGUAGE')),
            kw('STATISTICS'),
            kw('TABLESPACE'),
            seq(
              kws('TEXT', 'SEARCH'),
              choice(kw('CONFIGURATION'), kw('DICTIONARY'), kw('PARSER'), kw('TEMPLATE')),
            ),
          ),
          optional($._if_exists),
          field('name', $.object_reference),
        ),
        seq(kws('LARGE', 'OBJECT'), field('oid', $.number)),
      ),
      $._alter_actions,
    ),

    alter_system_statement: $ => seq(
      kws('ALTER', 'SYSTEM'),
      $._set_parameter,
    ),

    // WAIT FOR LSN waits until the server replays a location of the
    // write-ahead log.
    wait_statement: $ => seq(
      kws('WAIT', 'FOR', 'LSN'),
      field('lsn', $.string),
      optional(seq(kw('WITH'), $.option_list)),
    ),

    _query: $ => prec.right(seq(
      optional($.with_clause),
      $._query_body,
      optional($.order_by_clause),
      repeat(choice(
        $.limit_clause,
        $.offset_clause,
        $.fetch_clause,
        $.locking_clause,
      )),
    )),

    with_clause: $ => seq(
      kw('WITH'),
      optional(kw('RECURSIVE')),
      commaSep1($.cte),
    ),

    cte: $ => seq(
      field('name', $._name),
      optional($.column_list),
      kw('AS'),
      optional(seq(optional(kw('NOT')), kw('MATERIALIZED'))),
      parens(field('query', choice($._query, $._modifying_statement))),
      optional($.search_clause),
      optional($.cycle_clause),
    ),

    search_clause: $ => seq(
      kw('SEARCH'),
      choice(kw('BREADTH'), kw('DEPTH')),
      kws('FIRST', 'BY'),
      commaSep1($._name),
      kw('SET'),
      field('column', $._name),
    ),

    cycle_clause: $ => seq(
      kw('CYCLE'),
      commaSep1($._name),
      kw('SET'),
      field('mark', $._name),
      optional(seq(
        kw('TO'),
        field('value', $._primary_expression),
        kw('DEFAULT'),
        field('default', $._primary_expression),
      )),
      kw('USING'),
      field('path', $._name),
    ),

    column_list: $ => parens(commaSep1($._name)),

    _query_body: $ => choice(
      $.select,
      $.values,
      $.table_query,
      $.set_operation,
      $.parenthesized_query,
    ),

    parenthesized_query: $ => parens($._query),

    // INTERSECT binds more tightly than UNION and EXCEPT.
    set_operation: $ => choice(
      prec.left(1, seq(
        field('left', $._query_body),
        field('operator', choice(kw('UNION'), kw('EXCEPT'))),
        optional(choice(kw('ALL'), kw('DISTINCT'))),
        field('right', $._query_body),
      )),
      prec.left(2, seq(
        field('left', $._query_body),
        field('operator', kw('INTERSECT')),
        optional(choice(kw('ALL'), kw('DISTINCT'))),
        field('right', $._query_body),
      )),
    ),

    select: $ => prec.right(seq(
      kw('SELECT'),
      optional(choice(kw('ALL'), $.distinct_clause)),
      optional(commaSep1($._select_item)),
      optional($.into_clause),
      optional($.from_clause),
      optional($.where_clause),
      optional($.group_by_clause),
      optional($.having_clause),
      optional($.window_clause),
    )),

    // ON after DISTINCT starts DISTINCT ON, and not ON CONFLICT of INSERT.
    distinct_clause: $ => prec.right(seq(
      kw('DISTINCT'),
      optional(seq(kw('ON'), parens(commaSep1($._expression)))),
    )),

    _select_item: $ => choice(
      $._expression,
      $.aliased_expression,
      alias('*', $.all_columns),
    ),

    aliased_expression: $ => choice(
      seq(field('value', $._expression), kw('AS'), field('alias', $._label)),
      seq(field('value', $._expression), field('alias', $._name)),
    ),

    // t.* is all the columns of t. It is an expression, because it can be
    // the argument of a function, such as count(t.*).
    all_columns: $ => choice(
      seq(field('table', $._name), '.', '*'),
      seq(field('schema', $._name), '.', field('table', $._label), '.', '*'),
    ),

    into_clause: $ => seq(
      kw('INTO'),
      optional(choice(kw('TEMPORARY'), kw('TEMP'), kw('UNLOGGED'))),
      optional(kw('TABLE')),
      field('table', $.object_reference),
    ),

    values: $ => seq(kw('VALUES'), commaSep1($.value_list)),

    value_list: $ => parens(commaSep1(choice($._expression, kw('DEFAULT')))),

    table_query: $ => seq(
      kw('TABLE'),
      optional(kw('ONLY')),
      field('table', $.object_reference),
      optional('*'),
    ),

    from_clause: $ => seq(kw('FROM'), commaSep1($._table_reference)),

    _table_reference: $ => choice(
      $.table_reference,
      $.derived_table,
      $.function_table,
      $.join,
      $.parenthesized_join,
    ),

    parenthesized_join: $ => prec.right(seq(
      parens(choice($.join, $.parenthesized_join)),
      optional($._alias),
    )),

    table_reference: $ => prec.right(seq(
      optional(kw('ONLY')),
      field('table', $.object_reference),
      optional('*'),
      optional($._alias),
      optional($.tablesample_clause),
    )),

    _alias: $ => seq(
      choice(
        seq(kw('AS'), field('alias', $._label)),
        field('alias', $._name),
      ),
      optional($.column_list),
    ),

    tablesample_clause: $ => seq(
      kw('TABLESAMPLE'),
      field('method', $._name),
      parens(commaSep1($._expression)),
      optional(seq(kw('REPEATABLE'), parens(field('seed', $._expression)))),
    ),

    derived_table: $ => prec.right(seq(
      optional(kw('LATERAL')),
      $.subquery,
      optional($._alias),
    )),

    // A function in FROM can have an alias with a list of columns, or with a
    // list of column definitions for a function that returns record.
    function_table: $ => prec.right(seq(
      optional(kw('LATERAL')),
      choice(
        $.function_call,
        // ROWS is not a keyword here, so that rows can name a table.
        seq(
          alias($.identifier, 'ROWS'),
          kw('FROM'),
          parens(commaSep1(seq(
            $.function_call,
            optional(seq(kw('AS'), $.column_definitions)),
          ))),
        ),
      ),
      optional(kws('WITH', 'ORDINALITY')),
      optional(choice(
        seq(
          choice(
            seq(kw('AS'), field('alias', $._label)),
            field('alias', $._name),
          ),
          optional(choice($.column_list, $.column_definitions)),
        ),
        seq(kw('AS'), $.column_definitions),
      )),
    )),

    column_definitions: $ => parens(commaSep1($.column_definition)),

    // A join can nest to the right without parentheses, as in a JOIN b
    // JOIN c ON x ON y, so the parser keeps both readings until an ON
    // decides.
    join: $ => prec.dynamic(0, seq(
      field('left', $._table_reference),
      choice(
        kws('CROSS', 'JOIN'),
        seq(
          optional(kw('NATURAL')),
          optional(choice(
            kw('INNER'),
            seq(choice(kw('LEFT'), kw('RIGHT'), kw('FULL')), optional(kw('OUTER'))),
          )),
          kw('JOIN'),
        ),
      ),
      field('right', $._table_reference),
      optional(choice(
        seq(kw('ON'), field('condition', $._expression)),
        seq(
          kw('USING'),
          parens(commaSep1(field('using', $._name))),
          optional(seq(kw('AS'), field('alias', $._name))),
        ),
      )),
    )),

    // WHERE CURRENT OF names the row of a cursor in UPDATE and DELETE.
    // CURRENT is not a keyword there, so that current can also name a
    // column.
    where_clause: $ => seq(kw('WHERE'), choice($._expression, $.current_of)),

    current_of: $ => seq(
      alias($.identifier, 'CURRENT'),
      kw('OF'),
      field('cursor', $._name),
    ),

    group_by_clause: $ => seq(
      kws('GROUP', 'BY'),
      optional(choice(kw('ALL'), kw('DISTINCT'))),
      commaSep1($._grouping_element),
    ),

    _grouping_element: $ => choice(
      $._expression,
      $.empty_grouping_set,
      $.rollup,
      $.cube,
      $.grouping_sets,
    ),

    empty_grouping_set: _ => seq('(', ')'),

    rollup: $ => seq(kw('ROLLUP'), parens(commaSep1($._expression))),

    cube: $ => seq(kw('CUBE'), parens(commaSep1($._expression))),

    grouping_sets: $ => seq(
      kws('GROUPING', 'SETS'),
      parens(commaSep1($._grouping_element)),
    ),

    having_clause: $ => seq(kw('HAVING'), $._expression),

    window_clause: $ => seq(kw('WINDOW'), commaSep1($.window_definition)),

    window_definition: $ => seq(
      field('name', $._name),
      kw('AS'),
      $.window_specification,
    ),

    window_specification: $ => parens(seq(
      optional(field('name', $._name)),
      optional(seq(kws('PARTITION', 'BY'), commaSep1($._expression))),
      optional($.order_by_clause),
      optional($.frame_clause),
    )),

    frame_clause: $ => seq(
      choice(kw('RANGE'), kw('ROWS'), kw('GROUPS')),
      choice(
        $.frame_bound,
        seq(kw('BETWEEN'), $.frame_bound, kw('AND'), $.frame_bound),
      ),
      optional(seq(
        kw('EXCLUDE'),
        choice(kws('CURRENT', 'ROW'), kw('GROUP'), kw('TIES'), kws('NO', 'OTHERS')),
      )),
    ),

    frame_bound: $ => choice(
      kws('UNBOUNDED', 'PRECEDING'),
      kws('UNBOUNDED', 'FOLLOWING'),
      kws('CURRENT', 'ROW'),
      seq($._expression, choice(kw('PRECEDING'), kw('FOLLOWING'))),
    ),

    order_by_clause: $ => seq(kws('ORDER', 'BY'), commaSep1($.order_by_item)),

    order_by_item: $ => seq(
      field('value', $._expression),
      optional(choice(
        kw('ASC'),
        kw('DESC'),
        seq(kw('USING'), field('operator', $._operator)),
      )),
      optional(seq(kw('NULLS'), choice(kw('FIRST'), kw('LAST')))),
    ),

    limit_clause: $ => seq(
      kw('LIMIT'),
      choice(field('count', $._expression), kw('ALL')),
    ),

    offset_clause: $ => seq(
      kw('OFFSET'),
      field('start', $._expression),
      optional(choice(kw('ROW'), kw('ROWS'))),
    ),

    fetch_clause: $ => seq(
      kw('FETCH'),
      choice(kw('FIRST'), kw('NEXT')),
      optional(field('count', $._fetch_count)),
      choice(kw('ROW'), kw('ROWS')),
      choice(kw('ONLY'), kws('WITH', 'TIES')),
    ),

    _fetch_count: $ => choice(
      $._primary_expression,
      $.unary_expression,
    ),

    locking_clause: $ => seq(
      kw('FOR'),
      $._lock_strength,
      optional(seq(kw('OF'), commaSep1($.object_reference))),
      optional(choice(kw('NOWAIT'), kws('SKIP', 'LOCKED'))),
    ),

    _lock_strength: _ => choice(
      kw('UPDATE'),
      kws('NO', 'KEY', 'UPDATE'),
      kw('SHARE'),
      kws('KEY', 'SHARE'),
    ),

    subquery: $ => parens($._query),

    // Expressions

    _expression: $ => choice(
      $._primary_expression,
      $.unary_expression,
      $.binary_expression,
      $.is_expression,
      $.between_expression,
      $.in_expression,
      $.like_expression,
      $.at_time_zone_expression,
      $.collate_expression,
    ),

    _primary_expression: $ => choice(
      $._literal,
      $.typed_literal,
      $.column_reference,
      $.all_columns,
      $.parameter,
      $.function_call,
      $.cast_expression,
      $.subscript_expression,
      $.field_selection,
      $.parenthesized_expression,
      $.row_constructor,
      $.array_constructor,
      $.subquery,
      $.exists_expression,
      $.case_expression,
    ),

    _literal: $ => choice(
      $.number,
      $.string,
      $.dollar_quoted_string,
      $.boolean,
      $.null,
    ),

    boolean: _ => choice(kw('TRUE'), kw('FALSE')),

    null: _ => kw('NULL'),

    // A type name before a string gives a constant of the type, such as
    // date '2026-10-31'. The name is an identifier, so interval and time
    // also name a column. The fields of an interval, such as DAY, can come
    // after the string.
    typed_literal: $ => prec.right(seq(
      field('type', alias($._literal_type, $.type)),
      field('value', $.string),
      optional($.interval_fields),
    )),

    _literal_type: $ => choice(
      $.identifier,
      alias($._time_zone_type, $.identifier),
    ),

    // The type of a time with WITH TIME ZONE or WITHOUT TIME ZONE before a
    // string is one token, so that WITH after a name can start a clause,
    // as in WITH CHECK OPTION after the query of a view.
    _time_zone_type: _ => token(seq(
      choice(keyword('TIMESTAMP'), keyword('TIME')),
      /\s+/,
      choice(keyword('WITH'), keyword('WITHOUT')),
      /\s+/,
      keyword('TIME'),
      /\s+/,
      keyword('ZONE'),
    )),

    column_reference: $ => choice(
      field('name', $._name),
      seq(field('table', $._name), '.', field('name', $._label)),
      seq(
        field('schema', $._name),
        '.',
        field('table', $._label),
        '.',
        field('name', $._label),
      ),
      seq(
        field('database', $._name),
        '.',
        field('schema', $._label),
        '.',
        field('table', $._label),
        '.',
        field('name', $._label),
      ),
    ),

    parameter: _ => /\$\d+/,

    // A function call has the special forms of the functions that the
    // standard gives a syntax of their own, such as substring(s FROM 2),
    // for each function name. A keyword that names such a function and that
    // PostgreSQL does not reserve is an identifier, so position and trim
    // can also name a column.
    function_call: $ => choice(
      prec.right(seq(
        optional(seq(field('schema', $._name), '.')),
        field('name', $._function_name),
        $.arguments,
        optional($.within_group_clause),
        optional($.filter_clause),
        optional($.over_clause),
      )),
      prec.right(seq(
        field('name', keywordName(
          $,
          'CURRENT_DATE',
          'CURRENT_TIME',
          'CURRENT_TIMESTAMP',
          'LOCALTIME',
          'LOCALTIMESTAMP',
        )),
        optional(parens($.number)),
      )),
      field('name', keywordName(
        $,
        'CURRENT_CATALOG',
        'CURRENT_ROLE',
        'CURRENT_SCHEMA',
        'CURRENT_USER',
        'SESSION_USER',
        'SYSTEM_USER',
        'USER',
      )),
      seq(field('name', keywordName($, 'CURRENT_SCHEMA')), '(', ')'),
      seq(
        field('name', keywordName($, 'COLLATION')),
        kw('FOR'),
        parens($._expression),
      ),
    ),

    _function_name: $ => choice(
      reserved('function_name', $.identifier),
      alias($._quoted_identifier, $.identifier),
    ),

    arguments: $ => parens(optional(choice(
      '*',
      seq(
        optional(choice(kw('DISTINCT'), kw('ALL'))),
        $._argument_list,
        optional($.order_by_clause),
        repeat($._json_clause),
      ),
      seq(repeat1($._json_clause)),
      // The query of json_array. A clause after it splits the parse states
      // of each query, so the grammar leaves out RETURNING after it.
      $._query,
    ))),

    // The arguments of a function. Most functions take a list of
    // expressions with commas between them. The functions that the standard
    // gives a syntax of their own have keywords between their arguments,
    // such as substring(s FROM 2 FOR 3), trim(BOTH 'x' FROM s) and
    // json_object('a' VALUE 1). The grammar accepts each of these keywords
    // between two arguments of any function, so that each argument is in
    // the same parse states.
    _argument_list: $ => seq(
      optional(choice(
        kw('FROM'),
        seq(choice(kw('BOTH'), kw('LEADING'), kw('TRAILING')), optional(kw('FROM'))),
      )),
      $._argument,
      repeat(seq($._argument_separator, $._argument)),
    ),

    _argument_separator: _ => choice(
      ',',
      ':',
      kw('FROM'),
      kw('FOR'),
      kw('PLACING'),
      kw('VALUE'),
    ),

    _argument: $ => choice(
      // FORMAT JSON after an argument, as in json_object('a': x FORMAT
      // JSON), splits the parse states of each expression in two, because
      // FORMAT starts with FOR, the keyword of substring(s FROM 2 FOR 3).
      // So the grammar leaves it out.
      $._expression,
      $.named_argument,
      $.variadic_argument,
      seq($._expression, $._as_type),
      // NAME x of xmlelement and xmlpi, and CONTENT x or DOCUMENT x of
      // xmlparse and xmlserialize. The first word is not a keyword, so
      // that name and content can also name a column.
      seq(alias($.identifier, $.keyword), $._name, optional($._as_type)),
    ),

    _json_format: $ => seq(kws('FORMAT', 'JSON'), optional(seq(kw('ENCODING'), $._name))),

    // AS and a type after an argument of treat, xmlserialize and the
    // functions of SQL/JSON.
    _as_type: $ => seq(kw('AS'), $.type, optional(seq(optional(kw('NO')), kw('INDENT')))),

    named_argument: $ => seq(
      field('name', $._name),
      choice('=>', ':='),
      field('value', $._expression),
    ),

    variadic_argument: $ => seq(kw('VARIADIC'), choice($._expression, $.named_argument)),

    // The clauses of the functions of SQL/JSON, after their arguments.
    _json_clause: $ => choice(
      seq(kw('RETURNING'), $.type, optional($._json_format)),
      seq(choice(kw('NULL'), kw('ABSENT')), kw('ON'), kw('NULL')),
      seq(choice(kw('WITH'), kw('WITHOUT')), kw('UNIQUE'), optional(kw('KEYS'))),
      seq(kw('PASSING'), optional(seq(kw('BY'), choice(kw('REF'), kw('VALUE')))), $._argument_list),
      seq(
        choice(kw('WITH'), kw('WITHOUT')),
        optional(choice(kw('CONDITIONAL'), kw('UNCONDITIONAL'))),
        optional(kw('ARRAY')),
        kw('WRAPPER'),
      ),
      seq(choice(kw('KEEP'), kw('OMIT')), kw('QUOTES'), optional(kws('ON', 'SCALAR', 'STRING'))),
      seq(
        choice(
          kw('ERROR'),
          kw('NULL'),
          seq(kw('EMPTY'), optional(choice(kw('ARRAY'), kw('OBJECT')))),
          seq(kw('DEFAULT'), $._primary_expression),
          kw('TRUE'),
          kw('FALSE'),
          kw('UNKNOWN'),
        ),
        kw('ON'),
        choice(kw('EMPTY'), kw('ERROR')),
      ),
    ),

    within_group_clause: $ => seq(
      kws('WITHIN', 'GROUP'),
      parens($.order_by_clause),
    ),

    filter_clause: $ => seq(
      kw('FILTER'),
      parens(seq(kw('WHERE'), $._expression)),
    ),

    over_clause: $ => seq(
      kw('OVER'),
      choice(field('window', $._name), $.window_specification),
    ),

    // CAST(x AS type), and x::type, the form of PostgreSQL.
    cast_expression: $ => choice(
      seq(kw('CAST'), parens(seq(field('value', $._expression), kw('AS'), field('type', $.type)))),
      prec.left(PREC.cast, seq(field('value', $._primary_expression), '::', field('type', $.type))),
    ),

    subscript_expression: $ => prec(PREC.subscript, seq(
      field('value', $._primary_expression),
      '[',
      choice(
        field('index', $._expression),
        seq(
          optional(field('lower', $._expression)),
          ':',
          optional(field('upper', $._expression)),
        ),
      ),
      ']',
    )),

    // A field of a value of a composite type, such as (t).name or
    // (f(x)).*.
    field_selection: $ => prec(PREC.field, seq(
      field('value', choice($.parenthesized_expression, $.subscript_expression, $.function_call, $.field_selection)),
      '.',
      choice(field('field', $._label), '*'),
    )),

    parenthesized_expression: $ => parens($._expression),

    row_constructor: $ => choice(
      seq(kw('ROW'), parens(optional(commaSep1($._expression)))),
      parens(seq($._expression, ',', commaSep1($._expression))),
    ),

    array_constructor: $ => choice(
      seq(kw('ARRAY'), $._array_elements),
      seq(kw('ARRAY'), $.subquery),
    ),

    _array_elements: $ => seq(
      '[',
      optional(choice(
        commaSep1($._expression),
        commaSep1(alias($._array_elements, $.array_constructor)),
      )),
      ']',
    ),

    exists_expression: $ => seq(kw('EXISTS'), $.subquery),

    case_expression: $ => seq(
      kw('CASE'),
      optional(field('value', $._expression)),
      repeat1($.when_clause),
      optional(seq(kw('ELSE'), field('else', $._expression))),
      kw('END'),
    ),

    when_clause: $ => seq(
      kw('WHEN'),
      field('condition', $._expression),
      kw('THEN'),
      field('result', $._expression),
    ),

    // A choice of operators flattens into one production for each
    // operator, and each production has parse states of its own. So each
    // level of precedence takes its operators from one hidden rule.
    unary_expression: $ => choice(
      prec.right(PREC.not, seq(field('operator', kw('NOT')), field('operand', $._expression))),
      prec.right(PREC.unary, seq(field('operator', alias($._sign, $.operator)), field('operand', $._expression))),
      prec.right(PREC.other, seq(field('operator', $._other_operator), field('operand', $._expression))),
    ),

    // The operators of a level of precedence are one token, which is the
    // node operator.
    _sign: _ => token(choice('-', '+')),

    binary_expression: $ => {
      /** @type {[number, RuleOrLiteral, RuleOrLiteral][]} */
      const table = [
        [PREC.or, kw('OR'), $._expression],
        [PREC.and, kw('AND'), $._expression],
        [PREC.exponent, alias('^', $.operator), $._expression],
        [PREC.multiply, alias($._multiplicative_operator, $.operator), $._expression],
        [PREC.add, alias($._sign, $.operator), $._expression],
        [PREC.like, kw('OVERLAPS'), $._expression],
        [PREC.compare, alias($._comparison_operator, $.operator), $._operand],
        [PREC.other, $._other_operator, $._operand],
      ];
      return choice(...table.map(([p, op, right]) => prec.left(p, seq(
        field('left', $._expression),
        field('operator', op),
        field('right', right),
      ))));
    },

    _multiplicative_operator: _ => token(choice('*', '/', '%')),

    _comparison_operator: _ => token(choice('=', '<', '>', '<=', '>=', '<>', '!=')),

    _other_operator: $ => choice($.operator, $.qualified_operator),

    _operand: $ => choice($._expression, $.quantified_expression),

    // ANY, SOME or ALL after an operator compares the left side with each
    // row of a subquery or each element of an array.
    quantified_expression: $ => seq(
      field('quantifier', choice(kw('ANY'), kw('SOME'), kw('ALL'))),
      parens(choice($._query, $._expression)),
    ),

    // OPERATOR(schema.op) names an operator with its schema.
    qualified_operator: $ => seq(
      kw('OPERATOR'),
      parens(seq(optional(seq(field('schema', $._name), '.')), field('name', $._operator))),
    ),

    _operator: $ => choice(
      $.operator,
      alias(choice($._sign, $._multiplicative_operator, '^', $._comparison_operator), $.operator),
    ),

    is_expression: $ => prec.left(PREC.is, choice(
      seq(
        field('left', $._expression),
        $._is_operator,
        field('right', choice(
          $.boolean,
          $.null,
          kw('UNKNOWN'),
          kw('DOCUMENT'),
          seq(optional(choice(kw('NFC'), kw('NFD'), kw('NFKC'), kw('NFKD'))), kw('NORMALIZED')),
          $.distinct_from,
          $.json_predicate,
        )),
      ),
      seq(field('left', $._expression), choice(kw('ISNULL'), kw('NOTNULL'))),
    )),

    _is_operator: _ => seq(kw('IS'), optional(kw('NOT'))),

    distinct_from: $ => seq(kws('DISTINCT', 'FROM'), $._expression),

    json_predicate: $ => prec.right(seq(
      kw('JSON'),
      optional(choice(kw('VALUE'), kw('ARRAY'), kw('OBJECT'), kw('SCALAR'))),
      optional(seq(choice(kw('WITH'), kw('WITHOUT')), kw('UNIQUE'), optional(kw('KEYS')))),
    )),

    between_expression: $ => prec.left(PREC.like, seq(
      field('left', $._expression),
      $._between_operator,
      field('low', $._expression),
      kw('AND'),
      field('high', $._expression),
    )),

    _between_operator: _ => seq(
      optional(kw('NOT')),
      kw('BETWEEN'),
      optional(choice(kw('SYMMETRIC'), kw('ASYMMETRIC'))),
    ),

    // The right side of IN is a list or a subquery. It can also be a
    // string or a name, as in position('b' IN 'abc'), the form of the
    // function position.
    in_expression: $ => prec.left(PREC.like, seq(
      field('left', $._expression),
      $._in_operator,
      field('right', choice(
        $.subquery,
        $.list,
        $.string,
        $.column_reference,
        $.function_call,
        $.typed_literal,
        $.parameter,
      )),
    )),

    _in_operator: _ => seq(optional(kw('NOT')), kw('IN')),

    list: $ => parens(commaSep1($._expression)),

    like_expression: $ => prec.left(PREC.like, seq(
      field('left', $._expression),
      field('operator', $._like_operator),
      field('right', $._operand),
      optional(seq(kw('ESCAPE'), field('escape', $._expression))),
    )),

    _like_operator: _ => seq(
      optional(kw('NOT')),
      // SIMILAR without TO is the form of substring(s SIMILAR p ESCAPE e).
      choice(kw('LIKE'), kw('ILIKE'), seq(kw('SIMILAR'), optional(kw('TO')))),
    ),

    at_time_zone_expression: $ => prec.left(PREC.at, seq(
      field('value', $._expression),
      kw('AT'),
      choice(
        seq(kws('TIME', 'ZONE'), field('zone', $._expression)),
        kw('LOCAL'),
      ),
    )),

    collate_expression: $ => prec.left(PREC.collate, seq(
      field('value', $._expression),
      kw('COLLATE'),
      field('collation', $.object_reference),
    )),

    // Types

    type: $ => prec.right(seq(
      choice(
        seq(
          optional(seq(field('schema', $._name), '.')),
          field('name', $._name),
          optional($.type_modifiers),
        ),
        kws('DOUBLE', 'PRECISION'),
        seq(
          choice(
            seq(optional(kw('NATIONAL')), choice(kw('CHARACTER'), kw('CHAR'))),
            kw('NCHAR'),
            kw('BIT'),
          ),
          optional(kw('VARYING')),
          optional($.type_modifiers),
        ),
        seq(
          choice(kw('TIME'), kw('TIMESTAMP')),
          optional($.type_modifiers),
          optional(seq(choice(kw('WITH'), kw('WITHOUT')), kws('TIME', 'ZONE'))),
        ),
        seq(
          kw('INTERVAL'),
          optional($.interval_fields),
          optional($.type_modifiers),
        ),
      ),
      optional(choice(
        repeat1(seq('[', optional($.number), ']')),
        seq(kw('ARRAY'), optional(seq('[', $.number, ']'))),
      )),
    )),

    type_modifiers: $ => parens(commaSep1(choice($._signed_number, $.string, $._name))),

    interval_fields: $ => prec.right(choice(
      kw('YEAR'),
      kw('MONTH'),
      kw('DAY'),
      kw('HOUR'),
      kw('MINUTE'),
      seq(kw('SECOND'), optional(parens($.number))),
      kws('YEAR', 'TO', 'MONTH'),
      kws('DAY', 'TO', 'HOUR'),
      kws('DAY', 'TO', 'MINUTE'),
      seq(kws('DAY', 'TO', 'SECOND'), optional(parens($.number))),
      kws('HOUR', 'TO', 'MINUTE'),
      seq(kws('HOUR', 'TO', 'SECOND'), optional(parens($.number))),
      seq(kws('MINUTE', 'TO', 'SECOND'), optional(parens($.number))),
    )),

    // Names and literals

    object_reference: $ => choice(
      field('name', $._name),
      seq(field('schema', $._name), '.', field('name', $._label)),
      seq(
        field('database', $._name),
        '.',
        field('schema', $._label),
        '.',
        field('name', $._label),
      ),
    ),

    // A name is an identifier or a quoted identifier, and both are the node
    // identifier.
    _name: $ => choice(
      $.identifier,
      alias($._quoted_identifier, $.identifier),
    ),

    // A label after AS or after a dot can be any keyword.
    _label: $ => choice(
      reserved('label', $.identifier),
      alias($._quoted_identifier, $.identifier),
    ),

    identifier: _ => /[A-Za-z_\u0080-\u{10FFFF}][A-Za-z0-9_$\u0080-\u{10FFFF}]*/u,

    // A quoted identifier is in double quotes, with a double quote doubled.
    // U& before it allows Unicode escapes, with an escape character that
    // UESCAPE can change.
    _quoted_identifier: _ => token(choice(
      /"([^"]|"")*"/,
      seq(/[uU]&"([^"]|"")*"/, optional(/\s*[uU][eE][sS][cC][aA][pP][eE]\s*'[^']'/)),
    )),

    // An operator is a sequence of the characters + - * / < > = ~ ! @ # % ^
    // & | ` ?. A sequence that ends in + or - must hold one of ~ ! @ # % ^ &
    // | ` ?, so the grammar accepts as one operator only a sequence that
    // holds one of them. The operators of the grammar, such as <=, are
    // tokens of their own.
    operator: _ => token(choice(
      /[+\-*/<>=]*[~!@#%^&|`?][+\-*/<>=~!@#%^&|`?]*/,
      /[+\-*/<>=]+[*/<>=]/,
    )),

    // A number is an integer, a decimal number or a number with an exponent,
    // or an integer in hexadecimal, octal or binary form. An underscore can
    // separate two digits.
    number: _ => token(choice(
      /(\d(_?\d)*(\.(\d(_?\d)*)?)?|\.\d(_?\d)*)([eE][+-]?\d(_?\d)*)?/,
      /0[xX](_?[0-9A-Fa-f])+/,
      /0[oO](_?[0-7])+/,
      /0[bB](_?[01])+/,
    )),

    // A string is in single quotes, with a quote doubled. E before it
    // allows backslash escapes, U& allows Unicode escapes with an
    // escape character that UESCAPE can change, B and X give a bit string,
    // and N gives a national character string. Two strings with a newline
    // between them are one string.
    string: _ => {
      const plain = /'([^']|'')*'/;
      const parts = [
        /[eE]'([^'\\]|\\(.|\n)|'')*'/,
        seq(/[uU]&'([^']|'')*'/, optional(/\s*[uU][eE][sS][cC][aA][pP][eE]\s*'[^']'/)),
        /[bBxXnN]'([^']|'')*'/,
        plain,
      ];
      return token(seq(
        choice(...parts),
        repeat(seq(/[ \t\f\r]*\n\s*/, choice(plain, /[eE]'([^'\\]|\\(.|\n)|'')*'/))),
      ));
    },

    dollar_quoted_string: $ => seq(
      $._dollar_quote_start,
      optional($.string_content),
      $._dollar_quote_end,
    ),
  },
});
