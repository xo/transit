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
// ClickHouse reserves almost no words, and the grammar reserves none (D131),
// so a column can have the name of a keyword that starts a clause, such as
// from. Where a keyword and a name can both come, the keyword wins, so FROM
// after a column of SELECT starts the clause FROM. A word after AS or after
// a dot can be any keyword, and a function name can be a word such as and,
// or, in or like, because ClickHouse has functions with these names. Only an
// alias with no AS takes a reserved set, the words that ClickHouse does not
// take as such an alias, and SELECT (D132).
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
// The statements that are not queries and do not change data or schema,
// such as DROP, SHOW, KILL and BACKUP, end with the clauses of _tail, in
// any order. The parse states after an expression are copied for each set
// of the tokens that can come after it, so one set for all these statements
// keeps the copies few. The commands of SYSTEM and the forms of SHOW are
// many, so each of the two statements takes its words in any order.
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
// loosely, and then the arrow of a lambda and the ternary operator ?:. A
// clause of CREATE that ends with an expression, such as ORDER BY, binds
// more tightly than an alias, so that AS after it starts the query of
// CREATE TABLE ... AS SELECT. NOT
// binds more loosely than IS NULL, IS NULL more loosely than BETWEEN, and
// BETWEEN more loosely than a comparison, LIKE and IN. || binds more loosely
// than + and -. A sign binds more tightly than * and /, and a cast, a
// subscript and a dot bind the most tightly.
const PREC = {
  alias: 1,
  clause: 2,
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

// The words that ClickHouse does not take as an alias with no AS, the list
// restricted_keywords of ParserAlias in the parser of ClickHouse, and SELECT
// (D132). So in SELECT x FROM t, FROM is not an alias of x, and in SELECT 1
// SELECT 2, the second SELECT starts a statement. ClickHouse does not take
// FROM after a column as such an alias either. A word is in the reserved set
// only when the grammar uses it as a keyword, because a reserved word must
// be a token of the grammar.
const IMPLICIT_ALIAS_WORDS = [
  'ALL', 'ANTI', 'ANY', 'ARRAY', 'ASOF', 'BETWEEN', 'CROSS', 'PASTE', 'FETCH',
  'FINAL', 'FORMAT', 'FROM', 'FULL', 'GLOBAL', 'GROUP', 'HAVING', 'ILIKE',
  'INNER', 'INTO', 'JOIN', 'LEFT', 'LIKE', 'LIMIT', 'NATURAL', 'NOT', 'OFFSET',
  'ON', 'ONLY', 'ORDER', 'PARALLEL', 'PREWHERE', 'RIGHT', 'SAMPLE', 'SEMI',
  'SETTINGS', 'SIMILAR', 'STREAM', 'UNION', 'USING', 'WHERE', 'WINDOW',
  'QUALIFY', 'WITH', 'INTERSECT', 'EXCEPT', 'ELSE', 'SELECT',
];

// The words of the commands of SYSTEM and of the forms of SHOW that no
// other statement uses, such as FLUSH, LOGS and PROCESSLIST. One token, the
// node keyword, covers all of them, so that they take one column of each
// large parse state and not one column each. This saves 88 symbols and
// about 60 KB of the tables. A word that another rule uses as a keyword
// must not be in the list, because two keywords cannot match one word.
const COMMAND_WORDS = [
  'ACCESS', 'ASYNCHRONOUS', 'AVRO', 'CACHE', 'CACHES', 'CANCEL', 'CHANGED',
  'CLIENT', 'CLUSTERS', 'COLUMNS', 'COMPILED', 'CONDITION', 'CONFIG',
  'CONNECTIONS', 'DISABLE', 'DISTRIBUTED', 'DNS', 'EMBEDDED', 'ENABLE',
  'ENABLED', 'ENCRYPTION', 'ENGINES', 'ENTRY', 'EXTENDED', 'FAILPOINT',
  'FETCHES', 'FIELDS', 'FILE', 'FILELOG', 'FILESYSTEM', 'FLUSH', 'FUNCTIONS',
  'HEADERS', 'ICEBERG', 'INDEXES', 'INDICES', 'INSTRUMENT', 'KEYS',
  'LIGHTWEIGHT', 'LISTEN', 'LOAD', 'LOADING', 'LOG', 'LOGS', 'MARK',
  'MERGES', 'METADATA', 'METRICS', 'MMAP', 'MOVES', 'PAGE', 'PAIMON',
  'PARQUET', 'PAUSE', 'POINT', 'POLICIES', 'POLYGON', 'PREWARM',
  'PRIVILEGES', 'PROCESSLIST', 'PULL', 'PULLING', 'QUERIES', 'QUEUE',
  'QUEUES', 'QUOTAS', 'RELOAD', 'REPLICA', 'REPLICAS', 'REPLICATION',
  'RESTART', 'ROLES', 'SCHEMA', 'SENDS', 'SERIES', 'SHARD', 'SHUTDOWN',
  'SIMILARITY', 'SLEEP', 'START', 'STOP', 'STORAGE', 'TEXT', 'UNCOMPRESSED',
  'UNLOAD', 'USERS', 'VECTOR', 'VIEWS', 'WAIT', 'ZKPATH',
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
    $.insert_data,
  ],

  word: $ => $.identifier,

  // The reserved set global is empty, because the grammar reserves no word
  // (D131). The set implicit_alias holds the words that an alias with no AS
  // cannot be (D132).
  reserved: {
    global: _ => [],
    implicit_alias: _ => IMPLICIT_ALIAS_WORDS.filter(w => KEYWORDS.has(w)).map(keyword),
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
    // SETTINGS after a column of ALTER TABLE can start the settings of the
    // column, as in SETTINGS (max_compress_block_size = 1024), or the
    // settings of the statement.
    [$.column_definition],
    // A comma after the partitions of an item of BACKUP can start the next
    // partition or the next item.
    [$.backup_item],
    // A comma after the limits of a quota can start the next limit or the
    // next interval.
    [$.quota_interval],
    // COMMENT after an expression of the query of CREATE can be an alias
    // with no AS or start the comment of the table (D133).
    [$._implicit_alias, $._select_item],
    [$._implicit_alias, $.array_join],
    [$.table_reference],
    [$.function_table],
    [$.derived_table],
    [$.parenthesized_join],
    [$.array_join],
  ],

  rules: {
    // An INSERT with data after FORMAT ends at the end of its line, so it
    // needs no semicolon after it.
    source_file: $ => seq(
      repeat(choice(
        seq(choice($._statement, $.parallel_statement), ';'),
        ';',
        alias($._insert_with_data, $.insert_statement),
      )),
      optional(choice($._statement, $.parallel_statement)),
    ),

    // PARALLEL WITH runs two or more statements at the same time, as in
    // DROP TABLE a PARALLEL WITH DROP TABLE b (D133). The keywords after
    // each statement put 46 more parse states in the large form. One token
    // for the two words cost 734 more states, because the token and an
    // identifier can match the same text.
    parallel_statement: $ => seq(
      $._statement,
      repeat1(seq(kws('PARALLEL', 'WITH'), $._statement)),
    ),

    _statement: $ => choice(
      $.select_statement,
      $.explain_statement,
      $.optimize_statement,
      $._explained_statement,
    ),

    // The statements that EXPLAIN takes. OPTIMIZE is not one of them,
    // because optimize also names a setting of EXPLAIN, as in EXPLAIN
    // optimize = 0 SELECT 1.
    _explained_statement: $ => choice(
      $.insert_statement,
      $.delete_statement,
      $.update_statement,
      $.truncate_statement,
      $.create_database_statement,
      $.create_table_statement,
      $.create_view_statement,
      $.create_dictionary_statement,
      $.create_function_statement,
      $.create_named_collection_statement,
      $.create_index_statement,
      $.create_projection_statement,
      $.alter_table_statement,
      $.alter_named_collection_statement,
      $.drop_statement,
      $.detach_statement,
      $.undrop_statement,
      $.rename_statement,
      $.exchange_statement,
      $.show_statement,
      $.describe_statement,
      $.exists_statement,
      $.check_statement,
      $.kill_statement,
      $.system_statement,
      $.set_statement,
      $.set_role_statement,
      $.use_statement,
      $.grant_statement,
      $.revoke_statement,
      $.create_access_statement,
      $.alter_access_statement,
      $.move_statement,
      $.watch_statement,
      $.backup_statement,
      $.restore_statement,
      $.transaction_statement,
      $.execute_as_statement,
      $.create_workload_statement,
      $.create_resource_statement,
      $.create_handler_statement,
      $.create_token_statement,
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
          choice($.select_statement, $.explain_statement, $._explained_statement),
        ),
        kws('CURRENT', 'TRANSACTION'),
      ),
    ),

    // The value of a setting of EXPLAIN is a literal or a name, because a
    // query in parentheses can come after it.
    _explain_setting: $ => seq(
      field('name', $._label),
      '=',
      field('value', choice($._literal, $._name, alias($._negative_number, $.unary_expression))),
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

    // SELECT, with DISTINCT ON and TOP. A comma can come after the last
    // item, before FROM (D131). The comma and FROM are in this rule, and
    // not a comma after the items, so that only FROM can come after the
    // comma. A comma at the end of the items cost 4 more large parse
    // states, and it took the word after it, such as where, as a keyword.
    _select_clause: $ => prec.right(seq(
      kw('SELECT'),
      optional(choice(kw('ALL'), $.distinct_clause)),
      optional($.top_clause),
      commaSep1($._select_item),
      optional(seq(',', $.from_clause)),
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

    _implicit_alias: $ => seq(field('value', $._expression), field('alias', $._implicit_alias_name)),

    // An alias with no AS cannot be a word of the reserved set
    // implicit_alias, but it can be such a word in quotes. The alias can be
    // comment, although COMMENT can come after the query of CREATE (D133).
    // Where both can come, the parser tries both, and the string after
    // COMMENT tells them apart. This costs 70 parse states. A token for
    // COMMENT and its string cost about 900.
    _implicit_alias_name: $ => choice(
      reserved('implicit_alias', $.identifier),
      alias($._quoted_identifier, $.identifier),
      alias(keyword('COMMENT'), $.identifier),
    ),

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
    table_reference: $ => seq(
      field('table', $.object_reference),
      optional($._alias),
      optional(kw('FINAL')),
      optional($.sample_clause),
    ),

    // An alias of a table, with the names of its columns.
    _alias: $ => seq(
      choice(
        seq(kw('AS'), field('alias', $._label)),
        field('alias', $._implicit_alias_name),
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
    function_table: $ => seq(
      field('function', alias($._table_function, $.function_call)),
      optional($._alias),
      optional(kw('FINAL')),
      optional($.sample_clause),
    ),

    _table_function: $ => seq(
      field('name', $._function_name),
      $.arguments,
    ),

    // A subquery in FROM. EXPLAIN in parentheses is a subquery too, which
    // gives the lines of the plan as rows. The grammar takes it only in
    // FROM, because EXPLAIN after each ( where an expression starts puts
    // 12 more parse states in the large form.
    derived_table: $ => seq(
      choice($.subquery, alias(parens($.explain_statement), $.subquery)),
      optional($._alias),
    ),

    // A list of tables and joins in parentheses.
    parenthesized_join: $ => seq(
      parens(commaSep1($._table_reference)),
      optional($._alias),
    ),

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
    array_join: $ => seq(
      field('left', $._table_reference),
      optional(choice(kw('LEFT'), kw('INNER'))),
      kws('ARRAY', 'JOIN'),
      commaSep1(choice($._expression, alias($._implicit_alias, $.aliased_expression))),
    ),

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
    order_by_item: $ => prec.right(PREC.clause, seq(
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

    // SETTINGS changes settings for one query. The settings of a
    // dictionary are in parentheses.
    settings_clause: $ => prec.right(seq(kw('SETTINGS'), choice(commaSep1($.setting), parens(commaSep1($.setting))))),

    // A setting with no value is on, as in SETTINGS log_queries.
    setting: $ => prec.right(PREC.clause, seq(
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

    // Changes of data

    // INSERT adds rows to a table or to a table function. The rows come
    // from VALUES, from a query, from a file of the client after FROM
    // INFILE, or as data in a format after FORMAT, as in INSERT INTO t
    // FORMAT CSV 1,2. The client of ClickHouse reads the data of a format
    // to the end of the line, and the external scanner reads it as one
    // token. SETTINGS can come before VALUES, FORMAT and the query.
    insert_statement: $ => seq(
      $._insert_target,
      optional($.insert_columns),
      choice(
        seq(
          optional(alias($._insert_settings, $.settings_clause)),
          choice($.values_clause, $.format_clause, $._query),
        ),
        seq($.infile_clause, optional($.settings_clause), optional($.format_clause)),
      ),
    ),

    // The settings of INSERT. A query can come after them, so the value
    // of a setting is a literal or a name, as in EXPLAIN. SETTINGS before
    // the query costs 13 parse states (D132). With any expression as the
    // value, it cost about 1,050 states, because the states after the
    // expression were copied for the words that start a query.
    _insert_settings: $ => seq(kw('SETTINGS'), commaSep1(alias($._insert_setting, $.setting))),

    // A setting with no value is on.
    _insert_setting: $ => seq(
      field('name', $._label),
      optional(seq('=', field('value', choice($._literal, $._name, alias($._negative_number, $.unary_expression))))),
    ),

    // An INSERT with data after FORMAT.
    _insert_with_data: $ => seq(
      $._insert_target,
      optional($.insert_columns),
      optional(alias($._insert_settings, $.settings_clause)),
      $.format_clause,
      $.insert_data,
    ),

    // The parts of INSERT before the columns are a rule of their own, so
    // that the parse states after them are not copied for each choice.
    _insert_target: $ => seq(
      kws('INSERT', 'INTO'),
      optional(kw('TABLE')),
      choice(
        field('table', $.object_reference),
        seq(kw('FUNCTION'), field('function', alias($._table_function, $.function_call))),
      ),
    ),

    // The columns of INSERT. A star with EXCEPT, or COLUMNS, can give them,
    // as in (* EXCEPT (a)).
    insert_columns: $ => parens(commaSep1(choice($.column_reference, $.star_expression, $.columns_expression))),

    // The rows of VALUES are in parentheses, and the comma between two rows
    // can be left out. A comma can come after the last row. The rows can
    // be left out, when the client sends them in another way.
    values_clause: $ => seq(
      kw('VALUES'),
      optional(seq(
        $._values_row,
        repeat(seq(optional(','), $._values_row)),
        optional(','),
      )),
    ),

    _values_row: $ => choice(
      alias($.tuple, $.row),
      alias($.parenthesized_expression, $.row),
    ),

    // FROM INFILE reads the rows from a file of the client, which can be
    // compressed.
    infile_clause: $ => seq(
      kws('FROM', 'INFILE'),
      field('file', $.string),
      optional(seq(kw('COMPRESSION'), field('compression', $.string))),
    ),

    // DELETE FROM marks rows as deleted, which a merge removes later.
    delete_statement: $ => seq(
      kws('DELETE', 'FROM'),
      field('table', $.object_reference),
      optional($.on_cluster),
      optional($.in_partition),
      $.where_clause,
      optional($.settings_clause),
    ),

    // UPDATE changes the values of the rows that match its condition. The
    // grammar does not take IN PARTITION after the assignments, because IN
    // after the value of an assignment starts the operator IN.
    update_statement: $ => seq(
      kw('UPDATE'),
      field('table', $.object_reference),
      optional($.on_cluster),
      kw('SET'),
      commaSep1($.assignment),
      $.where_clause,
      optional($.settings_clause),
    ),

    // An assignment of UPDATE and of TTL, as in a = a + 1.
    assignment: $ => seq(
      field('column', $._path),
      '=',
      field('value', $._expression),
    ),

    // TRUNCATE removes all the rows of a table, of the tables of a
    // database, or of the tables whose names match a pattern.
    truncate_statement: $ => choice(
      seq(
        kw('TRUNCATE'),
        optional(kw('TEMPORARY')),
        optional(choice(kw('TABLE'), kw('DATABASE'))),
        optional($._if_exists),
        field('name', $.object_reference),
        optional($.on_cluster),
        optional(kw('SYNC')),
        optional($.settings_clause),
      ),
      seq(
        kw('TRUNCATE'),
        optional(kw('ALL')),
        kws('TABLES', 'FROM'),
        optional($._if_exists),
        field('database', choice($._name, $.query_parameter)),
        optional(seq(optional(kw('NOT')), choice(kw('LIKE'), kw('ILIKE')), field('pattern', $.string))),
        optional($.on_cluster),
      ),
    ),

    // ON CLUSTER runs a statement on each server of a cluster. The name of
    // the cluster can be a string, as in '{cluster}'.
    on_cluster: $ => seq(kws('ON', 'CLUSTER'), field('cluster', choice($._label, $.string))),

    // IN PARTITION limits a change to one partition.
    in_partition: $ => seq(kw('IN'), $.partition),

    // A partition: PARTITION with the value of the partition key, with ID
    // and the name of the partition, or with ALL, or PART with the name of
    // a part.
    partition: $ => choice(
      seq(kw('PARTITION'), choice(
        seq(kw('ID'), field('id', choice($.string, $.query_parameter))),
        kw('ALL'),
        field('value', $._expression),
      )),
      seq(kw('PART'), field('value', $._expression)),
    ),

    _if_exists: _ => kws('IF', 'EXISTS'),

    _if_not_exists: _ => kws('IF', 'NOT', 'EXISTS'),

    // Changes of schema

    // CREATE, CREATE OR REPLACE or REPLACE. ATTACH takes the forms of
    // CREATE too, so it is here, as in ATTACH TABLE t, which attaches a
    // table that DETACH detached, or ATTACH TABLE t FROM 'path' (a UInt8).
    _create: _ => choice(
      seq(kw('CREATE'), optional(kws('OR', 'REPLACE'))),
      kw('REPLACE'),
      kw('ATTACH'),
    ),

    // CREATE DATABASE, with its engine, settings and comment.
    create_database_statement: $ => seq(
      $._create_database_head,
      repeat($._create_option),
    ),

    _create_database_head: $ => seq(
      $._create,
      kw('DATABASE'),
      optional($._if_not_exists),
      field('name', choice($._name, $.query_parameter)),
      optional($.uuid),
      optional($.on_cluster),
    ),

    // CREATE TABLE. The table takes its columns from the list of its
    // elements, from another table or table function after AS, or from a
    // query after AS. The clauses after the elements, such as ENGINE and
    // ORDER BY, come in any order.
    create_table_statement: $ => seq(
      $._create_table_head,
      optional($.table_elements),
      repeat($._create_option),
      optional(seq($._as_query, optional($.comment_clause))),
    ),

    _create_table_head: $ => seq(
      $._create,
      optional(kw('TEMPORARY')),
      kw('TABLE'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      optional($.uuid),
      optional($.on_cluster),
      optional(seq(kw('FROM'), field('path', $.string))),
    ),

    // UUID gives the UUID of a table, as ATTACH and a replica use it.
    uuid: $ => seq(kw('UUID'), field('value', $.string)),

    // The columns, indexes, projections and constraints of a table, or the
    // attributes of a dictionary. A comma can come after the last one.
    table_elements: $ => parens(trailingSep1(choice(
      $.column_definition,
      $.index_definition,
      $.projection_definition,
      $.constraint_definition,
      alias($._primary_key_element, $.primary_key),
    ))),

    // A column, with its type and its options in any order. The type can
    // be left out when DEFAULT, MATERIALIZED or ALIAS gives the value. The
    // options EXPRESSION, HIERARCHICAL, INJECTIVE and IS_OBJECT_ID are for
    // the attributes of a dictionary.
    column_definition: $ => seq(
      field('name', $._path),
      optional(field('type', $.type)),
      repeat($._column_option),
    ),

    // AUTO_INCREMENT is for the compatibility with MySQL, and ClickHouse
    // ignores it.
    _column_option: $ => choice(
      seq(optional(kw('NOT')), $.null),
      seq(
        choice(kw('DEFAULT'), kw('MATERIALIZED'), kw('ALIAS'), kw('EXPRESSION')),
        field('default', $._expression),
      ),
      prec.right(seq(kw('EPHEMERAL'), optional(field('default', $._expression)))),
      seq(kw('COMMENT'), field('comment', $.string)),
      $.codec,
      seq(kw('STATISTICS'), $.arguments),
      seq(kw('TTL'), field('ttl', $._expression)),
      kws('PRIMARY', 'KEY'),
      seq(kw('SETTINGS'), parens(commaSep1($.setting))),
      kw('HIERARCHICAL'),
      kw('BIDIRECTIONAL'),
      kw('INJECTIVE'),
      kw('IS_OBJECT_ID'),
      kw('AUTO_INCREMENT'),
    ),

    // CODEC gives the compression of a column, as in CODEC(Delta, ZSTD(3)).
    codec: $ => seq(kw('CODEC'), $.arguments),

    // A skipping index, as in INDEX i a TYPE minmax GRANULARITY 4.
    index_definition: $ => seq(
      kw('INDEX'),
      $._index_body,
    ),

    _index_body: $ => seq(
      optional($._if_not_exists),
      field('name', $._name),
      field('expression', $._expression),
      kw('TYPE'),
      field('type', $._expression),
      optional(seq(kw('GRANULARITY'), field('granularity', $.number))),
    ),

    // A projection keeps the rows of a table in another order, or the
    // results of an aggregation, as in PROJECTION p (SELECT a ORDER BY b).
    projection_definition: $ => seq(
      kw('PROJECTION'),
      $._projection_body,
    ),

    _projection_body: $ => seq(
      optional($._if_not_exists),
      field('name', $._name),
      choice(
        alias(parens($._query), $.subquery),
        seq(kw('INDEX'), field('expression', $._expression), kw('TYPE'), field('type', $._expression)),
      ),
      optional(seq(kws('WITH', 'SETTINGS'), parens(commaSep1($.setting)))),
    ),

    // A constraint, which CHECK tests on each insert and ASSUME gives to
    // the optimizer.
    constraint_definition: $ => seq(
      kw('CONSTRAINT'),
      $._constraint_body,
    ),

    _constraint_body: $ => seq(
      optional($._if_not_exists),
      field('name', $._name),
      choice(kw('CHECK'), kw('ASSUME')),
      field('condition', $._expression),
    ),

    // PRIMARY KEY after the elements, and in a dictionary, where a comma
    // can come between two columns.
    primary_key: $ => prec.right(PREC.clause, seq(kws('PRIMARY', 'KEY'), commaSep1($._expression))),

    // PRIMARY KEY in the elements, where a comma starts the next element.
    _primary_key_element: $ => seq(kws('PRIMARY', 'KEY'), $._expression),

    // The clauses of CREATE after the elements, in any order. They are the
    // clauses of a table, of a view, of a database and of a dictionary in
    // one choice, so that their parse states are shared.
    _create_option: $ => choice(
      $.engine_clause,
      $.order_by_clause,
      $.partition_by_clause,
      $.primary_key,
      $.sample_by_clause,
      $.ttl_clause,
      $.settings_clause,
      $.comment_clause,
      $.as_clause,
      $.to_clause,
      $.refresh_clause,
      $.source_clause,
      $.layout_clause,
      $.lifetime_clause,
      $.range_clause,
      $.format_clause,
      prec.right(seq(kw('APPEND'), optional(kw('INCREMENTAL')), optional($.table_elements))),
      kw('POPULATE'),
      $._live_view_refresh,
      kw('EMPTY'),
      seq(kw('DEFINER'), '=', field('definer', $._label)),
      seq(kws('SQL', 'SECURITY'), choice(kw('DEFINER'), kw('INVOKER'), kw('NONE'))),
      prec(PREC.clause, seq(choice(kw('WATERMARK'), kw('ALLOWED_LATENESS')), '=', $._expression)),
    ),

    // The engine of a table or of a database, with its arguments, as in
    // ENGINE = ReplicatedMergeTree('/path', '{replica}').
    engine_clause: $ => seq(
      kw('ENGINE'),
      optional('='),
      field('name', $._label),
      optional(alias($._engine_arguments, $.arguments)),
    ),

    // The arguments of an engine are expressions. ALL is a name here, as in
    // Join(ALL, LEFT, k), and not the ALL before the arguments of a
    // function.
    _engine_arguments: $ => parens(optional(commaSep1($._expression))),

    partition_by_clause: $ => prec(PREC.clause, seq(kws('PARTITION', 'BY'), $._expression)),

    sample_by_clause: $ => prec(PREC.clause, seq(kws('SAMPLE', 'BY'), $._expression)),

    // TTL removes rows, moves them to a disk or a volume, compresses them
    // again or aggregates them when their time comes.
    ttl_clause: $ => prec.right(seq(kw('TTL'), commaSep1($.ttl_item))),

    ttl_item: $ => prec.right(PREC.clause, seq(
      field('expression', $._expression),
      repeat(choice(
        kw('DELETE'),
        seq(kw('RECOMPRESS'), $.codec),
        seq(kw('TO'), choice(kw('DISK'), kw('VOLUME')), optional($._if_exists), $.string),
        seq(kw('WHERE'), field('condition', $._expression)),
      )),
      optional($._ttl_group_by),
    )),

    // GROUP BY ends a TTL item, so that a comma after it continues the
    // keys or the assignments.
    _ttl_group_by: $ => prec.right(PREC.clause, seq(
      kws('GROUP', 'BY'),
      commaSep1($._expression),
      optional(seq(kw('SET'), commaSep1($.assignment))),
    )),

    comment_clause: $ => seq(kw('COMMENT'), field('comment', $.string)),

    // AS takes the columns from another table or from a table function.
    // CLONE AS also copies the data.
    as_clause: $ => seq(
      optional(kw('CLONE')),
      kw('AS'),
      choice(
        field('table', $.object_reference),
        field('function', alias($._table_function, $.function_call)),
        seq(optional(kw('NOT')), kw('REPLICATED')),
      ),
    ),

    // AS with a query comes after the other clauses. The query in the
    // choice of the clauses cost about 940 parse states, because each state
    // of the query then took each word that starts a clause. COMMENT can
    // come after the query (D132). It costs 8 parse states, because the
    // states after the query merge with the states of a query alone. So
    // COMMENT after an expression of any query is the keyword, and
    // _implicit_alias_name takes the keyword as an alias too (D133). EMPTY
    // AS creates the table from a query with no rows.
    _as_query: $ => alias(seq(kw('AS'), $._query), $.as_clause),

    // WITH REFRESH and WITH TIMEOUT of a live view, with a number of
    // seconds (D132). As in an interval of REFRESH, the value is a number
    // or a name, so that it costs 10 parse states and not about 950.
    _live_view_refresh: $ => prec.right(seq(
      kw('WITH'),
      choice(
        seq(
          kw('TIMEOUT'),
          optional(field('timeout', choice($.number, $._name))),
          optional(seq(kw('AND'), kw('REFRESH'), optional(field('refresh', choice($.number, $._name))))),
        ),
        seq(kw('REFRESH'), optional(field('refresh', choice($.number, $._name)))),
      ),
    )),

    // TO gives the table where a materialized view writes its rows.
    to_clause: $ => seq(kw('TO'), field('table', $.object_reference), optional($.table_elements)),

    // REFRESH runs the query of a refreshable materialized view again at a
    // time, as in REFRESH EVERY 1 DAY OFFSET 2 HOUR. The columns of the
    // view can come after it. SETTINGS and APPEND after it are clauses of
    // CREATE, and the columns can come after APPEND.
    refresh_clause: $ => prec.right(seq(
      kw('REFRESH'),
      optional(seq(choice(kw('EVERY'), kw('AFTER')), $._refresh_interval)),
      repeat(choice(
        seq(kw('OFFSET'), $._refresh_interval),
        seq(kws('RANDOMIZE', 'FOR'), $._refresh_interval),
        prec.right(seq(kws('DEPENDS', 'ON'), commaSep1($.object_reference))),
      )),
      optional($.table_elements),
    )),

    // An interval of REFRESH can have more than one unit, as in EVERY 1 HOUR
    // 30 MINUTE (D132). The value of a unit after the first one is a number
    // or a name, which costs 4 parse states. A number alone cost about 920
    // states. Then the tool found a state where a number can come and an
    // identifier cannot, and the two tokens can match the same text, such
    // as 1e5, so it did not merge states that it merges now.
    _refresh_interval: $ => seq(
      $.number,
      $.interval_unit,
      repeat(seq(choice($.number, $._name), $.interval_unit)),
    ),

    // CREATE VIEW, MATERIALIZED VIEW, LIVE VIEW and WINDOW VIEW.
    create_view_statement: $ => seq(
      $._create_view_head,
      optional($.table_elements),
      repeat($._create_option),
      optional(seq($._as_query, optional($.comment_clause))),
    ),

    _create_view_head: $ => seq(
      $._create,
      optional(kw('TEMPORARY')),
      optional(choice(kw('MATERIALIZED'), kw('LIVE'), kw('WINDOW'))),
      kw('VIEW'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      optional($.uuid),
      optional($.on_cluster),
    ),

    // CREATE DICTIONARY, with its attributes, source, layout, lifetime and
    // range.
    create_dictionary_statement: $ => seq(
      $._create_dictionary_head,
      optional($.table_elements),
      repeat($._create_option),
    ),

    _create_dictionary_head: $ => seq(
      $._create,
      kw('DICTIONARY'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      optional($.uuid),
      optional($.on_cluster),
    ),

    // SOURCE and LAYOUT take a name and its parameters, as in
    // SOURCE(CLICKHOUSE(TABLE 'src' DB 'default')).
    source_clause: $ => seq(kw('SOURCE'), parens($._dictionary_function)),

    layout_clause: $ => seq(kw('LAYOUT'), parens($._dictionary_function)),

    _dictionary_function: $ => seq(field('name', $._label), optional($.dictionary_parameters)),

    // The parameters are pairs of a name and a value, with no comma or with
    // a comma between them. A value can hold parameters of its own, as in
    // headers(header(name 'a' value 'b')).
    dictionary_parameters: $ => parens(repeat(seq($.dictionary_parameter, optional(',')))),

    dictionary_parameter: $ => seq(
      field('name', $._label),
      field('value', choice(
        $._literal,
        $._name,
        alias($._table_function, $.function_call),
        $.array,
        $.dictionary_parameters,
      )),
    ),

    // LIFETIME gives how often the dictionary is loaded again, in seconds,
    // as a number or with MIN and MAX.
    lifetime_clause: $ => seq(kw('LIFETIME'), parens(choice(
      field('max', $.number),
      repeat1(seq(choice(kw('MIN'), kw('MAX')), $.number)),
    ))),

    // RANGE gives the columns of the start and of the end of a range.
    range_clause: $ => seq(kw('RANGE'), parens(repeat1(seq(choice(kw('MIN'), kw('MAX')), $._name)))),

    // CREATE FUNCTION gives a name to a lambda.
    create_function_statement: $ => seq(
      $._create,
      kw('FUNCTION'),
      optional($._if_not_exists),
      field('name', $._name),
      optional($.on_cluster),
      kw('AS'),
      field('body', $._expression),
    ),

    // CREATE NAMED COLLECTION keeps pairs of a key and a value, such as the
    // settings of a connection, that a table or a table function can use.
    create_named_collection_statement: $ => seq(
      $._create,
      kws('NAMED', 'COLLECTION'),
      optional($._if_not_exists),
      field('name', $._name),
      optional($.on_cluster),
      kw('AS'),
      commaSep1($.collection_item),
    ),

    // NOT OVERRIDABLE stops a query from changing the value.
    collection_item: $ => seq(
      field('key', $._label),
      '=',
      field('value', choice($._literal, alias($._negative_number, $.unary_expression))),
      optional(seq(optional(kw('NOT')), kw('OVERRIDABLE'))),
    ),

    // CREATE INDEX adds a skipping index to a table, as ALTER TABLE ADD
    // INDEX does. As in ClickHouse, the columns in parentheses can take ASC
    // or DESC, as in the CREATE INDEX of SQL, and ClickHouse ignores them.
    // ASC and DESC cost about 310 parse states, because the states after an
    // expression in the parentheses are copied for them (D132). With no
    // parentheses, the index takes a column or a function call. A
    // hypothetical index only tells EXPLAIN what the index would do.
    create_index_statement: $ => seq(
      kw('CREATE'),
      optional(kw('HYPOTHETICAL')),
      kw('INDEX'),
      optional($._if_not_exists),
      field('name', $._name),
      kw('ON'),
      field('table', $.object_reference),
      field('expression', choice($.index_columns, $.column_reference, $.function_call)),
      optional(seq(kw('TYPE'), field('type', $._expression))),
      optional(seq(kw('GRANULARITY'), field('granularity', $.number))),
    ),

    // CREATE HYPOTHETICAL PROJECTION tells EXPLAIN what a projection would
    // do.
    create_projection_statement: $ => seq(
      kws('CREATE', 'HYPOTHETICAL', 'PROJECTION'),
      optional($._if_not_exists),
      field('name', $._name),
      kw('ON'),
      field('table', $.object_reference),
      choice(
        alias(parens($._query), $.subquery),
        seq(kw('INDEX'), field('expression', $._expression), kw('TYPE'), field('type', $._expression)),
      ),
      optional(seq(kws('WITH', 'SETTINGS'), parens(commaSep1($.setting)))),
    ),

    index_columns: $ => parens(commaSep1(seq($._expression, optional($._direction)))),

    // ALTER TABLE and ALTER DATABASE take one or more actions, with a comma
    // between two of them, and SETTINGS and FORMAT after them. MODIFY QUERY
    // changes the query of a view, and it is the only action, because a
    // comma can come in the query.
    alter_table_statement: $ => seq(
      $._alter_table_head,
      choice(
        seq(commaSep1($._alter_action), repeat(choice($.settings_clause, $.format_clause))),
        alias($._modify_query, $.modify_action),
      ),
    ),

    _modify_query: $ => seq(kws('MODIFY', 'QUERY'), $._query),

    _alter_table_head: $ => seq(
      kw('ALTER'),
      choice(
        seq(optional(kw('TEMPORARY')), kw('TABLE')),
        kw('DATABASE'),
      ),
      optional($._if_exists),
      field('name', $.object_reference),
      optional($.on_cluster),
    ),

    // The actions of ALTER TABLE. They change the columns, the indexes,
    // the projections, the constraints, the statistics, the keys, the TTL,
    // the settings, the comment, the query of a view, the rows and the
    // partitions.
    _alter_action: $ => choice(
      $.add_action,
      $.drop_action,
      $.modify_action,
      $.rename_column_action,
      $.comment_column_action,
      $.alter_column_action,
      $.update_action,
      $.delete_action,
      $.partition_action,
      $.reset_setting_action,
      $.remove_action,
      $.execute_action,
    ),

    // ADD COLUMN, ADD INDEX, ADD PROJECTION, ADD CONSTRAINT and ADD
    // STATISTICS. FIRST or AFTER gives the place of a new column or index.
    add_action: $ => prec.right(seq(
      kw('ADD'),
      choice(
        seq(kw('COLUMN'), optional($._if_not_exists), $.column_definition, optional($._column_position)),
        seq(kw('INDEX'), $._index_body, optional($._column_position)),
        seq(kw('PROJECTION'), $._projection_body),
        seq(kw('CONSTRAINT'), $._constraint_body),
        seq(kw('STATISTICS'), $._statistics_body),
      ),
    )),

    _column_position: $ => choice(kw('FIRST'), seq(kw('AFTER'), field('after', $._path))),

    _statistics_body: $ => prec.right(seq(
      optional($._if_not_exists),
      commaSep1(field('column', $._name)),
      kw('TYPE'),
      commaSep1(field('type', $._expression)),
    )),

    // DROP, CLEAR and MATERIALIZE of a column, an index, a projection, a
    // constraint or statistics. CLEAR and MATERIALIZE can take IN
    // PARTITION.
    drop_action: $ => choice(
      seq(
        choice(kw('DROP'), kw('CLEAR'), kw('MATERIALIZE')),
        choice(kw('COLUMN'), kw('INDEX'), kw('PROJECTION'), kw('CONSTRAINT')),
        optional($._if_exists),
        field('name', $._path),
        optional($.in_partition),
      ),
      prec.right(seq(
        choice(kw('DROP'), kw('CLEAR'), kw('MATERIALIZE')),
        kw('STATISTICS'),
        optional($._if_exists),
        commaSep1(field('name', $._name)),
      )),
    ),

    // MODIFY changes a column, a constraint, statistics, the keys, the TTL,
    // the settings, the comment, or the refresh and the security of a
    // view.
    modify_action: $ => prec.right(seq(
      kw('MODIFY'),
      choice(
        seq(kw('COLUMN'), optional($._if_exists), choice(
          seq($.column_definition, optional($._column_position)),
          seq(field('name', $._path), kw('REMOVE'), field('property', $._label)),
          seq(field('name', $._path), kws('MODIFY', 'SETTING'), commaSep1($.setting)),
          seq(field('name', $._path), kws('RESET', 'SETTING'), commaSep1($._label)),
        )),
        seq(kw('CONSTRAINT'), $._constraint_body),
        seq(kw('STATISTICS'), $._statistics_body),
        seq(kws('ORDER', 'BY'), $._expression),
        seq(kws('SAMPLE', 'BY'), $._expression),
        seq(kws('PRIMARY', 'KEY'), $._expression),
        $.ttl_clause,
        seq(kw('SETTING'), commaSep1($.setting)),
        $.comment_clause,
        $.refresh_clause,
        seq(kws('SQL', 'SECURITY'), choice(kw('DEFINER'), kw('INVOKER'), kw('NONE'))),
        seq(kw('DEFINER'), '=', field('definer', $._label)),
      ),
    )),

    // RENAME COLUMN a TO b.
    rename_column_action: $ => seq(
      kws('RENAME', 'COLUMN'),
      optional($._if_exists),
      field('name', $._path),
      kw('TO'),
      field('new_name', $._path),
    ),

    comment_column_action: $ => seq(
      kws('COMMENT', 'COLUMN'),
      optional($._if_exists),
      field('name', $._path),
      field('comment', $.string),
    ),

    // ALTER COLUMN a TYPE t changes the type of a column, as in
    // PostgreSQL.
    alter_column_action: $ => seq(
      kws('ALTER', 'COLUMN'),
      optional($._if_exists),
      field('name', $._path),
      kw('TYPE'),
      field('type', $.type),
    ),

    // UPDATE and DELETE change the rows with a mutation.
    update_action: $ => seq(
      kw('UPDATE'),
      commaSep1($.assignment),
      $.where_clause,
    ),

    delete_action: $ => seq(
      kw('DELETE'),
      optional($.in_partition),
      $.where_clause,
    ),

    // The actions on the partitions and the parts of a table.
    partition_action: $ => choice(
      seq(
        choice(kw('DETACH'), kw('DROP'), kw('FORGET'), kw('ATTACH'), kw('REPLACE'), kws('DROP', 'DETACHED')),
        $.partition,
        optional(seq(kw('FROM'), field('from', $.object_reference))),
      ),
      seq(
        kw('MOVE'),
        $.partition,
        kw('TO'),
        choice(
          seq(choice(kw('DISK'), kw('VOLUME')), field('to', $.string)),
          seq(kw('TABLE'), field('to', $.object_reference)),
        ),
      ),
      seq(kw('FETCH'), $.partition, kw('FROM'), field('from', $.string)),
      seq(
        choice(kw('FREEZE'), kw('UNFREEZE')),
        optional($.partition),
        optional(seq(kws('WITH', 'NAME'), field('backup', $.string))),
      ),
      seq(
        choice(kws('APPLY', 'DELETED', 'MASK'), kws('APPLY', 'PATCHES'), kws('REWRITE', 'PARTS'), kws('MATERIALIZE', 'TTL')),
        optional($.in_partition),
      ),
    ),

    reset_setting_action: $ => prec.right(seq(kws('RESET', 'SETTING'), commaSep1($._label))),

    // REMOVE TTL and REMOVE SAMPLE BY.
    remove_action: _ => seq(kw('REMOVE'), choice(kw('TTL'), kws('SAMPLE', 'BY'))),

    // EXECUTE runs a command on a table of a data lake, as in EXECUTE
    // expire_snapshots('2025-01-01').
    execute_action: $ => seq(kw('EXECUTE'), field('command', alias($._table_function, $.function_call))),

    // ALTER NAMED COLLECTION sets keys and deletes keys.
    alter_named_collection_statement: $ => seq(
      kws('ALTER', 'NAMED', 'COLLECTION'),
      optional($._if_exists),
      field('name', $._name),
      optional($.on_cluster),
      repeat1(choice(
        seq(kw('SET'), commaSep1($.collection_item)),
        seq(kw('DELETE'), commaSep1(field('key', $._label))),
      )),
    ),

    // Other statements

    // DROP removes a table, a view, a dictionary, a database, a function, a
    // named collection, an entity of access control, an index or another
    // object. DETACH makes the server forget a table, a view, a dictionary
    // or a database until ATTACH, and UNDROP brings back a table that DROP
    // removed. The three statements share one rule, so that their parse
    // states are shared, and the rule takes the words of each of them. The
    // names of a row policy or of an index come with ON and their tables,
    // and an entity of access control can come FROM its storage.
    drop_statement: $ => seq(kw('DROP'), $._drop_body),

    // The clauses at the end of the statements of this part, such as ON
    // CLUSTER, SETTINGS, FORMAT, SYNC or the WHERE of KILL. Each statement
    // takes all of them, in any order. A parse state after an expression
    // has a column for each token that can come next, and the states after
    // an expression are copied for each set of such tokens. With one set
    // for all the statements, the states after an expression are copied
    // once.
    _tail: $ => choice(
      $.on_cluster,
      seq(kw('ON'), commaSep1(field('table', $._grant_target))),
      $.uuid,
      $.partition,
      $.settings_clause,
      $.format_clause,
      $.into_outfile_clause,
      $.where_clause,
      seq(choice(kw('FROM'), kw('IN')), field('from', $.object_reference)),
      seq(optional(kw('NOT')), choice(kw('LIKE'), kw('ILIKE')), field('pattern', $.string)),
      seq(kw('FOR'), commaSep1(field('user', $._user_name))),
      seq(kw('LIMIT'), field('limit', $._expression)),
      kws('WITH', 'IMPLICIT'),
      kws('NO', 'DELAY'),
      prec.right(seq(kw('DEDUPLICATE'), optional(seq(kw('BY'), commaSep1(choice($._expression, $.star_expression)))))),
      seq(kws('DRY', 'RUN', 'PARTS'), commaSep1($.string)),
      ...['SYNC', 'ASYNC', 'TEST', 'PERMANENTLY', 'FINAL', 'FORCE', 'CLEANUP', 'EVENTS'].map(kw),
    ),

    detach_statement: $ => seq(kw('DETACH'), $._drop_body),

    undrop_statement: $ => seq(kw('UNDROP'), $._drop_body),

    _drop_body: $ => prec.right(seq(
      $._object_kind,
      optional($._if_exists),
      optional(kws('IF', 'EMPTY')),
      commaSep1(field('name', choice($.object_reference, $.user_name, $.string))),
      repeat($._tail),
    )),

    // The kind of an object of DROP, DETACH and UNDROP.
    _object_kind: _ => choice(
      seq(optional(kw('TEMPORARY')), kw('TABLE')),
      seq(optional(choice(kw('TEMPORARY'), kw('MATERIALIZED'))), kw('VIEW')),
      kw('DICTIONARY'),
      kw('DATABASE'),
      kw('FUNCTION'),
      kws('NAMED', 'COLLECTION'),
      kw('HANDLER'),
      kw('WORKLOAD'),
      kw('RESOURCE'),
      kw('USER'),
      kw('ROLE'),
      kw('QUOTA'),
      seq(optional(kw('SETTINGS')), kw('PROFILE')),
      seq(optional(choice(kw('ROW'), kw('MASKING'))), kw('POLICY')),
      seq(optional(kw('HYPOTHETICAL')), choice(kw('INDEX'), kw('PROJECTION'))),
    ),

    // RENAME gives new names to tables, databases or dictionaries, as in
    // RENAME TABLE a TO b, c TO d.
    rename_statement: $ => seq(
      kw('RENAME'),
      optional(choice(kw('TABLE'), kw('DATABASE'), kw('DICTIONARY'))),
      optional($._if_exists),
      commaSep1($.rename_item),
      repeat($._tail),
    ),

    rename_item: $ => seq(
      field('name', $.object_reference),
      kw('TO'),
      field('new_name', $.object_reference),
    ),

    // EXCHANGE swaps the names of two tables or of two dictionaries.
    exchange_statement: $ => seq(
      kw('EXCHANGE'),
      choice(kw('TABLES'), kw('DICTIONARIES')),
      field('name', $.object_reference),
      kw('AND'),
      field('name', $.object_reference),
      repeat($._tail),
    ),

    // OPTIMIZE merges the parts of a table. DEDUPLICATE removes the rows
    // that are the same in all the columns, or in the columns after BY.
    optimize_statement: $ => prec.right(seq(
      kws('OPTIMIZE', 'TABLE'),
      field('table', $.object_reference),
      repeat($._tail),
    )),

    // SHOW lists the objects of the server, or shows the statement that
    // creates an object, as in SHOW CREATE TABLE t, SHOW TABLES FROM db
    // LIKE 'a%' or SHOW GRANTS FOR u. Its forms are many, so the grammar
    // takes the words of each form in one list, then the names, then the
    // clauses in any order.
    show_statement: $ => prec.right(seq(
      kw('SHOW'),
      repeat1($._show_word),
      optional(commaSep1(field('name', choice($.object_reference, $.user_name, $.string)))),
      repeat($._tail),
    )),

    // SETTINGS is a word, as in SHOW CHANGED SETTINGS, and it starts the
    // clause SETTINGS after the names. Before a name, it starts the clause.
    _show_word: $ => choice(prec(-1, kw('SETTINGS')), ...[
      'CREATE', 'TEMPORARY', 'TABLE', 'VIEW', 'DICTIONARY', 'DATABASE', 'USER', 'ROLE',
      'QUOTA', 'PROFILE', 'POLICY', 'ROW', 'MASKING', 'FULL', 'TABLES', 'DATABASES',
      'DICTIONARIES', 'INDEX', 'GRANTS', 'CURRENT', 'PROFILES', 'CLUSTER', 'SETTING',
    ].map(kw), $._command_word),

    // DESCRIBE shows the columns of a table, of a table function or of a
    // query. The clauses of the statement, such as FORMAT, come after a
    // table, a table function or a query in parentheses. A query with no
    // parentheses, or a set operation, takes no clauses of the statement,
    // and a clause after it belongs to the query (D133). This costs 6 parse
    // states. The clauses of the statement after any query cost more than
    // 400 large states. A query in parentheses with nothing after it is a
    // subquery.
    describe_statement: $ => prec.right(seq(
      choice(kw('DESCRIBE'), kw('DESC')),
      optional(kw('TEMPORARY')),
      optional(kw('TABLE')),
      choice(
        prec.dynamic(1, seq(
          field('table', choice(
            $.object_reference,
            alias($._table_function, $.function_call),
            $.subquery,
          )),
          repeat($._tail),
        )),
        field('query', $.select_statement),
      ),
    )),

    // EXISTS tells whether a table, a dictionary or a database exists.
    exists_statement: $ => prec.right(seq(
      kw('EXISTS'),
      optional(kw('TEMPORARY')),
      optional(choice(kw('TABLE'), kw('VIEW'), kw('DICTIONARY'), kw('DATABASE'))),
      field('name', $.object_reference),
      repeat($._tail),
    )),

    // CHECK TABLE checks the data of a table, and CHECK ALL TABLES checks
    // the data of each table. CHECK DATABASE checks that a database can
    // reach its catalog, and CHECK GRANT tells whether the user has a
    // privilege.
    check_statement: $ => choice(
      prec.right(seq(kws('CHECK', 'ALL', 'TABLES'), repeat($._tail))),
      prec.right(seq(
        kw('CHECK'),
        choice(kw('TABLE'), kw('DATABASE')),
        field('name', $.object_reference),
        repeat($._tail),
      )),
      seq(kws('CHECK', 'GRANT'), $._privileges),
    ),

    // KILL stops the queries or the mutations that its condition selects.
    kill_statement: $ => prec.right(seq(
      kw('KILL'),
      choice(kw('QUERY'), kw('MUTATION'), kw('TRANSACTION')),
      repeat1($._tail),
    )),

    // SYSTEM runs a command on the server, such as SYSTEM STOP MERGES t or
    // SYSTEM DROP MARK CACHE. Its commands are many, so the grammar takes
    // the words, the names, the strings and the numbers of a command in
    // any order. The words are keywords.
    system_statement: $ => seq(
      kw('SYSTEM'),
      repeat1(choice(
        $._system_word,
        field('name', $.object_reference),
        $.string,
        $.number,
        $.subquery,
        ',',
      )),
    ),

    _system_word: $ => choice(...[
      'ADD', 'ALL', 'ASYNC', 'CLEAR', 'CLUSTER', 'DATABASE', 'DICTIONARIES', 'DICTIONARY',
      'DISK', 'DROP', 'EXPRESSION', 'FOR', 'FORMAT', 'FROM', 'FUNCTION', 'IN', 'INDEX',
      'INSERT', 'KEY', 'KILL', 'NAME', 'OBJECT', 'OFFSET', 'ON', 'PARTS', 'PRIMARY', 'QUERY',
      'REFRESH', 'REMOVE', 'REPLICATED', 'RESET', 'RESTORE', 'STATISTICS', 'STRICT', 'SYNC',
      'TABLE', 'TIME', 'TRANSACTION', 'TTL', 'UNFREEZE', 'VIEW', 'VOLUME', 'WITH',
    ].map(kw), $._command_word),

    // A word of a command of SYSTEM or of a form of SHOW, as one token of
    // the word rule.
    _command_word: $ => alias(token(choice(...COMMAND_WORDS.map(keyword))), $.keyword),

    // SET changes settings for the session. A setting with no value is
    // on. SET TIME ZONE sets the time zone of the session.
    set_statement: $ => choice(
      seq(kw('SET'), commaSep1($.setting)),
      seq(kws('SET', 'TIME', 'ZONE'), optional('='), field('value', $.string)),
      seq(kws('SET', 'TRANSACTION', 'SNAPSHOT'), field('snapshot', $.number)),
    ),

    // SET ROLE sets the roles of the current user, and SET DEFAULT ROLE
    // sets the roles that a user has at login.
    set_role_statement: $ => seq(
      kw('SET'),
      choice(
        seq(kw('ROLE'), $._role_list),
        seq(
          kws('DEFAULT', 'ROLE'),
          optional($.on_cluster),
          $._role_list,
          kw('TO'),
          commaSep1(field('user', $._user_name)),
          optional($.on_cluster),
        ),
      ),
    ),

    // A list of roles or users: names, ALL, NONE or DEFAULT, with EXCEPT
    // and the names that it leaves out.
    _role_list: $ => prec.right(seq(
      choice(
        commaSep1(field('role', $._user_name)),
        kw('ALL'),
        kw('NONE'),
        kw('DEFAULT'),
      ),
      optional(seq(kw('EXCEPT'), commaSep1(field('role', $._user_name)))),
    )),

    // The name of a user or a role. The name of a user can have the host
    // that the user logs in from, as in ann@'10.0.0.%', and it can be a
    // string.
    _user_name: $ => choice(
      $._name,
      $.query_parameter,
      $.string,
      $.user_name,
    ),

    user_name: $ => seq(
      field('name', choice($._name, $.query_parameter)),
      '@',
      field('host', choice($.string, $._name)),
    ),

    // USE sets the database of the session.
    use_statement: $ => seq(kw('USE'), optional(kw('DATABASE')), field('database', choice($._name, $.query_parameter))),

    // GRANT gives privileges or roles to users and roles, and REVOKE takes
    // them back. A privilege is one word or more, such as SELECT, ALTER
    // UPDATE or SYSTEM DROP CACHE, with columns in parentheses. Roles have
    // no ON.
    grant_statement: $ => prec.right(seq(
      kw('GRANT'),
      optional($.on_cluster),
      choice(
        $._privileges,
        seq(kws('CURRENT', 'GRANTS'), choice(parens($._privileges), seq(kw('ON'), field('target', $._grant_target)))),
      ),
      kw('TO'),
      $._role_list,
      repeat(seq(kw('WITH'), choice(kw('GRANT'), kw('ADMIN'), kw('REPLACE')), kw('OPTION'))),
    )),

    revoke_statement: $ => seq(
      kw('REVOKE'),
      optional($.on_cluster),
      optional(seq(choice(kw('ADMIN'), kw('GRANT')), kws('OPTION', 'FOR'))),
      $._privileges,
      kw('FROM'),
      $._role_list,
    ),

    // A list of privileges. ON and a target can come after each one, as in
    // SELECT ON a.*, INSERT ON b.t.
    _privileges: $ => commaSep1(seq(
      $.privilege,
      optional(seq(kw('ON'), field('target', $._grant_target))),
    )),

    privilege: $ => choice(seq(repeat1($.identifier), optional($.column_list)), $.query_parameter),

    // The target of a privilege: a table, a database, or all of them, as
    // in db.t, db.*, *.*, *, or db.t* for the tables whose names start
    // with t. The target of READ and WRITE is a source, as in S3('s3://a/.*').
    _grant_target: $ => choice(
      $.object_reference,
      alias($._grant_pattern, $.object_reference),
      alias($._table_function, $.function_call),
    ),

    _grant_pattern: $ => choice(
      seq(field('database', choice('*', seq($._name, '*'))), optional(seq('.', field('name', choice('*', seq($._label, optional('*'))))))),
      seq(field('database', choice($._name, $.query_parameter)), '.', field('name', choice('*', seq($._label, '*')))),
    ),

    // CREATE USER, ROLE, ROW POLICY, MASKING POLICY, QUOTA and SETTINGS
    // PROFILE create the entities of access control, and ALTER changes
    // them. Their options come in any order, and the options of all the
    // kinds are in one choice, so that their parse states are shared. The
    // options are not clauses of _tail, because then each state after an
    // expression of _tail had more than 64 columns.
    create_access_statement: $ => seq(
      $._create,
      $._access_kind,
      optional(choice($._if_not_exists, kws('OR', 'REPLACE'))),
      $._access_names,
      repeat($._access_option),
    ),

    alter_access_statement: $ => seq(
      kw('ALTER'),
      $._access_kind,
      optional($._if_exists),
      $._access_names,
      repeat($._access_option),
    ),

    _access_kind: _ => choice(
      kw('USER'),
      kw('ROLE'),
      kw('QUOTA'),
      seq(optional(kw('SETTINGS')), kw('PROFILE')),
      seq(optional(choice(kw('ROW'), kw('MASKING'))), kw('POLICY')),
    ),

    // The names of the entities. ON and the tables of a policy are an
    // option, so that ON CLUSTER and ON with the tables can both come
    // after the names.
    _access_names: $ => commaSep1(field('name', $._user_name)),

    _access_option: $ => choice(
      $.on_cluster,
      seq(kw('ON'), commaSep1(field('table', $._grant_target))),
      seq(kw('IN'), field('storage', $._name)),
      seq(optional(choice(kw('ADD'), kw('MODIFY'))), kw('SETTINGS'), choice(commaSep1(choice($.profile_setting, $.settings_profile)), kw('NONE'))),
      $.where_clause,
      seq(kws('RENAME', 'TO'), field('new_name', $._user_name)),
      kws('NOT', 'IDENTIFIED'),
      seq(optional(kw('ADD')), kw('IDENTIFIED'), optional(kw('WITH')), commaSep1($.authentication)),
      kws('RESET', 'AUTHENTICATION', 'METHODS', 'TO', 'NEW'),
      $._valid,
      seq(optional(choice(kw('ADD'), kw('DROP'))), kw('HOST'), choice(commaSep1($.host), kw('ANY'), kw('NONE'))),
      seq(optional(kw('DEFAULT')), kw('ROLE'), $._role_list),
      seq(kws('DEFAULT', 'DATABASE'), choice(field('database', $._name), kw('NONE'))),
      seq(kw('GRANTEES'), $._grantees),
      seq(kw('DROP'), choice(kw('SETTINGS'), kw('PROFILES')), commaSep1(choice($._name, $.string))),
      seq(kws('DROP', 'ALL'), choice(kw('SETTINGS'), kw('PROFILES'))),
      seq(kws('ADD', 'PROFILES'), commaSep1($.string)),
      seq(kw('TO'), $._role_list),
      seq(kw('FOR'), kw('SELECT')),
      seq(kw('USING'), field('condition', $._expression)),
      seq(kw('AS'), choice(kw('PERMISSIVE'), kw('RESTRICTIVE'))),
      seq(kw('UPDATE'), commaSep1($.assignment)),
      seq(kw('PRIORITY'), field('priority', $.number)),
      seq(choice(kw('KEYED'), kw('KEY')), kw('BY'), commaSep1(field('key', choice($._name, $.string)))),
      kws('NOT', 'KEYED'),
      seq(choice(kw('IPV4_PREFIX_BITS'), kw('IPV6_PREFIX_BITS')), $.number),
      commaSep1($.quota_interval),
    ),

    // A method of authentication, as in sha256_password BY 'secret' or
    // ldap SERVER 'main'.
    authentication: $ => prec.right(choice(
      seq(field('method', $._name), repeat($._authentication_part)),
      repeat1($._authentication_part),
    )),

    _authentication_part: $ => choice(
      seq(choice(kw('BY'), kw('SERVER'), kw('REALM'), kw('CN'), kw('SAN'), kw('SCHEME'), kw('SALT')), choice($.string, $.query_parameter)),
      seq(kw('KEY'), $.string, kw('TYPE'), $.string),
      $._valid,
    ),

    // The time until a user or a method of authentication is valid, as in
    // VALID UNTIL '2030-01-01' or VALID FOR INTERVAL 1 DAY.
    _valid: $ => seq(kw('VALID'), choice(
      seq(kw('UNTIL'), field('until', $.string)),
      seq(kw('FOR'), field('for', $.interval_expression)),
    )),

    // A host that a user can log in from.
    host: $ => choice(
      seq(choice(kw('LOCAL'), kw('NAME'), kw('REGEXP'), kw('IP'), kw('LIKE')), optional($.string)),
      $.string,
    ),

    _grantees: $ => prec.right(seq(
      choice(commaSep1(field('grantee', $._user_name)), kw('ANY'), kw('NONE')),
      optional(seq(kw('EXCEPT'), commaSep1(field('grantee', $._user_name)))),
    )),

    // A setting of a profile, a role or a user, with its limits. Its
    // value is a literal or a name. A value that was only a literal cost
    // about 570 parse states, because a number could then come where an
    // identifier cannot, as in an interval of REFRESH.
    profile_setting: $ => prec.right(seq(
      field('name', $._label),
      optional(seq('=', field('value', $._setting_value))),
      repeat(choice(
        seq(choice(kw('MIN'), kw('MAX')), optional('='), $._setting_value),
        kw('CONST'),
        kw('READONLY'),
        kw('WRITABLE'),
        kw('CHANGEABLE_IN_READONLY'),
      )),
    )),

    // A profile that gives its settings, as in SETTINGS PROFILE 'default'.
    settings_profile: $ => seq(choice(kw('PROFILE'), kw('INHERIT')), field('profile', choice($.string, $._name))),

    _setting_value: $ => choice($._literal, $._name, alias($._negative_number, $.unary_expression)),

    // An interval of a quota and its limits, as in FOR INTERVAL 1 HOUR MAX
    // queries = 100.
    quota_interval: $ => seq(
      kw('FOR'),
      optional(kw('RANDOMIZED')),
      optional(kw('INTERVAL')),
      choice($.number, $._name),
      $.interval_unit,
      choice(
        seq(kw('MAX'), commaSep1(seq(optional(kw('MAX')), $.quota_limit))),
        kws('NO', 'LIMITS'),
        kws('TRACKING', 'ONLY'),
      ),
    ),

    quota_limit: $ => seq(field('name', $._label), '=', field('value', choice($.number, $.string))),

    // MOVE moves entities of access control to another storage.
    move_statement: $ => seq(
      kw('MOVE'),
      $._access_kind,
      commaSep1(field('name', $._name)),
      kw('TO'),
      field('storage', $._name),
    ),

    // WATCH reads the changes of a live view.
    watch_statement: $ => prec.right(seq(
      kw('WATCH'),
      field('view', $.object_reference),
      repeat($._tail),
    )),

    // BACKUP writes tables, dictionaries and databases to a backup, and
    // RESTORE reads them back, as in BACKUP TABLE t TO Disk('backups',
    // '1.zip').
    backup_statement: $ => seq(
      kw('BACKUP'),
      commaSep1($.backup_item),
      optional($.on_cluster),
      kw('TO'),
      field('destination', choice(alias($._table_function, $.function_call), $._name)),
      repeat($._tail),
    ),

    restore_statement: $ => seq(
      kw('RESTORE'),
      commaSep1($.backup_item),
      optional($.on_cluster),
      kw('FROM'),
      field('source', choice(alias($._table_function, $.function_call), $._name)),
      repeat($._tail),
    ),

    backup_item: $ => seq(
      choice(
        seq(optional(kw('TEMPORARY')), kw('TABLE')),
        kw('DICTIONARY'),
        kw('DATABASE'),
        kw('VIEW'),
        kw('ALL'),
      ),
      optional(field('name', $.object_reference)),
      optional(seq(kw('AS'), field('alias', $.object_reference))),
      optional(seq(kw('EXCEPT'), choice(kw('TABLES'), kw('DATABASES')), commaSep1($.object_reference))),
      optional(seq(choice(kw('PARTITION'), kw('PARTITIONS')), commaSep1($._expression))),
    ),

    // BEGIN, COMMIT and ROLLBACK of a transaction.
    transaction_statement: _ => choice(
      seq(kw('BEGIN'), optional(kw('TRANSACTION'))),
      kw('COMMIT'),
      kw('ROLLBACK'),
    ),

    // EXECUTE AS runs the statement after it, or the next statements of the
    // session, as another user.
    execute_as_statement: $ => prec.right(seq(
      kws('EXECUTE', 'AS'),
      field('user', $._name),
      optional($._statement),
    )),

    // CREATE WORKLOAD and CREATE RESOURCE set how the server schedules the
    // queries. A resource takes the operations that it covers, as in
    // (WRITE DISK s3, READ DISK s3).
    create_workload_statement: $ => seq(
      $._create,
      kw('WORKLOAD'),
      optional($._if_not_exists),
      field('name', $._name),
      repeat($._tail),
    ),

    create_resource_statement: $ => seq(
      $._create,
      kw('RESOURCE'),
      optional($._if_not_exists),
      field('name', $._name),
      optional($.on_cluster),
      parens(commaSep1($.resource_operation)),
    ),

    resource_operation: $ => repeat1($.identifier),

    // CREATE HANDLER and ALTER HANDLER give a query to a path of the HTTP
    // interface.
    create_handler_statement: $ => seq(
      choice($._create, kw('ALTER')),
      kw('HANDLER'),
      optional($._if_not_exists),
      field('name', $._name),
      optional($.on_cluster),
      repeat(choice(
        seq(kw('PROTOCOL'), field('protocol', $._name)),
        seq(kw('URL'), optional(choice(kw('PREFIX'), kw('REGEXP'))), field('url', $.string)),
        seq(kw('METHODS'), $.column_list),
        seq(kw('TYPE'), field('type', $._name)),
      )),
      optional(seq(kw('AS'), field('statement', $._statement))),
    ),

    // CREATE TOKEN creates a token of authentication for the current user.
    create_token_statement: $ => prec.right(seq(
      $._create,
      kw('TOKEN'),
      repeat(choice(
        seq(kws('VALID', 'UNTIL'), $.string),
        seq(kw('GRANTS'), parens(commaSep1(seq($.privilege, optional(seq(kw('ON'), field('target', $._grant_target))))))),
      )),
    )),

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
      $.at_time_zone_expression,
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
    null: _ => keyword('NULL'),

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
    // units, as in INTERVAL '1:30' HOUR TO MINUTE. Only a string takes two
    // units, so that TO after INTERVAL 1 WEEK in TTL starts TO DISK or TO
    // VOLUME.
    interval_expression: $ => prec.right(seq(
      kw('INTERVAL'),
      choice(
        seq(
          field('value', $.string),
          optional(seq($.interval_unit, optional(seq(kw('TO'), $.interval_unit)))),
        ),
        seq(
          field('value', choice(
            $.number,
            $.parenthesized_expression,
            $.column_reference,
            $.function_call,
            $.query_parameter,
            alias($._negative_number, $.unary_expression),
          )),
          optional($.interval_unit),
        ),
      ),
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
    _function_name: $ => $._name,

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

    // AT TIME ZONE gives a time in another time zone, and AT LOCAL gives it
    // in the time zone of the session (D133). The operator binds more
    // tightly than * and /, so a * t AT TIME ZONE 'UTC' converts t. It costs
    // about 140 parse states, 50 of them large, because AT can come after
    // each expression. Tokens for AT TIME ZONE and AT LOCAL cost 100 large
    // states, and a value or a zone that is not an operator cost more.
    at_time_zone_expression: $ => prec.left(PREC.unary, seq(
      field('value', $._expression),
      kw('AT'),
      choice(seq(kws('TIME', 'ZONE'), field('zone', $._expression)), kw('LOCAL')),
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
      seq(kw('SHARED'), $.field_definition),
      $.type_setting,
      $.skip_path,
    )))),

    // A value of an enum, as in 'a' = 1.
    enum_value: $ => seq(field('name', $.string), '=', field('value', $._signed_number)),

    // A field of a tuple or of a nested type, or a path of a JSON type, with
    // its type. SHARED before the path of a JSON type keeps the path in the
    // shared data.
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

    // A label after AS or after a dot can be any keyword. It is a name,
    // because the grammar reserves no word.
    _label: $ => $._name,

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
