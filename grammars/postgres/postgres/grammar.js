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
  'ALL', 'AND', 'ANY', 'ARRAY', 'AS', 'ASC', 'ASYMMETRIC', 'BOTH', 'CASE',
  'CAST', 'COLLATE', 'CURRENT_CATALOG', 'CURRENT_DATE', 'CURRENT_ROLE',
  'CURRENT_TIME', 'CURRENT_TIMESTAMP', 'CURRENT_USER', 'DEFAULT', 'DESC', 'DISTINCT',
  'ELSE', 'END', 'EXCEPT', 'FALSE', 'FETCH', 'FOR', 'FROM', 'GROUP',
  'HAVING', 'IN', 'INTERSECT', 'INTO', 'LATERAL', 'LEADING', 'LIMIT',
  'LOCALTIME', 'LOCALTIMESTAMP', 'NOT', 'NULL', 'OFFSET', 'ON', 'ONLY', 'OR',
  'ORDER', 'PLACING', 'RETURNING', 'SELECT', 'SESSION_USER', 'SOME',
  'SYMMETRIC', 'SYSTEM_USER', 'TABLE', 'THEN', 'TO', 'TRAILING', 'TRUE',
  'UNION', 'UNIQUE', 'USER', 'USING', 'VARIADIC', 'WHEN', 'WHERE', 'WINDOW', 'WITH',
];

const TYPE_FUNC_NAME_WORDS = [
  'COLLATION', 'CROSS', 'CURRENT_SCHEMA', 'FULL', 'ILIKE', 'INNER', 'IS',
  'ISNULL', 'JOIN', 'LEFT', 'LIKE', 'NATURAL', 'NOTNULL', 'OUTER',
  'OVERLAPS', 'RIGHT', 'SIMILAR', 'TABLESAMPLE',
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
  // columns takes the large form, so these rules are inlined.
  inline: $ => [
    $._name,
    $._label,
    $._literal,
    $._literal_type,
    $._function_name,
    $._primary_expression,
  ],

  conflicts: $ => [
    // A parenthesized query can be the left side of a set operation, or a
    // subquery in an expression or in FROM.
    [$.parenthesized_query, $.subquery],
    [$.join],
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
      parens(field('query', $._query)),
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

    distinct_clause: $ => seq(
      kw('DISTINCT'),
      optional(seq(kw('ON'), parens(commaSep1($._expression)))),
    ),

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

    column_definition: $ => seq(
      field('name', $._name),
      field('type', $.type),
      optional(seq(kw('COLLATE'), field('collation', $.object_reference))),
    ),

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

    where_clause: $ => seq(kw('WHERE'), $._expression),

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
      choice(
        kw('UPDATE'),
        kws('NO', 'KEY', 'UPDATE'),
        kw('SHARE'),
        kws('KEY', 'SHARE'),
      ),
      optional(seq(kw('OF'), commaSep1($.object_reference))),
      optional(choice(kw('NOWAIT'), kws('SKIP', 'LOCKED'))),
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
      prec(1, seq($.identifier, choice(kw('WITH'), kw('WITHOUT')), kws('TIME', 'ZONE'))),
    ),

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

    type_modifiers: $ => parens(commaSep1(choice($.number, $.string, $._name))),

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
