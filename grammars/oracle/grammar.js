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
// The statements that define objects, such as CREATE TABLE, read the names
// of the objects, the columns, the constraints and the partitions. The
// physical options, such as TABLESPACE, STORAGE and LOB, are a list of
// words, because the manual lists hundreds of them. A word of such a list
// is an identifier that takes the reserved set label, so that one token
// covers each word of the list, and a reserved word such as INDEX is a
// word too. The rule _create holds the words after CREATE, such as OR
// REPLACE and UNIQUE, in any order, so that the statements share the
// parse states after CREATE.
//
// The grammar reads PL/SQL with the same rules, because Oracle does not
// put PL/SQL in a quoted string: the anonymous blocks, the units that
// CREATE makes, such as procedures, packages and triggers, the functions
// of WITH, and the declarations and the statements of PL/SQL. The words
// that only PL/SQL reserves, such as BEGIN and END, are not in the reserved
// set, because a reserved set changes the lexer and not the parse states
// (D125). A keyword wins where the parser expects it, so a statement that
// starts with an identifier, such as a call, cannot start with BEGIN or IF.
// A statement or a declaration of PL/SQL ends at a semicolon or at a /
// alone on its line, as a statement of SQL does. The parser cannot merge
// two states when only one of them takes a token of the external scanner,
// so this keeps the states after a statement of PL/SQL and after a
// statement of SQL the same.
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

// The kinds of objects that CREATE, ALTER and DROP name, each as the words
// before the name of the object.
const OBJECT_KINDS = [
  'ANALYTIC VIEW', 'ATTRIBUTE DIMENSION', 'AUDIT POLICY', 'CLUSTER',
  'CONTEXT', 'DIMENSION', 'DIRECTORY', 'DISKGROUP', 'EDITION',
  'FLASHBACK ARCHIVE', 'HIERARCHY', 'INDEXTYPE', 'INMEMORY JOIN GROUP',
  'JAVA CLASS', 'JAVA RESOURCE', 'JAVA SOURCE', 'LIBRARY', 'LOCKDOWN PROFILE',
  'MATERIALIZED VIEW LOG ON', 'MATERIALIZED ZONEMAP', 'MLE ENV', 'MLE MODULE',
  'OPERATOR', 'OUTLINE', 'PLUGGABLE DATABASE', 'PMEM FILESTORE', 'PROFILE',
  'RESTORE POINT', 'ROLE', 'ROLLBACK SEGMENT', 'SEQUENCE', 'TABLESPACE',
  'TABLESPACE SET', 'USER',
];

// The kinds of objects that have a statement of their own for CREATE, such
// as CREATE TABLE, and the units of PL/SQL.
const OWN_KINDS = [
  'DOMAIN', 'FUNCTION', 'INDEX', 'MATERIALIZED VIEW', 'PACKAGE',
  'PACKAGE BODY', 'PROCEDURE', 'PROPERTY GRAPH', 'SYNONYM', 'TABLE',
  'TRIGGER', 'TYPE', 'TYPE BODY', 'VIEW',
];

/**
 * Returns the keywords of a kind of object, such as 'MATERIALIZED VIEW'.
 *
 * @param {string} kind
 */
function kind(kind) {
  return kws(...kind.split(' '));
}

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
 * Returns a word of the conditional compilation of PL/SQL, such as $IF: a
 * token of $ and the word in any case, as an anonymous node with the name $
 * and the word.
 *
 * @param {string} word
 */
function directive(word) {
  return alias(token(seq('$', keyword(word))), '$' + word);
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
  // the grammar uses as keywords. The set label is empty, because the
  // grammar accepts every keyword as a name after a dot and as a word of a
  // list of options. A state takes the last set of its items, so the sets
  // go from the strictest to the least strict.
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
      $.create_table_statement,
      $.create_index_statement,
      $.create_view_statement,
      $.create_materialized_view_statement,
      $.create_synonym_statement,
      $.create_type_statement,
      $.create_domain_statement,
      $.create_property_graph_statement,
      $.create_schema_statement,
      $.create_statement,
      $.alter_table_statement,
      $.alter_statement,
      $.drop_statement,
      $.associate_statistics_statement,
      $.disassociate_statistics_statement,
      $.administer_key_management_statement,
      $.block,
      seq(repeat1($.label), $.block),
      $.create_procedure_statement,
      $.create_function_statement,
      $.create_package_statement,
      $.create_type_body_statement,
      $.create_trigger_statement,
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

    // The functions and the procedures of PL/SQL in WITH come before the
    // subquery factoring clauses, as in WITH FUNCTION f RETURN NUMBER IS
    // BEGIN RETURN 1; END; SELECT f FROM dual.
    with_clause: $ => seq(
      kw('WITH'),
      choice(
        seq(
          repeat1(choice($.function_declaration, $.procedure_declaration)),
          optional(commaSep1($.cte)),
        ),
        commaSep1($.cte),
      ),
    ),

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
        $.subquery_restriction,
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
      commaSep1($._target),
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
    // of a rule, a dimension can be a FOR loop. A measure of a reference
    // model has the name of the model, as in r.b[1].
    cell_reference: $ => seq(
      choice(
        field('measure', choice($._name, $.function_call)),
        seq(field('model', $._name), '.', field('measure', $._label)),
      ),
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

    // WITH after a query of a JSON relational duality view gives the tags
    // of its table, as in FROM dept d WITH INSERT UPDATE DELETE.
    subquery_restriction: $ => prec.right(seq(
      kw('WITH'),
      choice(
        seq(
          choice(kws('READ', 'ONLY'), kws('CHECK', 'OPTION')),
          optional(seq(kw('CONSTRAINT'), field('constraint', $._name))),
        ),
        repeat1($._duality_tag),
      ),
    )),

    _duality_tag: _ => choice(
      ...['INSERT', 'UPDATE', 'DELETE', 'NOINSERT', 'NOUPDATE', 'NODELETE', 'CHECK',
        'NOCHECK'].map(kw),
    ),

    subquery: $ => parens($._query),

    // Changes of data

    // The clauses after the first part of a statement that changes data
    // come in any order, as the clauses of a query do.
    insert_statement: $ => seq(
      kw('INSERT'),
      choice(
        seq($.insert_into_clause, choice($._query, $.record_values), repeat($._dml_clause)),
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

    // A record of PL/SQL gives the values of the row, as in INSERT INTO t
    // VALUES r.
    record_values: $ => seq(
      kw('VALUES'),
      field('record', choice($.column_reference, $.function_call, $.field_expression)),
    ),

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
      repeat(choice($.from_clause, $.where_clause, $._dml_clause, $.wait_clause)),
    ),

    // SET a = 1, SET (a, b) = (SELECT ...), SET VALUE (t) = expr, or SET
    // ROW = r with a record of PL/SQL.
    assignment: $ => choice(
      seq(kw('ROW'), '=', field('right', $._expression)),
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
      repeat(choice($.from_clause, $.where_clause, $._dml_clause, $.wait_clause)),
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
      $._option_word,
      $.string,
      $._signed_number,
      '=',
      ',',
      ':',
      $._option_group,
    ),

    // A list of options in parentheses, as in STORAGE (INITIAL 64K).
    _option_group: $ => parens(repeat($._option)),

    // A word of a list of options. It takes the reserved set label, so a
    // reserved word is a word of the list too, as in COMPRESS FOR OLTP, but
    // where the state also expects the word as a keyword, as AS in CREATE
    // TABLE t NOLOGGING AS SELECT. One identifier covers all the words, so
    // that the states of the list are few and small. Each word is a keyword
    // in the tree, but a quoted identifier stays an identifier. A word can
    // have a schema and a database link, as in pdb2@link.
    _option_word: $ => seq(
      choice(
        alias(reserved('label', $.identifier), $.keyword),
        alias($._quoted_identifier, $.identifier),
      ),
      repeat(seq('.', choice(
        alias(reserved('label', $.identifier), $.keyword),
        alias($._quoted_identifier, $.identifier),
      ))),
      optional(seq('@', field('link', $.database_link))),
    ),

    // An option of a statement that also has clauses that the grammar
    // reads, such as CREATE TABLE. A list in parentheses comes after a
    // word, as in TABLESPACE t STORAGE (INITIAL 1M), and not at the start,
    // where ( starts the columns of the table.
    _property: $ => prec.right(seq(
      choice(
        $._option_word,
        $.string,
        $._signed_number,
        '=',
        // STORE AS and RETURN AS of a LOB or a nested table, and STORE IN
        // of the partitions, are one token, the keyword STORE or RETURN in
        // the tree, so that store is a word in ROW STORE COMPRESS.
        alias($._store_as, 'STORE'),
        alias($._return_as, 'RETURN'),
        // The operators of a time of refresh, as in NEXT SYSDATE + 1/24.
        alias($._multiplicative_operator, $.operator),
        alias($._additive_operator, $.operator),
      ),
      optional($._option_group),
    )),

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

    // Definitions of objects

    // The words before the kind of object in CREATE, such as OR REPLACE,
    // GLOBAL TEMPORARY or UNIQUE, come in any order, and each statement
    // takes them all. Then the parse states after CREATE are shared.
    _create: $ => seq(
      kw('CREATE'),
      repeat(choice(
        kws('OR', 'REPLACE'),
        kws('AND', 'RESOLVE'),
        kws('AND', 'COMPILE'),
        kw('NO'),
        ...['FORCE', 'NOFORCE', 'EDITIONABLE', 'NONEDITIONABLE', 'EDITIONING',
          'GLOBAL', 'PRIVATE', 'TEMPORARY', 'SHARDED', 'DUPLICATED', 'IMMUTABLE',
          'BLOCKCHAIN', 'PUBLIC', 'SHARED', 'UNIQUE', 'BITMAP', 'MULTIVALUE',
          'VECTOR', 'SEARCH', 'BIGFILE', 'SMALLFILE', 'UNDO', 'LOCAL', 'FLEXIBLE',
          'USECASE', 'JSON', 'RELATIONAL', 'DUALITY'].map(kw),
      )),
    ),

    _if_not_exists: _ => kws('IF', 'NOT', 'EXISTS'),

    _if_exists: _ => kws('IF', 'EXISTS'),

    // CREATE TABLE. The options of the table, such as TABLESPACE, STORAGE
    // and LOB, are a list of words, but for the partitions, the
    // annotations and the clauses that start with ON.
    create_table_statement: $ => seq(
      $._create,
      kw('TABLE'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      optional(choice(
        parens(commaSep1($._table_element)),
        seq(
          kw('OF'),
          field('type', $.object_reference),
          optional(parens(commaSep1($._table_element))),
        ),
      )),
      repeat($._table_property),
      optional(seq(kw('AS'), $._query)),
    ),

    _table_element: $ => choice(
      $.column_definition,
      $.table_constraint,
      $.period_definition,
      $.supplemental_logging,
    ),

    // The type of a column is optional in CREATE TABLE AS, in a virtual
    // column and in ALTER TABLE MODIFY.
    column_definition: $ => prec.right(seq(
      field('name', $._name),
      optional(field('type', $.type)),
      repeat($._column_attribute),
    )),

    _column_attribute: $ => choice(
      kw('SORT'),
      kw('VISIBLE'),
      kw('INVISIBLE'),
      kw('RESERVABLE'),
      seq(kw('DOMAIN'), field('domain', $.object_reference)),
      $.default_clause,
      $.identity_clause,
      seq(
        optional(kws('GENERATED', 'ALWAYS')),
        kw('AS'),
        parens(field('generated', $._expression)),
        optional(kw('VIRTUAL')),
      ),
      seq(kw('COLLATE'), field('collation', $._name)),
      $.encrypt_clause,
      $.column_constraint,
      $._constraint_state,
      $.annotations,
    ),

    // DEFAULT ends before a word that is not an operator, such as NOT of
    // NOT NULL.
    default_clause: $ => prec.left(PREC.compare, seq(
      kw('DEFAULT'),
      optional(seq(kws('ON', 'NULL'), optional(seq(
        kws('FOR', 'INSERT'),
        choice(kw('ONLY'), kws('AND', 'UPDATE')),
      )))),
      field('value', $._expression),
    )),

    identity_clause: $ => seq(
      kw('GENERATED'),
      optional(choice(
        kw('ALWAYS'),
        seq(kws('BY', 'DEFAULT'), optional(kws('ON', 'NULL'))),
      )),
      kws('AS', 'IDENTITY'),
      optional($._option_group),
    ),

    encrypt_clause: $ => prec.right(seq(
      kw('ENCRYPT'),
      repeat(choice(
        seq(kw('USING'), $.string),
        seq(kws('IDENTIFIED', 'BY'), $._name),
        $.string,
        seq(optional(kw('NO')), kw('SALT')),
      )),
    )),

    column_constraint: $ => seq(
      optional(seq(kw('CONSTRAINT'), field('name', $._name))),
      choice(
        seq(optional(kw('NOT')), kw('NULL')),
        kw('UNIQUE'),
        kws('PRIMARY', 'KEY'),
        $.references_clause,
        seq(kw('CHECK'), parens(field('condition', $._expression))),
      ),
    ),

    table_constraint: $ => prec.right(seq(
      optional(seq(kw('CONSTRAINT'), field('name', $._name))),
      choice(
        seq(kw('UNIQUE'), $.column_list),
        seq(kws('PRIMARY', 'KEY'), $.column_list),
        seq(kws('FOREIGN', 'KEY'), $.column_list, $.references_clause),
        seq(kw('CHECK'), parens(field('condition', $._expression))),
      ),
      repeat($._constraint_state),
    )),

    references_clause: $ => prec.right(seq(
      kw('REFERENCES'),
      field('table', $.object_reference),
      optional($.column_list),
      optional(seq(kws('ON', 'DELETE'), choice(kw('CASCADE'), kws('SET', 'NULL')))),
    )),

    _constraint_state: $ => choice(
      seq(optional(kw('NOT')), kw('DEFERRABLE')),
      seq(kw('INITIALLY'), choice(kw('IMMEDIATE'), kw('DEFERRED'))),
      kw('RELY'),
      kw('NORELY'),
      kw('ENABLE'),
      kw('DISABLE'),
      kw('VALIDATE'),
      kw('NOVALIDATE'),
      $.using_index_clause,
      seq(kws('EXCEPTIONS', 'INTO'), field('exceptions', $.object_reference)),
    ),

    using_index_clause: $ => prec.right(seq(
      kws('USING', 'INDEX'),
      optional(choice(
        field('index', $.object_reference),
        parens($.create_index_statement),
      )),
      repeat(choice(
        seq(choice(kw('TABLESPACE'), kw('PCTFREE'), kw('INITRANS')), choice($._name, $.number)),
        seq(kw('STORAGE'), $._option_group),
        kws('COMPUTE', 'STATISTICS'),
        kw('LOGGING'),
        kw('NOLOGGING'),
        prec.right(seq(kw('COMPRESS'), optional($.number))),
        kw('NOCOMPRESS'),
        kw('REVERSE'),
        kw('NOSORT'),
        kw('ONLINE'),
      )),
    )),

    // PERIOD FOR is one token, the keyword PERIOD in the tree, so that a
    // column can have the name period.
    period_definition: $ => seq(
      alias($._period_for, 'PERIOD'),
      field('name', $._name),
      optional($.column_list),
    ),

    _period_for: _ => token(seq(keyword('PERIOD'), /\s+/, keyword('FOR'))),

    supplemental_logging: $ => prec.right(seq(
      alias($._supplemental_log, 'SUPPLEMENTAL'),
      repeat1($._property),
    )),

    _supplemental_log: _ => token(seq(keyword('SUPPLEMENTAL'), /\s+/, keyword('LOG'))),

    _store_as: _ => token(seq(keyword('STORE'), /\s+/, choice(keyword('AS'), keyword('IN')))),

    _return_as: _ => token(seq(keyword('RETURN'), /\s+/, keyword('AS'))),

    // The options of a table, an index or a materialized view.
    _table_property: $ => choice(
      $._property,
      $.table_partitioning,
      $.clustering_clause,
      $.annotations,
      prec.right(seq(
        kw('ON'),
        choice(
          seq(kw('COMMIT'), optional(seq(
            choice(kw('DELETE'), kw('PRESERVE'), kw('DROP')),
            choice(kw('ROWS'), kw('DEFINITION')),
          ))),
          kw('DEMAND'),
          kw('STATEMENT'),
          seq(kws('PREBUILT', 'TABLE'), optional(seq(
            choice(kw('WITH'), kw('WITHOUT')),
            kws('REDUCED', 'PRECISION'),
          ))),
          // A word, as in YES ON LOAD of the clustering of a table.
          $._option_word,
        ),
      )),
    ),

    // The attribute clustering of a table, as in CLUSTERING BY LINEAR ORDER
    // (a, b). Its options after the order are a list of words.
    clustering_clause: $ => seq(
      kw('CLUSTERING'),
      optional(seq(
        field('table', $.object_reference),
        repeat1(seq(
          kw('JOIN'),
          field('join', $.object_reference),
          kw('ON'),
          field('condition', $.parenthesized_expression),
        )),
      )),
      kw('BY'),
      choice(kw('LINEAR'), kw('INTERLEAVED')),
      kw('ORDER'),
      parens(commaSep1(choice($.column_reference, parens(commaSep1($.column_reference))))),
    ),

    // PARTITION BY of a table or an index, with its partitions.
    table_partitioning: $ => prec.right(seq(
      alias($._partition_by, 'PARTITION'),
      $._partitioning_method,
      repeat(choice(
        seq(kw('INTERVAL'), parens(field('interval', $._expression))),
        seq(kws('SUBPARTITION', 'BY'), $._partitioning_method),
        seq(choice(kw('PARTITIONS'), kw('SUBPARTITIONS')), $.number),
        seq(alias($._store_as, 'STORE'), $.column_list),
        kw('AUTOMATIC'),
        seq(kws('SUBPARTITION', 'TEMPLATE'), parens(commaSep1($.partition_definition))),
      )),
      optional(parens(commaSep1($.partition_definition))),
    )),

    _partitioning_method: $ => prec.right(seq(
      choice(
        kw('RANGE'),
        kw('HASH'),
        kw('LIST'),
        kw('REFERENCE'),
        kw('SYSTEM'),
        kws('CONSISTENT', 'HASH'),
      ),
      optional($.column_list),
    )),

    // A partition or a subpartition, with its values and its options.
    partition_definition: $ => prec.right(seq(
      choice(kw('PARTITION'), kw('SUBPARTITION')),
      field('name', $._name),
      optional(seq(
        kw('VALUES'),
        choice(
          seq(kws('LESS', 'THAN'), parens(commaSep1(choice($._expression, kw('MAXVALUE'))))),
          parens(commaSep1(choice($._expression, kw('DEFAULT')))),
        ),
      )),
      repeat($._property),
      optional(parens(commaSep1($.partition_definition))),
    )),

    // The annotations of 23ai, as in ANNOTATIONS (display 'Name', hidden).
    annotations: $ => seq(
      kw('ANNOTATIONS'),
      parens(commaSep1($.annotation)),
    ),

    annotation: $ => seq(
      optional(choice(
        seq(kw('ADD'), optional($._if_not_exists)),
        seq(kw('DROP'), optional($._if_exists)),
        kw('REPLACE'),
      )),
      field('name', $._name),
      optional(field('value', $.string)),
    ),

    // CREATE INDEX. The options of the index are a list of words, as in
    // INDEXTYPE IS ctxsys.context PARAMETERS ('sync (on commit)').
    create_index_statement: $ => seq(
      $._create,
      kw('INDEX'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      kw('ON'),
      choice(
        seq(
          field('table', $.object_reference),
          optional(field('alias', $._name)),
          parens(commaSep1($.index_column)),
        ),
        seq(kw('CLUSTER'), field('cluster', $.object_reference)),
      ),
      // A comma can come between the options, as in FILTER BY a, b. The
      // FROM and the WHERE of a bitmap join index are words of the list
      // too, because their rules would add about 800 parse states.
      repeat(choice($._table_property, ',')),
    ),

    index_column: $ => seq(
      field('value', $._expression),
      optional(choice(kw('ASC'), kw('DESC'))),
    ),

    // CREATE VIEW, and the JSON relational duality views of 23ai.
    create_view_statement: $ => seq(
      $._create,
      kw('VIEW'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      optional(parens(commaSep1($._table_element))),
      repeat(choice(
        seq(kw('SHARING'), '=', alias($.identifier, $.keyword)),
        seq(kws('DEFAULT', 'COLLATION'), field('collation', $._name)),
        seq(kw('BEQUEATH'), choice(kw('CURRENT_USER'), kw('DEFINER'))),
        seq(kw('OF'), field('type', $.object_reference)),
        seq(
          kws('WITH', 'OBJECT'),
          choice(kw('IDENTIFIER'), kw('ID'), kw('OID')),
          choice(kw('DEFAULT'), parens(commaSep1($._expression))),
        ),
        seq(kw('UNDER'), field('under', $.object_reference)),
        seq(choice(kw('ENABLE'), kw('DISABLE')), kws('LOGICAL', 'REPLICATION')),
        $.annotations,
      )),
      kw('AS'),
      $._query,
      optional(choice(kw('CONTAINER_MAP'), kw('CONTAINERS_DEFAULT'))),
    ),

    create_materialized_view_statement: $ => seq(
      $._create,
      kws('MATERIALIZED', 'VIEW'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      optional(parens(commaSep1($._table_element))),
      optional(seq(kw('OF'), field('type', $.object_reference))),
      repeat($._table_property),
      optional(seq(kw('AS'), $._query)),
    ),

    create_synonym_statement: $ => seq(
      $._create,
      kw('SYNONYM'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      optional(seq(kw('SHARING'), '=', alias($.identifier, $.keyword))),
      kw('FOR'),
      field('target', $.object_reference),
    ),

    // CREATE TYPE: an object type, a VARRAY type, a nested table type or
    // an incomplete type. The body of the methods is in CREATE TYPE BODY.
    create_type_statement: $ => seq(
      $._create,
      kw('TYPE'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      repeat(choice(
        kw('FORCE'),
        seq(kw('OID'), $.string),
        seq(kw('SHARING'), '=', alias($.identifier, $.keyword)),
        seq(kws('DEFAULT', 'COLLATION'), field('collation', $._name)),
        seq(kw('AUTHID'), choice(kw('CURRENT_USER'), kw('DEFINER'))),
        $._accessible_by,
      )),
      optional(choice(
        seq(
          choice(kw('AS'), kw('IS')),
          choice(
            seq(kw('OBJECT'), optional($._type_elements)),
            seq(
              choice(kw('VARRAY'), seq(optional(kw('VARYING')), kw('ARRAY'))),
              parens(field('limit', $._expression)),
              kw('OF'),
              field('element', $.type),
              optional(kws('NOT', 'NULL')),
            ),
            seq(
              kws('TABLE', 'OF'),
              field('element', $.type),
              optional(kws('NOT', 'NULL')),
            ),
          ),
        ),
        seq(kw('UNDER'), field('under', $.object_reference), optional($._type_elements)),
      )),
      repeat(seq(
        optional(kw('NOT')),
        choice(kw('FINAL'), kw('INSTANTIABLE'), kw('PERSISTABLE')),
      )),
    ),

    _type_elements: $ => parens(commaSep1(choice(
      $.type_attribute,
      $.type_method,
    ))),

    type_attribute: $ => seq(field('name', $._name), field('type', $.type)),

    type_method: $ => seq(
      repeat(seq(
        optional(kw('NOT')),
        choice(kw('OVERRIDING'), kw('FINAL'), kw('INSTANTIABLE')),
      )),
      optional(choice(kw('MAP'), kw('ORDER'))),
      choice(kw('MEMBER'), kw('STATIC'), kw('CONSTRUCTOR')),
      choice(
        seq(kw('PROCEDURE'), field('name', $._name), optional($.parameters)),
        seq(
          kw('FUNCTION'),
          field('name', $._name),
          optional($.parameters),
          kw('RETURN'),
          field('return', choice($.type, kws('SELF', 'AS', 'RESULT'))),
        ),
      ),
      repeat(choice(kw('DETERMINISTIC'), kw('PARALLEL_ENABLE'), kw('PIPELINED'), kw('RESULT_CACHE'))),
    ),

    // The parameters of a method or a subprogram.
    parameters: $ => parens(commaSep1($.parameter_declaration)),

    parameter_declaration: $ => seq(
      field('name', $._name),
      optional(kw('IN')),
      optional(kw('OUT')),
      optional(kw('NOCOPY')),
      field('type', $.type),
      optional(seq(choice(':=', kw('DEFAULT')), field('default', $._expression))),
    ),

    // CREATE DOMAIN of 23ai: a domain of one column, of several columns,
    // an enumeration domain or a flexible domain.
    create_domain_statement: $ => seq(
      $._create,
      kw('DOMAIN'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      choice(
        seq(
          kw('AS'),
          choice(
            field('type', $.type),
            seq(kw('ENUM'), parens(commaSep1($.enum_item))),
            parens(commaSep1(seq(field('column', $._name), kw('AS'), field('type', $.type)))),
          ),
        ),
        seq(
          $.column_list,
          kws('CHOOSE', 'DOMAIN', 'USING'),
          parens(commaSep1(seq(field('column', $._name), field('type', $.type)))),
          kw('FROM'),
          field('choose', $._expression),
        ),
      ),
      repeat(choice(
        kw('STRICT'),
        $.default_clause,
        seq(kw('COLLATE'), field('collation', $._name)),
        $.column_constraint,
        $._constraint_state,
        seq(kw('DISPLAY'), field('display', $._expression)),
        seq(kw('ORDER'), field('order', $._expression)),
        $.annotations,
      )),
    ),

    enum_item: $ => seq(
      field('name', $._name),
      optional(seq('=', field('value', $._expression))),
    ),

    // CREATE PROPERTY GRAPH of 23ai.
    create_property_graph_statement: $ => seq(
      $._create,
      kws('PROPERTY', 'GRAPH'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      kws('VERTEX', 'TABLES'),
      parens(commaSep1($.graph_element_table)),
      optional(seq(kws('EDGE', 'TABLES'), parens(commaSep1($.graph_element_table)))),
      optional(seq(kw('OPTIONS'), $._option_group)),
    ),

    graph_element_table: $ => seq(
      field('table', $.object_reference),
      optional($._alias),
      repeat(choice(
        seq(kw('KEY'), $.column_list),
        seq(
          choice(kw('SOURCE'), kw('DESTINATION')),
          optional(seq(kw('KEY'), $.column_list, kw('REFERENCES'))),
          field('reference', $._name),
          optional($.column_list),
        ),
        seq(kw('LABEL'), field('label', $._name)),
        kws('DEFAULT', 'LABEL'),
        seq(kw('PROPERTIES'), choice(
          seq(optional(kw('ARE')), kws('ALL', 'COLUMNS'), optional(seq(kw('EXCEPT'), $.column_list))),
          parens(commaSep1(choice($._expression, $.aliased_expression))),
        )),
        kws('NO', 'PROPERTIES'),
      )),
    ),

    // CREATE SCHEMA AUTHORIZATION creates tables and views and grants
    // privileges in one statement.
    create_schema_statement: $ => seq(
      kws('CREATE', 'SCHEMA', 'AUTHORIZATION'),
      field('schema', $._name),
      repeat1(choice(
        $.create_table_statement,
        $.create_view_statement,
        $.grant_statement,
      )),
    ),

    // CREATE of the other kinds of objects, such as SEQUENCE, USER and
    // TABLESPACE. The options are a list of words. AS can give a query, a
    // string or a list of words, as in CREATE DIRECTORY d AS '/tmp'.
    create_statement: $ => seq(
      $._create,
      choice(
        seq(
          $._create_kind,
          optional($._if_not_exists),
          field('name', $.object_reference),
        ),
        seq(kws('DATABASE', 'LINK'), optional($._if_not_exists), field('name', $.database_link)),
        kw('DATABASE'),
        kw('CONTROLFILE'),
        kw('PFILE'),
        kw('SPFILE'),
      ),
      repeat($._option),
      optional(seq(
        kw('AS'),
        choice($._query, seq(choice($.string, $._option_word), repeat($._option))),
      )),
    ),

    // The kinds of objects are hidden rules, so that the parse states after
    // the kind are shared.
    _create_kind: _ => choice(...OBJECT_KINDS.map(kind)),

    _alter_kind: _ => choice(
      ...[...OBJECT_KINDS, ...OWN_KINDS]
        .filter(k => k !== 'TABLE' && k !== 'PLUGGABLE DATABASE')
        .map(kind),
    ),

    _drop_kind: _ => choice(
      ...[...OBJECT_KINDS, ...OWN_KINDS, 'PUBLIC SYNONYM'].map(kind),
    ),

    // ALTER TABLE. The clauses that add, change, drop or rename a column, a
    // constraint or a partition have rules. The other clauses are a list of
    // words, as in MOVE ONLINE TABLESPACE t.
    alter_table_statement: $ => seq(
      kws('ALTER', 'TABLE'),
      optional($._if_exists),
      field('name', $.object_reference),
      repeat(choice(
        $.add_clause,
        $.modify_clause,
        $.drop_clause,
        $.rename_clause,
        $.set_unused_clause,
        $._table_property,
        ',',
      )),
      // A column without parentheses comes last, so that the words after
      // its default value are only its attributes. More words after an
      // expression cost large parse states.
      optional(choice(
        alias($._add_column, $.add_clause),
        alias($._modify_column, $.modify_clause),
      )),
    ),

    _add_column: $ => seq(kw('ADD'), $.column_definition),

    _modify_column: $ => seq(kw('MODIFY'), $.column_definition),

    add_clause: $ => seq(
      kw('ADD'),
      choice(
        parens(commaSep1($._table_element)),
        $.table_constraint,
        $.period_definition,
        $.supplemental_logging,
        $.partition_definition,
        $.clustering_clause,
      ),
    ),

    modify_clause: $ => seq(
      kw('MODIFY'),
      choice(
        parens(commaSep1($.column_definition)),
        seq(kw('CONSTRAINT'), field('constraint', $._name)),
        kws('PRIMARY', 'KEY'),
        seq(kw('UNIQUE'), $.column_list),
        seq(choice(kw('PARTITION'), kw('SUBPARTITION')), field('partition', $._name)),
        kws('DEFAULT', 'ATTRIBUTES'),
        seq(kw('LOB'), $.column_list),
      ),
    ),

    drop_clause: $ => seq(
      kw('DROP'),
      choice(
        seq(kw('COLUMN'), field('column', $._name)),
        $.column_list,
        seq(kw('CONSTRAINT'), field('constraint', $._name)),
        kws('PRIMARY', 'KEY'),
        seq(kw('UNIQUE'), $.column_list),
        seq(choice(kw('PARTITION'), kw('SUBPARTITION')), field('partition', $._name)),
        kws('UNUSED', 'COLUMNS'),
        kws('COLUMNS', 'CONTINUE'),
        seq(kws('PERIOD', 'FOR'), field('period', $._name)),
        $.supplemental_logging,
        kw('CLUSTERING'),
        // DROP INDEX after DROP CONSTRAINT drops the index of the
        // constraint.
        kw('INDEX'),
      ),
    ),

    rename_clause: $ => seq(
      kw('RENAME'),
      choice(
        seq(kw('TO'), field('new_name', $._name)),
        seq(
          choice(kw('COLUMN'), kw('CONSTRAINT'), kw('PARTITION'), kw('SUBPARTITION')),
          field('name', $._name),
          kw('TO'),
          field('new_name', $._name),
        ),
      ),
    ),

    // SET UNUSED marks columns as unused. The other forms of SET, such as
    // SET INTERVAL (), are a list of words.
    set_unused_clause: $ => seq(
      kw('SET'),
      choice(
        seq(kw('UNUSED'), choice(seq(kw('COLUMN'), field('column', $._name)), $.column_list)),
        $._property,
      ),
    ),

    // ALTER of the other kinds of objects, such as ALTER INDEX i REBUILD.
    // The clauses are a list of words.
    alter_statement: $ => seq(
      kw('ALTER'),
      choice(
        seq(
          $._alter_kind,
          optional($._if_exists),
          field('name', $.object_reference),
        ),
        seq(
          optional(kw('PUBLIC')),
          kws('DATABASE', 'LINK'),
          optional($._if_exists),
          field('name', $.database_link),
        ),
        kw('DATABASE'),
        kws('DATABASE', 'DICTIONARY'),
        kws('PLUGGABLE', 'DATABASE'),
        kws('RESOURCE', 'COST'),
      ),
      repeat1($._option),
    ),

    // DROP of each kind of object.
    drop_statement: $ => seq(
      kw('DROP'),
      choice(
        seq(
          $._drop_kind,
          optional($._if_exists),
          field('name', $.object_reference),
        ),
        seq(
          optional(kw('PUBLIC')),
          kws('DATABASE', 'LINK'),
          optional($._if_exists),
          field('name', $.database_link),
        ),
        kw('DATABASE'),
      ),
      repeat($._option),
    ),

    // ASSOCIATE STATISTICS gives statistics to columns, functions,
    // packages, types, indexes or index types.
    associate_statistics_statement: $ => seq(
      kws('ASSOCIATE', 'STATISTICS', 'WITH'),
      $._statistics_objects,
      repeat1(choice(
        seq(kw('USING'), choice(field('type', $.object_reference), kw('NULL'))),
        seq(kws('DEFAULT', 'COST'), parens(commaSep1($.number))),
        seq(kws('DEFAULT', 'SELECTIVITY'), $.number),
        seq(kw('WITH'), choice(kw('SYSTEM'), kw('USER')), kws('MANAGED', 'STORAGE', 'TABLES')),
      )),
    ),

    disassociate_statistics_statement: $ => seq(
      kws('DISASSOCIATE', 'STATISTICS', 'FROM'),
      $._statistics_objects,
      optional(kw('FORCE')),
    ),

    _statistics_objects: $ => choice(
      seq(kw('COLUMNS'), commaSep1($.column_reference)),
      seq(
        choice(kw('FUNCTIONS'), kw('PACKAGES'), kw('TYPES'), kw('INDEXES'), kw('INDEXTYPES')),
        commaSep1($.object_reference),
      ),
    ),

    // ADMINISTER KEY MANAGEMENT. Its clauses are a list of words.
    administer_key_management_statement: $ => seq(
      kws('ADMINISTER', 'KEY', 'MANAGEMENT'),
      repeat1($._option),
    ),

    // PL/SQL

    // CREATE PROCEDURE. The body is a block of PL/SQL after IS or AS, or a
    // call specification, as in AS LANGUAGE JAVA NAME 'C.m()'.
    create_procedure_statement: $ => seq(
      $._create,
      kw('PROCEDURE'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      optional($.parameters),
      repeat($._unit_option),
      choice(kw('IS'), kw('AS')),
      $._unit_body,
    ),

    // CREATE FUNCTION. A function with AGGREGATE USING or PIPELINED USING
    // has no body.
    create_function_statement: $ => seq(
      $._create,
      kw('FUNCTION'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      optional($.parameters),
      kw('RETURN'),
      field('return', $.type),
      repeat($._unit_option),
      optional(seq(choice(kw('IS'), kw('AS')), $._unit_body)),
    ),

    // CREATE PACKAGE and CREATE PACKAGE BODY. The specification holds
    // declarations. The body holds declarations and definitions, and it can
    // end with a block that initializes the package.
    create_package_statement: $ => seq(
      $._create,
      kw('PACKAGE'),
      optional(kw('BODY')),
      optional($._if_not_exists),
      field('name', $.object_reference),
      repeat($._unit_option),
      choice(kw('IS'), kw('AS')),
      repeat($._declaration),
      choice(
        seq(kw('END'), optional(field('end', $._name))),
        $._body,
      ),
    ),

    // CREATE TYPE BODY, with the bodies of the methods of an object type.
    create_type_body_statement: $ => seq(
      $._create,
      kws('TYPE', 'BODY'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      repeat($._unit_option),
      choice(kw('IS'), kw('AS')),
      repeat(choice($.method_definition, seq($.pragma, $._terminator))),
      kw('END'),
    ),

    // A method in a type body, as in MEMBER FUNCTION f RETURN NUMBER IS
    // BEGIN RETURN 1; END;.
    method_definition: $ => seq(
      repeat(seq(
        optional(kw('NOT')),
        choice(kw('OVERRIDING'), kw('FINAL'), kw('INSTANTIABLE')),
      )),
      optional(choice(kw('MAP'), kw('ORDER'))),
      choice(kw('MEMBER'), kw('STATIC'), kw('CONSTRUCTOR')),
      choice($.procedure_declaration, $.function_declaration),
    ),

    // CREATE TRIGGER: a trigger on a table or a view, an INSTEAD OF
    // trigger, a compound trigger with FOR, or a trigger on an event of
    // the schema or the database, such as LOGON or CREATE.
    create_trigger_statement: $ => seq(
      $._create,
      kw('TRIGGER'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      repeat($._unit_option),
      choice(kw('BEFORE'), kw('AFTER'), kws('INSTEAD', 'OF'), kw('FOR')),
      $.trigger_event,
      repeat(seq(kw('OR'), $.trigger_event)),
      kw('ON'),
      choice(
        field('table', $.object_reference),
        seq(kws('NESTED', 'TABLE'), field('column', $._name), kw('OF'), field('table', $.object_reference)),
        kw('DATABASE'),
        kw('SCHEMA'),
        seq(field('schema', $._name), '.', kw('SCHEMA')),
        kws('PLUGGABLE', 'DATABASE'),
      ),
      repeat($._trigger_clause),
      choice(
        $.block,
        $.call_statement,
        $.compound_trigger,
      ),
    ),

    // An event of a trigger, as in UPDATE OF a, b. An event of the schema
    // or the database is a list of words, as in ASSOCIATE STATISTICS.
    trigger_event: $ => choice(
      kw('INSERT'),
      kw('DELETE'),
      seq(kw('UPDATE'), optional(seq(kw('OF'), commaSep1($._name)))),
      repeat1(alias(reserved('label', $.identifier), $.keyword)),
    ),

    _trigger_clause: $ => choice(
      seq(kw('REFERENCING'), repeat1($.referencing_alias)),
      kws('FOR', 'EACH', 'ROW'),
      seq(choice(kw('FORWARD'), kw('REVERSE')), kw('CROSSEDITION')),
      seq(choice(kw('FOLLOWS'), kw('PRECEDES')), commaSep1($.object_reference)),
      kw('ENABLE'),
      kw('DISABLE'),
      seq(kw('WHEN'), field('condition', $.parenthesized_expression)),
    ),

    referencing_alias: $ => seq(
      choice(kw('OLD'), kw('NEW'), kw('PARENT')),
      optional(kw('AS')),
      field('alias', $._name),
    ),

    // The body of a compound trigger, with a section for each timing point.
    compound_trigger: $ => seq(
      kws('COMPOUND', 'TRIGGER'),
      repeat($._declaration),
      repeat1($.timing_point_section),
      kw('END'),
      optional(field('end', $._name)),
    ),

    timing_point_section: $ => seq(
      $._timing_point,
      kw('IS'),
      $._begin,
      $._timing_point,
      $._terminator,
    ),

    _timing_point: $ => seq(
      choice(kw('BEFORE'), kw('AFTER'), kws('INSTEAD', 'OF')),
      choice(kw('STATEMENT'), kws('EACH', 'ROW')),
    ),

    // The options of a subprogram, a package or a trigger, which come in any
    // order.
    _unit_option: $ => choice(
      kw('DETERMINISTIC'),
      seq(
        kw('PIPELINED'),
        optional(seq(choice(kw('ROW'), kw('TABLE')), kw('POLYMORPHIC'))),
        optional(seq(kw('USING'), field('implementation', $.object_reference))),
      ),
      seq(kw('PARALLEL_ENABLE'), optional($._option_group)),
      prec.right(seq(
        kw('RESULT_CACHE'),
        optional(seq(kw('RELIES_ON'), parens(optional(commaSep1($.object_reference))))),
      )),
      seq(kws('AGGREGATE', 'USING'), field('implementation', $.object_reference)),
      seq(kw('SQL_MACRO'), optional(parens(seq(
        optional(seq(kw('TYPE'), '=>')),
        choice(kw('SCALAR'), kw('TABLE')),
      )))),
      seq(kw('AUTHID'), choice(kw('CURRENT_USER'), kw('DEFINER'))),
      $._accessible_by,
      seq(kws('DEFAULT', 'COLLATION'), field('collation', $._name)),
      seq(kw('SHARING'), '=', alias($.identifier, $.keyword)),
      kw('EDITIONABLE'),
      kw('NONEDITIONABLE'),
    ),

    _accessible_by: $ => seq(
      kws('ACCESSIBLE', 'BY'),
      parens(commaSep1(seq(
        optional(choice(kw('FUNCTION'), kw('PROCEDURE'), kw('PACKAGE'), kw('TRIGGER'), kw('TYPE'))),
        $.object_reference,
      ))),
    ),

    // The body of a subprogram after IS or AS: its declarations and its
    // block, or a call specification.
    _unit_body: $ => choice(
      seq(repeat($._declaration), $._body),
      $.call_specification,
    ),

    // A call specification gives a subprogram of Java, C or JavaScript, as
    // in LANGUAGE C NAME "f" LIBRARY lib. Its clauses are a list of words.
    call_specification: $ => seq(
      choice(kw('LANGUAGE'), kw('EXTERNAL'), kw('MLE')),
      repeat($._option),
    ),

    // An anonymous block, or a block in a block.
    block: $ => seq(
      optional(seq(kw('DECLARE'), repeat($._declaration))),
      $._body,
    ),

    // BEGIN, the statements, the handlers of exceptions and END. The name
    // of the unit or the label of the block can come after END.
    _body: $ => seq($._begin, optional(field('end', $._name))),

    _begin: $ => seq(
      kw('BEGIN'),
      repeat($._plsql_item),
      optional($.exception_section),
      kw('END'),
    ),

    exception_section: $ => seq(kw('EXCEPTION'), repeat1($.exception_handler)),

    exception_handler: $ => seq(
      kw('WHEN'),
      $._exception_name,
      repeat(seq(kw('OR'), $._exception_name)),
      $._then,
    ),

    _exception_name: $ => choice(
      field('exception', kw('OTHERS')),
      field('exception', $._name),
      seq(field('package', $._name), '.', field('exception', $._label)),
    ),

    // Declarations

    // A declaration of a block, a subprogram or a package.
    _declaration: $ => choice(
      $.variable_declaration,
      $.exception_declaration,
      $.cursor_declaration,
      $.type_definition,
      $.subtype_definition,
      seq($.pragma, $._terminator),
      $.procedure_declaration,
      $.function_declaration,
      $.conditional_compilation,
    ),

    // A variable or a constant, as in n CONSTANT NUMBER := 1.
    variable_declaration: $ => seq(
      field('name', $._name),
      optional(kw('CONSTANT')),
      field('type', $.type),
      optional(kws('NOT', 'NULL')),
      optional(seq(choice(':=', kw('DEFAULT')), field('value', $._expression))),
      $._terminator,
    ),

    exception_declaration: $ => seq(field('name', $._name), kw('EXCEPTION'), $._terminator),

    // A cursor, with its query, or the declaration of a cursor whose query
    // comes in the package body.
    cursor_declaration: $ => seq(
      kw('CURSOR'),
      field('name', $._name),
      optional($.parameters),
      optional(seq(kw('RETURN'), field('return', $.type))),
      optional(seq(kw('IS'), $._query)),
      $._terminator,
    ),

    // A type of PL/SQL: a record, a collection or a REF CURSOR.
    type_definition: $ => seq(
      kw('TYPE'),
      field('name', $._name),
      choice(kw('IS'), kw('AS')),
      choice(
        seq(kw('RECORD'), parens(commaSep1($.field_definition))),
        seq(
          kws('TABLE', 'OF'),
          field('element', $.type),
          optional(kws('NOT', 'NULL')),
          optional(seq(kws('INDEX', 'BY'), field('index', $.type))),
        ),
        seq(
          choice(kw('VARRAY'), seq(optional(kw('VARYING')), kw('ARRAY'))),
          parens(field('limit', $._expression)),
          kw('OF'),
          field('element', $.type),
          optional(kws('NOT', 'NULL')),
        ),
        seq(kws('REF', 'CURSOR'), optional(seq(kw('RETURN'), field('return', $.type)))),
      ),
      $._terminator,
    ),

    field_definition: $ => seq(
      field('name', $._name),
      field('type', $.type),
      optional(kws('NOT', 'NULL')),
      optional(seq(choice(':=', kw('DEFAULT')), field('default', $._expression))),
    ),

    subtype_definition: $ => seq(
      kw('SUBTYPE'),
      field('name', $._name),
      kw('IS'),
      field('type', $.type),
      optional(seq(kw('RANGE'), field('low', $._expression), '..', field('high', $._expression))),
      optional(kws('NOT', 'NULL')),
      $._terminator,
    ),

    // A pragma, as in PRAGMA EXCEPTION_INIT(e, -20001).
    pragma: $ => seq(kw('PRAGMA'), field('name', $._name), optional($.arguments)),

    // A procedure or a function in a block, a package, a type body or WITH:
    // its declaration, or its definition with a body after IS or AS.
    procedure_declaration: $ => seq(
      kw('PROCEDURE'),
      field('name', $._name),
      optional($.parameters),
      repeat($._unit_option),
      optional(seq(choice(kw('IS'), kw('AS')), $._unit_body)),
      $._terminator,
    ),

    function_declaration: $ => seq(
      kw('FUNCTION'),
      field('name', $._name),
      optional($.parameters),
      kw('RETURN'),
      field('return', choice($.type, kws('SELF', 'AS', 'RESULT'))),
      repeat($._unit_option),
      optional(seq(choice(kw('IS'), kw('AS')), $._unit_body)),
      $._terminator,
    ),

    // Conditional compilation, as in $IF $$DEBUG $THEN ... $END. It holds
    // declarations or statements.
    conditional_compilation: $ => choice(
      seq(
        directive('IF'),
        field('condition', $._expression),
        $._conditional_then,
        repeat(seq(directive('ELSIF'), field('condition', $._expression), $._conditional_then)),
        optional(seq(directive('ELSE'), repeat($._conditional_item))),
        directive('END'),
      ),
      seq(directive('ERROR'), field('message', $._expression), directive('END')),
    ),

    // $THEN and the items after it, which $IF and $ELSIF share.
    _conditional_then: $ => seq(directive('THEN'), repeat($._conditional_item)),

    _conditional_item: $ => choice(
      $._declaration,
      seq(repeat($.label), $._plsql_statement, $._terminator),
    ),

    // Statements

    // A statement of PL/SQL, with its labels and its semicolon.
    _plsql_item: $ => choice(
      seq(repeat($.label), $._plsql_statement, $._terminator),
      seq($.pragma, $._terminator),
      $.conditional_compilation,
    ),

    // THEN and the statements after it, which IF, ELSIF, CASE and the
    // handlers of exceptions share, so that they share parse states.
    _then: $ => seq(kw('THEN'), repeat($._plsql_item)),

    label: $ => seq('<<', field('name', $._name), '>>'),

    _plsql_statement: $ => choice(
      $.assignment_statement,
      $.procedure_call,
      $.block,
      $.if_statement,
      $.case_statement,
      $.loop_statement,
      $.exit_statement,
      $.continue_statement,
      $.goto_statement,
      $.null_statement,
      $.return_statement,
      $.raise_statement,
      $.execute_immediate_statement,
      $.open_statement,
      $.fetch_statement,
      $.close_statement,
      $.forall_statement,
      $.pipe_row_statement,
      $.select_statement,
      $.insert_statement,
      $.update_statement,
      $.delete_statement,
      $.merge_statement,
      $.commit_statement,
      $.rollback_statement,
      $.savepoint_statement,
      $.set_transaction_statement,
      $.lock_table_statement,
    ),

    // The target of an assignment, a procedure call, or a cursor.
    _target: $ => choice(
      $.column_reference,
      $.function_call,
      $.field_expression,
      $.bind_variable,
    ),

    assignment_statement: $ => seq(
      field('left', $._target),
      ':=',
      field('right', $._expression),
    ),

    procedure_call: $ => field('procedure', choice(
      $.column_reference,
      $.function_call,
      $.field_expression,
    )),

    if_statement: $ => seq(
      kw('IF'),
      field('condition', $._expression),
      $._then,
      repeat($.elsif_clause),
      optional($.else_clause),
      kws('END', 'IF'),
    ),

    elsif_clause: $ => seq(
      kw('ELSIF'),
      field('condition', $._expression),
      $._then,
    ),

    else_clause: $ => seq(kw('ELSE'), repeat($._plsql_item)),

    case_statement: $ => seq(
      kw('CASE'),
      optional(field('value', $._expression)),
      repeat1($.case_when_clause),
      optional($.else_clause),
      kws('END', 'CASE'),
      optional(field('end', $._name)),
    ),

    case_when_clause: $ => seq(
      kw('WHEN'),
      field('condition', $._expression),
      $._then,
    ),

    // LOOP, WHILE and FOR, with the iteration controls of 21c, as in FOR i
    // IN 1..3, REPEAT i * 2 WHILE i < 100 LOOP.
    loop_statement: $ => seq(
      optional(choice(
        seq(kw('WHILE'), field('condition', $._expression)),
        seq(
          kw('FOR'),
          commaSep1($.iterator),
          kw('IN'),
          commaSep1($.iteration_control),
        ),
      )),
      kw('LOOP'),
      repeat($._plsql_item),
      kws('END', 'LOOP'),
      optional(field('end', $._name)),
    ),

    iterator: $ => seq(
      field('name', $._name),
      optional(choice(kw('MUTABLE'), kw('IMMUTABLE'))),
      optional(field('type', $.type)),
    ),

    // A range, as in REVERSE 1..n, a cursor, a query, a collection with
    // VALUES OF, INDICES OF or PAIRS OF, or REPEAT with an expression.
    iteration_control: $ => seq(
      choice(
        seq(
          optional(kw('REVERSE')),
          field('low', $._expression),
          '..',
          field('high', $._expression),
          optional(seq(kw('BY'), field('step', $._expression))),
        ),
        seq(optional(kw('REPEAT')), field('value', $._expression)),
        seq(
          choice(kw('VALUES'), kw('INDICES'), kw('PAIRS')),
          kw('OF'),
          field('collection', $._expression),
        ),
      ),
      optional(seq(kw('WHILE'), field('while', $._expression))),
      optional(seq(kw('WHEN'), field('when', $._expression))),
    ),

    exit_statement: $ => seq(
      kw('EXIT'),
      optional(field('label', $._name)),
      optional(seq(kw('WHEN'), field('condition', $._expression))),
    ),

    continue_statement: $ => seq(
      kw('CONTINUE'),
      optional(field('label', $._name)),
      optional(seq(kw('WHEN'), field('condition', $._expression))),
    ),

    goto_statement: $ => seq(kw('GOTO'), field('label', $._name)),

    null_statement: _ => kw('NULL'),

    return_statement: $ => seq(kw('RETURN'), optional(field('value', $._expression))),

    raise_statement: $ => seq(kw('RAISE'), optional($._exception_name)),

    execute_immediate_statement: $ => seq(
      kws('EXECUTE', 'IMMEDIATE'),
      field('statement', $._expression),
      repeat(choice(
        $.into_clause,
        $.using_clause,
        seq(choice(kw('RETURNING'), kw('RETURN')), $.into_clause),
      )),
    ),

    // The bind arguments of a dynamic statement, as in USING IN OUT x.
    using_clause: $ => seq(
      kw('USING'),
      commaSep1(seq(optional(kw('IN')), optional(kw('OUT')), $._expression)),
    ),

    // OPEN a cursor, or OPEN a cursor variable FOR a query or a string.
    open_statement: $ => seq(
      kw('OPEN'),
      field('cursor', $._target),
      optional(seq(
        kw('FOR'),
        choice($._query, field('statement', $._expression)),
        optional($.using_clause),
      )),
    ),

    fetch_statement: $ => seq(
      kw('FETCH'),
      field('cursor', $._target),
      $.into_clause,
      optional(seq(kw('LIMIT'), field('limit', $._expression))),
    ),

    close_statement: $ => seq(kw('CLOSE'), field('cursor', $._target)),

    forall_statement: $ => seq(
      kw('FORALL'),
      field('index', $._name),
      kw('IN'),
      $.iteration_control,
      optional(kws('SAVE', 'EXCEPTIONS')),
      field('statement', choice(
        $.insert_statement,
        $.update_statement,
        $.delete_statement,
        $.merge_statement,
        $.execute_immediate_statement,
      )),
    ),

    pipe_row_statement: $ => seq(kws('PIPE', 'ROW'), $.parenthesized_expression),

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
      $.json_constructor,
      $.cursor_attribute,
      $.inquiry_directive,
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
        // An element of a collection that a function returns, as in
        // f(1)(2).
        $.arguments,
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
      // An argument with an alias, as in PREDICTION(m USING 'F' gender).
      alias($._aliased_argument, $.aliased_expression),
      // COUNT(*), t.* and all the columns of the functions of data mining,
      // as in PREDICTION(m USING *).
      $.all_columns,
      // A query in brackets gives an array in a JSON relational duality
      // view, as in JSON {'emps' : [SELECT JSON {...} FROM emp]}.
      seq('[', $._query, ']'),
    ),

    _aliased_argument: $ => seq(field('value', $._expression), field('alias', $._name)),

    // The name of an argument can be any word, as in f(comment => 'x'). A
    // qualified expression of PL/SQL gives the values of the elements of
    // an associative array, as in t(1 => 'a', 'k' => 'b').
    named_argument: $ => seq(
      field('name', choice($._label, $.number, $.string, $.unary_expression, $.function_call)),
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
      // The tags of a column of a JSON relational duality view, as in
      // 'name' : d.dname WITH NOUPDATE.
      seq(kw('WITH'), repeat1($._duality_tag)),
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
        $.cursor_attribute,
        $.generalized_invocation,
      )),
      '.',
      field('field', choice($._label, $.function_call)),
    )),

    // A method of a supertype, as in (SELF AS person_t).show().
    generalized_invocation: $ => parens(seq(
      field('value', $._expression),
      kw('AS'),
      field('type', $.type),
    )),

    // An attribute of a cursor of PL/SQL, as in c%ROWCOUNT, SQL%FOUND or
    // SQL%BULK_ROWCOUNT(1).
    cursor_attribute: $ => prec.right(PREC.field, seq(
      field('cursor', choice($.column_reference, $.bind_variable)),
      '%',
      field('attribute', $._label),
      optional($.arguments),
    )),

    // An inquiry directive of the conditional compilation of PL/SQL, as in
    // $$PLSQL_UNIT.
    inquiry_directive: _ => /\$\$[A-Za-z_][A-Za-z0-9_$#]*/,

    parenthesized_expression: $ => parens($._expression),

    // A list of expressions, as in (a, b) IN ((1, 2), (3, 4)).
    list: $ => parens(seq($._expression, ',', commaSep1($._expression))),

    exists_expression: $ => seq(kw('EXISTS'), $.subquery),

    // JSON {...} of 23ai, the short form of JSON_OBJECT, as in JSON {'a' :
    // 1}. JSON is the token identifier here, so that json can also name a
    // column.
    json_constructor: $ => seq(
      alias($.identifier, $.keyword),
      '{',
      optional($._argument_list),
      '}',
    ),

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

    // A name with three parts gives the type of a column in PL/SQL, as in
    // hr.employees.salary%TYPE.
    type: $ => prec.right(choice(
      seq(
        choice(
          field('name', $._name),
          seq(
            field('schema', $._name),
            '.',
            field('name', $._name),
            optional(seq('.', field('column', $._label))),
          ),
        ),
        optional($.type_modifiers),
        optional(seq('%', choice(kw('TYPE'), kw('ROWTYPE')))),
      ),
      seq(choice(...TYPE_WORDS.map(kw)), optional($.type_modifiers)),
      seq(kw('LONG'), optional(kw('RAW'))),
      // REF of an object type, as in REF person_t.
      seq(kw('REF'), field('name', $.object_reference)),
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
