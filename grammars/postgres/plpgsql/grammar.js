/**
 * @file The grammar of PL/pgSQL, the procedural language of PostgreSQL 19
 * @license MIT
 */

// xo writes this grammar from the chapter "PL/pgSQL - SQL Procedural
// Language" of the reference manual of PostgreSQL 19 (D122). It does not
// come from the Bison grammar of PostgreSQL.
//
// The input is the body of a function, a procedure or a DO block. The
// grammar reads the statements of PL/pgSQL. It does not read the SQL in
// them. Each expression and each SQL statement is one token, which the
// external scanner in src/scanner.c reads up to the word or the character
// that ends it in its place, as PostgreSQL does: the condition of IF ends at
// THEN, the expression of WHILE ends at LOOP, and an SQL statement ends at a
// semicolon. The token is the node sql_expression or sql_statement, and
// queries/injections.scm gives its text to the grammar postgres.
//
// A keyword is a token in any case, and it is an anonymous node with the
// name of the keyword in upper case, such as "BEGIN". The token identifier
// is the word of the grammar, so a keyword that PL/pgSQL does not reserve
// is an identifier in each place where the grammar does not expect the
// keyword. The reserved words of PL/pgSQL are in the reserved set global.

/// <reference types="tree-sitter-cli/dsl" />
// @ts-check

// The words that PL/pgSQL reserves, which src/pl/plpgsql/src/
// pl_reserved_kwlist.h of PostgreSQL lists. A word is in the list only when
// the grammar uses it as a keyword, because a reserved word must be a token
// of the grammar.
const RESERVED_WORDS = [
  'ALL', 'BEGIN', 'BY', 'CASE', 'DECLARE', 'ELSE', 'END', 'EXECUTE', 'FOR',
  'FOREACH', 'FROM', 'IF', 'IN', 'INTO', 'LOOP', 'NOT', 'NULL', 'OR',
  'STRICT', 'THEN', 'USING', 'WHEN', 'WHILE',
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

module.exports = grammar({
  name: 'plpgsql',

  extras: $ => [
    /\s/,
    $.comment,
  ],

  // The order of the external tokens is the order of enum TokenType in
  // src/scanner.c. Each token whose name starts with _sql is a piece of SQL
  // that ends where its place ends. The scanner reads none of them while
  // the parser recovers from an error, which _error_sentinel shows.
  externals: $ => [
    $.comment,
    $._sql_statement,
    $._sql_semicolon,
    $._sql_return,
    $._sql_query,
    $._sql_then,
    $._sql_when,
    $._sql_loop,
    $._sql_range,
    $._sql_by,
    $._sql_list,
    $._sql_list_loop,
    $._sql_argument,
    $._sql_subscript,
    $._dollar_quoted_string,
    $._error_sentinel,
  ],

  word: $ => $.identifier,

  reserved: {
    global: _ => RESERVED_WORDS.map(keyword),
  },

  rules: {
    // A body holds one block. PostgreSQL takes a semicolon after its END,
    // and the compiler options before it.
    source_file: $ => seq(
      repeat($.compiler_option),
      optional(seq($.block, optional(';'))),
    ),

    // A compiler option, such as #variable_conflict use_column.
    compiler_option: $ => seq(
      '#',
      field('name', $.identifier),
      field('value', $.identifier),
    ),

    // A block, with a label, the declarations, the statements and the
    // handlers of the exceptions. The label can follow END.
    block: $ => seq(
      optional($.label),
      optional($.declare_section),
      kw('BEGIN'),
      repeat($._statement),
      optional($.exception_section),
      kw('END'),
      optional(field('end_label', $._name)),
    ),

    label: $ => seq('<<', field('name', $._name), '>>'),

    // DECLARE can come again between two declarations.
    declare_section: $ => seq(
      kw('DECLARE'),
      repeat(choice(
        $.variable_declaration,
        $.alias_declaration,
        $.cursor_declaration,
        kw('DECLARE'),
      )),
    ),

    variable_declaration: $ => seq(
      field('name', $._name),
      optional(kw('CONSTANT')),
      field('type', $.type),
      optional(seq(kw('COLLATE'), field('collation', $._qualified_name))),
      optional(kws('NOT', 'NULL')),
      optional(seq(
        choice(kw('DEFAULT'), ':=', '='),
        field('default', alias($._sql_semicolon, $.sql_expression)),
      )),
      ';',
    ),

    alias_declaration: $ => seq(
      field('name', $._name),
      kws('ALIAS', 'FOR'),
      field('target', choice($.parameter, $._name)),
      ';',
    ),

    // A cursor, with its arguments and its query.
    cursor_declaration: $ => seq(
      field('name', $._name),
      optional(seq(optional(kw('NO')), kw('SCROLL'))),
      kw('CURSOR'),
      optional(seq('(', commaSep1($.cursor_parameter), ')')),
      choice(kw('FOR'), kw('IS')),
      field('query', alias($._sql_semicolon, $.sql_statement)),
      ';',
    ),

    cursor_parameter: $ => seq(field('name', $._name), field('type', $.type)),

    // A type is a name of one or more words, such as double precision or
    // timestamp with time zone, with its modifiers and its array bounds.
    // %TYPE gives the type of a variable or a column, and %ROWTYPE the type
    // of a row of a table. The grammar takes more forms than PostgreSQL.
    type: $ => seq(
      $._qualified_name,
      repeat(choice($._name, $.type_modifiers)),
      repeat(seq('[', optional($.number), ']')),
      optional(seq('%', choice(kw('TYPE'), kw('ROWTYPE')))),
    ),

    type_modifiers: $ => seq('(', commaSep1(choice($.number, $._name)), ')'),

    exception_section: $ => seq(kw('EXCEPTION'), repeat1($.exception_handler)),

    exception_handler: $ => seq(
      kw('WHEN'),
      $._condition,
      repeat(seq(kw('OR'), $._condition)),
      kw('THEN'),
      repeat($._statement),
    ),

    // A condition is the name of a condition, such as division_by_zero or
    // OTHERS, or SQLSTATE and a code.
    _condition: $ => field('condition', choice(
      $._name,
      $.sqlstate,
    )),

    sqlstate: $ => seq(kw('SQLSTATE'), field('code', $.string)),

    _statement: $ => choice(
      seq($.block, ';'),
      $.assignment,
      $.perform_statement,
      $.sql_command,
      $.execute_statement,
      $.get_diagnostics_statement,
      $.if_statement,
      $.case_statement,
      $.loop_statement,
      $.while_statement,
      $.for_statement,
      $.foreach_statement,
      $.exit_statement,
      $.return_statement,
      $.open_statement,
      $.fetch_statement,
      $.move_statement,
      $.close_statement,
      $.raise_statement,
      $.assert_statement,
      $.null_statement,
      $.commit_statement,
      $.rollback_statement,
    ),

    // An SQL statement, such as SELECT, INSERT or CALL. The scanner takes
    // it when its first word starts a command of SQL.
    sql_command: $ => seq(alias($._sql_statement, $.sql_statement), ';'),

    assignment: $ => seq(
      field('target', $.variable),
      choice(':=', '='),
      field('value', alias($._sql_semicolon, $.sql_expression)),
      ';',
    ),

    // A variable, a field of a record or an element of an array. A
    // parameter can be a variable too.
    variable: $ => seq(
      choice($._name, $.parameter),
      repeat(choice(
        seq('.', $._name),
        seq('[', field('subscript', alias($._sql_subscript, $.sql_expression)), ']'),
      )),
    ),

    perform_statement: $ => seq(
      kw('PERFORM'),
      field('query', alias($._sql_semicolon, $.sql_expression)),
      ';',
    ),

    // EXECUTE takes INTO and USING in either order.
    execute_statement: $ => seq(
      kw('EXECUTE'),
      field('command', alias($._sql_list, $.sql_expression)),
      repeat(choice($.into_clause, $.using_clause)),
      ';',
    ),

    into_clause: $ => seq(
      kw('INTO'),
      optional(kw('STRICT')),
      commaSep1(field('target', $.variable)),
    ),

    using_clause: $ => seq(
      kw('USING'),
      commaSep1(field('argument', alias($._sql_list, $.sql_expression))),
    ),

    get_diagnostics_statement: $ => seq(
      kw('GET'),
      optional(choice(kw('CURRENT'), kw('STACKED'))),
      kw('DIAGNOSTICS'),
      commaSep1($.diagnostics_item),
      ';',
    ),

    diagnostics_item: $ => seq(
      field('target', $.variable),
      choice(':=', '='),
      field('item', $._name),
    ),

    if_statement: $ => seq(
      kw('IF'),
      field('condition', alias($._sql_then, $.sql_expression)),
      kw('THEN'),
      repeat($._statement),
      repeat($.elsif_clause),
      optional($.else_clause),
      kws('END', 'IF'),
      ';',
    ),

    elsif_clause: $ => seq(
      choice(kw('ELSIF'), kw('ELSEIF')),
      field('condition', alias($._sql_then, $.sql_expression)),
      kw('THEN'),
      repeat($._statement),
    ),

    else_clause: $ => seq(kw('ELSE'), repeat($._statement)),

    // A simple CASE compares its value with the expressions of each WHEN,
    // and a searched CASE tests the condition of each WHEN.
    case_statement: $ => seq(
      kw('CASE'),
      optional(field('value', alias($._sql_when, $.sql_expression))),
      repeat1($.case_when_clause),
      optional($.else_clause),
      kws('END', 'CASE'),
      ';',
    ),

    case_when_clause: $ => seq(
      kw('WHEN'),
      field('condition', alias($._sql_then, $.sql_expression)),
      kw('THEN'),
      repeat($._statement),
    ),

    loop_statement: $ => seq(optional($.label), $._loop_body),

    while_statement: $ => seq(
      optional($.label),
      kw('WHILE'),
      field('condition', alias($._sql_loop, $.sql_expression)),
      $._loop_body,
    ),

    // FOR over a range of integers, over the rows of a query or a cursor,
    // or over the rows of a dynamic query. The scanner cannot tell a range
    // from a query until it reaches .. or LOOP, so the lower bound of a
    // range and the query are the same token.
    for_statement: $ => seq(
      optional($.label),
      kw('FOR'),
      commaSep1(field('variable', $.variable)),
      kw('IN'),
      choice(
        seq(
          optional(kw('REVERSE')),
          field('lower', alias($._sql_range, $.sql_expression)),
          '..',
          field('upper', alias($._sql_by, $.sql_expression)),
          optional(seq(kw('BY'), field('step', alias($._sql_loop, $.sql_expression)))),
        ),
        field('query', alias($._sql_range, $.sql_expression)),
        seq(
          kw('EXECUTE'),
          field('command', alias($._sql_list_loop, $.sql_expression)),
          optional(seq(
            kw('USING'),
            commaSep1(field('argument', alias($._sql_list_loop, $.sql_expression))),
          )),
        ),
      ),
      $._loop_body,
    ),

    foreach_statement: $ => seq(
      optional($.label),
      kw('FOREACH'),
      commaSep1(field('variable', $.variable)),
      optional(seq(kw('SLICE'), field('slice', $.number))),
      kws('IN', 'ARRAY'),
      field('array', alias($._sql_loop, $.sql_expression)),
      $._loop_body,
    ),

    _loop_body: $ => seq(
      kw('LOOP'),
      repeat($._statement),
      kws('END', 'LOOP'),
      optional(field('end_label', $._name)),
      ';',
    ),

    exit_statement: $ => seq(
      choice(kw('EXIT'), kw('CONTINUE')),
      optional(field('label', $._name)),
      optional(seq(kw('WHEN'), field('condition', alias($._sql_semicolon, $.sql_expression)))),
      ';',
    ),

    return_statement: $ => seq(
      kw('RETURN'),
      optional(choice(
        field('value', alias($._sql_return, $.sql_expression)),
        seq(kw('NEXT'), optional(field('value', alias($._sql_semicolon, $.sql_expression)))),
        seq(kw('QUERY'), $._query),
      )),
      ';',
    ),

    // The query of RETURN QUERY and of OPEN FOR: a query, or EXECUTE and a
    // dynamic query.
    _query: $ => choice(
      field('query', alias($._sql_query, $.sql_statement)),
      seq(
        kw('EXECUTE'),
        field('command', alias($._sql_list, $.sql_expression)),
        optional($.using_clause),
      ),
    ),

    open_statement: $ => seq(
      kw('OPEN'),
      field('cursor', $._name),
      choice(
        seq(optional(seq(optional(kw('NO')), kw('SCROLL'))), kw('FOR'), $._query),
        optional(seq('(', commaSep1($.cursor_argument), ')')),
      ),
      ';',
    ),

    // An argument of a cursor, with the name of its parameter or without.
    cursor_argument: $ => seq(
      optional(seq(field('name', $._name), choice(':=', '=>'))),
      field('value', alias($._sql_argument, $.sql_expression)),
    ),

    fetch_statement: $ => seq(
      kw('FETCH'),
      optional($.direction),
      optional(choice(kw('FROM'), kw('IN'))),
      field('cursor', $._name),
      kw('INTO'),
      commaSep1(field('target', $.variable)),
      ';',
    ),

    move_statement: $ => seq(
      kw('MOVE'),
      optional($.direction),
      optional(choice(kw('FROM'), kw('IN'))),
      field('cursor', $._name),
      ';',
    ),

    // The direction of FETCH and MOVE. The grammar takes a number or a
    // variable as a count, where PostgreSQL takes an expression. After
    // FORWARD and BACKWARD, it takes only a number, because a name there
    // can also be the cursor.
    direction: $ => choice(
      kw('NEXT'),
      kw('PRIOR'),
      kw('FIRST'),
      kw('LAST'),
      seq(choice(kw('ABSOLUTE'), kw('RELATIVE')), $._count),
      seq(
        choice(kw('FORWARD'), kw('BACKWARD')),
        optional(choice(field('count', seq(optional('-'), $.number)), kw('ALL'))),
      ),
      $._count,
      kw('ALL'),
    ),

    _count: $ => field('count', choice(seq(optional('-'), $.number), $._name)),

    close_statement: $ => seq(kw('CLOSE'), field('cursor', $._name), ';'),

    // RAISE with a level and a message: a format and its arguments, the
    // name of a condition, or SQLSTATE and a code. USING adds the options of
    // the message. RAISE alone raises the current exception again.
    raise_statement: $ => seq(
      kw('RAISE'),
      optional(field('level', choice(
        kw('DEBUG'),
        kw('LOG'),
        kw('INFO'),
        kw('NOTICE'),
        kw('WARNING'),
        kw('EXCEPTION'),
      ))),
      optional(choice(
        seq(
          field('format', $.string),
          repeat(seq(',', field('argument', alias($._sql_list, $.sql_expression)))),
        ),
        $._condition,
      )),
      optional(seq(kw('USING'), commaSep1($.raise_option))),
      ';',
    ),

    raise_option: $ => seq(
      field('name', $._name),
      choice('=', ':='),
      field('value', alias($._sql_list, $.sql_expression)),
    ),

    assert_statement: $ => seq(
      kw('ASSERT'),
      field('condition', alias($._sql_list, $.sql_expression)),
      optional(seq(',', field('message', alias($._sql_list, $.sql_expression)))),
      ';',
    ),

    null_statement: _ => seq(kw('NULL'), ';'),

    commit_statement: $ => seq(kw('COMMIT'), optional($._chain), ';'),

    rollback_statement: $ => seq(kw('ROLLBACK'), optional($._chain), ';'),

    _chain: _ => seq(kw('AND'), optional(kw('NO')), kw('CHAIN')),

    _qualified_name: $ => seq($._name, repeat(seq('.', $._name))),

    // A name is an identifier or a quoted identifier, and both are the node
    // identifier.
    _name: $ => choice(
      $.identifier,
      alias($._quoted_identifier, $.identifier),
    ),

    identifier: _ => /[A-Za-z_\u0080-\u{10FFFF}][A-Za-z0-9_$\u0080-\u{10FFFF}]*/u,

    _quoted_identifier: _ => /"([^"]|"")*"/,

    parameter: _ => /\$\d+/,

    number: _ => /\d+/,

    // A string is in single quotes, with a quote doubled, or in dollar
    // quotes. E before the quotes allows backslash escapes.
    string: $ => choice(
      $._quoted_string,
      $._dollar_quoted_string,
    ),

    _quoted_string: _ => token(choice(
      /'([^']|'')*'/,
      /[eE]'([^'\\]|\\(.|\n)|'')*'/,
    )),
  },
});
