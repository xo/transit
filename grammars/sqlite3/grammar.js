/**
 * @file The grammar of the SQL of SQLite
 * @license MIT
 */

// xo writes this grammar from the SQL reference of the current release of
// SQLite, at https://sqlite.org/lang.html (D126). It does not come from
// parse.y of SQLite or from the grammar of dhcmrlchtdj/tree-sitter-sqlite.
//
// The grammar is for a highlighter and for completion, so it accepts more
// than SQLite does where that keeps the rules small. A keyword is a token in
// any case, and it is an anonymous node with the name of the keyword in
// upper case, such as "SELECT".
//
// SQLite reserves few of its keywords. Its parser takes most keywords as a
// name in each place where the keyword cannot come. The token identifier is
// the word of the grammar, so a keyword that SQLite does not reserve is an
// identifier in each place where the grammar does not expect the keyword.
// The reserved words of SQLite are in the reserved set global, so they are
// not identifiers.
//
// The grammar keeps its parse tables small (D122, D126). A parse state with
// more than 64 columns takes the large form, so the grammar keeps the
// columns of a state few. The operators of one level of precedence are one
// token, the three keywords CURRENT_DATE, CURRENT_TIME and CURRENT_TIMESTAMP
// are one token, and so are TRUE and FALSE. The words of a join before JOIN,
// such as LEFT OUTER, are one token with JOIN. The clauses of a query after
// FROM, and the clauses after its body, come in any order, because a
// sequence of optional clauses costs parse states for each set of the
// clauses.
//
// A statement ends at a semicolon or at the end of the input. The commands
// of the sqlite3 shell, such as .tables and .mode, are not SQL, so the
// grammar does not read them. The variables of usql belong to the usql
// grammar, which hands this grammar a placeholder of the same length in
// place of each variable (D101). The grammar has no external scanner.

/// <reference types="tree-sitter-cli/dsl" />
// @ts-check

// The precedences of the operators, from the lowest to the highest, as the
// section "Operators, and Parse-Affecting Attributes" of lang_expr.html lists
// them. IS, IN, LIKE, GLOB, REGEXP, MATCH, BETWEEN, ISNULL, NOTNULL and NOT
// NULL have the precedence of = and !=. ESCAPE binds to the LIKE before it.
// A unary operator binds more tightly than COLLATE.
const PREC = {
  or: 1,
  and: 2,
  not: 3,
  equal: 4,
  compare: 5,
  bit: 6,
  add: 7,
  multiply: 8,
  concat: 9,
  collate: 10,
  unary: 11,
};

// The keywords that SQLite reserves: the keywords of lang_keywords.html that
// the parser of SQLite does not take as a name. The parser takes the other
// keywords as a name where the keyword cannot come, and it takes the words
// of a join, such as LEFT and NATURAL, as a name too. A reserved word must
// be a token of the grammar, so the reserved set holds only the reserved
// words that the rules use as keywords. FILTER, OVER and WINDOW are not
// reserved, because SQLite reads them as keywords only where they can come.
const RESERVED_WORDS = [
  'ADD', 'ALL', 'ALTER', 'AND', 'AS', 'AUTOINCREMENT', 'BETWEEN', 'CASE',
  'CHECK', 'COLLATE', 'COMMIT', 'CONSTRAINT', 'CREATE', 'DEFAULT',
  'DEFERRABLE', 'DELETE', 'DISTINCT', 'DROP', 'ELSE', 'ESCAPE', 'EXCEPT',
  'EXISTS', 'FOREIGN', 'FROM', 'GROUP', 'HAVING', 'IN', 'INDEX', 'INSERT',
  'INTERSECT', 'INTO', 'IS', 'ISNULL', 'JOIN', 'LIMIT', 'NOT', 'NOTHING',
  'NOTNULL', 'NULL', 'ON', 'OR', 'ORDER', 'PRIMARY', 'REFERENCES',
  'RETURNING', 'SELECT', 'SET', 'TABLE', 'THEN', 'TO', 'TRANSACTION', 'UNION',
  'UNIQUE', 'UPDATE', 'USING', 'VALUES', 'WHEN', 'WHERE',
];

// The words that the rules use as keywords. The rules run before the
// reserved sets, so kw fills it before the reserved sets read it.
const KEYWORDS = new Set();

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
  name: 'sqlite3',

  extras: $ => [
    /\s/,
    $.comment,
  ],

  word: $ => $.identifier,

  // The reserved set global holds the reserved words of SQLite that the
  // grammar uses as keywords.
  reserved: {
    global: _ => RESERVED_WORDS.filter(w => KEYWORDS.has(w)).map(keyword),
  },

  // Each hidden rule that an expression can start with is a column of each
  // state where an expression starts, so these rules are inlined. The rules
  // _name and _table_factor are inlined for the same reason, for the states
  // where a name or a table starts.
  inline: $ => [
    $._name,
    $._literal,
    $._primary_expression,
    $._table_factor,
  ],

  rules: {
    source_file: $ => seq(
      repeat(choice(seq($._statement, ';'), ';')),
      optional($._statement),
    ),

    _statement: $ => choice(
      $.select_statement,
    ),

    // Queries

    select_statement: $ => $._query,

    // ORDER BY and LIMIT come after the body of a query, in any order.
    _query: $ => prec.right(seq(
      optional($.with_clause),
      $._query_body,
      repeat(choice($.order_by_clause, $.limit_clause)),
    )),

    // A common table expression can name itself after RECURSIVE.
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
      $.subquery,
    ),

    column_list: $ => parens(commaSep1($._name)),

    _query_body: $ => choice(
      $.select,
      $.values,
      $.set_operation,
    ),

    // The compound operators have one precedence, and they bind to the
    // left. A part of a compound query has no ORDER BY and no LIMIT of its
    // own, and it is not in parentheses.
    set_operation: $ => prec.left(seq(
      field('left', $._query_body),
      field('operator', choice(
        seq(kw('UNION'), optional(kw('ALL'))),
        kw('INTERSECT'),
        kw('EXCEPT'),
      )),
      field('right', choice($.select, $.values)),
    )),

    // The clauses after FROM come in any order.
    select: $ => prec.right(seq(
      kw('SELECT'),
      optional(choice(kw('DISTINCT'), kw('ALL'))),
      commaSep1($._select_item),
      optional($.from_clause),
      repeat(choice(
        $.where_clause,
        $.group_by_clause,
        $.having_clause,
        $.window_clause,
      )),
    )),

    // VALUES gives rows, in place of a SELECT.
    values: $ => seq(kw('VALUES'), commaSep1($.value_list)),

    value_list: $ => parens(commaSep1($._expression)),

    _select_item: $ => choice(
      $._expression,
      $.aliased_expression,
      $.all_columns,
    ),

    // An alias with AS is a name or a string. An alias with no AS is an
    // identifier or a string.
    aliased_expression: $ => seq(
      field('value', $._expression),
      optional(kw('AS')),
      field('alias', choice($._name, $.string)),
    ),

    all_columns: $ => seq(
      optional(seq(field('table', $._name), '.')),
      '*',
    ),

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
      $.parenthesized_join,
    ),

    // A table, with its alias and its index: INDEXED BY names the index
    // that SQLite must use, and NOT INDEXED uses none.
    table_reference: $ => prec.right(seq(
      field('table', $.object_reference),
      optional($._alias),
      optional(choice(
        seq(kws('INDEXED', 'BY'), field('index', $._name)),
        kws('NOT', 'INDEXED'),
      )),
    )),

    // An alias of a table, a subquery or a table-valued function.
    _alias: $ => seq(optional(kw('AS')), field('alias', choice($._name, $.string))),

    // A table-valued function, such as json_each(x) or generate_series(1,
    // 10).
    function_table: $ => prec.right(seq(
      field('function', alias($._table_function, $.function_call)),
      optional($._alias),
    )),

    _table_function: $ => seq(
      optional(seq(field('schema', $._name), '.')),
      field('name', $._name),
      $.arguments,
    ),

    derived_table: $ => prec.right(seq(
      $.subquery,
      optional($._alias),
    )),

    // A list of tables and joins in parentheses.
    parenthesized_join: $ => prec.right(seq(
      parens(commaSep1($._table_reference)),
      optional($._alias),
    )),

    // The words of a join before JOIN, such as NATURAL LEFT OUTER JOIN, are
    // one token with JOIN, so that they take one column of a parse state.
    // It is the keyword JOIN in the tree. SQLite does not reserve these
    // words, and the token takes them only before JOIN, so they can name a
    // column too.
    join: $ => prec.left(seq(
      field('left', $._table_reference),
      choice(kw('JOIN'), alias($._join, 'JOIN')),
      field('right', $._table_factor),
      optional(choice(
        seq(kw('ON'), field('condition', $._expression)),
        seq(kw('USING'), $.column_list),
      )),
    )),

    _join: _ => token(seq(
      optional(seq(keyword('NATURAL'), /\s+/)),
      choice(
        keyword('INNER'),
        keyword('CROSS'),
        seq(
          choice(keyword('LEFT'), keyword('RIGHT'), keyword('FULL')),
          optional(seq(/\s+/, keyword('OUTER'))),
        ),
      ),
      /\s+/,
      keyword('JOIN'),
    )),

    where_clause: $ => seq(kw('WHERE'), field('condition', $._expression)),

    group_by_clause: $ => seq(kws('GROUP', 'BY'), $._expressions),

    // SQLite takes HAVING with no GROUP BY.
    having_clause: $ => seq(kw('HAVING'), field('condition', $._expression)),

    window_clause: $ => seq(kw('WINDOW'), commaSep1($.window_definition)),

    window_definition: $ => seq(
      field('name', $._name),
      kw('AS'),
      $.window_specification,
    ),

    // A window can start with the name of a window of the WINDOW clause.
    // The grammar takes its clauses in any order.
    window_specification: $ => parens(seq(
      optional(field('name', $._name)),
      repeat(choice(
        seq(kws('PARTITION', 'BY'), $._expressions),
        $.order_by_clause,
        $.frame_clause,
      )),
    )),

    frame_clause: $ => seq(
      choice(kw('RANGE'), kw('ROWS'), kw('GROUPS')),
      choice(
        $.frame_bound,
        seq(kw('BETWEEN'), $.frame_bound, kw('AND'), $.frame_bound),
      ),
      optional(seq(
        kw('EXCLUDE'),
        choice(
          kws('NO', 'OTHERS'),
          kws('CURRENT', 'ROW'),
          kw('GROUP'),
          kw('TIES'),
        ),
      )),
    ),

    frame_bound: $ => choice(
      kws('UNBOUNDED', 'PRECEDING'),
      kws('UNBOUNDED', 'FOLLOWING'),
      kws('CURRENT', 'ROW'),
      seq($._expression, choice(kw('PRECEDING'), kw('FOLLOWING'))),
    ),

    order_by_clause: $ => prec.right(seq(
      kws('ORDER', 'BY'),
      commaSep1($.order_by_item),
    )),

    // A COLLATE of an item is a part of its expression.
    order_by_item: $ => seq(
      field('value', $._expression),
      optional(choice(kw('ASC'), kw('DESC'))),
      optional(seq(kw('NULLS'), choice(kw('FIRST'), kw('LAST')))),
    ),

    // LIMIT count OFFSET offset, or LIMIT offset, count.
    limit_clause: $ => seq(
      kw('LIMIT'),
      field('count', $._expression),
      optional(choice(
        seq(kw('OFFSET'), field('offset', $._expression)),
        seq(',', field('count', $._expression)),
      )),
    ),

    subquery: $ => parens($._query),

    // Expressions

    _expression: $ => choice(
      $._primary_expression,
      $.unary_expression,
      $.binary_expression,
      $.is_expression,
      $.isnull_expression,
      $.between_expression,
      $.in_expression,
      $.like_expression,
      $.collate_expression,
    ),

    _primary_expression: $ => choice(
      $._literal,
      $.parameter,
      $.column_reference,
      $.function_call,
      $.parenthesized_expression,
      $.list,
      $.subquery,
      $.exists_expression,
      $.case_expression,
      $.cast_expression,
      $.raise_expression,
    ),

    _literal: $ => choice(
      $.number,
      $.string,
      $.blob,
      $.null,
      $.boolean,
      $.niladic_function,
    ),

    null: _ => kw('NULL'),

    // SQLite does not reserve TRUE and FALSE. They are one token, so that
    // they take one column of each state where an expression starts.
    boolean: _ => token(choice(keyword('TRUE'), keyword('FALSE'))),

    // CURRENT_DATE, CURRENT_TIME and CURRENT_TIMESTAMP give the date and the
    // time. SQLite does not reserve them, and they are one token.
    niladic_function: _ => token(choice(
      keyword('CURRENT_DATE'),
      keyword('CURRENT_TIME'),
      keyword('CURRENT_TIMESTAMP'),
    )),

    // A column, with its table and its schema.
    column_reference: $ => choice(
      field('name', $._name),
      seq(field('table', $._name), '.', field('name', $._name)),
      seq(
        field('schema', $._name),
        '.',
        field('table', $._name),
        '.',
        field('name', $._name),
      ),
    ),

    // A function call. The arguments of an aggregate function can have
    // DISTINCT and ORDER BY. FILTER and OVER come after the arguments.
    function_call: $ => prec.right(seq(
      field('name', $._name),
      $.arguments,
      optional($.filter_clause),
      optional($.over_clause),
    )),

    arguments: $ => parens(optional(choice(
      '*',
      seq(
        optional(choice(kw('DISTINCT'), kw('ALL'))),
        $._expressions,
        optional($.order_by_clause),
      ),
    ))),

    filter_clause: $ => seq(
      kw('FILTER'),
      parens(seq(kw('WHERE'), field('condition', $._expression))),
    ),

    over_clause: $ => seq(
      kw('OVER'),
      choice(field('window', $._name), $.window_specification),
    ),

    parenthesized_expression: $ => parens($._expression),

    // A row value: a list of two or more expressions, as in (a, b) = (1, 2)
    // or a IN (1, 2, 3).
    list: $ => parens(seq($._expression, ',', commaSep1($._expression))),

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

    // SQLite takes CAST(x AS) with no type, which gives the type BLOB.
    cast_expression: $ => seq(
      kw('CAST'),
      '(',
      field('value', $._expression),
      kw('AS'),
      optional(field('type', $.type)),
      ')',
    ),

    // RAISE stops a trigger. It takes IGNORE, or ROLLBACK, ABORT or FAIL and
    // a message, which is an expression.
    raise_expression: $ => seq(
      kw('RAISE'),
      parens(choice(
        kw('IGNORE'),
        seq(
          choice(kw('ROLLBACK'), kw('ABORT'), kw('FAIL')),
          ',',
          field('message', $._expression),
        ),
      )),
    ),

    // Each level of precedence takes its operators from one token, so that
    // it has one production.
    unary_expression: $ => choice(
      prec.right(PREC.not, seq(field('operator', kw('NOT')), field('operand', $._expression))),
      prec.right(PREC.unary, seq(field('operator', alias($._unary_operator, $.operator)), field('operand', $._expression))),
    ),

    _unary_operator: _ => token(choice('-', '+', '~')),

    binary_expression: $ => {
      /** @type {[number, RuleOrLiteral][]} */
      const table = [
        [PREC.or, kw('OR')],
        [PREC.and, kw('AND')],
        [PREC.equal, alias($._equal_operator, $.operator)],
        [PREC.compare, alias($._compare_operator, $.operator)],
        [PREC.bit, alias($._bit_operator, $.operator)],
        [PREC.add, alias($._add_operator, $.operator)],
        [PREC.multiply, alias($._multiply_operator, $.operator)],
        [PREC.concat, alias($._concat_operator, $.operator)],
      ];
      return choice(...table.map(([p, op]) => prec.left(p, seq(
        field('left', $._expression),
        field('operator', op),
        field('right', $._expression),
      ))));
    },

    _equal_operator: _ => token(choice('=', '==', '!=', '<>')),

    _compare_operator: _ => token(choice('<', '>', '<=', '>=')),

    _bit_operator: _ => token(choice('&', '|', '<<', '>>')),

    _add_operator: _ => token(choice('+', '-')),

    _multiply_operator: _ => token(choice('*', '/', '%')),

    // || joins two strings, and -> and ->> extract a part of a JSON value.
    _concat_operator: _ => token(choice('||', '->', '->>')),

    // a IS b, a IS NOT b, a IS DISTINCT FROM b and a IS NOT DISTINCT FROM
    // b compare two values, and NULL is equal to NULL.
    is_expression: $ => prec.left(PREC.equal, seq(
      field('left', $._expression),
      $._is_operator,
      field('right', $._expression),
    )),

    _is_operator: _ => prec.right(seq(
      kw('IS'),
      optional(kw('NOT')),
      optional(kws('DISTINCT', 'FROM')),
    )),

    // ISNULL, NOTNULL and NOT NULL after an expression.
    isnull_expression: $ => prec.left(PREC.equal, seq(
      field('value', $._expression),
      field('operator', choice(kw('ISNULL'), kw('NOTNULL'), kws('NOT', 'NULL'))),
    )),

    between_expression: $ => prec.left(PREC.equal, seq(
      field('left', $._expression),
      $._between_operator,
      field('low', $._expression),
      kw('AND'),
      field('high', $._expression),
    )),

    _between_operator: _ => seq(optional(kw('NOT')), kw('BETWEEN')),

    // The right side of IN is a subquery, a list, which can be empty, a
    // table or a table-valued function.
    in_expression: $ => prec.left(PREC.equal, seq(
      field('left', $._expression),
      $._in_operator,
      field('right', choice(
        $.subquery,
        $.list,
        $.parenthesized_expression,
        alias($._empty_list, $.list),
        $.object_reference,
        alias($._table_function, $.function_call),
      )),
    )),

    _in_operator: _ => seq(optional(kw('NOT')), kw('IN')),

    _empty_list: _ => seq('(', ')'),

    // LIKE, GLOB, REGEXP and MATCH, with an optional ESCAPE.
    like_expression: $ => prec.left(PREC.equal, seq(
      field('left', $._expression),
      field('operator', $._like_operator),
      field('right', $._expression),
      optional(seq(kw('ESCAPE'), field('escape', $._expression))),
    )),

    _like_operator: _ => seq(
      optional(kw('NOT')),
      choice(kw('LIKE'), kw('GLOB'), kw('REGEXP'), kw('MATCH')),
    ),

    collate_expression: $ => prec.left(PREC.collate, seq(
      field('value', $._expression),
      kw('COLLATE'),
      field('collation', choice($._name, $.string)),
    )),

    _expressions: $ => commaSep1($._expression),

    // Types

    // A type is one or more names, with a size, as in VARCHAR(255),
    // DECIMAL(10, 5) or UNSIGNED BIG INT. SQLite takes any names as a type,
    // and the names decide the affinity of a column.
    type: $ => prec.right(seq(
      repeat1(field('name', $._name)),
      optional(parens(commaSep1($._signed_number))),
    )),

    _signed_number: $ => seq(optional(alias($._unary_operator, $.operator)), $.number),

    // Names and literals

    // A name of a table, with its schema.
    object_reference: $ => seq(
      optional(seq(field('schema', $._name), '.')),
      field('name', $._name),
    ),

    // A name is an identifier or a quoted identifier, and both are the node
    // identifier.
    _name: $ => choice(
      $.identifier,
      alias($._quoted_identifier, $.identifier),
    ),

    // An identifier starts with a letter, _ or a character above 0x7f, and
    // $ can come after its first character. A name that starts with _ is
    // also the placeholder of a variable of usql (D101).
    identifier: _ => /[A-Za-z_\u0080-\u{10FFFF}][A-Za-z0-9_$\u0080-\u{10FFFF}]*/u,

    // A quoted identifier is in double quotes, with "" for a ", in
    // brackets, or in backquotes, with `` for a `.
    _quoted_identifier: _ => token(choice(
      /"([^"]|"")*"/,
      /\[[^\]]*\]/,
      /`([^`]|``)*`/,
    )),

    // A parameter: ?, ?NNN, or a name after :, @ or $. A name after $ can
    // hold :: and end with a suffix in parentheses, as a name of Tcl does.
    parameter: _ => token(choice(
      /\?[0-9]*/,
      /[:@][A-Za-z0-9_$\u0080-\u{10FFFF}]+/u,
      /\$[A-Za-z0-9_$\u0080-\u{10FFFF}]+(::+[A-Za-z0-9_$\u0080-\u{10FFFF}]+)*(\([^)\s]*\))?/u,
    )),

    // A number is an integer, a decimal number or a float with an exponent,
    // or a hexadecimal integer in the form 0x1f. One _ can come between two
    // digits.
    number: _ => token(choice(
      /[0-9](_?[0-9])*(\.([0-9](_?[0-9])*)?)?([eE][+-]?[0-9](_?[0-9])*)?/,
      /\.[0-9](_?[0-9])*([eE][+-]?[0-9](_?[0-9])*)?/,
      /0[xX][0-9A-Fa-f](_?[0-9A-Fa-f])*/,
    )),

    // A string is in single quotes, with a quote doubled.
    string: _ => /'([^']|'')*'/,

    // A BLOB is hexadecimal data in quotes after X.
    blob: _ => /[xX]'[0-9A-Fa-f]*'/,

    // A comment starts with -- and runs to the end of the line, or it is
    // between /* and */. A comment of the second form does not nest.
    comment: _ => token(choice(
      seq('--', /[^\n]*/),
      seq('/*', /[^*]*\*+([^/*][^*]*\*+)*/, '/'),
    )),
  },
});
