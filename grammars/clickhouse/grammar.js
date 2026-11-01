/**
 * @file The grammar of the SQL of ClickHouse
 * @license MIT
 */

// xo writes this grammar from the SQL reference of the current release of
// ClickHouse, at https://clickhouse.com/docs/sql-reference (D127). It does
// not copy the parser of ClickHouse.
//
// The grammar is for a highlighter and for completion, so it accepts more
// than ClickHouse does where that keeps the rules small. A keyword is a
// token in any case, and it is an anonymous node with the name of the
// keyword in upper case, such as "SELECT".
//
// The token identifier is the word of the grammar, so a keyword is an
// identifier in each place where the grammar does not expect the keyword.
// ClickHouse reserves almost no words. The reserved set global holds the
// words that start a clause of a query, which ClickHouse does not take as
// an alias with no AS. A word after AS or after a dot can be any keyword,
// and a function name can be a word such as and, or, in or like, because
// ClickHouse has functions with these names.
//
// The grammar keeps its parse tables small (D122, D127). A parse state with
// more than 64 columns takes the large form, so the grammar keeps the
// columns of a state few. The operators of one level of precedence are one
// token, TRUE and FALSE are one token, inf and nan are one token, and NULL
// is a token of its own. The clauses of a query after SELECT, the clauses
// after the arguments of a function and the clauses of a window come in
// any order, because a sequence of optional clauses costs parse states for
// each set of the clauses. The words before JOIN, such as GLOBAL ANY LEFT,
// are keywords in any order. One token for them took AS, IN, INNER and
// GLOBAL out of the keywords of the word rule, and it cost 908 more parse
// states.
//
// ClickHouse gives ORDER BY, LIMIT, SETTINGS and FORMAT to the SELECT
// before them, and not to a whole set operation, so they are clauses of
// select.
//
// The external scanner in src/scanner.c reads the comments and the heredoc
// strings. A comment is -- or // to the end of the line, # followed by a
// space or by !, to the end of the line, or /* ... */, which can hold other
// block comments. A statement ends at a semicolon or at the end of the
// input. The variables of usql belong to the usql grammar, which hands this
// grammar a placeholder of the same length in place of each variable (D101).

/// <reference types="tree-sitter-cli/dsl" />
// @ts-check

// The precedences of the operators, from the lowest to the highest, as the
// parser of ClickHouse orders them. An alias with AS binds the most
// loosely, and then the arrow of a lambda and the ternary operator ?:. NOT
// binds more loosely than IS NULL, IS NULL more loosely than BETWEEN, and
// BETWEEN more loosely than a comparison, LIKE and IN. || binds more loosely
// than + and -. A sign binds more tightly than * and /, and a cast, a
// subscript and a dot bind the most tightly.
const PREC = {
  alias: 1,
  lambda: 2,
  ternary: 3,
  or: 4,
  and: 5,
  not: 6,
  is: 7,
  between: 8,
  compare: 9,
  concat: 10,
  add: 11,
  multiply: 12,
  unary: 13,
  postfix: 14,
  field: 15,
};

// The words that start a clause of a query. ClickHouse does not take them
// as an alias with no AS, so they are not names in the grammar. SETTINGS
// is not in the list, because it names a column of system.query_log. A
// word is in the list only when the grammar uses it as a keyword, because
// a reserved word must be a token of the grammar.
const RESERVED_WORDS = [
  'AS', 'EXCEPT', 'FROM', 'GROUP', 'HAVING', 'INTERSECT', 'JOIN', 'LIMIT',
  'ON', 'ORDER', 'PREWHERE', 'QUALIFY', 'SELECT', 'UNION', 'USING', 'WHERE',
  'WINDOW',
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
 * Returns one or more of rule, with a comma between two of them and an
 * optional comma after the last one, as ClickHouse allows in some lists.
 *
 * @param {RuleOrLiteral} rule
 */
function trailingSep1(rule) {
  return seq(rule, repeat(seq(',', rule)), optional(','));
}

/**
 * Returns the optional table before * or COLUMNS, with its database, as in
 * t.* or db.t.COLUMNS('a'). The parts are sequences and not a rule of their
 * own, so that they share their parse states with a column reference.
 *
 * @param {GrammarSymbols<string>} $
 */
function starTable($) {
  return optional(choice(
    seq(field('table', choice($._name, $.query_parameter)), '.'),
    seq(field('database', choice($._name, $.query_parameter)), '.', field('table', $._label), '.'),
  ));
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
  name: 'clickhouse',

  // ClickHouse also skips the white space of Unicode, such as U+3000. The
  // grammar does not, because an identifier can hold such a character,
  // and white space of Unicode in extras cost more than 1,000 parse
  // states.
  extras: $ => [
    /\s/,
    $.comment,
  ],

  // The order of the external tokens is the order of enum TokenType in
  // src/scanner.c.
  externals: $ => [
    $.comment,
    $._heredoc_start,
    $.string_content,
    $._heredoc_end,
  ],

  word: $ => $.identifier,

  // The reserved set global holds the words that start a clause of a
  // query. The set function_name and the set label are empty, because
  // ClickHouse accepts every keyword as a function name and as a label
  // after AS and after a dot. A state takes the last set of its items, so
  // the sets go from the strictest to the least strict.
  reserved: {
    global: _ => RESERVED_WORDS.filter(w => KEYWORDS.has(w)).map(keyword),
    function_name: _ => [],
    label: _ => [],
  },

  // Each hidden rule that an expression can start with is a column of each
  // state where an expression starts, so these rules are inlined. The rules
  // _name, _label and _table_factor are inlined for the same reason, for
  // the states where a name or a table starts.
  inline: $ => [
    $._name,
    $._label,
    $._literal,
    $._function_name,
    $._primary_expression,
    $._operand,
    $._table_factor,
    $._member,
  ],

  conflicts: $ => [
    // A parenthesized query can be the left side of a set operation, or a
    // subquery in an expression or in FROM.
    [$.parenthesized_query, $.subquery],
  ],

  rules: {
    source_file: $ => seq(
      repeat(choice(seq($._statement, ';'), ';')),
      optional($._statement),
    ),

    _statement: $ => choice(
      $.select_statement,
      $.explain_statement,
    ),

    // Queries

    select_statement: $ => $._query,

    _query: $ => $._query_body,

    // EXPLAIN shows the syntax tree, the plan or the pipeline of a query,
    // with its kind and settings, such as EXPLAIN PLAN actions = 1.
    explain_statement: $ => seq(
      kw('EXPLAIN'),
      choice(
        seq(
          optional(choice(
            kw('AST'),
            kw('SYNTAX'),
            kws('QUERY', 'TREE'),
            kw('PLAN'),
            kw('PIPELINE'),
            kw('ESTIMATE'),
            kw('ANALYZE'),
            kws('TABLE', 'OVERRIDE'),
          )),
          optional(commaSep1(alias($._explain_setting, $.setting))),
          $._query,
        ),
        kws('CURRENT', 'TRANSACTION'),
      ),
    ),

    // The value of a setting of EXPLAIN is a literal or a name, because a
    // query in parentheses can come after it.
    _explain_setting: $ => seq(
      field('name', $._label),
      '=',
      field('value', choice($._literal, $._name)),
    ),

    // WITH belongs to the SELECT after it, so a SELECT after UNION can
    // have a WITH of its own. WITH gives a name to a query, as in WITH t AS
    // (SELECT 1), or to an expression, as in WITH 1 AS x or WITH (SELECT
    // max(a) FROM t) AS m. A comma can come after the last one.
    with_clause: $ => seq(
      kw('WITH'),
      optional(kw('RECURSIVE')),
      trailingSep1(choice(
        $.cte,
        $._expression,
        alias($._with_column_alias, $.aliased_expression),
      )),
    ),

    // After WITH x AS, the parser reads x as the name of a common table
    // expression, and a name after AS makes x a column with an alias.
    cte: $ => prec(2, seq(
      $._column_name,
      kw('AS'),
      optional(kw('MATERIALIZED')),
      $.subquery,
    )),

    _with_column_alias: $ => prec(2, seq(
      field('value', alias($._column_name, $.column_reference)),
      kw('AS'),
      field('alias', $._label),
    )),

    column_list: $ => parens(commaSep1($._name)),

    _query_body: $ => choice(
      $.select,
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

    // A query can start with FROM, and then SELECT comes after it. The
    // clauses after SELECT come in any order.
    select: $ => prec.right(seq(
      optional($.with_clause),
      choice(
        $._select_clause,
        seq($.from_clause, $._select_clause),
      ),
      repeat(choice(
        $.from_clause,
        $.prewhere_clause,
        $.where_clause,
        $.group_by_clause,
        $.group_by_modifier,
        $.having_clause,
        $.window_clause,
        $.qualify_clause,
        $.order_by_clause,
        $.interpolate_clause,
        $.limit_clause,
        $.offset_clause,
        $.fetch_clause,
        $.settings_clause,
        $.into_outfile_clause,
        $.format_clause,
      )),
    )),

    // SELECT, with DISTINCT ON and TOP.
    _select_clause: $ => prec.right(seq(
      kw('SELECT'),
      optional(choice(kw('ALL'), $.distinct_clause)),
      optional($.top_clause),
      commaSep1($._select_item),
    )),

    distinct_clause: $ => prec.right(seq(
      kw('DISTINCT'),
      optional(seq(kw('ON'), parens(commaSep1($._expression)))),
    )),

    // TOP and its count are one token, because ClickHouse takes TOP as a
    // keyword only before a number, and top also names a column.
    top_clause: $ => prec.right(seq(
      alias($._top, 'TOP'),
      optional(kws('WITH', 'TIES')),
    )),

    _top: _ => token(seq(keyword('TOP'), /\s+/, /[0-9]+/)),

    _select_item: $ => choice(
      $._expression,
      alias($._implicit_alias, $.aliased_expression),
      $.star_expression,
    ),

    // An alias with AS can come after any expression, as in (1 AS x) + x
    // or [f(y) AS z, z]. It binds the most loosely. An alias with no AS
    // comes only after a column of SELECT and of ARRAY JOIN.
    aliased_expression: $ => prec.right(PREC.alias, seq(
      field('value', choice($._expression, $.star_expression)),
      kw('AS'),
      field('alias', $._label),
    )),

    _implicit_alias: $ => seq(field('value', $._expression), field('alias', $._name)),

    // * or t.* gives all the columns of a table, and COLUMNS gives the
    // columns whose names match a regular expression or the columns of a
    // list. The transformers after them change the columns: EXCEPT leaves
    // out columns, REPLACE replaces them with expressions, and APPLY calls
    // a function on each of them, with a prefix of the names of the
    // columns of its result after the function. The replacements of
    // REPLACE are expressions with an alias, as in REPLACE x + 1 AS x, or
    // such expressions in parentheses, which are one expression or a
    // tuple.
    star_expression: $ => prec.right(choice(
      seq(starTable($), '*', repeat($._column_transformer)),
      prec.right(1, seq($.columns_expression, repeat1($._column_transformer))),
    )),

    // COLUMNS and the opening parenthesis are one token, so that columns
    // stays the name of a column. COLUMNS is also an expression, as in
    // COLUMNS('a') + 1.
    columns_expression: $ => seq(
      starTable($),
      alias($._columns, 'COLUMNS'),
      optional(commaSep1(choice($.string, $.column_reference))),
      ')',
    ),

    _columns: _ => token(seq(keyword('COLUMNS'), /\s*/, '(')),

    _column_transformer: $ => choice(
      seq(kw('EXCEPT'), optional(kw('STRICT')), choice(
        parens(commaSep1(choice($._name, $.string))),
        $._name,
        $.string,
      )),
      seq(kw('REPLACE'), optional(kw('STRICT')), field('replace', $._expression)),
      seq(kw('APPLY'), choice(
        parens(seq(
          field('function', choice($._function_name, $.lambda_expression, $.function_call)),
          optional(seq(',', field('prefix', $.string))),
        )),
        field('function', $._function_name),
      )),
    ),

    // ClickHouse takes a comma after the last column of SELECT, before
    // FROM. The grammar does not, because the comma cost more than 400
    // parse states.
    from_clause: $ => seq(kw('FROM'), commaSep1($._table_reference)),

    _table_reference: $ => choice(
      $._table_factor,
      $.join,
      $.array_join,
    ),

    // The right side of a join is a table factor: a join on the right side
    // must be in parentheses.
    _table_factor: $ => choice(
      $.table_reference,
      $.function_table,
      $.derived_table,
      $.parenthesized_join,
    ),

    // A table, with an alias. FINAL merges the rows of a table of the
    // MergeTree family before the query reads them, and SAMPLE reads a
    // sample of the rows.
    table_reference: $ => prec.right(seq(
      field('table', $.object_reference),
      optional($._alias),
      optional(kw('FINAL')),
      optional($.sample_clause),
    )),

    // An alias of a table, with the names of its columns.
    _alias: $ => seq(
      choice(
        seq(kw('AS'), field('alias', $._label)),
        field('alias', $._name),
      ),
      optional($.column_list),
    ),

    // SAMPLE takes a part of the rows, such as 0.1 or 1/10, or a number of
    // rows, and OFFSET skips a part of them.
    sample_clause: $ => prec.right(seq(
      kw('SAMPLE'),
      field('size', $._expression),
      optional(seq(kw('OFFSET'), field('offset', $._expression))),
    )),

    // A table function, such as numbers(10) or remote('host', db, t).
    function_table: $ => prec.right(seq(
      field('function', alias($._table_function, $.function_call)),
      optional($._alias),
      optional(kw('FINAL')),
      optional($.sample_clause),
    )),

    _table_function: $ => seq(
      field('name', $._function_name),
      $.arguments,
    ),

    // A subquery in FROM. EXPLAIN in parentheses is a subquery too, which
    // gives the lines of the plan as rows. The grammar takes it only in
    // FROM, because EXPLAIN after each ( where an expression starts puts
    // 12 more parse states in the large form.
    derived_table: $ => prec.right(seq(
      choice($.subquery, alias(parens($.explain_statement), $.subquery)),
      optional($._alias),
    )),

    // A list of tables and joins in parentheses.
    parenthesized_join: $ => prec.right(seq(
      parens(commaSep1($._table_reference)),
      optional($._alias),
    )),

    // A join. The words before JOIN give its kind, such as GLOBAL ANY LEFT
    // OUTER JOIN. GLOBAL sends the right side to each server of a
    // distributed query, ANY, ALL, ASOF, SEMI and ANTI give how the rows
    // match, and NATURAL joins on the columns with the same names. The
    // grammar takes the words in any order. ON or USING comes after the
    // right side, but for CROSS JOIN, PASTE JOIN and a join of each row
    // with each row. The columns of USING can come with no parentheses.
    join: $ => prec.right(seq(
      field('left', $._table_reference),
      optional($._join_kind),
      kw('JOIN'),
      field('right', $._table_factor),
      optional(choice(
        seq(kw('ON'), field('condition', $._expression)),
        seq(kw('USING'), choice($.column_list, commaSep1($._name))),
      )),
    )),

    _join_kind: _ => repeat1(choice(...[
      'GLOBAL', 'LOCAL', 'NATURAL', 'ANY', 'ALL', 'ASOF', 'SEMI', 'ANTI', 'INNER',
      'LEFT', 'RIGHT', 'FULL', 'CROSS', 'PASTE', 'OUTER',
    ].map(kw))),

    // ARRAY JOIN gives a row for each element of an array, and LEFT ARRAY
    // JOIN keeps a row with an empty array too.
    array_join: $ => prec.right(seq(
      field('left', $._table_reference),
      optional(choice(kw('LEFT'), kw('INNER'))),
      kws('ARRAY', 'JOIN'),
      commaSep1(choice($._expression, alias($._implicit_alias, $.aliased_expression))),
    )),

    // PREWHERE filters the rows before the query reads the other columns.
    prewhere_clause: $ => seq(kw('PREWHERE'), field('condition', $._expression)),

    where_clause: $ => seq(kw('WHERE'), field('condition', $._expression)),

    // GROUP BY ALL groups by each column of SELECT that is not an
    // aggregate.
    group_by_clause: $ => seq(
      kws('GROUP', 'BY'),
      choice(
        kw('ALL'),
        commaSep1($._grouping_element),
      ),
    ),

    _grouping_element: $ => choice(
      $._expression,
      $.rollup,
      $.cube,
      $.grouping_sets,
    ),

    rollup: $ => seq(kw('ROLLUP'), parens(commaSep1($._expression))),

    cube: $ => seq(kw('CUBE'), parens(commaSep1($._expression))),

    grouping_sets: $ => seq(
      kws('GROUPING', 'SETS'),
      parens(commaSep1($._expression)),
    ),

    // WITH ROLLUP and WITH CUBE add the subtotals, and WITH TOTALS adds a
    // row with the totals.
    group_by_modifier: _ => seq(
      kw('WITH'),
      choice(kw('ROLLUP'), kw('CUBE'), kw('TOTALS')),
    ),

    having_clause: $ => seq(kw('HAVING'), field('condition', $._expression)),

    // QUALIFY filters the rows after the window functions.
    qualify_clause: $ => seq(kw('QUALIFY'), field('condition', $._expression)),

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
        seq(kws('PARTITION', 'BY'), commaSep1($._expression)),
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
    ),

    frame_bound: $ => choice(
      kws('UNBOUNDED', 'PRECEDING'),
      kws('UNBOUNDED', 'FOLLOWING'),
      kws('CURRENT', 'ROW'),
      seq($._expression, choice(kw('PRECEDING'), kw('FOLLOWING'))),
    ),

    // ORDER BY ALL orders by each column of SELECT.
    order_by_clause: $ => seq(
      kws('ORDER', 'BY'),
      choice(
        seq(kw('ALL'), optional($._direction), optional($._nulls)),
        commaSep1($.order_by_item),
      ),
    ),

    // An item of ORDER BY, with COLLATE and WITH FILL, which adds the rows
    // that the values between two rows would have, with FROM, TO, STEP and
    // STALENESS.
    order_by_item: $ => prec.right(seq(
      field('value', $._expression),
      optional($._direction),
      optional($._nulls),
      optional(seq(kw('COLLATE'), field('collation', $.string))),
      optional($.with_fill),
    )),

    _direction: _ => choice(kw('ASC'), kw('DESC'), kw('ASCENDING'), kw('DESCENDING')),

    _nulls: _ => seq(kw('NULLS'), choice(kw('FIRST'), kw('LAST'))),

    with_fill: $ => prec.right(seq(
      kws('WITH', 'FILL'),
      repeat(seq(
        choice(kw('FROM'), kw('TO'), kw('STEP'), kw('STALENESS')),
        $._expression,
      )),
    )),

    // INTERPOLATE gives the values of the columns in the rows that WITH
    // FILL adds.
    interpolate_clause: $ => prec.right(seq(
      kw('INTERPOLATE'),
      optional(parens(optional(commaSep1($.interpolate_item)))),
    )),

    interpolate_item: $ => seq(
      field('column', $._name),
      optional(seq(kw('AS'), field('value', $._expression))),
    ),

    // LIMIT takes a count, or an offset and a count with a comma or OFFSET
    // between them. LIMIT BY takes that many rows for each value of its
    // expressions, and WITH TIES adds the rows that tie with the last one.
    limit_clause: $ => prec.right(seq(
      kw('LIMIT'),
      field('count', $._expression),
      optional(seq(choice(',', kw('OFFSET')), field('count', $._expression))),
      optional(choice(
        seq(kw('BY'), commaSep1($._expression)),
        kws('WITH', 'TIES'),
      )),
    )),

    offset_clause: $ => prec.right(seq(
      kw('OFFSET'),
      field('start', $._expression),
      optional(choice(kw('ROW'), kw('ROWS'))),
    )),

    fetch_clause: $ => seq(
      kw('FETCH'),
      choice(kw('FIRST'), kw('NEXT')),
      optional(field('count', $._expression)),
      choice(kw('ROW'), kw('ROWS')),
      choice(kw('ONLY'), kws('WITH', 'TIES')),
    ),

    // SETTINGS changes settings for one query.
    settings_clause: $ => seq(kw('SETTINGS'), commaSep1($.setting)),

    // A setting with no value is on, as in SETTINGS log_queries.
    setting: $ => prec.right(seq(
      field('name', $._label),
      optional(seq('=', field('value', $._expression))),
    )),

    // INTO OUTFILE writes the result into a file on the client.
    into_outfile_clause: $ => prec.right(seq(
      kws('INTO', 'OUTFILE'),
      field('file', $.string),
      repeat(choice(
        kws('AND', 'STDOUT'),
        kw('APPEND'),
        kw('TRUNCATE'),
        seq(kw('COMPRESSION'), field('compression', $.string), optional(seq(kw('LEVEL'), $.number))),
      )),
    )),

    // FORMAT names the format of the result, such as JSONEachRow.
    format_clause: $ => seq(kw('FORMAT'), field('format', $._label)),

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
      $.ternary_expression,
      $.lambda_expression,
      $.aliased_expression,
    ),

    _primary_expression: $ => choice(
      $._literal,
      $.typed_literal,
      $.interval_expression,
      $.column_reference,
      $.query_parameter,
      $.columns_expression,
      $.function_call,
      $.cast_expression,
      $.subscript_expression,
      $.field_selection,
      $.parenthesized_expression,
      $.tuple,
      $.array,
      $.map,
      $.subquery,
      $.exists_expression,
      $.case_expression,
    ),

    _literal: $ => choice(
      $.number,
      $.string,
      $.heredoc_string,
      $.hex_string,
      $.bit_string,
      $.boolean,
      $.null,
      alias($._inf_nan, $.number),
    ),

    // TRUE and FALSE are one token, so that they take one column of each
    // state where an expression starts.
    boolean: _ => token(choice(keyword('TRUE'), keyword('FALSE'))),

    // NULL is a token of its own, so that it takes one column of each state
    // where an expression starts.
    null: _ => (KEYWORDS.add('NULL'), keyword('NULL')),

    // inf, infinity and nan are numbers, and one token.
    _inf_nan: _ => token(choice(keyword('INF'), keyword('INFINITY'), keyword('NAN'))),

    // A type name before a string gives a constant of the type, such as
    // DATE '2024-01-01'. The name is an identifier, so date also names a
    // column.
    typed_literal: $ => seq(
      field('type', alias($.identifier, $.type)),
      field('value', $.string),
    ),

    // An interval: INTERVAL with a value and a unit, as in INTERVAL 3 DAY,
    // with a string that holds both, as in INTERVAL '1 day', or with two
    // units, as in INTERVAL '1:30' HOUR TO MINUTE.
    interval_expression: $ => prec.right(seq(
      kw('INTERVAL'),
      field('value', choice(
        $.string,
        $.number,
        $.parenthesized_expression,
        $.column_reference,
        $.function_call,
        $.query_parameter,
        alias($._negative_number, $.unary_expression),
      )),
      optional(seq($.interval_unit, optional(seq(kw('TO'), $.interval_unit)))),
    )),

    _negative_number: $ => seq(field('operator', alias('-', $.operator)), field('operand', $.number)),

    // A unit of an interval is one token, which takes the singular and the
    // plural, and the forms with SQL_TSI_.
    interval_unit: _ => token(choice(
      ...[
        'NANOSECOND', 'MICROSECOND', 'MILLISECOND', 'SECOND', 'MINUTE', 'HOUR',
        'DAY', 'WEEK', 'MONTH', 'QUARTER', 'YEAR',
      ].flatMap(u => [u, u + 'S', 'SQL_TSI_' + u]).map(keyword),
    )),

    // A column, with its table and its database. A name with more parts is
    // a field of a column that holds a tuple, a nested column or JSON. A
    // number after a dot is an element of a tuple, as in t.1, and it is a
    // token of its own, so that t.1.2 has two dots.
    column_reference: $ => prec.right(1, choice(
      $._column_name,
      seq(field('table', $._name), '.', field('name', $._member)),
      seq(field('table', $.query_parameter), '.', field('name', $._member)),
      seq(
        field('database', $.query_parameter),
        '.',
        field('table', $._member),
        '.',
        field('name', $._member),
      ),
      seq(
        field('database', $._name),
        '.',
        field('table', $._member),
        '.',
        field('name', $._member),
      ),
    )),

    _column_name: $ => field('name', $._name),

    // A member after a dot is a name, the number of an element of a tuple,
    // a path of a JSON column with its type, as in json.a.:Int64, the
    // paths of a JSON column that start with a prefix, as in json.^a, or
    // the value and the paths under a path together, as in json.@a.
    _member: $ => choice(
      $._label,
      alias($._index, $.number),
      $.subcolumn_type,
      $.path_prefix,
    ),

    subcolumn_type: $ => seq(':', field('type', $.type)),

    path_prefix: $ => seq(choice('^', '@'), field('name', $._label)),

    _index: _ => /[0-9]+/,

    // A query parameter, as in {id:UInt32}, which the client gives a value.
    query_parameter: $ => seq(
      '{',
      field('name', $._label),
      ':',
      field('type', $.type),
      '}',
    ),

    // A function call. A parametric aggregate function takes its
    // parameters before its arguments, as in quantile(0.9)(x). The name of
    // an aggregate function can end with combinators, such as sumIf or
    // uniqArrayMerge, and it is one name. IGNORE NULLS, RESPECT NULLS,
    // FILTER and OVER come after the arguments, in any order.
    function_call: $ => prec.right(seq(
      field('name', $._function_name),
      choice(
        $.arguments,
        seq(field('parameters', $.arguments), $.arguments),
      ),
      repeat(choice(
        seq(choice(kw('IGNORE'), kw('RESPECT')), kw('NULLS')),
        $.filter_clause,
        $.over_clause,
      )),
    )),

    // A function name can be any word, such as if, and, or or in.
    _function_name: $ => choice(
      reserved('function_name', $.identifier),
      alias($._quoted_identifier, $.identifier),
    ),

    // The arguments of a function. DISTINCT or ALL can come before them.
    // A table function can take a query, as in view(SELECT 1).
    arguments: $ => parens(optional(seq(
      optional(choice(kw('DISTINCT'), kw('ALL'))),
      choice(
        $._argument_list,
        $._query,
      ),
    ))),

    // The arguments of a function. Most functions take a list of
    // expressions with commas between them. The functions that the standard
    // gives a syntax of their own have keywords between their arguments,
    // such as substring(s FROM 2 FOR 3), trim(BOTH 'x' FROM s),
    // overlay(s PLACING 'x' FROM 2) and extract(YEAR FROM d). The grammar
    // accepts each of these keywords between two arguments of any
    // function, so that each argument is in the same parse states. An
    // argument can have an alias, as in sum(x AS y).
    _argument_list: $ => seq(
      optional(seq(choice(kw('BOTH'), kw('LEADING'), kw('TRAILING')), optional(kw('FROM')))),
      $._argument,
      repeat(seq(choice(',', kw('FROM'), kw('FOR'), kw('PLACING')), $._argument)),
      optional(','),
    ),

    _argument: $ => choice(
      $._expression,
      $.star_expression,
    ),

    // FILTER takes a condition, as the combinator -If does.
    filter_clause: $ => seq(
      kw('FILTER'),
      parens(seq(kw('WHERE'), field('condition', $._expression))),
    ),

    over_clause: $ => seq(
      kw('OVER'),
      choice(field('window', $._name), $.window_specification),
    ),

    // CAST(x AS type), CAST(x, 'type') with the type in a string, and
    // x::type.
    cast_expression: $ => choice(
      seq(
        kw('CAST'),
        parens(seq(
          field('value', $._expression),
          choice(
            seq(kw('AS'), field('type', $.type)),
            seq(',', field('type', $._expression)),
          ),
        )),
      ),
      prec.left(PREC.postfix, seq(field('value', $._primary_expression), '::', field('type', $.type))),
    ),

    // An element of an array, or a value of a map, as in a[1] or m['key'].
    // [] after a JSON path gives the array of the values of its objects,
    // as in json.arr[].a.
    subscript_expression: $ => prec(PREC.postfix, seq(
      field('value', $._primary_expression),
      '[',
      optional(field('index', $._expression)),
      ']',
    )),

    // An element of a tuple, by its number or by its name, as in f(x).1,
    // or a field of a column with more than three parts.
    field_selection: $ => prec(PREC.field, seq(
      field('value', choice(
        $.column_reference,
        $.tuple,
        $.array,
        $.cast_expression,
        $.parenthesized_expression,
        $.subscript_expression,
        $.function_call,
        $.field_selection,
      )),
      '.',
      field('field', $._member),
    )),

    parenthesized_expression: $ => parens($._expression),

    // A tuple: no values, or two or more values, in parentheses. A tuple
    // of one value is tuple(x).
    tuple: $ => parens(optional(seq(
      $._expression,
      ',',
      optional(seq(commaSep1($._expression), optional(','))),
    ))),

    // An array, as in [1, 2, 3].
    array: $ => seq('[', optional(trailingSep1($._expression)), ']'),

    // A map, as in {'a': 1, 'b': 2}. A key is a string or a number, so that
    // a name after { starts a query parameter.
    map: $ => seq('{', optional(commaSep1($.map_entry)), '}'),

    map_entry: $ => seq(
      field('key', choice($.string, $.number)),
      ':',
      field('value', $._expression),
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

    // A lambda: x -> x + 1, or (x, y) -> x + y.
    lambda_expression: $ => prec.right(PREC.lambda, seq(
      field('parameters', $._expression),
      '->',
      field('body', $._expression),
    )),

    // The ternary operator: cond ? a : b.
    ternary_expression: $ => prec.right(PREC.ternary, seq(
      field('condition', $._expression),
      '?',
      field('consequence', $._expression),
      ':',
      field('alternative', $._expression),
    )),

    // A choice of operators flattens into one production for each
    // operator, and each production has parse states of its own. So each
    // level of precedence takes its operators from one token, which is the
    // node operator.
    unary_expression: $ => choice(
      prec.right(PREC.not, seq(field('operator', kw('NOT')), field('operand', $._expression))),
      prec.right(PREC.unary, seq(field('operator', alias($._sign, $.operator)), field('operand', $._expression))),
    ),

    binary_expression: $ => {
      /** @type {[number, RuleOrLiteral, RuleOrLiteral][]} */
      const table = [
        [PREC.or, kw('OR'), $._expression],
        [PREC.and, kw('AND'), $._expression],
        [PREC.compare, alias($._comparison_operator, $.operator), $._operand],
        [PREC.concat, alias('||', $.operator), $._expression],
        [PREC.add, alias($._sign, $.operator), $._expression],
        [PREC.multiply, choice(alias($._multiplicative_operator, $.operator), kw('DIV'), kw('MOD')), $._expression],
      ];
      return choice(...table.map(([p, op, right]) => prec.left(p, seq(
        field('left', $._expression),
        field('operator', op),
        field('right', right),
      ))));
    },

    _sign: _ => token(choice('-', '+')),

    _comparison_operator: _ => token(choice('=', '==', '<', '>', '<=', '>=', '<>', '!=', '<=>')),

    _multiplicative_operator: _ => token(choice('*', '/', '%')),

    _operand: $ => choice($._expression, $.quantified_expression),

    // ANY or ALL after an operator compares the left side with each row of
    // a subquery or with each element of an array.
    quantified_expression: $ => seq(
      field('quantifier', choice(kw('ANY'), kw('ALL'))),
      choice($.subquery, $.parenthesized_expression),
    ),

    // IS NULL, IS TRUE, IS FALSE, IS UNKNOWN and IS DISTINCT FROM, with
    // NOT.
    is_expression: $ => prec.left(PREC.is, seq(
      field('left', $._expression),
      kw('IS'),
      optional(kw('NOT')),
      field('right', choice(
        $.null,
        $.boolean,
        kw('UNKNOWN'),
        $.distinct_from,
      )),
    )),

    distinct_from: $ => seq(kws('DISTINCT', 'FROM'), $._expression),

    between_expression: $ => prec.left(PREC.between, seq(
      field('left', $._expression),
      optional(kw('NOT')),
      kw('BETWEEN'),
      field('low', $._expression),
      kw('AND'),
      field('high', $._expression),
    )),

    // The right side of IN is a tuple, a subquery, a table, or a value that
    // holds an array. GLOBAL IN sends the right side to each server of a
    // distributed query.
    in_expression: $ => prec.left(PREC.compare, seq(
      field('left', $._expression),
      optional(kw('GLOBAL')),
      optional(kw('NOT')),
      kw('IN'),
      field('right', $._expression),
    )),

    // LIKE, ILIKE, REGEXP and SIMILAR TO, with NOT. ESCAPE gives the
    // character that escapes % and _ in the pattern.
    like_expression: $ => prec.left(PREC.compare, seq(
      field('left', $._expression),
      optional(kw('NOT')),
      field('operator', choice(kw('LIKE'), kw('ILIKE'), kw('REGEXP'), kws('SIMILAR', 'TO'))),
      field('right', $._expression),
      optional(seq(kw('ESCAPE'), field('escape', $.string))),
    )),

    // Types

    // A type: a name, with arguments in parentheses, as in Decimal(10, 2),
    // Array(String), Tuple(a UInt8, b String), Enum8('a' = 1, 'b' = 2),
    // DateTime64(3, 'UTC'), AggregateFunction(quantiles(0.5), Float64) or
    // JSON(max_dynamic_paths = 10, a.b UInt32, SKIP a.c). The names of SQL
    // with more than one word, such as DOUBLE PRECISION and INT UNSIGNED,
    // are types too.
    type: $ => prec.right(choice(
      seq(field('name', $._label), optional($.type_arguments)),
      seq(kw('DOUBLE'), optional(kw('PRECISION'))),
      seq(
        choice(
          seq(optional(kw('NATIONAL')), choice(kw('CHARACTER'), kw('CHAR'))),
          kw('NCHAR'),
          kw('BINARY'),
        ),
        optional(choice(kw('VARYING'), kws('LARGE', 'OBJECT'))),
        optional($.type_arguments),
      ),
      seq(field('name', $._label), optional($.type_arguments), choice(kw('SIGNED'), kw('UNSIGNED'))),
    )),

    type_arguments: $ => parens(optional(commaSep1(choice(
      $._signed_number,
      $.string,
      $.type,
      $.enum_value,
      $.field_definition,
      $.type_setting,
      $.skip_path,
    )))),

    // A value of an enum, as in 'a' = 1.
    enum_value: $ => seq(field('name', $.string), '=', field('value', $._signed_number)),

    // A field of a tuple or of a nested type, or a path of a JSON type, with
    // its type.
    field_definition: $ => seq(
      field('name', $._path),
      field('type', $.type),
    ),

    _path: $ => seq($._name, repeat(seq('.', $._label))),

    // A setting of a type, as in max_dynamic_paths = 10.
    type_setting: $ => seq(field('name', $._name), '=', field('value', choice($._signed_number, $.string))),

    // SKIP leaves out a path of a JSON type, or the paths that match a
    // regular expression.
    skip_path: $ => seq(kw('SKIP'), choice(
      field('path', alias($._path, $.identifier)),
      seq(kw('REGEXP'), field('pattern', $.string)),
    )),

    _signed_number: $ => seq(optional(alias('-', $.operator)), $.number),

    // Names and literals

    // A name of a table, with its database. A query parameter can give a
    // name, as in {db:Identifier}.t.
    object_reference: $ => choice(
      field('name', choice($._name, $.query_parameter)),
      seq(
        field('database', choice($._name, $.query_parameter)),
        '.',
        field('name', choice($._label, $.query_parameter)),
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

    // A number is an integer, a decimal number with an exponent or not, a
    // hexadecimal number with 0x, which can have a fraction and a binary
    // exponent after p, or a binary number with 0b. An underscore can
    // separate two digits. The rule comes before identifier, so that the
    // lexer takes a number such as 1e10 or 0x1F, which identifier also
    // matches, as a number.
    number: _ => token(choice(
      /[0-9](_?[0-9])*(\.([0-9](_?[0-9])*)?)?([eE][+-]?[0-9]+)?/,
      /\.[0-9](_?[0-9])*([eE][+-]?[0-9]+)?/,
      /0[xX][0-9A-Fa-f](_?[0-9A-Fa-f])*(\.[0-9A-Fa-f]*)?([pP][+-]?[0-9]+)?/,
      /0[bB][01](_?[01])*/,
    )),

    // An identifier starts with a letter, _ or a character above 0x7f, and
    // $ can come after its first character. It can also start with digits
    // and a letter or _ after them, as in 01686_test, as ClickHouse reads a
    // number with a letter after it. A name that starts with _ is also the
    // placeholder of a variable of usql (D101).
    identifier: _ => /[0-9]*[A-Za-z_\u0080-\u{10FFFF}][A-Za-z0-9_$\u0080-\u{10FFFF}]*/u,

    // A quoted identifier is in double quotes or in backquotes, with the
    // quote doubled or after a backslash.
    _quoted_identifier: _ => token(choice(
      /"([^"\\]|\\(.|\n)|"")*"/,
      /`([^`\\]|\\(.|\n)|``)*`/,
    )),

    // A string is in single quotes, with a quote doubled or after a
    // backslash, and the other escapes of a backslash.
    string: _ => /'([^'\\]|\\(.|\n)|'')*'/,

    // A hex string, as in x'4142', and a binary string, as in b'0100', give
    // a string of the bytes.
    hex_string: _ => /[xX]'[^']*'/,

    bit_string: _ => /[bB]'[^']*'/,

    // A heredoc, as in $doc$text$doc$, is a string with no escapes.
    heredoc_string: $ => seq(
      $._heredoc_start,
      optional($.string_content),
      $._heredoc_end,
    ),
  },
});
