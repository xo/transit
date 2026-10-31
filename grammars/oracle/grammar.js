/**
 * @file The grammar of the SQL and the PL/SQL of Oracle Database 23ai
 * @license MIT
 */

// xo writes this grammar from the SQL Language Reference and the PL/SQL
// Language Reference of Oracle Database 23ai (D124). It does not come from
// the grammar of andreasmaierde/tree-sitter-plsql.
//
// The grammar is for a highlighter and for completion, so it accepts more
// than the server does where that keeps the rules small. A keyword is a
// token in any case, and it is an anonymous node with the name of the
// keyword in upper case, such as "SELECT". The token identifier is the word
// of the grammar, so a keyword that Oracle does not reserve is an
// identifier in each place where the grammar does not expect the keyword.
// The reserved words of Oracle SQL are in the reserved set global, so they
// are not identifiers, but for a name after a dot, where the grammar
// accepts every keyword.
//
// The grammar keeps its parse tables small (D122, D124). A parse state with
// more than 64 columns takes the large form, so the grammar keeps the
// columns of a state few. The clauses of a query after FROM, and the
// clauses of a table after its name, come in any order, because a sequence
// of optional clauses costs parse states for each set of the clauses.
//
// The external scanner in src/scanner.c reads the comments, the strings
// with the alternative quoting of Oracle, such as q'[it's]', and a / alone
// on its line. In a script of SQL*Plus, such a / ends a statement or a
// PL/SQL unit, so the grammar takes it as the end of a statement, as it
// takes a semicolon. The other commands of SQL*Plus belong to usql. The
// variables of usql belong to the usql grammar, which hands this grammar a
// placeholder of the same length in place of each variable (D101). A
// statement ends at a semicolon, at a / alone on its line, or at the end of
// the input.

/// <reference types="tree-sitter-cli/dsl" />
// @ts-check

// The precedences of the operators and the conditions, from the lowest to
// the highest, as the sections "About SQL Operators" and "About SQL
// Conditions" of the SQL Language Reference list them.
const PREC = {
  or: 1,
  and: 2,
  not: 3,
  condition: 4,
  compare: 5,
  add: 6,
  multiply: 7,
  exponent: 8,
  unary: 9,
  at: 10,
  collate: 11,
  field: 12,
};

// The reserved words of Oracle SQL, from the appendix "Oracle SQL Reserved
// Words" of the SQL Language Reference, which lists the words of the view
// V$RESERVED_WORDS that are reserved.
const RESERVED_WORDS = [
  'ACCESS', 'ADD', 'ALL', 'ALTER', 'AND', 'ANY', 'AS', 'ASC', 'AUDIT',
  'BETWEEN', 'BY', 'CHAR', 'CHECK', 'CLUSTER', 'COLUMN', 'COLUMN_VALUE',
  'COMMENT', 'COMPRESS', 'CONNECT', 'CREATE', 'CURRENT', 'DATE', 'DECIMAL',
  'DEFAULT', 'DELETE', 'DESC', 'DISTINCT', 'DROP', 'ELSE', 'EXCLUSIVE',
  'EXISTS', 'FILE', 'FLOAT', 'FOR', 'FROM', 'GRANT', 'GROUP', 'HAVING',
  'IDENTIFIED', 'IMMEDIATE', 'IN', 'INCREMENT', 'INDEX', 'INITIAL', 'INSERT',
  'INTEGER', 'INTERSECT', 'INTO', 'IS', 'LEVEL', 'LIKE', 'LOCK', 'LONG',
  'MAXEXTENTS', 'MINUS', 'MLSLABEL', 'MODE', 'MODIFY', 'NESTED_TABLE_ID',
  'NOAUDIT', 'NOCOMPRESS', 'NOT', 'NOWAIT', 'NULL', 'NUMBER', 'OF', 'OFFLINE',
  'ON', 'ONLINE', 'OPTION', 'OR', 'ORDER', 'PCTFREE', 'PRIOR', 'PUBLIC', 'RAW',
  'RENAME', 'RESOURCE', 'REVOKE', 'ROW', 'ROWID', 'ROWNUM', 'ROWS', 'SELECT',
  'SESSION', 'SET', 'SHARE', 'SIZE', 'SMALLINT', 'START', 'SUCCESSFUL',
  'SYNONYM', 'SYSDATE', 'TABLE', 'THEN', 'TO', 'TRIGGER', 'UID', 'UNION',
  'UNIQUE', 'UPDATE', 'USER', 'VALIDATE', 'VALUES', 'VARCHAR', 'VARCHAR2',
  'VIEW', 'WHENEVER', 'WHERE', 'WITH',
];

// The words that the rules use as keywords. The rules run before the
// reserved sets, so kw fills it before the reserved sets read it. A
// reserved word must be a token of the grammar, so the reserved set holds
// only the reserved words that are in it.
const KEYWORDS = new Set();

// The reserved words that are pseudocolumns or functions with no
// arguments, such as ROWNUM and SYSDATE. Each one is an expression.
const PSEUDOCOLUMNS = [
  'COLUMN_VALUE', 'LEVEL', 'NESTED_TABLE_ID', 'ROWID', 'ROWNUM', 'SYSDATE',
  'UID', 'USER',
];

// The reserved words that name a built-in type.
const TYPE_WORDS = [
  'CHAR', 'DATE', 'DECIMAL', 'FLOAT', 'INTEGER', 'NUMBER', 'RAW', 'ROWID',
  'SMALLINT', 'VARCHAR', 'VARCHAR2',
];

/**
 * Returns a keyword: a token that matches word in any case, as an
 * anonymous node with the name word.
 *
 * @param {string} word
 */
function kw(word) {
  KEYWORDS.add(word);
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
  name: 'oracle',

  extras: $ => [
    /\s/,
    $.comment,
  ],

  // The order of the external tokens is the order of enum TokenType in
  // src/scanner.c.
  externals: $ => [
    $.comment,
    $._q_string,
    $._slash,
  ],

  word: $ => $.identifier,

  // The reserved set global holds the reserved words of Oracle SQL that
  // the grammar uses as keywords. The set
  // label is empty, because the grammar accepts every keyword as a name
  // after a dot. A state takes the last set of its items, so the sets go
  // from the strictest to the least strict.
  reserved: {
    global: _ => RESERVED_WORDS.filter(w => KEYWORDS.has(w)).map(keyword),
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
    $._primary_expression,
    $._operand,
    $._alias,
  ],

  conflicts: $ => [
    // A parenthesized query can be the left side of a set operation, or a
    // subquery in an expression or in FROM.
    [$.parenthesized_query, $.subquery],
    // ( after the table of INSERT can start its columns or its query.
    [$.insert_into_clause],
  ],

  rules: {
    source_file: $ => seq(
      repeat(choice(seq($._statement, $._terminator), $._terminator)),
      optional($._statement),
    ),

    // A / alone on its line ends a statement, as a semicolon does.
    _terminator: $ => choice(';', alias($._slash, '/')),

    _statement: $ => choice(
      $.select_statement,
      $.insert_statement,
      $.update_statement,
      $.delete_statement,
      $.merge_statement,
      $.commit_statement,
      $.rollback_statement,
      $.savepoint_statement,
      $.set_transaction_statement,
      $.set_constraints_statement,
      $.set_role_statement,
      $.lock_table_statement,
      $.alter_session_statement,
      $.alter_system_statement,
      $.explain_plan_statement,
      $.call_statement,
      $.truncate_statement,
      $.purge_statement,
      $.flashback_table_statement,
      $.flashback_database_statement,
      $.analyze_statement,
      $.audit_statement,
      $.noaudit_statement,
      $.comment_statement,
      $.grant_statement,
      $.revoke_statement,
      $.rename_statement,
    ),

    // Queries

    select_statement: $ => $._query,

    // The clauses after the body of a query come in any order.
    _query: $ => prec.right(seq(
      optional($.with_clause),
      $._query_body,
      repeat(choice(
        $.order_by_clause,
        $.offset_clause,
        $.fetch_clause,
        $.for_update_clause,
        $.subquery_restriction,
      )),
    )),

    with_clause: $ => seq(kw('WITH'), commaSep1($.cte)),

    // A subquery factoring clause.
    cte: $ => seq(
      field('name', $._name),
      optional($.column_list),
      kw('AS'),
      $.subquery,
      repeat(choice($.search_clause, $.cycle_clause)),
    ),

    search_clause: $ => seq(
      kw('SEARCH'),
      choice(kw('DEPTH'), kw('BREADTH')),
      kws('FIRST', 'BY'),
      commaSep1($.order_by_item),
      kw('SET'),
      field('column', $._name),
    ),

    cycle_clause: $ => seq(
      kw('CYCLE'),
      commaSep1($._name),
      kw('SET'),
      field('mark', $._name),
      kw('TO'),
      field('value', $._primary_expression),
      kw('DEFAULT'),
      field('default', $._primary_expression),
    ),

    column_list: $ => parens(commaSep1($._name)),

    _query_body: $ => choice(
      $.select,
      $.values,
      $.set_operation,
      $.parenthesized_query,
    ),

    parenthesized_query: $ => parens($._query),

    // The set operators of Oracle have the same precedence.
    set_operation: $ => prec.left(seq(
      field('left', $._query_body),
      field('operator', choice(kw('UNION'), kw('INTERSECT'), kw('MINUS'), kw('EXCEPT'))),
      optional(choice(kw('ALL'), kw('DISTINCT'))),
      field('right', $._query_body),
    )),

    // The clauses after FROM come in any order.
    select: $ => prec.right(seq(
      kw('SELECT'),
      optional(choice(kw('ALL'), kw('DISTINCT'), kw('UNIQUE'))),
      commaSep1($._select_item),
      optional($.into_clause),
      optional($.from_clause),
      repeat(choice(
        $.where_clause,
        $.start_with_clause,
        $.connect_by_clause,
        $.group_by_clause,
        $.having_clause,
        $.window_clause,
        $.model_clause,
      )),
    )),

    _select_item: $ => choice(
      $._expression,
      $.aliased_expression,
      $.all_columns,
    ),

    aliased_expression: $ => seq(
      field('value', $._expression),
      optional(kw('AS')),
      field('alias', $._name),
    ),

    all_columns: $ => choice(
      '*',
      seq(field('table', $._name), '.', '*'),
      seq(field('schema', $._name), '.', field('table', $._label), '.', '*'),
    ),

    // INTO of a query in PL/SQL.
    into_clause: $ => seq(
      optional(kws('BULK', 'COLLECT')),
      kw('INTO'),
      commaSep1(choice($.column_reference, $.bind_variable)),
    ),

    // A query of values, as in VALUES (1, 'a'), (2, 'b').
    values: $ => seq(kw('VALUES'), commaSep1($.value_list)),

    // DEFAULT is a value of a row of INSERT.
    value_list: $ => parens(commaSep1(choice($._expression, kw('DEFAULT')))),

    from_clause: $ => seq(kw('FROM'), commaSep1($._table_reference)),

    _table_reference: $ => choice(
      $._table_factor,
      $.join,
    ),

    // The right side of a join is a table factor: a join on the right side
    // must be in parentheses.
    _table_factor: $ => choice(
      $.table_reference,
      $.function_table,
      $.derived_table,
      $.collection_table,
      $.json_table,
      $.xml_table,
      $.parenthesized_join,
    ),

    // The clauses of a table after its name come in any order.
    table_reference: $ => prec.right(seq(
      field('table', $.object_reference),
      repeat($._table_clause),
      optional($._alias),
      repeat($._table_operation),
    )),

    _table_clause: $ => choice(
      $.partition_extension,
      $.sample_clause,
      $.flashback_clause,
    ),

    // NESTED after a table gives the rows of a JSON column, as in t NESTED
    // doc COLUMNS (a, b).
    nested_clause: $ => seq(
      kw('NESTED'),
      optional(kw('PATH')),
      field('column', $.column_reference),
      optional(field('path', $.string)),
      $.json_columns,
    ),

    // PIVOT, UNPIVOT and MATCH_RECOGNIZE can come before the alias of a
    // table or after it, and NESTED comes after it.
    _table_operation: $ => prec.right(seq(
      choice($.pivot_clause, $.unpivot_clause, $.match_recognize_clause, $.nested_clause),
      optional($._alias),
    )),

    // Oracle 23ai accepts AS before the alias of a table.
    _alias: $ => seq(optional(kw('AS')), field('alias', $._name)),

    derived_table: $ => prec.right(seq(
      optional(kw('LATERAL')),
      $.subquery,
      optional(seq($._alias, optional($.column_list))),
      repeat($._table_operation),
    )),

    // A table function, a polymorphic table function or a SQL macro, as in
    // FROM pkg.f(x).
    function_table: $ => prec.right(seq(
      $.function_call,
      optional($._alias),
    )),

    // TABLE(expr), the rows of a collection. (+) after it makes an outer
    // join.
    collection_table: $ => prec.right(seq(
      kw('TABLE'),
      parens($._expression),
      optional($.outer_join),
      optional($._alias),
    )),

    parenthesized_join: $ => prec.right(seq(
      parens(choice($.join, $.parenthesized_join)),
      optional($._alias),
    )),

    partition_extension: $ => seq(
      choice(kw('PARTITION'), kw('SUBPARTITION')),
      choice(
        parens(field('name', $._name)),
        seq(kw('FOR'), parens(commaSep1($._expression))),
      ),
    ),

    sample_clause: $ => prec.right(seq(
      kw('SAMPLE'),
      optional(kw('BLOCK')),
      parens(field('percent', $._expression)),
      optional(seq(kw('SEED'), parens(field('seed', $._expression)))),
    )),

    flashback_clause: $ => prec.right(PREC.condition, choice(
      seq(
        kw('VERSIONS'),
        choice(
          seq(kw('BETWEEN'), choice(kw('SCN'), kw('TIMESTAMP'))),
          seq(kws('PERIOD', 'FOR'), field('period', $._name), kw('BETWEEN')),
        ),
        field('low', $._expression),
        kw('AND'),
        field('high', $._expression),
      ),
      seq(
        kws('AS', 'OF'),
        choice(
          kw('SCN'),
          kw('TIMESTAMP'),
          seq(kws('PERIOD', 'FOR'), field('period', $._name)),
        ),
        field('value', $._expression),
      ),
    )),

    // The words of a join before JOIN, such as LEFT OUTER JOIN, are one
    // token with JOIN, so that they take one column of a parse state, and
    // so that left can also be the alias of a table. It is the keyword JOIN
    // in the tree. CROSS APPLY and OUTER APPLY are the keyword APPLY.
    join: $ => prec.left(seq(
      field('left', $._table_reference),
      optional($.query_partition_clause),
      choice(
        kw('JOIN'),
        alias($._join, 'JOIN'),
        alias($._apply, 'APPLY'),
      ),
      field('right', $._table_factor),
      optional($.query_partition_clause),
      optional(choice(
        seq(kw('ON'), field('condition', $._expression)),
        seq(kw('USING'), parens(commaSep1(field('using', $._name)))),
      )),
    )),

    _join: _ => token(seq(
      choice(
        keyword('INNER'),
        keyword('CROSS'),
        seq(
          choice(keyword('LEFT'), keyword('RIGHT'), keyword('FULL')),
          optional(seq(/\s+/, keyword('OUTER'))),
        ),
        seq(
          keyword('NATURAL'),
          optional(seq(/\s+/, choice(
            keyword('INNER'),
            seq(
              choice(keyword('LEFT'), keyword('RIGHT'), keyword('FULL')),
              optional(seq(/\s+/, keyword('OUTER'))),
            ),
          ))),
        ),
      ),
      /\s+/,
      keyword('JOIN'),
    )),

    _apply: _ => token(seq(
      choice(keyword('CROSS'), keyword('OUTER')),
      /\s+/,
      keyword('APPLY'),
    )),

    // PARTITION BY of a partitioned outer join. The expressions are in
    // parentheses, because a comma after the join can start the next table.
    // PARTITION BY is one token, the keyword PARTITION in the tree, so that
    // the parser can tell it from the partition of a table, as in
    // PARTITION (p), after the name of a table.
    query_partition_clause: $ => seq(
      alias($._partition_by, 'PARTITION'),
      parens(commaSep1($._expression)),
    ),

    _partition_by: _ => token(seq(keyword('PARTITION'), /\s+/, keyword('BY'))),

    // PARTITION BY of MATCH_RECOGNIZE.
    partition_by_clause: $ => seq(
      kws('PARTITION', 'BY'),
      commaSep1($._expression),
    ),

    pivot_clause: $ => seq(
      kw('PIVOT'),
      optional(kw('XML')),
      '(',
      commaSep1($._pivot_item),
      kw('FOR'),
      field('for', choice($._name, $.column_list)),
      kw('IN'),
      parens(choice(
        commaSep1($._pivot_item),
        kw('ANY'),
        $._query,
      )),
      ')',
    ),

    _pivot_item: $ => choice($._expression, $.aliased_expression),

    unpivot_clause: $ => seq(
      kw('UNPIVOT'),
      optional(seq(choice(kw('INCLUDE'), kw('EXCLUDE')), kw('NULLS'))),
      '(',
      field('value', choice($._name, $.column_list)),
      kw('FOR'),
      field('for', choice($._name, $.column_list)),
      kw('IN'),
      parens(commaSep1($.unpivot_item)),
      ')',
    ),

    unpivot_item: $ => seq(
      choice($._name, $.column_list),
      optional(seq(kw('AS'), field('value', $._expression))),
    ),

    // The clauses of MATCH_RECOGNIZE come in the order of the manual, but
    // the grammar takes them in any order.
    match_recognize_clause: $ => seq(
      kw('MATCH_RECOGNIZE'),
      parens(repeat1(choice(
        $.partition_by_clause,
        $.order_by_clause,
        $.measures_clause,
        $._rows_per_match,
        $.skip_clause,
        $.pattern_clause,
        $.subset_clause,
        $.define_clause,
      ))),
    ),

    measures_clause: $ => seq(kw('MEASURES'), commaSep1($.measure)),

    measure: $ => seq(
      optional(choice(kw('RUNNING'), kw('FINAL'))),
      field('value', $._expression),
      optional(kw('AS')),
      field('alias', $._name),
    ),

    _rows_per_match: $ => choice(
      kws('ONE', 'ROW', 'PER', 'MATCH'),
      seq(
        kws('ALL', 'ROWS', 'PER', 'MATCH'),
        optional(choice(
          kws('SHOW', 'EMPTY', 'MATCHES'),
          kws('OMIT', 'EMPTY', 'MATCHES'),
          kws('WITH', 'UNMATCHED', 'ROWS'),
        )),
      ),
    ),

    skip_clause: $ => seq(
      kws('AFTER', 'MATCH', 'SKIP'),
      choice(
        kws('TO', 'NEXT', 'ROW'),
        kws('PAST', 'LAST', 'ROW'),
        seq(kw('TO'), optional(choice(kw('FIRST'), kw('LAST'))), field('variable', $._name)),
      ),
    ),

    pattern_clause: $ => seq(kw('PATTERN'), parens($.row_pattern)),

    // A row pattern is a regular expression of pattern variables.
    row_pattern: $ => repeat1(choice(
      $._name,
      parens(optional($.row_pattern)),
      seq('{-', $.row_pattern, '-}'),
      seq(kw('PERMUTE'), parens(commaSep1($.row_pattern))),
      '|',
      '^',
      '$',
      alias($._pattern_quantifier, $.operator),
    )),

    _pattern_quantifier: _ => token(choice(
      /[*+?]\??/,
      /\{\s*\d*\s*(,\s*\d*\s*)?\}\??/,
    )),

    subset_clause: $ => seq(kw('SUBSET'), commaSep1($.subset)),

    subset: $ => seq(field('name', $._name), '=', $.column_list),

    define_clause: $ => seq(kw('DEFINE'), commaSep1($.pattern_definition)),

    pattern_definition: $ => seq(
      field('variable', $._name),
      kw('AS'),
      field('condition', $._expression),
    ),

    // JSON_TABLE in FROM, with its columns.
    json_table: $ => prec.right(seq(
      kw('JSON_TABLE'),
      '(',
      $._argument_item,
      optional(seq(',', field('path', $.string), repeat($._function_clause))),
      $.json_columns,
      ')',
      optional($._alias),
    )),

    json_columns: $ => seq(kw('COLUMNS'), parens(commaSep1($.json_column))),

    json_column: $ => choice(
      seq(field('name', $._name), kws('FOR', 'ORDINALITY')),
      seq(
        field('name', $._name),
        optional(field('type', $.type)),
        repeat(choice(
          kw('EXISTS'),
          seq(kw('PATH'), field('path', choice($.string, $._name))),
          $._function_clause,
        )),
      ),
      seq(
        kw('NESTED'),
        optional(kw('PATH')),
        field('path', choice($.string, seq($._name, optional(seq('[', '*', ']'))))),
        $.json_columns,
      ),
    ),

    // XMLTABLE in FROM, with its columns.
    xml_table: $ => prec.right(seq(
      kw('XMLTABLE'),
      '(',
      optional(seq($.function_call, ',')),
      field('query', $.string),
      repeat($._function_clause),
      optional(seq(kw('COLUMNS'), commaSep1($.xml_column))),
      ')',
      optional($._alias),
    )),

    xml_column: $ => seq(
      field('name', $._name),
      choice(
        kws('FOR', 'ORDINALITY'),
        seq(
          field('type', $.type),
          repeat(choice(
            seq(kw('PATH'), field('path', $.string)),
            seq(kw('DEFAULT'), field('default', $._expression)),
          )),
        ),
      ),
    ),

    // WHERE CURRENT OF changes the row that a cursor of PL/SQL fetched last.
    where_clause: $ => seq(
      kw('WHERE'),
      choice($._expression, seq(kws('CURRENT', 'OF'), field('cursor', $._name))),
    ),

    start_with_clause: $ => seq(kws('START', 'WITH'), $._expression),

    connect_by_clause: $ => seq(
      kws('CONNECT', 'BY'),
      optional(kw('NOCYCLE')),
      $._expression,
    ),

    group_by_clause: $ => seq(
      kws('GROUP', 'BY'),
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

    // The clauses of a window come in the order of the manual, but the
    // grammar takes them in any order.
    window_specification: $ => parens(seq(
      optional(field('name', $._name)),
      repeat(choice(
        seq(kws('PARTITION', 'BY'), commaSep1($._expression)),
        $.order_by_clause,
        $.frame_clause,
      )),
    )),

    frame_clause: $ => seq(
      choice(kw('ROWS'), kw('RANGE'), kw('GROUPS')),
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

    // The clauses of MODEL come in the order of the manual, but the grammar
    // takes the options in any order.
    model_clause: $ => seq(
      kw('MODEL'),
      repeat(choice($._model_option, $.reference_model)),
      optional(seq(kw('MAIN'), field('name', $._name))),
      $._model_columns,
      repeat($._model_option),
      $.model_rules,
    ),

    _model_option: $ => choice(
      seq(choice(kw('IGNORE'), kw('KEEP')), kw('NAV')),
      seq(kw('UNIQUE'), choice(kw('DIMENSION'), kws('SINGLE', 'REFERENCE'))),
      seq(kw('RETURN'), choice(kw('UPDATED'), kw('ALL')), kw('ROWS')),
    ),

    reference_model: $ => seq(
      kw('REFERENCE'),
      field('name', $._name),
      kw('ON'),
      $.subquery,
      $._model_columns,
    ),

    _model_columns: $ => seq(
      optional(seq(kws('PARTITION', 'BY'), parens(commaSep1($._model_column)))),
      kws('DIMENSION', 'BY'),
      parens(commaSep1($._model_column)),
      kw('MEASURES'),
      parens(commaSep1($._model_column)),
    ),

    _model_column: $ => choice($._expression, $.aliased_expression),

    model_rules: $ => seq(
      optional(seq(
        kw('RULES'),
        repeat(choice(
          kw('UPDATE'),
          seq(kw('UPSERT'), optional(kw('ALL'))),
          seq(choice(kw('AUTOMATIC'), kw('SEQUENTIAL')), kw('ORDER')),
          seq(
            kw('ITERATE'),
            parens($.number),
            optional(seq(kw('UNTIL'), parens($._expression))),
          ),
        )),
      )),
      parens(optional(commaSep1($.model_rule))),
    ),

    model_rule: $ => seq(
      optional(choice(kw('UPDATE'), seq(kw('UPSERT'), optional(kw('ALL'))))),
      field('cell', $.cell_reference),
      optional($.order_by_clause),
      '=',
      field('value', $._expression),
    ),

    // A cell of a model, as in s['x', 2000] or s[ANY, CV() - 1], or an
    // aggregate of cells, as in MAX(s)['x', year < 2000]. On the left side
    // of a rule, a dimension can be a FOR loop.
    cell_reference: $ => seq(
      field('measure', choice($._name, $.function_call)),
      '[',
      commaSep1(choice($._expression, kw('ANY'), $.model_for_loop)),
      ']',
    ),

    model_for_loop: $ => seq(
      kw('FOR'),
      choice($._name, $.column_list),
      choice(
        seq(kw('IN'), parens(choice(commaSep1($._expression), $._query))),
        seq(
          optional(seq(kw('LIKE'), $.string)),
          kw('FROM'),
          $._expression,
          kw('TO'),
          $._expression,
          choice(kw('INCREMENT'), kw('DECREMENT')),
          $._expression,
        ),
      ),
    ),

    // A comma after ORDER BY in the arguments of a function, as in
    // JSON_ARRAYAGG(x ORDER BY a, b), continues the list of ORDER BY.
    order_by_clause: $ => prec.right(seq(
      kw('ORDER'),
      optional(kw('SIBLINGS')),
      kw('BY'),
      commaSep1($.order_by_item),
    )),

    order_by_item: $ => seq(
      field('value', $._expression),
      optional(choice(kw('ASC'), kw('DESC'))),
      optional(seq(kw('NULLS'), choice(kw('FIRST'), kw('LAST')))),
    ),

    offset_clause: $ => seq(
      kw('OFFSET'),
      field('start', $._expression),
      choice(kw('ROW'), kw('ROWS')),
    ),

    fetch_clause: $ => seq(
      kw('FETCH'),
      choice(kw('FIRST'), kw('NEXT')),
      optional(seq(field('count', $._expression), optional(kw('PERCENT')))),
      choice(kw('ROW'), kw('ROWS')),
      choice(kw('ONLY'), kws('WITH', 'TIES')),
    ),

    for_update_clause: $ => prec.right(seq(
      kws('FOR', 'UPDATE'),
      optional(seq(kw('OF'), commaSep1($.column_reference))),
      optional(choice($.wait_clause, kws('SKIP', 'LOCKED'))),
    )),

    subquery_restriction: $ => seq(
      kw('WITH'),
      choice(kws('READ', 'ONLY'), kws('CHECK', 'OPTION')),
      optional(seq(kw('CONSTRAINT'), field('constraint', $._name))),
    ),

    subquery: $ => parens($._query),

    // Changes of data

    // The clauses after the first part of a statement that changes data
    // come in any order, as the clauses of a query do.
    insert_statement: $ => seq(
      kw('INSERT'),
      choice(
        seq($.insert_into_clause, $._query, repeat($._dml_clause)),
        seq(kw('ALL'), repeat1($._insert_target), $._query),
        seq(
          optional(choice(kw('ALL'), kw('FIRST'))),
          repeat1($.insert_when_clause),
          optional(seq(kw('ELSE'), repeat1($._insert_target))),
          $._query,
        ),
      ),
    ),

    // The table of an INSERT, with its alias and its columns.
    insert_into_clause: $ => seq(
      kw('INTO'),
      choice(
        seq(field('table', $.object_reference), optional($.partition_extension)),
        $.subquery,
        seq(kw('TABLE'), parens(choice($._expression, $._query))),
      ),
      optional($._alias),
      optional($.column_list),
    ),

    // A table of a multitable INSERT. VALUES after it gives the values of
    // the row, and VALUES after the last one starts the query.
    _insert_target: $ => prec.right(seq(
      $.insert_into_clause,
      optional($.values),
      optional($.error_logging_clause),
    )),

    insert_when_clause: $ => seq(
      kw('WHEN'),
      field('condition', $._expression),
      kw('THEN'),
      repeat1($._insert_target),
    ),

    update_statement: $ => seq(
      kw('UPDATE'),
      $._dml_table,
      kw('SET'),
      commaSep1($.assignment),
      repeat(choice($.from_clause, $.where_clause, $._dml_clause)),
    ),

    // SET a = 1, SET (a, b) = (SELECT ...) or SET VALUE (t) = expr.
    assignment: $ => choice(
      seq(
        field('left', choice($.column_reference, $.column_list)),
        '=',
        field('right', choice($._expression, kw('DEFAULT'))),
      ),
      seq(
        kw('VALUE'),
        parens(field('left', $._name)),
        '=',
        field('right', $._expression),
      ),
    ),

    delete_statement: $ => seq(
      kw('DELETE'),
      optional(kw('FROM')),
      $._dml_table,
      repeat(choice($.from_clause, $.where_clause, $._dml_clause)),
    ),

    merge_statement: $ => seq(
      kws('MERGE', 'INTO'),
      $._dml_table,
      kw('USING'),
      field('source', $._dml_table),
      kw('ON'),
      field('condition', $.parenthesized_expression),
      repeat(choice(
        $.merge_update_clause,
        $.merge_insert_clause,
        $.error_logging_clause,
      )),
    ),

    merge_update_clause: $ => seq(
      kws('WHEN', 'MATCHED', 'THEN', 'UPDATE', 'SET'),
      commaSep1($.assignment),
      optional($.where_clause),
      optional(seq(kw('DELETE'), $.where_clause)),
    ),

    merge_insert_clause: $ => seq(
      kws('WHEN', 'NOT', 'MATCHED', 'THEN', 'INSERT'),
      optional(parens(commaSep1($.column_reference))),
      $.values,
      optional($.where_clause),
    ),

    // The table of UPDATE, DELETE and MERGE, with its alias. It takes fewer
    // clauses than a table in FROM, so that it has fewer parse states.
    _dml_table: $ => seq(
      choice(
        seq(field('table', $.object_reference), optional($.partition_extension)),
        $.subquery,
        seq(kw('TABLE'), parens(choice($._expression, $._query)), optional($.outer_join)),
        seq(kw('ONLY'), parens(field('table', $.object_reference))),
      ),
      optional($._alias),
    ),

    _dml_clause: $ => choice(
      $.returning_clause,
      $.error_logging_clause,
    ),

    // RETURNING gives the values of the rows that the statement changed. OLD
    // and NEW of 23ai give the value before or after an UPDATE.
    returning_clause: $ => seq(
      choice(kw('RETURNING'), kw('RETURN')),
      commaSep1(seq(optional(choice(kw('OLD'), kw('NEW'))), $._select_item)),
      $.into_clause,
    ),

    // ( after LOG ERRORS starts its tag, not the query of INSERT.
    error_logging_clause: $ => prec.right(seq(
      kws('LOG', 'ERRORS'),
      optional(seq(kw('INTO'), field('table', $.object_reference))),
      optional(parens(field('tag', $._expression))),
      optional(seq(kws('REJECT', 'LIMIT'), choice($.number, kw('UNLIMITED')))),
    )),

    wait_clause: $ => choice(
      kw('NOWAIT'),
      seq(
        kw('WAIT'),
        choice(
          seq($.number, optional(choice(kw('SECONDS'), kw('MILLISECONDS'), kw('MICROSECONDS')))),
          kw('FOREVER'),
        ),
      ),
    ),

    // Transactions

    commit_statement: $ => seq(
      kw('COMMIT'),
      optional(kw('WORK')),
      repeat(choice(
        seq(kw('COMMENT'), $.string),
        kw('WRITE'),
        kw('WAIT'),
        kw('NOWAIT'),
        kw('IMMEDIATE'),
        kw('BATCH'),
        seq(kw('FORCE'), choice(
          seq($.string, optional(seq(',', $.number))),
          seq(kw('CORRUPT_XID'), $.string),
          kw('CORRUPT_XID_ALL'),
        )),
      )),
    ),

    rollback_statement: $ => seq(
      kw('ROLLBACK'),
      optional(kw('WORK')),
      optional(choice(
        seq(kw('TO'), optional(kw('SAVEPOINT')), field('savepoint', $._name)),
        seq(kw('FORCE'), $.string),
      )),
    ),

    savepoint_statement: $ => seq(kw('SAVEPOINT'), field('name', $._name)),

    set_transaction_statement: $ => seq(
      kws('SET', 'TRANSACTION'),
      repeat1(choice(
        seq(kw('READ'), choice(kw('ONLY'), kw('WRITE'))),
        seq(kws('ISOLATION', 'LEVEL'), choice(kw('SERIALIZABLE'), kws('READ', 'COMMITTED'))),
        seq(kws('USE', 'ROLLBACK', 'SEGMENT'), field('segment', $._name)),
        seq(kw('NAME'), $.string),
      )),
    ),

    set_constraints_statement: $ => seq(
      kw('SET'),
      choice(kw('CONSTRAINT'), kw('CONSTRAINTS')),
      choice(kw('ALL'), commaSep1($.object_reference)),
      choice(kw('IMMEDIATE'), kw('DEFERRED')),
    ),

    lock_table_statement: $ => seq(
      kws('LOCK', 'TABLE'),
      commaSep1(seq($.object_reference, optional($.partition_extension))),
      kw('IN'),
      repeat1(choice(kw('ROW'), kw('SHARE'), kw('EXCLUSIVE'), kw('UPDATE'))),
      kw('MODE'),
      optional($.wait_clause),
    ),

    // Sessions and the system

    set_role_statement: $ => seq(
      kws('SET', 'ROLE'),
      choice(
        commaSep1(seq(
          field('role', $._name),
          optional(seq(kws('IDENTIFIED', 'BY'), field('password', $._name))),
        )),
        seq(kw('ALL'), optional(seq(kw('EXCEPT'), commaSep1(field('role', $._name))))),
        kw('NONE'),
      ),
    ),

    // ALTER SESSION SET takes parameters. The other forms of ALTER SESSION
    // and of ALTER SYSTEM are a list of words.
    alter_session_statement: $ => seq(
      kws('ALTER', 'SESSION'),
      choice(
        seq(kw('SET'), repeat1($.parameter)),
        $._options,
      ),
    ),

    alter_system_statement: $ => seq(
      kws('ALTER', 'SYSTEM'),
      choice(
        seq(choice(kw('SET'), kw('RESET')), repeat1($.parameter)),
        $._options,
      ),
    ),

    // A parameter of the session or of the system, as in NLS_DATE_FORMAT =
    // 'YYYY-MM-DD', SCOPE = SPFILE or EVENTS '10046 trace name context
    // forever'. A parameter of the system can take a list of values.
    parameter: $ => seq(
      field('name', choice($._name, kw('COMMENT'), kws('ROW', 'ARCHIVAL', 'VISIBILITY'))),
      optional(choice(
        seq('=', commaSep1(field('value', choice($._expression, kw('ALL'), kw('CURRENT'))))),
        field('value', $.string),
      )),
    ),

    // A list of words, strings and numbers, for the forms of a statement
    // that the grammar does not read word by word, as in ALTER SYSTEM KILL
    // SESSION '12,345' IMMEDIATE. Each word is a keyword in the tree.
    _options: $ => seq(
      alias($.identifier, $.keyword),
      repeat($._option),
    ),

    _option: $ => choice(
      alias($.identifier, $.keyword),
      $.string,
      $.number,
      '=',
      ',',
      ...['ALL', 'BY', 'DELETE', 'FOR', 'IDENTIFIED', 'IMMEDIATE', 'IN', 'INDEX', 'INTO',
        'NULL', 'OFFLINE', 'ON', 'ONLINE', 'ROWS', 'SESSION', 'SET', 'TABLE', 'TO',
        'UPDATE', 'VALIDATE', 'WITH'].map(kw),
    ),

    // EXPLAIN PLAN FOR a statement.
    explain_plan_statement: $ => seq(
      kws('EXPLAIN', 'PLAN'),
      repeat(choice(
        seq(kws('SET', 'STATEMENT_ID'), '=', field('id', $.string)),
        seq(kw('INTO'), field('table', $.object_reference)),
      )),
      kw('FOR'),
      field('statement', choice(
        $.select_statement,
        $.insert_statement,
        $.update_statement,
        $.delete_statement,
        $.merge_statement,
      )),
    ),

    call_statement: $ => seq(
      kw('CALL'),
      choice($.function_call, $.field_expression),
      optional(seq(
        kw('INTO'),
        $.bind_variable,
        optional(seq(optional(kw('INDICATOR')), $.bind_variable)),
      )),
    ),

    // Objects

    truncate_statement: $ => seq(
      kw('TRUNCATE'),
      choice(kw('TABLE'), kw('CLUSTER')),
      field('name', $.object_reference),
      repeat(choice(
        seq(choice(kw('PRESERVE'), kw('PURGE')), kws('MATERIALIZED', 'VIEW', 'LOG')),
        seq(kw('DROP'), optional(kw('ALL')), kw('STORAGE')),
        kws('REUSE', 'STORAGE'),
        kw('CASCADE'),
      )),
    ),

    purge_statement: $ => seq(
      kw('PURGE'),
      choice(
        seq(choice(kw('TABLE'), kw('INDEX')), field('name', $.object_reference)),
        seq(
          kw('TABLESPACE'),
          optional(kw('SET')),
          field('name', $._name),
          optional(seq(kw('USER'), field('user', $._name))),
        ),
        kw('RECYCLEBIN'),
        kw('DBA_RECYCLEBIN'),
      ),
    ),

    flashback_table_statement: $ => seq(
      kws('FLASHBACK', 'TABLE'),
      commaSep1($.object_reference),
      kw('TO'),
      choice(
        seq(choice(kw('SCN'), kw('TIMESTAMP')), field('value', $._expression)),
        seq(kws('RESTORE', 'POINT'), field('restore_point', $._name)),
        seq(kws('BEFORE', 'DROP'), optional(seq(kws('RENAME', 'TO'), field('new_name', $._name)))),
      ),
      optional(seq(choice(kw('ENABLE'), kw('DISABLE')), kw('TRIGGERS'))),
    ),

    flashback_database_statement: $ => seq(
      kw('FLASHBACK'),
      optional(kw('STANDBY')),
      optional(kw('PLUGGABLE')),
      kw('DATABASE'),
      optional(field('name', $._name)),
      kw('TO'),
      choice(
        seq(
          optional(kw('BEFORE')),
          choice(kw('SCN'), kw('TIMESTAMP')),
          field('value', $._expression),
        ),
        seq(
          optional(kw('BEFORE')),
          kws('RESTORE', 'POINT'),
          field('restore_point', $._name),
        ),
        kws('BEFORE', 'RESETLOGS'),
      ),
    ),

    // The clauses after the table of ANALYZE, such as COMPUTE STATISTICS or
    // VALIDATE STRUCTURE CASCADE, are a list of words.
    analyze_statement: $ => seq(
      kw('ANALYZE'),
      choice(kw('TABLE'), kw('INDEX'), kw('CLUSTER')),
      field('name', $.object_reference),
      optional($.partition_extension),
      repeat1($._option),
    ),

    // Each statement option or privilege of AUDIT is a privilege, as in
    // AUDIT SELECT TABLE, UPDATE TABLE BY hr. The name of a unified audit
    // policy is a privilege too, as in AUDIT POLICY p EXCEPT hr.
    audit_statement: $ => seq(kw('AUDIT'), $._audit_body),

    noaudit_statement: $ => seq(kw('NOAUDIT'), $._audit_body),

    _audit_body: $ => seq(
      commaSep1($.privilege),
      optional(seq(kw('ON'), choice(kw('DEFAULT'), $._grant_object))),
      repeat(choice(
        seq(kw('BY'), choice(kw('SESSION'), kw('ACCESS'), commaSep1(field('user', $._name)))),
        seq(kw('EXCEPT'), commaSep1(field('user', $._name))),
        seq(kw('WHENEVER'), optional(kw('NOT')), kw('SUCCESSFUL')),
        $._container,
      )),
    ),

    comment_statement: $ => seq(
      kws('COMMENT', 'ON'),
      choice(
        seq(kw('COLUMN'), field('column', $.column_reference)),
        seq(
          choice(
            kw('TABLE'),
            kws('MATERIALIZED', 'VIEW'),
            kws('AUDIT', 'POLICY'),
            kw('EDITION'),
            kw('INDEXTYPE'),
            kws('MINING', 'MODEL'),
            kw('OPERATOR'),
            kws('PROPERTY', 'GRAPH'),
          ),
          field('name', $.object_reference),
        ),
      ),
      kw('IS'),
      field('comment', $.string),
    ),

    // Privileges

    // A system privilege, an object privilege or a role names one or more
    // words, as in CREATE ANY TABLE. An object privilege can take columns.
    grant_statement: $ => seq(
      kw('GRANT'),
      commaSep1($._privilege_item),
      optional(seq(kw('ON'), $._grant_object)),
      kw('TO'),
      commaSep1($._grantee),
      repeat(choice(
        seq(kws('IDENTIFIED', 'BY'), field('password', $._name)),
        seq(
          kw('WITH'),
          choice(kw('ADMIN'), kw('DELEGATE'), kw('GRANT'), kw('HIERARCHY')),
          kw('OPTION'),
        ),
        $._container,
      )),
    ),

    revoke_statement: $ => seq(
      kw('REVOKE'),
      commaSep1($._privilege_item),
      optional(seq(kw('ON'), $._grant_object)),
      kw('FROM'),
      commaSep1($._grantee),
      repeat(choice(
        kws('CASCADE', 'CONSTRAINTS'),
        kw('FORCE'),
        $._container,
      )),
    ),

    _privilege_item: $ => seq($.privilege, optional($.column_list)),

    privilege: $ => prec.right(repeat1(choice(
      $._name,
      ...['ACCESS', 'ALL', 'ALTER', 'ANY', 'AUDIT', 'CLUSTER', 'COMMENT', 'CONNECT',
        'CREATE', 'DELETE', 'DROP', 'GRANT', 'INDEX', 'INSERT', 'LOCK', 'PUBLIC',
        'RENAME', 'RESOURCE', 'SELECT', 'SESSION', 'SET', 'SYNONYM', 'TABLE',
        'TRIGGER', 'UPDATE', 'USER', 'VIEW'].map(kw),
    ))),

    // The object of a privilege, such as hr.employees, DIRECTORY d or
    // SCHEMA hr.
    _grant_object: $ => seq(
      optional(choice(
        kw('USER'),
        kw('DIRECTORY'),
        kw('EDITION'),
        kw('SCHEMA'),
        kws('MINING', 'MODEL'),
        seq(kw('JAVA'), choice(kw('SOURCE'), kw('RESOURCE'))),
        kws('SQL', 'TRANSLATION', 'PROFILE'),
      )),
      field('object', $.object_reference),
    ),

    // A user, a role or PUBLIC, or a program unit that gets a role.
    _grantee: $ => choice(
      field('grantee', $._name),
      seq(
        choice(kw('PROCEDURE'), kw('FUNCTION'), kw('PACKAGE')),
        field('grantee', $.object_reference),
      ),
      field('grantee', kw('PUBLIC')),
    ),

    _container: $ => seq(kw('CONTAINER'), '=', choice(kw('CURRENT'), kw('ALL'))),

    rename_statement: $ => seq(
      kw('RENAME'),
      field('name', $._name),
      kw('TO'),
      field('new_name', $._name),
    ),

    // Expressions

    _expression: $ => choice(
      $._primary_expression,
      $.unary_expression,
      $.binary_expression,
      $.is_expression,
      $.between_expression,
      $.in_expression,
      $.like_expression,
      $.member_expression,
      $.at_time_zone_expression,
      $.collate_expression,
    ),

    _primary_expression: $ => choice(
      $._literal,
      $.typed_literal,
      $.column_reference,
      $.pseudocolumn,
      $.bind_variable,
      $.function_call,
      $.field_expression,
      $.cell_reference,
      $.parenthesized_expression,
      $.interval_expression,
      $.list,
      $.subquery,
      $.exists_expression,
      $.case_expression,
      $.cursor_expression,
      $.multiset_expression,
    ),

    _literal: $ => choice(
      $.number,
      $.string,
      $.boolean,
      $.null,
    ),

    boolean: _ => choice(kw('TRUE'), kw('FALSE')),

    null: _ => kw('NULL'),

    // A type name before a string gives a constant of the type, such as
    // DATE '2026-10-31' or INTERVAL '1' DAY. Oracle reserves DATE, and the
    // other names, such as TIMESTAMP and INTERVAL, are identifiers, so
    // they also name a column. An interval qualifier, such as DAY TO
    // SECOND, can come after the string.
    typed_literal: $ => prec.right(seq(
      field('type', choice(kw('DATE'), alias($.identifier, $.type))),
      field('value', $.string),
      optional($.interval_qualifier),
    )),

    interval_qualifier: $ => prec.right(seq(
      $._interval_field,
      optional($.type_modifiers),
      optional(seq(kw('TO'), $._interval_field, optional($.type_modifiers))),
    )),

    _interval_field: _ => choice(
      kw('YEAR'),
      kw('MONTH'),
      kw('DAY'),
      kw('HOUR'),
      kw('MINUTE'),
      kw('SECOND'),
    ),

    // An interval expression, as in (a - b) DAY TO SECOND.
    interval_expression: $ => prec.right(seq(
      field('value', $.parenthesized_expression),
      choice(
        seq(kw('DAY'), optional($.type_modifiers), kw('TO'), kw('SECOND'), optional($.type_modifiers)),
        seq(kw('YEAR'), optional($.type_modifiers), kw('TO'), kw('MONTH')),
      ),
    )),

    // A column, with its table and its schema. (+) after it makes an outer
    // join of the old form of Oracle.
    column_reference: $ => prec.right(seq(
      choice(
        field('name', $._name),
        seq(field('table', choice($._name, $.bind_variable)), '.', field('name', $._label)),
        seq(
          field('schema', $._name),
          '.',
          field('table', $._label),
          '.',
          field('name', $._label),
        ),
      ),
      optional($.outer_join),
    )),

    outer_join: _ => token(seq('(', /\s*/, '+', /\s*/, ')')),

    // The reserved words that are pseudocolumns, such as ROWNUM and
    // SYSDATE.
    pseudocolumn: _ => choice(...PSEUDOCOLUMNS.map(kw)),

    // A bind variable, as in :name or :1.
    bind_variable: _ => /:[A-Za-z0-9_$#\u0080-\u{10FFFF}]+/u,

    // A function call, with its package and its schema. The analytic
    // clauses after the arguments come in any order.
    function_call: $ => prec.right(seq(
      choice(
        field('name', $._name),
        seq(field('package', $._name), '.', field('name', $._label)),
        seq(
          field('schema', $._name),
          '.',
          field('package', $._label),
          '.',
          field('name', $._label),
        ),
      ),
      $.arguments,
      repeat(choice(
        $.within_group_clause,
        $.keep_clause,
        $.null_treatment,
        $.over_clause,
      )),
    )),

    arguments: $ => parens(optional(seq(
      optional(choice(kw('DISTINCT'), kw('ALL'), kw('UNIQUE'))),
      $._argument_list,
    ))),

    // The arguments of a function. Most functions take a list of
    // expressions with commas between them. Some functions have keywords
    // between their arguments, such as EXTRACT(YEAR FROM d), TRIM(BOTH 'x'
    // FROM s) and JSON_OBJECT('a' VALUE 1). The grammar accepts each of
    // these keywords between two arguments of any function, so that each
    // argument is in the same parse states. The clauses of the JSON
    // functions, such as RETURNING and NULL ON ERROR, can come after each
    // argument.
    _argument_list: $ => seq(
      optional(choice(
        kw('FROM'),
        seq(choice(kw('BOTH'), kw('LEADING'), kw('TRAILING')), optional(kw('FROM'))),
      )),
      $._argument_item,
      repeat(seq($._argument_separator, $._argument_item)),
    ),

    _argument_separator: _ => choice(
      ',',
      ':',
      kw('FROM'),
      kw('VALUE'),
      kw('USING'),
    ),

    _argument_item: $ => seq($._argument, repeat($._function_clause)),

    _argument: $ => choice(
      $._expression,
      $.named_argument,
      // CAST(x AS type), TREAT(x AS type) and the alias of an argument of
      // XMLELEMENT.
      seq($._expression, kw('AS'), $.type),
      // NAME x and EVALNAME x of XMLELEMENT. The first word is not a
      // keyword, so that name can also name a column.
      seq(alias($.identifier, $.keyword), $._name),
      // COUNT(*), and all the columns of the functions of data mining, as
      // in PREDICTION(m USING *).
      alias('*', $.all_columns),
    ),

    named_argument: $ => seq(
      field('name', $._name),
      '=>',
      field('value', $._expression),
    ),

    // The clauses of the JSON functions and of some others, after an
    // argument.
    _function_clause: $ => choice(
      seq(kw('RETURNING'), $.type),
      kws('FORMAT', 'JSON'),
      seq(choice(kw('NULL'), kw('ABSENT')), kws('ON', 'NULL')),
      seq(
        choice(
          kw('ERROR'),
          kw('NULL'),
          seq(kw('EMPTY'), optional(choice(kw('ARRAY'), kw('OBJECT')))),
          seq(kw('DEFAULT'), $._expression),
          kw('TRUE'),
          kw('FALSE'),
        ),
        kw('ON'),
        choice(kw('ERROR'), kw('EMPTY'), kw('MISMATCH'), kws('CONVERSION', 'ERROR')),
      ),
      seq(
        choice(kw('WITH'), kw('WITHOUT')),
        choice(
          seq(
            optional(choice(kw('CONDITIONAL'), kw('UNCONDITIONAL'))),
            optional(kw('ARRAY')),
            kw('WRAPPER'),
          ),
          kws('UNIQUE', 'KEYS'),
        ),
      ),
      kw('STRICT'),
      kw('LAX'),
      kw('PRETTY'),
      kw('ASCII'),
      kw('TRUNCATE'),
      seq(choice(kw('ALLOW'), kw('DISALLOW')), kw('SCALARS')),
      prec.right(seq(
        kws('ON', 'OVERFLOW'),
        choice(
          kw('ERROR'),
          seq(
            kw('TRUNCATE'),
            optional($.string),
            optional(seq(choice(kw('WITH'), kw('WITHOUT')), kw('COUNT'))),
          ),
        ),
      )),
      $.order_by_clause,
      // PASSING takes one value here. A comma after it starts the next
      // argument, which can be a value with AS and its name.
      seq(
        kw('PASSING'),
        optional(seq(kw('BY'), kw('VALUE'))),
        $._expression,
        optional(seq(kw('AS'), $._name)),
      ),
      seq(kws('RETURNING', 'SEQUENCE', 'BY', 'REF')),
      // COST MODEL of the functions of data mining.
      seq(kws('COST', 'MODEL'), optional(kw('AUTO'))),
      $.null_treatment,
    ),

    within_group_clause: $ => seq(
      kws('WITHIN', 'GROUP'),
      parens($.order_by_clause),
    ),

    keep_clause: $ => seq(
      kw('KEEP'),
      parens(seq(
        kw('DENSE_RANK'),
        choice(kw('FIRST'), kw('LAST')),
        $.order_by_clause,
      )),
    ),

    null_treatment: _ => seq(choice(kw('IGNORE'), kw('RESPECT')), kw('NULLS')),

    over_clause: $ => seq(
      kw('OVER'),
      choice(field('window', $._name), $.window_specification),
    ),

    // A field of an object, an element of a JSON document in the dot
    // notation, or a method, as in (o).name, t.doc.a.b or f(x).name.
    field_expression: $ => prec(PREC.field, seq(
      field('value', choice(
        $.parenthesized_expression,
        $.function_call,
        $.column_reference,
        $.field_expression,
      )),
      '.',
      field('field', choice($._label, $.function_call)),
    )),

    parenthesized_expression: $ => parens($._expression),

    // A list of expressions, as in (a, b) IN ((1, 2), (3, 4)).
    list: $ => parens(seq($._expression, ',', commaSep1($._expression))),

    exists_expression: $ => seq(kw('EXISTS'), $.subquery),

    // CURSOR(query), a cursor expression.
    cursor_expression: $ => seq(kw('CURSOR'), $.subquery),

    // MULTISET(query), the argument of CAST that gives a collection.
    multiset_expression: $ => seq(kw('MULTISET'), $.subquery),

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
      prec.right(PREC.unary, seq(field('operator', alias($._additive_sign, $.operator)), field('operand', $._expression))),
      prec.right(PREC.unary, seq(
        field('operator', choice(kw('PRIOR'), kw('CONNECT_BY_ROOT'))),
        field('operand', $._expression),
      )),
    ),

    _additive_sign: _ => token(choice('-', '+')),

    binary_expression: $ => {
      /** @type {[number, RuleOrLiteral, RuleOrLiteral][]} */
      const table = [
        [PREC.or, kw('OR'), $._expression],
        [PREC.and, kw('AND'), $._expression],
        [PREC.exponent, alias('**', $.operator), $._expression],
        [PREC.multiply, alias($._multiplicative_operator, $.operator), $._expression],
        [PREC.add, alias($._additive_operator, $.operator), $._expression],
        [PREC.add, $.multiset_operator, $._expression],
        [PREC.compare, alias($._comparison_operator, $.operator), $._operand],
      ];
      return choice(...table.map(([p, op, right]) => prec.left(p, seq(
        field('left', $._expression),
        field('operator', op),
        field('right', right),
      ))));
    },

    _multiplicative_operator: _ => token(choice('*', '/')),

    // || concatenates two strings, and it has the precedence of + and -.
    _additive_operator: _ => token(choice('+', '-', '||')),

    _comparison_operator: _ => token(choice(
      '=', '<', '>', '<=', '>=', '<>', '!=', '^=', '~=',
    )),

    // MULTISET EXCEPT, MULTISET INTERSECT and MULTISET UNION combine two
    // nested tables.
    multiset_operator: _ => seq(
      kw('MULTISET'),
      choice(kw('EXCEPT'), kw('INTERSECT'), kw('UNION')),
      optional(choice(kw('ALL'), kw('DISTINCT'))),
    ),

    _operand: $ => choice($._expression, $.quantified_expression),

    // ANY, SOME or ALL after a comparison operator compares the left side
    // with each value of a list or each row of a subquery.
    quantified_expression: $ => seq(
      field('quantifier', choice(kw('ANY'), kw('SOME'), kw('ALL'))),
      parens(choice($._query, commaSep1($._expression))),
    ),

    // Each condition takes its operator from one hidden rule, so that it
    // has one production, and NOT before the operator gives no other.
    is_expression: $ => prec.left(PREC.condition, seq(
      field('left', $._expression),
      $._is_operator,
      field('right', $._is_right),
    )),

    _is_operator: _ => seq(kw('IS'), optional(kw('NOT'))),

    _is_right: $ => choice(
      $.boolean,
      $.null,
      kw('NAN'),
      kw('INFINITE'),
      kw('EMPTY'),
      kw('PRESENT'),
      kw('DANGLING'),
      kws('A', 'SET'),
      seq(kw('OF'), optional(kw('TYPE')), parens(commaSep1(seq(optional(kw('ONLY')), $.type)))),
      $.json_condition,
    ),

    json_condition: $ => prec.right(seq(
      kw('JSON'),
      repeat(choice(
        kws('FORMAT', 'JSON'),
        kw('STRICT'),
        kw('LAX'),
        kw('VALUE'),
        kw('ARRAY'),
        kw('OBJECT'),
        kw('SCALAR'),
        seq(choice(kw('WITH'), kw('WITHOUT')), kws('UNIQUE', 'KEYS')),
        seq(choice(kw('ALLOW'), kw('DISALLOW')), kw('SCALARS')),
      )),
    )),

    between_expression: $ => prec.left(PREC.condition, seq(
      field('left', $._expression),
      $._between_operator,
      field('low', $._expression),
      kw('AND'),
      field('high', $._expression),
    )),

    _between_operator: _ => seq(optional(kw('NOT')), kw('BETWEEN')),

    in_expression: $ => prec.left(PREC.condition, seq(
      field('left', $._expression),
      $._in_operator,
      field('right', $._in_right),
    )),

    _in_operator: _ => seq(optional(kw('NOT')), kw('IN')),

    _in_right: $ => choice($.subquery, $.parenthesized_expression, $.list),

    like_expression: $ => prec.left(PREC.condition, seq(
      field('left', $._expression),
      field('operator', $._like_operator),
      field('right', $._expression),
      optional($._escape),
    )),

    _like_operator: _ => seq(
      optional(kw('NOT')),
      choice(kw('LIKE'), kw('LIKEC'), kw('LIKE2'), kw('LIKE4')),
    ),

    _escape: $ => seq(kw('ESCAPE'), field('escape', $._expression)),

    // MEMBER OF and SUBMULTISET OF test a nested table.
    member_expression: $ => prec.left(PREC.condition, seq(
      field('left', $._expression),
      field('operator', $._member_operator),
      field('right', $._expression),
    )),

    _member_operator: _ => seq(
      optional(kw('NOT')),
      choice(kw('MEMBER'), kw('SUBMULTISET')),
      optional(kw('OF')),
    ),

    at_time_zone_expression: $ => prec.left(PREC.at, seq(
      field('value', $._expression),
      kw('AT'),
      $._time_zone,
    )),

    _time_zone: $ => choice(
      seq(kws('TIME', 'ZONE'), field('zone', $._expression)),
      kw('LOCAL'),
    ),

    collate_expression: $ => prec.left(PREC.collate, seq(
      field('value', $._expression),
      kw('COLLATE'),
      field('collation', $._name),
    )),

    // Types

    type: $ => prec.right(choice(
      seq(
        optional(seq(field('schema', $._name), '.')),
        field('name', $._name),
        optional($.type_modifiers),
        optional(seq('%', choice(kw('TYPE'), kw('ROWTYPE')))),
      ),
      seq(choice(...TYPE_WORDS.map(kw)), optional($.type_modifiers)),
      seq(kw('LONG'), optional(kw('RAW'))),
      kws('DOUBLE', 'PRECISION'),
      seq(
        choice(
          kw('CHARACTER'),
          seq(kw('NATIONAL'), choice(kw('CHARACTER'), kw('CHAR'))),
          kw('NCHAR'),
        ),
        optional(kw('VARYING')),
        optional($.type_modifiers),
      ),
      seq(
        kw('TIMESTAMP'),
        optional($.type_modifiers),
        optional(seq(kw('WITH'), optional(kw('LOCAL')), kws('TIME', 'ZONE'))),
      ),
      seq(kw('INTERVAL'), $.interval_qualifier),
    )),

    // The precision and the scale of a type, as in NUMBER(10, 2), NUMBER(*,
    // 2) or VARCHAR2(20 CHAR).
    type_modifiers: $ => parens(seq(
      commaSep1(choice($._signed_number, '*', $._name)),
      optional(choice(kw('BYTE'), kw('CHAR'))),
    )),

    _signed_number: $ => seq(optional(alias($._additive_sign, $.operator)), $.number),

    // Names and literals

    // A name of an object, with its schema, and a database link after @.
    object_reference: $ => seq(
      choice(
        field('name', $._name),
        seq(field('schema', $._name), '.', field('name', $._label)),
      ),
      optional(seq('@', field('link', $.database_link))),
    ),

    database_link: $ => seq($._name, repeat(seq('.', $._label))),

    // A name is an identifier or a quoted identifier, and both are the node
    // identifier.
    _name: $ => choice(
      $.identifier,
      alias($._quoted_identifier, $.identifier),
    ),

    // A name after a dot can be any keyword.
    _label: $ => choice(
      reserved('label', $.identifier),
      alias($._quoted_identifier, $.identifier),
    ),

    // An identifier of Oracle starts with a letter. The grammar also takes
    // an identifier that starts with _, which is the placeholder of a
    // variable of usql (D101).
    identifier: _ => /[A-Za-z_\u0080-\u{10FFFF}][A-Za-z0-9_$#\u0080-\u{10FFFF}]*/u,

    // A quoted identifier is in double quotes, and it holds no double
    // quote.
    _quoted_identifier: _ => /"[^"]*"/,

    // A number is an integer, a decimal number or a number with an
    // exponent. F or D after it makes a BINARY_FLOAT or a BINARY_DOUBLE. A
    // dot must have a digit after it, so that 1..10 is a range of PL/SQL.
    number: _ => token(seq(
      choice(/\d+(\.\d+)?/, /\.\d+/),
      optional(/[eE][+-]?\d+/),
      optional(/[fFdD]/),
    )),

    // A string is in single quotes, with a quote doubled. N before it gives
    // a national character string. The external scanner reads a string
    // with the alternative quoting, such as q'[it's]' and nq'{x}'.
    string: $ => choice(
      /[nN]?'([^']|'')*'/,
      $._q_string,
    ),
  },
});
