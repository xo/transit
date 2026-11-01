/**
 * @file The grammar of the SQL of DuckDB
 * @license MIT
 */

// xo writes this grammar from the SQL documentation of the current release
// of DuckDB, 2.0, at https://duckdb.org/docs/stable/sql/introduction (D126).
// It does not copy the PEG grammar of DuckDB.
//
// The grammar is for a highlighter and for completion, so it accepts more
// than DuckDB does where that keeps the rules small. A keyword is a token in
// any case, and it is an anonymous node with the name of the keyword in
// upper case, such as "SELECT".
//
// The token identifier is the word of the grammar, so a keyword that DuckDB
// does not reserve is an identifier in each place where the grammar does
// not expect the keyword. The reserved words of DuckDB are in the reserved
// set global, so they are not identifiers, but for a label after AS or
// after a dot, where DuckDB accepts every keyword. A function name can be a
// word that DuckDB reserves only for column names, such as left.
//
// The grammar keeps its parse tables small (D122, D126). A parse state with
// more than 64 columns takes the large form, so the grammar keeps the
// columns of a state few. The operators of one level of precedence are one
// token, TRUE and FALSE are one token, and NULL is a token of its own. The
// words of a join before JOIN, such as LEFT OUTER, are one token with JOIN,
// but INNER JOIN is two keywords. The clauses of a query after FROM, the
// clauses after its body, the clauses after the arguments of a function
// and the clauses of a window come in any order, because a sequence of
// optional clauses costs parse states for each set of the clauses. The %
// of LIMIT 10% is the token of the operator %, so that the states after
// the count are the states after other expressions.
//
// The external scanner in src/scanner.c reads the comments, which nest, and
// the dollar-quoted strings. A statement ends at a semicolon or at the end
// of the input. The commands of the duckdb shell, such as .tables and
// .mode, are not SQL, so the grammar does not read them. usql does not
// remove them, so such a line can come before a statement. The error
// recovery of tree-sitter puts the line in an ERROR node, and the statement
// after it parses with no error. The variables of usql belong to the usql
// grammar, which hands this grammar a placeholder of the same length in
// place of each variable (D101).

/// <reference types="tree-sitter-cli/dsl" />
// @ts-check

// The precedences of the operators, from the lowest to the highest, as the
// grammar of DuckDB 2.0 orders them. The arrow of a lambda binds the most
// loosely. IS DISTINCT FROM binds more loosely than a comparison, and IN,
// BETWEEN and LIKE bind more tightly. The other operators, such as || and
// ->>, bind more loosely than the bitwise operators, and so does an
// operator before a value, such as ~ or @. A sign binds more tightly than
// ^, and a cast, a subscript and ! bind the most tightly.
const PREC = {
  lambda: 1,
  or: 2,
  and: 3,
  not: 4,
  is: 5,
  compare: 6,
  like: 7,
  other: 8,
  bit: 9,
  add: 10,
  multiply: 11,
  exponent: 12,
  collate: 13,
  at: 14,
  unary: 15,
  postfix: 16,
  field: 17,
};

// The words that DuckDB 2.0 reserves: the reserved keywords of the function
// duckdb_keywords. A word is in the list only when the grammar uses it as a
// keyword, because a reserved word must be a token of the grammar.
const RESERVED_WORDS = [
  'ALL', 'ANALYSE', 'ANALYZE', 'AND', 'ANY', 'ARRAY', 'AS', 'ASC',
  'ASYMMETRIC', 'BOTH', 'CASE', 'CAST', 'CHECK', 'COLLATE', 'COLUMN',
  'CONSTRAINT', 'CREATE', 'DEFAULT', 'DEFERRABLE', 'DESC', 'DESCRIBE',
  'DISTINCT', 'DO', 'ELSE', 'END', 'EXCEPT', 'FALSE', 'FETCH', 'FOR',
  'FOREIGN', 'FROM', 'GROUP', 'HAVING', 'QUALIFY', 'IN', 'INITIALLY',
  'INTERSECT', 'INTO', 'LAMBDA', 'LATERAL', 'LEADING', 'LIMIT', 'NOT', 'NULL',
  'OFFSET', 'ON', 'ONLY', 'OR', 'ORDER', 'PIVOT', 'PIVOT_WIDER',
  'PIVOT_LONGER', 'PLACING', 'PRIMARY', 'REFERENCES', 'RETURNING', 'SELECT',
  'SHOW', 'SOME', 'SUMMARIZE', 'SYMMETRIC', 'TABLE', 'THEN', 'TO', 'TRAILING',
  'TRUE', 'UNION', 'UNIQUE', 'UNPIVOT', 'USING', 'VARIADIC', 'WHEN', 'WHERE',
  'WINDOW', 'WITH',
];

// The words that DuckDB reserves for column names but accepts as a function
// name or a type name, such as LEFT and ILIKE.
const FUNC_NAME_WORDS = [
  'ASOF', 'AT', 'AUTHORIZATION', 'BINARY', 'COLLATION', 'CONCURRENTLY',
  'CROSS', 'FREEZE', 'FULL', 'GLOB', 'ILIKE', 'INNER', 'IS', 'ISNULL', 'JOIN',
  'LEFT', 'LIKE', 'NATURAL', 'NOTNULL', 'OUTER', 'OVERLAPS', 'POSITIONAL',
  'RIGHT', 'SIMILAR', 'TABLESAMPLE', 'VERBOSE',
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
 * optional comma after the last one, as DuckDB allows in a list. The
 * grammar takes the comma after the last one only in the lists where the
 * documentation shows it, because each such list costs parse states.
 *
 * @param {RuleOrLiteral} rule
 */
function trailingSep1(rule) {
  return seq(rule, repeat(seq(',', rule)), optional(','));
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
  name: 'duckdb',

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

  // The reserved set global holds the words that DuckDB reserves. The set
  // function_name leaves out the words that DuckDB accepts as a function
  // name, and the set label is empty, because DuckDB accepts every keyword
  // as a label after AS and after a dot. A state takes the last set of its
  // items, so the sets go from the strictest to the least strict.
  reserved: {
    global: _ => [...RESERVED_WORDS, ...FUNC_NAME_WORDS].filter(w => KEYWORDS.has(w)).map(keyword),
    function_name: _ => RESERVED_WORDS.filter(w => KEYWORDS.has(w)).map(keyword),
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
    $._literal_type,
    $._function_name,
    $._qualifier,
    $._primary_expression,
    $._operand,
    $._table_factor,
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
    ),

    // Queries

    select_statement: $ => $._query,

    // ORDER BY, LIMIT, OFFSET and FETCH come after the body of a query, in
    // any order.
    _query: $ => prec.right(seq(
      optional($.with_clause),
      $._query_body,
      repeat(choice(
        $.order_by_clause,
        $.limit_clause,
        $.offset_clause,
        $.fetch_clause,
      )),
    )),

    // A common table expression can name itself after RECURSIVE. USING KEY
    // names the key of the rows of a recursive query, and the aggregates of
    // the other columns.
    with_clause: $ => seq(
      kw('WITH'),
      optional(kw('RECURSIVE')),
      commaSep1($.cte),
    ),

    cte: $ => seq(
      field('name', choice($._name, $.string)),
      optional($.column_list),
      optional(seq(kws('USING', 'KEY'), parens(commaSep1(choice($._expression, $.aliased_expression))))),
      kw('AS'),
      optional(seq(optional(kw('NOT')), kw('MATERIALIZED'))),
      parens(field('query', $._query)),
    ),

    column_list: $ => parens(trailingSep1(choice($._name, $.string))),

    // VALUES in parentheses is a subquery, and not a table in a
    // parenthesized join.
    _query_body: $ => choice(
      $.select,
      prec(1, $.values),
      $.table_query,
      $.pivot,
      $.unpivot,
      $.set_operation,
      $.parenthesized_query,
    ),

    parenthesized_query: $ => parens($._query),

    // INTERSECT binds more tightly than UNION and EXCEPT. BY NAME matches
    // the columns of the two sides by their names.
    set_operation: $ => choice(
      prec.left(1, seq(
        field('left', $._query_body),
        field('operator', choice(kw('UNION'), kw('EXCEPT'))),
        optional(choice(kw('ALL'), kw('DISTINCT'))),
        optional(kws('BY', 'NAME')),
        field('right', $._query_body),
      )),
      prec.left(2, seq(
        field('left', $._query_body),
        field('operator', kw('INTERSECT')),
        optional(choice(kw('ALL'), kw('DISTINCT'))),
        optional(kws('BY', 'NAME')),
        field('right', $._query_body),
      )),
    ),

    // A query can start with FROM, and then SELECT can come after FROM or
    // not at all. The clauses after FROM come in any order.
    select: $ => prec.right(seq(
      choice(
        seq($._select_clause, optional($.from_clause)),
        seq($.from_clause, optional($._select_clause)),
      ),
      repeat(choice(
        $.where_clause,
        $.group_by_clause,
        $.having_clause,
        $.window_clause,
        $.qualify_clause,
        $.sample_clause,
      )),
    )),

    // SELECT can have no columns, and a comma can come after the last one.
    _select_clause: $ => prec.right(seq(
      kw('SELECT'),
      optional(choice(kw('ALL'), $.distinct_clause)),
      optional(trailingSep1($._select_item)),
    )),

    distinct_clause: $ => prec.right(seq(
      kw('DISTINCT'),
      optional(seq(kw('ON'), parens(commaSep1($._expression)))),
    )),

    _select_item: $ => choice(
      $._expression,
      $.aliased_expression,
      $.star_expression,
    ),

    // An alias comes after AS, after the expression with no AS, or before
    // the expression with a colon, as in SELECT n: 42.
    aliased_expression: $ => choice(
      seq(field('value', choice($._expression, $.star_expression)), kw('AS'), field('alias', choice($._label, $.string))),
      seq(field('value', $._expression), field('alias', $._name)),
      seq(field('alias', $._name), ':', field('value', $._expression)),
    ),

    // * or t.* gives all the columns of a table. EXCLUDE leaves out
    // columns, REPLACE replaces them with expressions, RENAME renames them,
    // and a pattern keeps the columns whose names match it.
    star_expression: $ => prec.right(seq(
      optional(seq(field('table', $._name), '.')),
      '*',
      repeat(choice(
        seq(choice(kw('EXCLUDE'), kw('EXCEPT')), choice(
          parens(commaSep1($._star_column)),
          $._star_column,
        )),
        seq(kw('REPLACE'), choice(
          parens(commaSep1($.star_replace)),
          $.star_replace,
        )),
        seq(kw('RENAME'), choice(
          parens(commaSep1($.star_rename)),
          $.star_rename,
        )),
        seq(
          field('operator', $._like_operator),
          field('pattern', $.string),
          optional(seq(kw('ESCAPE'), field('escape', $.string))),
        ),
      )),
    )),

    _star_column: $ => choice($.column_reference, $.string),

    star_replace: $ => seq(field('value', $._expression), kw('AS'), field('name', $._label)),

    star_rename: $ => seq(field('column', $._star_column), kw('AS'), field('name', $._label)),

    // VALUES gives rows, in place of a SELECT.
    values: $ => prec.right(seq(kw('VALUES'), trailingSep1($.value_list))),

    value_list: $ => parens(trailingSep1($._expression)),

    table_query: $ => seq(kw('TABLE'), field('table', $.object_reference)),

    // PIVOT turns the values of the columns after ON into columns, with the
    // aggregates after USING. PIVOT_WIDER is the same statement.
    pivot: $ => prec.right(seq(
      choice(kw('PIVOT'), kw('PIVOT_WIDER')),
      field('table', $._table_reference),
      optional(seq(kw('ON'), commaSep1($._expression))),
      optional(seq(kw('USING'), commaSep1($._select_item))),
      optional($.group_by_clause),
    )),

    // UNPIVOT turns columns into rows. INTO NAME names the column of the
    // names, and VALUE names the columns of the values. PIVOT_LONGER is the
    // same statement.
    unpivot: $ => prec.right(seq(
      choice(kw('UNPIVOT'), kw('PIVOT_LONGER')),
      field('table', $._table_reference),
      kw('ON'),
      commaSep1($._select_item),
      optional(seq(
        kws('INTO', 'NAME'),
        field('name', choice($._name, $.string)),
        choice(kw('VALUE'), kw('VALUES')),
        choice($.column_list, commaSep1(choice($._name, $.string))),
      )),
    )),

    from_clause: $ => seq(kw('FROM'), trailingSep1($._table_reference)),

    _table_reference: $ => choice(
      $._table_factor,
      $.join,
      $.pivot_table,
      $.unpivot_table,
    ),

    // The right side of a join is a table factor: a join on the right side
    // must be in parentheses.
    _table_factor: $ => choice(
      $.table_reference,
      $.function_table,
      $.derived_table,
      $.parenthesized_join,
    ),

    // A table, or a file that DuckDB reads as a table, as in FROM
    // 'data.parquet'. An alias can come before the table with a colon, as
    // in FROM t: tbl. AT reads a version of the table.
    table_reference: $ => prec.right(seq(
      optional($._prefix_alias),
      field('table', choice($.object_reference, $.string)),
      optional($._alias),
      optional($.at_clause),
      optional($.tablesample_clause),
    )),

    _prefix_alias: $ => seq(field('alias', choice($._name, $.string)), ':'),

    // An alias of a table, with the names of its columns.
    _alias: $ => seq(
      choice(
        seq(kw('AS'), field('alias', choice($._label, $.string))),
        field('alias', $._name),
      ),
      optional($.column_list),
    ),

    at_clause: $ => seq(
      kw('AT'),
      parens(seq(
        choice(kw('VERSION'), kw('TIMESTAMP')),
        '=>',
        field('value', $._expression),
      )),
    ),

    // A table function, such as range(10) or read_csv('a.csv'), with WITH
    // ORDINALITY.
    function_table: $ => prec.right(seq(
      optional($._prefix_alias),
      optional(kw('LATERAL')),
      field('function', alias($._table_function, $.function_call)),
      optional(kws('WITH', 'ORDINALITY')),
      optional($._alias),
      optional($.tablesample_clause),
    )),

    _table_function: $ => seq(
      optional($._qualifier),
      field('name', $._function_name),
      $.arguments,
    ),

    // A subquery or VALUES in FROM.
    derived_table: $ => prec.right(seq(
      optional($._prefix_alias),
      optional(kw('LATERAL')),
      choice($.subquery, $.values),
      optional($._alias),
      optional($.tablesample_clause),
    )),

    // A list of tables and joins in parentheses.
    parenthesized_join: $ => prec.right(seq(
      parens(commaSep1($._table_reference)),
      optional($._alias),
    )),

    // A join. A regular join needs ON, USING or NEAREST after its right
    // side, and CROSS JOIN, NATURAL JOIN and POSITIONAL JOIN take none of
    // them. The words before JOIN, such as ASOF LEFT OUTER, are one token
    // with JOIN, so that they take one column of a parse state. It is the
    // keyword JOIN in the tree. INNER JOIN is two keywords, so that IN
    // stays a keyword of the word rule.
    join: $ => prec.left(seq(
      field('left', $._table_reference),
      choice(
        seq(
          choice(
            kw('JOIN'),
            kws('INNER', 'JOIN'),
            alias($._join, 'JOIN'),
            seq(kws('JOIN', 'BY'), parens(seq(kw('TYPE'), field('type', $._label)))),
          ),
          field('right', $._table_factor),
          choice(
            seq(kw('ON'), field('condition', $._expression)),
            seq(kw('USING'), $.column_list),
            $.nearest_clause,
          ),
        ),
        seq(
          alias($._join_without_condition, 'JOIN'),
          field('right', $._table_factor),
        ),
      ),
    )),

    _join: _ => token(seq(
      choice(
        seq(
          keyword('ASOF'),
          optional(seq(/\s+/, choice(
            keyword('INNER'),
            keyword('SEMI'),
            keyword('ANTI'),
            seq(
              choice(keyword('LEFT'), keyword('RIGHT'), keyword('FULL')),
              optional(seq(/\s+/, keyword('OUTER'))),
            ),
          ))),
        ),
        seq(
          choice(keyword('LEFT'), keyword('RIGHT'), keyword('FULL')),
          optional(seq(/\s+/, keyword('OUTER'))),
        ),
        keyword('SEMI'),
        keyword('ANTI'),
      ),
      /\s+/,
      keyword('JOIN'),
    )),

    _join_without_condition: _ => token(seq(
      choice(
        keyword('CROSS'),
        keyword('POSITIONAL'),
        seq(
          keyword('NATURAL'),
          optional(seq(/\s+/, choice(
            keyword('INNER'),
            keyword('SEMI'),
            keyword('ANTI'),
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

    // NEAREST joins each row with the k rows of the right side that are
    // the most similar or the least distant.
    nearest_clause: $ => seq(
      optional(choice(kw('APPROX'), kw('EXACT'))),
      kw('NEAREST'),
      optional(field('count', $.number)),
      kw('BY'),
      choice(kw('DISTANCE'), kw('SIMILARITY')),
      field('measure', $._expression),
    ),

    // The PIVOT of the SQL standard after a table: the aggregates, FOR and
    // the values of each column, and GROUP BY.
    pivot_table: $ => prec.right(seq(
      field('table', $._table_reference),
      kw('PIVOT'),
      parens(seq(
        commaSep1($._select_item),
        kw('FOR'),
        repeat1($.pivot_values),
        optional($.group_by_clause),
      )),
      optional($._alias),
    )),

    // The UNPIVOT of the SQL standard after a table: the columns of the
    // values, FOR and the columns to unpivot.
    unpivot_table: $ => prec.right(seq(
      field('table', $._table_reference),
      kw('UNPIVOT'),
      optional(seq(choice(kw('INCLUDE'), kw('EXCLUDE')), kw('NULLS'))),
      parens(seq(
        field('value', choice($._name, $.string, $.column_list)),
        kw('FOR'),
        repeat1($.pivot_values),
      )),
      optional($._alias),
    )),

    // A column and its values, as in year IN (2000, 2010), or a column and
    // an enum type that holds the values.
    pivot_values: $ => seq(
      field('column', choice($.column_reference, $.column_list)),
      kw('IN'),
      choice(
        parens(commaSep1($._select_item)),
        field('type', $._name),
      ),
    ),

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
      $.empty_grouping_set,
      $.rollup,
      $.cube,
      $.grouping_sets,
    ),

    empty_grouping_set: _ => seq('(', ')'),

    rollup: $ => seq(kw('ROLLUP'), parens(optional(commaSep1($._expression)))),

    cube: $ => seq(kw('CUBE'), parens(optional(commaSep1($._expression)))),

    grouping_sets: $ => seq(
      kws('GROUPING', 'SETS'),
      parens(commaSep1($._grouping_element)),
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

    // USING SAMPLE takes a sample of the rows of a query, and TABLESAMPLE
    // takes a sample of a table.
    sample_clause: $ => seq(kws('USING', 'SAMPLE'), $._sample),

    tablesample_clause: $ => seq(kw('TABLESAMPLE'), $._sample),

    // A sample is a size, as in 10% or 50 ROWS, with the method and the
    // seed in parentheses, or a method with the size in parentheses, as in
    // reservoir(50 ROWS), with the seed after REPEATABLE.
    _sample: $ => choice(
      seq(
        $.sample_size,
        optional(parens(seq(
          field('method', $._name),
          optional(seq(',', field('seed', $.number))),
        ))),
      ),
      seq(
        optional(field('method', $._name)),
        parens($.sample_size),
        optional(seq(kw('REPEATABLE'), parens(field('seed', $.number)))),
      ),
    ),

    sample_size: $ => seq(
      field('value', choice($.number, $.parameter)),
      optional(choice(alias($._multiplicative_operator, '%'), kw('PERCENT'), kw('ROWS'))),
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

    // ORDER BY ALL orders by each column of SELECT.
    order_by_clause: $ => seq(
      kws('ORDER', 'BY'),
      choice(
        seq(kw('ALL'), optional($._direction), optional($._nulls_order)),
        commaSep1($.order_by_item),
      ),
    ),

    order_by_item: $ => seq(
      field('value', $._expression),
      optional($._direction),
      optional($._nulls_order),
    ),

    _direction: _ => choice(kw('ASC'), kw('DESC'), kw('ASCENDING'), kw('DESCENDING')),

    _nulls_order: _ => seq(kw('NULLS'), choice(kw('FIRST'), kw('LAST'))),

    // LIMIT takes a count, ALL, or a percent of the rows, as in LIMIT 10%.
    // The % of a percent is the token of the operators *, / and %, so that
    // the states after the count are the states after other expressions.
    limit_clause: $ => seq(
      kw('LIMIT'),
      choice(
        seq(field('count', $._expression), optional(choice(alias($._multiplicative_operator, '%'), kw('PERCENT')))),
        kw('ALL'),
      ),
    ),

    offset_clause: $ => seq(
      kw('OFFSET'),
      field('start', $._expression),
      optional(choice(kw('ROW'), kw('ROWS'))),
    ),

    fetch_clause: $ => seq(
      kw('FETCH'),
      choice(kw('FIRST'), kw('NEXT')),
      optional(field('count', $._expression)),
      choice(kw('ROW'), kw('ROWS')),
      kw('ONLY'),
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
      $.lambda_expression,
    ),

    _primary_expression: $ => choice(
      $._literal,
      $.typed_literal,
      $.interval_expression,
      $.column_reference,
      $.parameter,
      $.positional_reference,
      $.function_call,
      $.method_call,
      $.columns_expression,
      $.cast_expression,
      $.subscript_expression,
      $.field_selection,
      $.parenthesized_expression,
      $.row_constructor,
      $.list,
      $.list_comprehension,
      $.struct,
      $.map,
      $.subquery,
      $.exists_expression,
      $.case_expression,
    ),

    _literal: $ => choice(
      $.number,
      $.string,
      $.dollar_quoted_string,
      $.blob,
      $.bit_string,
      $.boolean,
      $.null,
    ),

    // TRUE and FALSE are one token, so that they take one column of each
    // state where an expression starts.
    boolean: _ => token(choice(keyword('TRUE'), keyword('FALSE'))),

    // NULL is a token of its own, so that it takes one column of each state
    // where an expression starts.
    null: _ => (KEYWORDS.add('NULL'), keyword('NULL')),

    // A type name before a string gives a constant of the type, such as
    // DATE '1992-03-22'. The name is an identifier, so date also names a
    // column.
    typed_literal: $ => seq(
      field('type', alias($._literal_type, $.type)),
      field('value', $.string),
    ),

    _literal_type: $ => choice(
      $.identifier,
      alias($._time_zone_type, $.identifier),
    ),

    // The type of a time with WITH TIME ZONE before a string is one token,
    // so that WITH after a name can start a clause.
    _time_zone_type: _ => token(seq(
      choice(keyword('TIMESTAMP'), keyword('TIME')),
      /\s+/,
      choice(keyword('WITH'), keyword('WITHOUT')),
      /\s+/,
      keyword('TIME'),
      /\s+/,
      keyword('ZONE'),
    )),

    // An interval: INTERVAL with a string, a number or an expression in
    // parentheses, and the unit, as in INTERVAL 3 DAYS or INTERVAL '1' YEAR
    // TO MONTH.
    interval_expression: $ => prec.right(seq(
      kw('INTERVAL'),
      field('value', choice($.string, $.number, $.parenthesized_expression)),
      optional($.interval_unit),
    )),

    // A unit of an interval, or two units with TO between them. Each unit
    // is one token, which takes the singular and the plural.
    interval_unit: $ => prec.right(seq(
      $._interval_unit,
      optional(seq(kw('TO'), $._interval_unit)),
    )),

    _interval_unit: _ => token(choice(
      ...[
        'YEAR', 'YEARS', 'MONTH', 'MONTHS', 'DAY', 'DAYS', 'HOUR', 'HOURS',
        'MINUTE', 'MINUTES', 'SECOND', 'SECONDS', 'MILLISECOND', 'MILLISECONDS',
        'MICROSECOND', 'MICROSECONDS', 'WEEK', 'WEEKS', 'QUARTER', 'QUARTERS',
        'DECADE', 'DECADES', 'CENTURY', 'CENTURIES', 'MILLENNIUM', 'MILLENNIA',
      ].map(keyword),
    )),

    // A column, with its table, its schema and its database. The grammar
    // cannot tell a field of a struct from a column, so s.a is a column a of
    // a table s.
    column_reference: $ => prec.right(1, choice(
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
    )),

    // A parameter of a prepared statement: ?, ?1, $1, or $name, which also
    // reads a variable of SET VARIABLE.
    parameter: _ => token(choice(
      /\?[0-9]*/,
      /\$[0-9]+/,
      /\$[A-Za-z_\u0080-\u{10FFFF}][A-Za-z0-9_\u0080-\u{10FFFF}]*/u,
    )),

    // #1 names the first column of SELECT.
    positional_reference: _ => /#[0-9]+/,

    // A function call. The arguments of an aggregate function can have
    // DISTINCT, ORDER BY and IGNORE NULLS. WITHIN GROUP, FILTER, EXPORT_STATE
    // and OVER come after the arguments. The grammar takes them in any
    // order, because a sequence of optional clauses costs parse states for
    // each set of the clauses.
    function_call: $ => prec.right(seq(
      optional($._qualifier),
      field('name', $._function_name),
      $.arguments,
      repeat(choice(
        $.within_group_clause,
        $.filter_clause,
        kw('EXPORT_STATE'),
        $.over_clause,
      )),
    )),

    // A function name can be a word that DuckDB reserves only for column
    // names, such as left and like.
    _function_name: $ => choice(
      reserved('function_name', $.identifier),
      alias($._quoted_identifier, $.identifier),
    ),

    arguments: $ => parens(optional(seq(
      optional(choice(kw('DISTINCT'), kw('ALL'))),
      optional($._argument_list),
      optional($.order_by_clause),
      optional(seq(choice(kw('IGNORE'), kw('RESPECT')), kw('NULLS'))),
    ))),

    // The arguments of a function. Most functions take a list of
    // expressions with commas between them. The functions that the standard
    // gives a syntax of their own have keywords between their arguments,
    // such as substring(s FROM 2 FOR 3), trim(BOTH 'x' FROM s) and
    // extract(year FROM d). The grammar accepts each of these keywords
    // between two arguments of any function, so that each argument is in
    // the same parse states.
    _argument_list: $ => seq(
      optional(choice(
        kw('FROM'),
        seq(choice(kw('BOTH'), kw('LEADING'), kw('TRAILING')), optional(kw('FROM'))),
      )),
      $._argument,
      repeat(seq($._argument_separator, $._argument)),
      optional(','),
    ),

    _argument_separator: _ => choice(
      ',',
      kw('FROM'),
      kw('FOR'),
      kw('PLACING'),
    ),

    _argument: $ => choice(
      $._expression,
      $.named_argument,
      $.star_expression,
    ),

    named_argument: $ => seq(
      field('name', $._name),
      choice(':=', '=>'),
      field('value', $._expression),
    ),

    within_group_clause: $ => seq(
      kws('WITHIN', 'GROUP'),
      parens($.order_by_clause),
    ),

    // FILTER takes a condition, with WHERE before it or not.
    filter_clause: $ => seq(
      kw('FILTER'),
      parens(seq(optional(kw('WHERE')), field('condition', $._expression))),
    ),

    over_clause: $ => seq(
      kw('OVER'),
      choice(field('window', $._name), $.window_specification),
    ),

    // The dot after a value calls a function with the value as its first
    // argument, as in ('hello').upper() or l.list_sum(). A name before the
    // dot gives a function call with a schema, because the grammar cannot
    // tell a column from a schema.
    method_call: $ => prec(PREC.field, seq(
      field('value', choice(
        $.string,
        $.list,
        $.parenthesized_expression,
        $.subscript_expression,
        $.function_call,
        $.method_call,
        $.field_selection,
      )),
      '.',
      field('name', $._label),
      $.arguments,
    )),

    // COLUMNS gives the columns whose names match a pattern, a list or a
    // lambda, and *COLUMNS gives them as the arguments of a function. A
    // COLUMNS with no star is a function call.
    columns_expression: $ => seq(
      alias($._star_columns, '*COLUMNS'),
      $.arguments,
    ),

    _star_columns: _ => token(seq('*', keyword('COLUMNS'))),

    // CAST(x AS type), TRY_CAST(x AS type), which gives NULL in place of an
    // error, and x::type, the form of PostgreSQL.
    cast_expression: $ => choice(
      seq(
        choice(kw('CAST'), kw('TRY_CAST')),
        parens(seq(field('value', $._expression), kw('AS'), field('type', $.type))),
      ),
      prec.left(PREC.postfix, seq(field('value', $._primary_expression), '::', field('type', $.type))),
    ),

    // An element or a slice of a list or a string. A slice has a lower and
    // an upper bound and a step, and each can be left out, as in l[2:],
    // l[:-1] and l[::2]. A - alone as the upper bound is the end of the
    // list, as in l[:-:2].
    subscript_expression: $ => prec(PREC.postfix, seq(
      field('value', $._primary_expression),
      '[',
      choice(
        field('index', $._expression),
        seq(
          optional(field('lower', $._expression)),
          ':',
          optional(field('upper', choice($._expression, alias($._sign, $.operator)))),
          optional(seq(':', optional(field('step', $._expression)))),
        ),
      ),
      ']',
    )),

    // A field of a struct, such as (s).a, f(x).b or {'a': 1}.a. A column
    // reference with more than four parts is a field of a struct, as in
    // db.s.t.c.f.
    field_selection: $ => prec(PREC.field, seq(
      field('value', choice(
        $.column_reference,
        $.struct,
        $.case_expression,
        $.cast_expression,
        $.parenthesized_expression,
        $.subscript_expression,
        $.function_call,
        $.method_call,
        $.field_selection,
      )),
      '.',
      choice(field('field', $._label), '*'),
    )),

    parenthesized_expression: $ => parens($._expression),

    // A row value: a list of two or more expressions in parentheses. ROW
    // and a list is a function call.
    row_constructor: $ => parens(seq($._expression, ',', commaSep1($._expression))),

    // A list, as in [1, 2, 3] or ARRAY[1, 2, 3], or ARRAY and a subquery.
    list: $ => choice(
      seq(optional(kw('ARRAY')), '[', optional(trailingSep1($._expression)), ']'),
      seq(kw('ARRAY'), $.subquery),
    ),

    // A list comprehension, as in [x * 2 FOR x IN l IF x > 0].
    list_comprehension: $ => seq(
      '[',
      field('value', $._expression),
      kw('FOR'),
      commaSep1(field('variable', choice($._name, $.string))),
      kw('IN'),
      field('list', $._expression),
      optional(seq(kw('IF'), field('condition', $._expression))),
      ']',
    ),

    // A struct, as in {'a': 1, b: 2}.
    struct: $ => seq('{', optional(commaSep1($.struct_field)), '}'),

    struct_field: $ => seq(
      field('name', choice($._label, $.string)),
      ':',
      field('value', $._expression),
    ),

    // A map, as in MAP {'a': 1}. MAP is not a keyword here, so that map can
    // also name a column or a function, as in map(['a'], [1]).
    map: $ => seq(
      alias($.identifier, 'MAP'),
      '{',
      optional(commaSep1($.map_entry)),
      '}',
    ),

    map_entry: $ => seq(
      field('key', $._expression),
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

    // A lambda: x -> x + 1, (x, y) -> x + y, or lambda x, y: x + y. The
    // arrow is also the operator of JSON that extracts a field, as in
    // j -> '$.a', and DuckDB decides which one it is when it binds the
    // query.
    lambda_expression: $ => choice(
      prec.right(PREC.lambda, seq(
        field('parameters', $._expression),
        '->',
        field('body', $._expression),
      )),
      prec.right(PREC.lambda, seq(
        kw('LAMBDA'),
        commaSep1(field('parameter', choice($._name, $.string))),
        ':',
        field('body', $._expression),
      )),
    ),

    // A choice of operators flattens into one production for each
    // operator, and each production has parse states of its own. So each
    // level of precedence takes its operators from one token, which is the
    // node operator.
    unary_expression: $ => choice(
      prec.right(PREC.not, seq(field('operator', kw('NOT')), field('operand', $._expression))),
      prec.right(PREC.unary, seq(field('operator', alias($._sign, $.operator)), field('operand', $._expression))),
      prec.right(PREC.other, seq(field('operator', $.operator), field('operand', $._expression))),
      prec.left(PREC.postfix, seq(field('operand', $._primary_expression), field('operator', alias('!', $.operator)))),
    ),

    _sign: _ => token(choice('-', '+')),

    binary_expression: $ => {
      /** @type {[number, RuleOrLiteral, RuleOrLiteral][]} */
      const table = [
        [PREC.or, kw('OR'), $._expression],
        [PREC.and, kw('AND'), $._expression],
        [PREC.compare, alias($._comparison_operator, $.operator), $._operand],
        [PREC.other, $.operator, $._operand],
        [PREC.bit, alias($._bit_operator, $.operator), $._expression],
        [PREC.add, alias($._sign, $.operator), $._expression],
        [PREC.multiply, alias($._multiplicative_operator, $.operator), $._expression],
        [PREC.exponent, alias($._exponent_operator, $.operator), $._expression],
      ];
      return choice(...table.map(([p, op, right]) => prec.left(p, seq(
        field('left', $._expression),
        field('operator', op),
        field('right', right),
      ))));
    },

    // These tokens come before the token operator, so that the lexer takes
    // them when both match the same text, such as ** or <=.
    _comparison_operator: _ => token(choice('=', '==', '<', '>', '<=', '>=', '<>', '!=')),

    _bit_operator: _ => token(choice('&', '|', '<<', '>>')),

    _multiplicative_operator: _ => token(choice('*', '/', '//', '%')),

    _exponent_operator: _ => token(choice('^', '**')),

    // The operators of LIKE, ILIKE, GLOB and of the regular expressions in
    // the form of PostgreSQL.
    _like_symbol: _ => token(choice('~~', '~~*', '~~~', '~', '~*', '!~~', '!~~*', '!~*', '!~')),

    _operand: $ => choice($._expression, $.quantified_expression),

    // ANY, SOME or ALL after an operator compares the left side with each
    // row of a subquery or each element of a list.
    quantified_expression: $ => seq(
      field('quantifier', choice(kw('ANY'), kw('SOME'), kw('ALL'))),
      parens(choice($._query, $._expression)),
    ),

    // IS NULL, IS TRUE, IS DISTINCT FROM and the others, and ISNULL,
    // NOTNULL and NOT NULL after a value.
    is_expression: $ => choice(prec.left(PREC.is, choice(
      seq(
        field('left', $._expression),
        kw('IS'),
        optional(kw('NOT')),
        field('right', choice(
          $.boolean,
          $.null,
          kw('UNKNOWN'),
          $.distinct_from,
        )),
      ),
      seq(field('left', $._expression), choice(kw('ISNULL'), kw('NOTNULL'))),
    )),
    // NOT NULL after a value has the precedence of NOT LIKE, so that NOT
    // after an operand starts one of them with no conflict.
    prec.left(PREC.like, seq(field('left', $._expression), seq(kw('NOT'), $.null))),
    ),

    distinct_from: $ => seq(kws('DISTINCT', 'FROM'), $._expression),

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

    // The right side of IN is a list in parentheses, which is a row value,
    // a subquery, or a value that holds a list or a map, as in 2 IN l.
    in_expression: $ => prec.left(PREC.like, seq(
      field('left', $._expression),
      $._in_operator,
      field('right', $._expression),
    )),

    _in_operator: _ => seq(optional(kw('NOT')), kw('IN')),

    like_expression: $ => prec.left(PREC.like, seq(
      field('left', $._expression),
      field('operator', $._like_operator),
      field('right', $._operand),
      optional(seq(kw('ESCAPE'), field('escape', $._expression))),
    )),

    _like_operator: $ => choice(
      seq(
        optional(kw('NOT')),
        choice(kw('LIKE'), kw('ILIKE'), kw('GLOB'), kws('SIMILAR', 'TO')),
      ),
      alias($._like_symbol, $.operator),
    ),

    at_time_zone_expression: $ => prec.left(PREC.at, seq(
      field('value', $._expression),
      kws('AT', 'TIME', 'ZONE'),
      field('zone', $._expression),
    )),

    // The collation can have a dot, as in NOCASE.NOACCENT.
    collate_expression: $ => prec.left(PREC.collate, seq(
      field('value', $._expression),
      kw('COLLATE'),
      field('collation', choice($.object_reference, $.string)),
    )),

    // Types

    // A type: a name, with modifiers in parentheses, as in DECIMAL(10, 2),
    // ENUM('a', 'b') or STRUCT(a INTEGER, b VARCHAR), or a type of the
    // standard with keywords, such as DOUBLE PRECISION. [] or ARRAY after a
    // type gives a list, and [3] gives an array of that size.
    type: $ => prec.right(seq(
      choice(
        seq(
          optional($._qualifier),
          field('name', $._label),
          optional($.type_modifiers),
        ),
        seq(kw('UNION'), $.type_modifiers),
        seq(kw('SETOF'), $.type),
        seq(kw('DOUBLE'), optional(kw('PRECISION'))),
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
        seq(kw('INTERVAL'), optional($.interval_unit)),
      ),
      repeat(choice(
        seq('[', optional($.number), ']'),
        seq(kw('ARRAY'), optional(seq('[', $.number, ']'))),
      )),
    )),

    // The modifiers of a type: numbers, strings, types, and fields, which
    // have a name and a type.
    type_modifiers: $ => parens(optional(commaSep1(choice(
      $._signed_number,
      $.string,
      $.type,
      $.field_definition,
    )))),

    field_definition: $ => seq(
      field('name', choice($._name, $.string)),
      field('type', $.type),
    ),

    _signed_number: $ => seq(optional(alias($._sign, $.operator)), $.number),

    // Names and literals

    // A name of a table, with its schema and its database.
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

    // The schema before the name of a function or a type, with its
    // database, as in main.f or system.main.f.
    _qualifier: $ => choice(
      seq(field('schema', $._name), '.'),
      seq(field('database', $._name), '.', field('schema', $._label), '.'),
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

    // An identifier starts with a letter, _ or a character above 0x7f. A
    // name that starts with _ is also the placeholder of a variable of usql
    // (D101).
    identifier: _ => /[A-Za-z_\u0080-\u{10FFFF}][A-Za-z0-9_$\u0080-\u{10FFFF}]*/u,

    // A quoted identifier is in double quotes, with "" for a ", or in
    // backquotes, with `` for a `.
    _quoted_identifier: _ => token(choice(
      /"([^"]|"")*"/,
      /`([^`]|``)*`/,
    )),

    // An operator of the other operators, such as ||, ->>, @> and ^@: a
    // sequence of the characters + - * / < > = ~ ! @ % ^ & |. The operators
    // of the grammar, such as <= and **, are tokens of their own, and the
    // lexer takes them before this token when both match. A sequence of
    // only + - * < > = holds no /, so that the lexer does not read the start
    // of a comment, as in 1*/**/2, as an operator.
    operator: _ => token(choice(
      /[+\-*/<>=]*[~!@%^&|][+\-*/<>=~!@%^&|]*/,
      /[+\-*<>=]+[*<>=]/,
    )),

    // A number is an integer or a decimal number, with an exponent or not.
    // An underscore can separate two digits.
    number: _ => token(choice(
      /[0-9](_?[0-9])*(\.([0-9](_?[0-9])*)?)?([eE][+-]?[0-9](_?[0-9])*)?/,
      /\.[0-9](_?[0-9])*([eE][+-]?[0-9](_?[0-9])*)?/,
    )),

    // A string is in single quotes, with a quote doubled. E before it
    // allows backslash escapes. Two strings with a newline between them are
    // one string.
    string: _ => {
      const plain = /'([^']|'')*'/;
      const escaped = /[eE]'([^'\\]|\\(.|\n)|'')*'/;
      return token(seq(
        choice(escaped, plain),
        repeat(seq(/[ \t\f\r]*\n\s*/, choice(plain, escaped))),
      ));
    },

    // A hex string, as in X'CAFE', gives a BLOB.
    blob: _ => /[xX]'[^']*'/,

    // A bit string, as in B'0101', gives a BITSTRING.
    bit_string: _ => /[bB]'[^']*'/,

    dollar_quoted_string: $ => seq(
      $._dollar_quote_start,
      optional($.string_content),
      $._dollar_quote_end,
    ),
  },
});
