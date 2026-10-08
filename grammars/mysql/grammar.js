/**
 * @file The grammar of the SQL that MySQL 8.4 and MariaDB share
 * @license MIT
 */

// xo writes this grammar from the reference manual of MySQL 8.4 (D42,
// D106). It does not come from the source code of MySQL or of MariaDB.
//
// The grammar is for a highlighter and for completion, so it accepts more
// than the server does where that keeps the rules small. A keyword is a
// token in any case, and it is an anonymous node with the name of the
// keyword in upper case, such as "SELECT". A keyword is a keyword only where
// the grammar expects it, so a word such as status or name is an identifier
// in every other place, as for the words that MySQL does not reserve.
//
// The external scanner in src/scanner.c reads the comments, and the start
// and the end of a version comment, /*!50100 ... */. The text inside a
// version comment is parsed as SQL, so it gets its highlights. DELIMITER is
// a command of the mysql client, and usql does not use it, so the grammar
// leaves it out. A statement ends at a semicolon or at the end of the input.

/// <reference types="tree-sitter-cli/dsl" />
// @ts-check

// The precedences of the operators, from the lowest to the highest, as the
// section "Operator Precedence" of the manual lists them.
const PREC = {
  assign: 1,
  or: 2,
  xor: 3,
  and: 4,
  not: 5,
  between: 6,
  compare: 7,
  bitor: 8,
  bitand: 9,
  shift: 10,
  add: 11,
  multiply: 12,
  bitxor: 13,
  unary: 14,
  bang: 15,
  collate: 16,
  json: 17,
};

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
 * alias. A keyword that names a function or a column is this token with the
 * alias identifier.
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
  name: 'mysql',

  extras: $ => [
    /\s/,
    $.comment,
    $.version_comment_start,
    $.version_comment_end,
  ],

  // The order of the external tokens is the order of enum TokenType in
  // src/scanner.c.
  externals: $ => [
    $.comment,
    $.version_comment_start,
    $.version_comment_end,
  ],

  word: $ => $._identifier,

  conflicts: $ => [
    // WITH after GROUP BY starts WITH ROLLUP, or WITH CHECK OPTION of a view.
    [$.group_by_clause],
  ],

  rules: {
    source_file: $ => seq(
      repeat(choice(seq($._top_statement, ';'), ';')),
      optional($._top_statement),
    ),

    // BEGIN starts a transaction only outside a stored program. Inside one,
    // it starts a block.
    _top_statement: $ => choice(
      $._statement,
      alias($._begin_work, $.start_transaction_statement),
    ),

    _statement: $ => choice(
      $.select_statement,
      $.insert_statement,
      $.replace_statement,
      $.update_statement,
      $.delete_statement,
      $.create_database_statement,
      $.alter_database_statement,
      $.drop_database_statement,
      $.create_table_statement,
      $.alter_table_statement,
      $.drop_table_statement,
      $.rename_table_statement,
      $.truncate_statement,
      $.create_index_statement,
      $.drop_index_statement,
      $.create_view_statement,
      $.alter_view_statement,
      $.drop_view_statement,
      $.create_trigger_statement,
      $.drop_trigger_statement,
      $.create_procedure_statement,
      $.create_function_statement,
      $.alter_routine_statement,
      $.drop_routine_statement,
      $.create_user_statement,
      $.alter_user_statement,
      $.drop_user_statement,
      $.grant_statement,
      $.revoke_statement,
      $.use_statement,
      $.set_statement,
      $.show_statement,
      $.describe_statement,
      $.explain_statement,
      $.start_transaction_statement,
      $.commit_statement,
      $.rollback_statement,
      $.savepoint_statement,
      $.release_savepoint_statement,
      $.lock_tables_statement,
      $.unlock_tables_statement,
      $.prepare_statement,
      $.execute_statement,
      $.deallocate_statement,
      $.call_statement,
      $.do_statement,
      $.load_data_statement,
    ),

    // Queries

    select_statement: $ => $._query_expression,

    _query_expression: $ => seq(
      optional($.with_clause),
      $._query_body,
      optional($.order_by_clause),
      optional($.limit_clause),
      repeat(choice($.into_clause, $.locking_clause)),
    ),

    with_clause: $ => seq(
      kw('WITH'),
      optional(kw('RECURSIVE')),
      commaSep1($.cte),
    ),

    cte: $ => seq(
      field('name', $.identifier),
      optional($.column_list),
      kw('AS'),
      $.subquery,
    ),

    _query_body: $ => choice(
      $.select,
      $.set_operation,
      $.parenthesized_query,
    ),

    parenthesized_query: $ => prec(1, parens($._query_expression)),

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
      repeat($._select_modifier),
      commaSep1($._select_item),
      optional($.into_clause),
      optional($.from_clause),
      optional($.where_clause),
      optional($.group_by_clause),
      optional($.having_clause),
      optional($.window_clause),
    )),

    _select_modifier: _ => choice(
      kw('ALL'),
      kw('DISTINCT'),
      kw('DISTINCTROW'),
      kw('HIGH_PRIORITY'),
      kw('STRAIGHT_JOIN'),
      kw('SQL_SMALL_RESULT'),
      kw('SQL_BIG_RESULT'),
      kw('SQL_BUFFER_RESULT'),
      kw('SQL_NO_CACHE'),
      kw('SQL_CALC_FOUND_ROWS'),
    ),

    _select_item: $ => choice(
      $._expression,
      $.aliased_expression,
      $.all_columns,
    ),

    aliased_expression: $ => seq(
      field('value', $._expression),
      optional(kw('AS')),
      field('alias', choice($.identifier, $.string)),
    ),

    all_columns: $ => seq(
      optional(seq(
        optional(seq(field('schema', $.identifier), '.')),
        field('table', $.identifier),
        '.',
      )),
      '*',
    ),

    into_clause: $ => seq(
      kw('INTO'),
      choice(
        seq(
          kw('OUTFILE'),
          field('file', $.string),
          optional($._character_set),
          optional($._field_options),
          optional($._line_options),
        ),
        seq(kw('DUMPFILE'), field('file', $.string)),
        commaSep1(choice($.user_variable, $.identifier)),
      ),
    ),

    _character_set: $ => seq(
      choice(kws('CHARACTER', 'SET'), kw('CHARSET')),
      field('charset', $._charset_name),
    ),

    _charset_name: $ => choice($.identifier, $.string, kw('BINARY')),

    _field_options: $ => seq(
      choice(kw('FIELDS'), kw('COLUMNS')),
      repeat1(choice(
        seq(kws('TERMINATED', 'BY'), $.string),
        seq(optional(kw('OPTIONALLY')), kws('ENCLOSED', 'BY'), $.string),
        seq(kws('ESCAPED', 'BY'), $.string),
      )),
    ),

    _line_options: $ => seq(
      kw('LINES'),
      repeat1(choice(
        seq(kws('STARTING', 'BY'), $.string),
        seq(kws('TERMINATED', 'BY'), $.string),
      )),
    ),

    from_clause: $ => seq(
      kw('FROM'),
      choice(kw('DUAL'), commaSep1($._table_reference)),
    ),

    _table_reference: $ => choice(
      $.table_reference,
      $.derived_table,
      $.join,
      parens(commaSep1($._table_reference)),
    ),

    table_reference: $ => prec.right(seq(
      optional(seq(field('schema', $.identifier), '.')),
      field('name', $.identifier),
      optional($.partition_list),
      optional(seq(optional(kw('AS')), field('alias', $.identifier))),
      repeat($.index_hint),
    )),

    partition_list: $ => seq(kw('PARTITION'), parens(commaSep1($.identifier))),

    derived_table: $ => prec.right(seq(
      optional(kw('LATERAL')),
      $.subquery,
      optional(seq(
        optional(kw('AS')),
        field('alias', $.identifier),
        optional($.column_list),
      )),
    )),

    index_hint: $ => seq(
      choice(kw('USE'), kw('IGNORE'), kw('FORCE')),
      choice(kw('INDEX'), kw('KEY')),
      optional(seq(
        kw('FOR'),
        choice(kw('JOIN'), kws('ORDER', 'BY'), kws('GROUP', 'BY')),
      )),
      parens(optional(commaSep1(choice($.identifier, kw('PRIMARY'))))),
    ),

    join: $ => prec.left(seq(
      field('left', $._table_reference),
      choice(
        seq(optional(choice(kw('INNER'), kw('CROSS'))), kw('JOIN')),
        kw('STRAIGHT_JOIN'),
        seq(choice(kw('LEFT'), kw('RIGHT')), optional(kw('OUTER')), kw('JOIN')),
        seq(
          kw('NATURAL'),
          optional(choice(
            kw('INNER'),
            seq(choice(kw('LEFT'), kw('RIGHT')), optional(kw('OUTER'))),
          )),
          kw('JOIN'),
        ),
      ),
      field('right', $._table_reference),
      optional(choice(
        seq(kw('ON'), field('condition', $._expression)),
        seq(kw('USING'), parens(commaSep1(field('using', $.identifier)))),
      )),
    )),

    where_clause: $ => seq(kw('WHERE'), $._expression),

    group_by_clause: $ => seq(
      kws('GROUP', 'BY'),
      commaSep1($._expression),
      optional(kws('WITH', 'ROLLUP')),
    ),

    having_clause: $ => seq(kw('HAVING'), $._expression),

    window_clause: $ => seq(kw('WINDOW'), commaSep1($.window_definition)),

    window_definition: $ => seq(
      field('name', $.identifier),
      kw('AS'),
      $.window_specification,
    ),

    window_specification: $ => parens(seq(
      optional(field('name', $.identifier)),
      optional(seq(kws('PARTITION', 'BY'), commaSep1($._expression))),
      optional($.order_by_clause),
      optional($.frame_clause),
    )),

    frame_clause: $ => seq(
      choice(kw('ROWS'), kw('RANGE')),
      choice(
        $._frame_bound,
        seq(kw('BETWEEN'), $._frame_bound, kw('AND'), $._frame_bound),
      ),
    ),

    _frame_bound: $ => choice(
      kws('UNBOUNDED', 'PRECEDING'),
      kws('UNBOUNDED', 'FOLLOWING'),
      kws('CURRENT', 'ROW'),
      seq($._expression, choice(kw('PRECEDING'), kw('FOLLOWING'))),
    ),

    order_by_clause: $ => seq(
      kws('ORDER', 'BY'),
      commaSep1(seq($._expression, optional(choice(kw('ASC'), kw('DESC'))))),
    ),

    limit_clause: $ => seq(
      kw('LIMIT'),
      choice(
        field('count', $._limit_value),
        seq(field('offset', $._limit_value), ',', field('count', $._limit_value)),
        seq(field('count', $._limit_value), kw('OFFSET'), field('offset', $._limit_value)),
      ),
    ),

    _limit_value: $ => choice($.number, $.parameter, $.identifier, $.user_variable),

    locking_clause: $ => choice(
      seq(
        kw('FOR'),
        choice(kw('UPDATE'), kw('SHARE')),
        optional(seq(kw('OF'), commaSep1($.identifier))),
        optional(choice(kw('NOWAIT'), kws('SKIP', 'LOCKED'))),
      ),
      kws('LOCK', 'IN', 'SHARE', 'MODE'),
    ),

    subquery: $ => parens($._query_expression),

    // Changes of data

    insert_statement: $ => seq(
      kw('INSERT'),
      repeat(choice(kw('LOW_PRIORITY'), kw('DELAYED'), kw('HIGH_PRIORITY'), kw('IGNORE'))),
      ...insertBody($),
      optional($.on_duplicate_key_update),
    ),

    replace_statement: $ => seq(
      kw('REPLACE'),
      repeat(choice(kw('LOW_PRIORITY'), kw('DELAYED'))),
      ...insertBody($),
    ),

    values_clause: $ => seq(
      choice(kw('VALUES'), kw('VALUE')),
      commaSep1($.value_list),
    ),

    value_list: $ => seq(
      optional(kw('ROW')),
      parens(optional(commaSep1(choice($._expression, kw('DEFAULT'))))),
    ),

    on_duplicate_key_update: $ => seq(
      kws('ON', 'DUPLICATE', 'KEY', 'UPDATE'),
      commaSep1($.assignment),
    ),

    assignment: $ => seq(
      field('left', $.column_reference),
      '=',
      field('right', choice($._expression, kw('DEFAULT'))),
    ),

    update_statement: $ => seq(
      optional($.with_clause),
      kw('UPDATE'),
      repeat(choice(kw('LOW_PRIORITY'), kw('IGNORE'))),
      commaSep1($._table_reference),
      kw('SET'),
      commaSep1($.assignment),
      optional($.where_clause),
      optional($.order_by_clause),
      optional($.limit_clause),
    ),

    delete_statement: $ => seq(
      optional($.with_clause),
      kw('DELETE'),
      repeat(choice(kw('LOW_PRIORITY'), kw('QUICK'), kw('IGNORE'))),
      optional(commaSep1($.table_reference)),
      kw('FROM'),
      commaSep1($._table_reference),
      optional(seq(kw('USING'), commaSep1($._table_reference))),
      optional($.where_clause),
      optional($.order_by_clause),
      optional($.limit_clause),
    ),

    load_data_statement: $ => seq(
      kw('LOAD'),
      choice(kw('DATA'), kw('XML')),
      optional(choice(kw('LOW_PRIORITY'), kw('CONCURRENT'))),
      optional(kw('LOCAL')),
      kw('INFILE'),
      field('file', $.string),
      optional(choice(kw('REPLACE'), kw('IGNORE'))),
      kws('INTO', 'TABLE'),
      field('table', $.object_reference),
      optional($.partition_list),
      optional($._character_set),
      optional($._field_options),
      optional($._line_options),
      optional(seq(kw('IGNORE'), $.number, choice(kw('LINES'), kw('ROWS')))),
      optional($.column_list),
      optional(seq(kw('SET'), commaSep1($.assignment))),
    ),

    // Databases

    create_database_statement: $ => seq(
      kw('CREATE'),
      choice(kw('DATABASE'), kw('SCHEMA')),
      optional($._if_not_exists),
      field('name', $.identifier),
      repeat($.database_option),
    ),

    alter_database_statement: $ => seq(
      kw('ALTER'),
      choice(kw('DATABASE'), kw('SCHEMA')),
      optional(field('name', $.identifier)),
      repeat1($.database_option),
    ),

    drop_database_statement: $ => seq(
      kw('DROP'),
      choice(kw('DATABASE'), kw('SCHEMA')),
      optional($._if_exists),
      field('name', $.identifier),
    ),

    database_option: $ => seq(
      optional(kw('DEFAULT')),
      choice(
        seq(choice(kws('CHARACTER', 'SET'), kw('CHARSET')), optional('='), field('value', choice($._charset_name, kw('DEFAULT')))),
        seq(kw('COLLATE'), optional('='), field('value', choice($.identifier, $.string, kw('DEFAULT')))),
        seq(kw('ENCRYPTION'), optional('='), field('value', $.string)),
        seq(kws('READ', 'ONLY'), optional('='), field('value', choice($.number, kw('DEFAULT')))),
      ),
    ),

    _if_exists: _ => kws('IF', 'EXISTS'),

    _if_not_exists: _ => kws('IF', 'NOT', 'EXISTS'),

    // Tables

    create_table_statement: $ => seq(
      kw('CREATE'),
      optional(kw('TEMPORARY')),
      kw('TABLE'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      choice(
        seq(kw('LIKE'), field('like', $.object_reference)),
        parens(seq(kw('LIKE'), field('like', $.object_reference))),
        seq(
          optional(parens(commaSep1($._table_element))),
          optional($._table_options),
          optional($.partition_by),
          optional(seq(
            optional(choice(kw('IGNORE'), kw('REPLACE'))),
            optional(kw('AS')),
            $.select_statement,
          )),
        ),
      ),
    ),

    _table_element: $ => choice(
      $.column_definition,
      $.index_definition,
      $.table_constraint,
    ),

    column_definition: $ => seq(
      field('name', $.identifier),
      field('type', $.data_type),
      repeat($._column_attribute),
    ),

    _column_attribute: $ => choice(
      seq(optional(kw('NOT')), kw('NULL')),
      seq(kw('DEFAULT'), field('default', $._default_value)),
      seq(kws('ON', 'UPDATE'), field('on_update', $.function_call)),
      kw('AUTO_INCREMENT'),
      prec.right(seq(kw('UNIQUE'), optional(kw('KEY')))),
      seq(optional(kw('PRIMARY')), kw('KEY')),
      seq(kw('COMMENT'), field('comment', $.string)),
      seq(kw('COLUMN_FORMAT'), choice(kw('FIXED'), kw('DYNAMIC'), kw('DEFAULT'))),
      seq(kw('STORAGE'), choice(kw('DISK'), kw('MEMORY'))),
      kw('VISIBLE'),
      kw('INVISIBLE'),
      seq(kw('SRID'), $.number),
      seq(kws('SERIAL', 'DEFAULT', 'VALUE')),
      seq(
        optional(kws('GENERATED', 'ALWAYS')),
        kw('AS'),
        parens(field('generated', $._expression)),
        optional(choice(kw('VIRTUAL'), kw('STORED'))),
      ),
      $.references_clause,
      seq(
        optional(seq(kw('CONSTRAINT'), optional(field('constraint', $.identifier)))),
        $.check_constraint,
      ),
    ),

    // The manual allows a literal, an expression in parentheses or a
    // function such as CURRENT_TIMESTAMP after DEFAULT. A bare expression
    // would make NOT NULL after it ambiguous.
    _default_value: $ => choice(
      $._literal,
      alias($._signed_number, $.unary_expression),
      $.parenthesized_expression,
      $.function_call,
      $.identifier,
    ),

    _signed_number: $ => seq(
      field('operator', choice('-', '+')),
      field('operand', $.number),
    ),

    data_type: $ => prec.right(seq(
      $.type_name,
      optional(parens(commaSep1(choice($.number, $.string)))),
      repeat(choice(
        kw('UNSIGNED'),
        kw('SIGNED'),
        kw('ZEROFILL'),
        kw('BINARY'),
        kw('ARRAY'),
        $._character_set,
        seq(kw('COLLATE'), field('collation', choice($.identifier, $.string))),
      )),
    )),

    type_name: _ => choice(
      kw('TINYINT'),
      kw('SMALLINT'),
      kw('MEDIUMINT'),
      kw('INT'),
      kw('INTEGER'),
      kw('BIGINT'),
      kw('INT1'),
      kw('INT2'),
      kw('INT3'),
      kw('INT4'),
      kw('INT8'),
      kw('DECIMAL'),
      kw('DEC'),
      kw('NUMERIC'),
      kw('FIXED'),
      kw('FLOAT'),
      seq(kw('DOUBLE'), optional(kw('PRECISION'))),
      kw('REAL'),
      kw('BIT'),
      kw('BOOL'),
      kw('BOOLEAN'),
      kw('SERIAL'),
      kw('DATE'),
      kw('DATETIME'),
      kw('TIMESTAMP'),
      kw('TIME'),
      kw('YEAR'),
      seq(optional(kw('NATIONAL')), kw('CHAR'), optional(kw('VARYING'))),
      seq(optional(kw('NATIONAL')), kw('CHARACTER'), optional(kw('VARYING'))),
      kw('NCHAR'),
      seq(optional(kw('NATIONAL')), kw('VARCHAR')),
      kw('NVARCHAR'),
      kw('VARCHARACTER'),
      kw('BINARY'),
      kw('VARBINARY'),
      kw('TINYBLOB'),
      kw('BLOB'),
      kw('MEDIUMBLOB'),
      kw('LONGBLOB'),
      kw('TINYTEXT'),
      kw('TEXT'),
      kw('MEDIUMTEXT'),
      kw('LONGTEXT'),
      kw('ENUM'),
      kw('SET'),
      kw('JSON'),
      kw('GEOMETRY'),
      kw('POINT'),
      kw('LINESTRING'),
      kw('POLYGON'),
      kw('MULTIPOINT'),
      kw('MULTILINESTRING'),
      kw('MULTIPOLYGON'),
      kw('GEOMETRYCOLLECTION'),
      kw('SIGNED'),
      kw('UNSIGNED'),
    ),

    index_definition: $ => seq(
      choice(
        seq(choice(kw('INDEX'), kw('KEY'))),
        seq(choice(kw('FULLTEXT'), kw('SPATIAL')), optional(choice(kw('INDEX'), kw('KEY')))),
      ),
      optional(field('name', $.identifier)),
      optional($._index_type),
      $.key_parts,
      repeat($._index_option),
    ),

    table_constraint: $ => seq(
      optional(seq(kw('CONSTRAINT'), optional(field('name', $.identifier)))),
      choice(
        seq(
          kws('PRIMARY', 'KEY'),
          optional($._index_type),
          $.key_parts,
          repeat($._index_option),
        ),
        seq(
          kw('UNIQUE'),
          optional(choice(kw('INDEX'), kw('KEY'))),
          optional(field('index', $.identifier)),
          optional($._index_type),
          $.key_parts,
          repeat($._index_option),
        ),
        seq(
          kws('FOREIGN', 'KEY'),
          optional(field('index', $.identifier)),
          $.column_list,
          $.references_clause,
        ),
        $.check_constraint,
      ),
    ),

    check_constraint: $ => prec.right(seq(
      kw('CHECK'),
      parens($._expression),
      optional(seq(optional(kw('NOT')), kw('ENFORCED'))),
    )),

    references_clause: $ => prec.right(seq(
      kw('REFERENCES'),
      field('table', $.object_reference),
      optional($.column_list),
      optional(seq(kw('MATCH'), choice(kw('FULL'), kw('PARTIAL'), kw('SIMPLE')))),
      repeat(seq(
        kw('ON'),
        choice(kw('DELETE'), kw('UPDATE')),
        choice(
          kw('RESTRICT'),
          kw('CASCADE'),
          kws('SET', 'NULL'),
          kws('SET', 'DEFAULT'),
          kws('NO', 'ACTION'),
        ),
      )),
    )),

    key_parts: $ => parens(commaSep1($.key_part)),

    key_part: $ => seq(
      choice(
        seq(field('column', $.identifier), optional(parens(field('length', $.number)))),
        parens(field('expression', $._expression)),
      ),
      optional(choice(kw('ASC'), kw('DESC'))),
    ),

    _index_type: _ => seq(kw('USING'), choice(kw('BTREE'), kw('HASH'))),

    _index_option: $ => choice(
      seq(kw('KEY_BLOCK_SIZE'), optional('='), $.number),
      $._index_type,
      seq(kws('WITH', 'PARSER'), $.identifier),
      seq(kw('COMMENT'), $.string),
      kw('VISIBLE'),
      kw('INVISIBLE'),
    ),

    column_list: $ => parens(commaSep1($.identifier)),

    _table_options: $ => seq(
      $.table_option,
      repeat(seq(optional(','), $.table_option)),
    ),

    table_option: $ => choice(
      seq(
        optional(kw('DEFAULT')),
        choice(kws('CHARACTER', 'SET'), kw('CHARSET')),
        optional('='),
        field('value', choice($._charset_name, kw('DEFAULT'))),
      ),
      seq(
        optional(kw('DEFAULT')),
        kw('COLLATE'),
        optional('='),
        field('value', choice($.identifier, $.string, kw('DEFAULT'))),
      ),
      seq(
        field('name', choice(
          kw('ENGINE'),
          kw('AUTO_INCREMENT'),
          kw('AVG_ROW_LENGTH'),
          kw('CHECKSUM'),
          kw('COMMENT'),
          kw('COMPRESSION'),
          kw('CONNECTION'),
          kws('DATA', 'DIRECTORY'),
          kws('INDEX', 'DIRECTORY'),
          kw('DELAY_KEY_WRITE'),
          kw('ENCRYPTION'),
          kw('INSERT_METHOD'),
          kw('KEY_BLOCK_SIZE'),
          kw('MAX_ROWS'),
          kw('MIN_ROWS'),
          kw('PACK_KEYS'),
          kw('PASSWORD'),
          kw('ROW_FORMAT'),
          kw('STATS_AUTO_RECALC'),
          kw('STATS_PERSISTENT'),
          kw('STATS_SAMPLE_PAGES'),
          kw('TABLESPACE'),
        )),
        optional('='),
        field('value', choice($.identifier, $.string, $.number, kw('DEFAULT'))),
      ),
      seq(kw('UNION'), optional('='), parens(commaSep1($.object_reference))),
    ),

    partition_by: $ => prec.right(seq(
      kws('PARTITION', 'BY'),
      choice(
        seq(optional(kw('LINEAR')), kw('HASH'), parens($._expression)),
        seq(
          optional(kw('LINEAR')),
          kw('KEY'),
          optional(seq(kw('ALGORITHM'), '=', $.number)),
          parens(optional(commaSep1($.identifier))),
        ),
        seq(
          choice(kw('RANGE'), kw('LIST')),
          choice(
            parens($._expression),
            seq(kw('COLUMNS'), parens(commaSep1($.identifier))),
          ),
        ),
      ),
      optional(seq(kw('PARTITIONS'), $.number)),
      optional(parens(commaSep1($.partition_definition))),
    )),

    partition_definition: $ => seq(
      kw('PARTITION'),
      field('name', $.identifier),
      optional(seq(
        kw('VALUES'),
        choice(
          seq(
            kws('LESS', 'THAN'),
            choice(kw('MAXVALUE'), parens(commaSep1($._expression))),
          ),
          seq(kw('IN'), parens(commaSep1($._expression))),
        ),
      )),
      repeat($.table_option),
    ),

    alter_table_statement: $ => seq(
      kw('ALTER'),
      kw('TABLE'),
      field('name', $.object_reference),
      optional(commaSep1($._alter_specification)),
      optional($.partition_by),
    ),

    _alter_specification: $ => choice(
      $.add_column,
      $.add_constraint,
      $.drop_column,
      $.drop_constraint,
      $.modify_column,
      $.change_column,
      $.rename_column,
      $.rename_index,
      $.rename_table,
      $.alter_column,
      $.alter_index,
      $.convert_to_charset,
      $.alter_option,
      $.table_option,
    ),

    add_column: $ => seq(
      kw('ADD'),
      optional(kw('COLUMN')),
      choice(
        seq($.column_definition, optional($._column_position)),
        parens(commaSep1($.column_definition)),
      ),
    ),

    _column_position: $ => choice(
      kw('FIRST'),
      seq(kw('AFTER'), field('after', $.identifier)),
    ),

    add_constraint: $ => seq(
      kw('ADD'),
      choice($.index_definition, $.table_constraint),
    ),

    drop_column: $ => seq(
      kw('DROP'),
      optional(kw('COLUMN')),
      field('name', $.identifier),
    ),

    drop_constraint: $ => seq(
      kw('DROP'),
      choice(
        seq(choice(kw('INDEX'), kw('KEY')), field('name', $.identifier)),
        kws('PRIMARY', 'KEY'),
        seq(kws('FOREIGN', 'KEY'), field('name', $.identifier)),
        seq(choice(kw('CHECK'), kw('CONSTRAINT')), field('name', $.identifier)),
      ),
    ),

    modify_column: $ => seq(
      kw('MODIFY'),
      optional(kw('COLUMN')),
      $.column_definition,
      optional($._column_position),
    ),

    change_column: $ => seq(
      kw('CHANGE'),
      optional(kw('COLUMN')),
      field('old', $.identifier),
      $.column_definition,
      optional($._column_position),
    ),

    rename_column: $ => seq(
      kws('RENAME', 'COLUMN'),
      field('old', $.identifier),
      kw('TO'),
      field('new', $.identifier),
    ),

    rename_index: $ => seq(
      kw('RENAME'),
      choice(kw('INDEX'), kw('KEY')),
      field('old', $.identifier),
      kw('TO'),
      field('new', $.identifier),
    ),

    rename_table: $ => seq(
      kw('RENAME'),
      optional(choice(kw('TO'), kw('AS'))),
      field('new', $.object_reference),
    ),

    alter_column: $ => seq(
      kw('ALTER'),
      optional(kw('COLUMN')),
      field('name', $.identifier),
      choice(
        seq(kws('SET', 'DEFAULT'), field('default', $._default_value)),
        kws('DROP', 'DEFAULT'),
        seq(kw('SET'), choice(kw('VISIBLE'), kw('INVISIBLE'))),
      ),
    ),

    alter_index: $ => seq(
      kws('ALTER', 'INDEX'),
      field('name', $.identifier),
      choice(kw('VISIBLE'), kw('INVISIBLE')),
    ),

    convert_to_charset: $ => seq(
      kws('CONVERT', 'TO'),
      $._character_set,
      optional(seq(kw('COLLATE'), field('collation', choice($.identifier, $.string)))),
    ),

    alter_option: $ => choice(
      seq(kw('ALGORITHM'), optional('='), field('value', choice($.identifier, kw('DEFAULT')))),
      seq(kw('LOCK'), optional('='), field('value', choice($.identifier, kw('DEFAULT')))),
      kw('FORCE'),
      seq(choice(kw('ENABLE'), kw('DISABLE')), kw('KEYS')),
      prec.right(seq(kws('ORDER', 'BY'), commaSep1($.identifier))),
    ),

    drop_table_statement: $ => seq(
      kw('DROP'),
      optional(kw('TEMPORARY')),
      choice(kw('TABLE'), kw('TABLES')),
      optional($._if_exists),
      commaSep1($.object_reference),
      optional(choice(kw('RESTRICT'), kw('CASCADE'))),
    ),

    rename_table_statement: $ => seq(
      kw('RENAME'),
      choice(kw('TABLE'), kw('TABLES')),
      commaSep1(seq(
        field('old', $.object_reference),
        kw('TO'),
        field('new', $.object_reference),
      )),
    ),

    truncate_statement: $ => seq(
      kw('TRUNCATE'),
      optional(kw('TABLE')),
      field('name', $.object_reference),
    ),

    // Indexes

    create_index_statement: $ => seq(
      kw('CREATE'),
      optional(choice(kw('UNIQUE'), kw('FULLTEXT'), kw('SPATIAL'))),
      kw('INDEX'),
      field('name', $.identifier),
      optional($._index_type),
      kw('ON'),
      field('table', $.object_reference),
      $.key_parts,
      repeat(choice($._index_option, $.alter_option)),
    ),

    drop_index_statement: $ => seq(
      kws('DROP', 'INDEX'),
      field('name', $.identifier),
      kw('ON'),
      field('table', $.object_reference),
      repeat($.alter_option),
    ),

    // Views

    create_view_statement: $ => seq(
      kw('CREATE'),
      optional(kws('OR', 'REPLACE')),
      repeat($._view_option),
      kw('VIEW'),
      optional($._if_not_exists),
      ...viewBody($),
    ),

    alter_view_statement: $ => seq(
      kw('ALTER'),
      repeat($._view_option),
      kw('VIEW'),
      ...viewBody($),
    ),

    _view_option: $ => choice(
      seq(kw('ALGORITHM'), '=', choice(kw('UNDEFINED'), kw('MERGE'), kw('TEMPTABLE'))),
      $.definer,
      $._sql_security,
    ),

    _sql_security: _ => seq(kws('SQL', 'SECURITY'), choice(kw('DEFINER'), kw('INVOKER'))),

    definer: $ => seq(
      kw('DEFINER'),
      '=',
      $.account,
    ),

    drop_view_statement: $ => seq(
      kw('DROP'),
      kw('VIEW'),
      optional($._if_exists),
      commaSep1($.object_reference),
      optional(choice(kw('RESTRICT'), kw('CASCADE'))),
    ),

    // Stored programs

    create_trigger_statement: $ => seq(
      kw('CREATE'),
      optional($.definer),
      kw('TRIGGER'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      field('time', choice(kw('BEFORE'), kw('AFTER'))),
      field('event', choice(kw('INSERT'), kw('UPDATE'), kw('DELETE'))),
      kw('ON'),
      field('table', $.object_reference),
      kws('FOR', 'EACH', 'ROW'),
      optional(seq(choice(kw('FOLLOWS'), kw('PRECEDES')), field('other', $.identifier))),
      field('body', $._routine_body),
    ),

    drop_trigger_statement: $ => seq(
      kws('DROP', 'TRIGGER'),
      optional($._if_exists),
      field('name', $.object_reference),
    ),

    create_procedure_statement: $ => seq(
      kw('CREATE'),
      optional($.definer),
      kw('PROCEDURE'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      parens(optional(commaSep1($.routine_parameter))),
      repeat($.characteristic),
      field('body', $._routine_body),
    ),

    create_function_statement: $ => seq(
      kw('CREATE'),
      optional($.definer),
      kw('FUNCTION'),
      optional($._if_not_exists),
      field('name', $.object_reference),
      parens(optional(commaSep1($.routine_parameter))),
      kw('RETURNS'),
      field('return_type', $.data_type),
      repeat($.characteristic),
      field('body', $._routine_body),
    ),

    // A parameter of a function takes no mode, but the grammar accepts one.
    routine_parameter: $ => seq(
      optional(choice(kw('IN'), kw('OUT'), kw('INOUT'))),
      field('name', $.identifier),
      field('type', $.data_type),
    ),

    characteristic: $ => choice(
      seq(kw('COMMENT'), $.string),
      kws('LANGUAGE', 'SQL'),
      seq(optional(kw('NOT')), kw('DETERMINISTIC')),
      kws('CONTAINS', 'SQL'),
      kws('NO', 'SQL'),
      kws('READS', 'SQL', 'DATA'),
      kws('MODIFIES', 'SQL', 'DATA'),
      $._sql_security,
    ),

    alter_routine_statement: $ => seq(
      kw('ALTER'),
      choice(kw('PROCEDURE'), kw('FUNCTION')),
      field('name', $.object_reference),
      repeat($.characteristic),
    ),

    drop_routine_statement: $ => seq(
      kw('DROP'),
      choice(kw('PROCEDURE'), kw('FUNCTION')),
      optional($._if_exists),
      field('name', $.object_reference),
    ),

    _routine_body: $ => $._block_item,

    // A statement of a stored program: an SQL statement, a declaration or
    // a compound statement.
    _block_item: $ => choice(
      $._statement,
      $.block,
      $.declare_variable,
      $.declare_condition,
      $.declare_cursor,
      $.declare_handler,
      $.if_statement,
      $.case_statement,
      $.loop_statement,
      $.while_statement,
      $.repeat_statement,
      $.leave_statement,
      $.iterate_statement,
      $.return_statement,
      $.open_statement,
      $.fetch_statement,
      $.close_statement,
      $.signal_statement,
      $.resignal_statement,
    ),

    _block_items: $ => repeat1(seq($._block_item, ';')),

    _label: $ => seq(field('label', $.identifier), ':'),

    block: $ => seq(
      optional($._label),
      kw('BEGIN'),
      optional($._block_items),
      kw('END'),
      optional(field('end_label', $.identifier)),
    ),

    declare_variable: $ => seq(
      kw('DECLARE'),
      commaSep1(field('name', $.identifier)),
      field('type', $.data_type),
      optional(seq(kw('DEFAULT'), field('default', $._expression))),
    ),

    declare_condition: $ => seq(
      kw('DECLARE'),
      field('name', $.identifier),
      kws('CONDITION', 'FOR'),
      choice($._sqlstate, $.number),
    ),

    _sqlstate: $ => seq(kw('SQLSTATE'), optional(kw('VALUE')), $.string),

    declare_cursor: $ => seq(
      kw('DECLARE'),
      field('name', $.identifier),
      kws('CURSOR', 'FOR'),
      $.select_statement,
    ),

    declare_handler: $ => seq(
      kw('DECLARE'),
      choice(kw('CONTINUE'), kw('EXIT'), kw('UNDO')),
      kws('HANDLER', 'FOR'),
      commaSep1(choice(
        $.number,
        $._sqlstate,
        kw('SQLWARNING'),
        kws('NOT', 'FOUND'),
        kw('SQLEXCEPTION'),
        $.identifier,
      )),
      field('body', $._block_item),
    ),

    if_statement: $ => seq(
      kw('IF'),
      field('condition', $._expression),
      kw('THEN'),
      $._block_items,
      repeat($.elseif_clause),
      optional($.else_clause),
      kws('END', 'IF'),
    ),

    elseif_clause: $ => seq(
      kw('ELSEIF'),
      field('condition', $._expression),
      kw('THEN'),
      $._block_items,
    ),

    else_clause: $ => seq(kw('ELSE'), $._block_items),

    case_statement: $ => seq(
      kw('CASE'),
      optional(field('value', $._expression)),
      repeat1(alias($._case_statement_when, $.when_clause)),
      optional($.else_clause),
      kws('END', 'CASE'),
    ),

    _case_statement_when: $ => seq(
      kw('WHEN'),
      field('condition', $._expression),
      kw('THEN'),
      $._block_items,
    ),

    loop_statement: $ => seq(
      optional($._label),
      kw('LOOP'),
      $._block_items,
      kws('END', 'LOOP'),
      optional(field('end_label', $.identifier)),
    ),

    while_statement: $ => seq(
      optional($._label),
      kw('WHILE'),
      field('condition', $._expression),
      kw('DO'),
      $._block_items,
      kws('END', 'WHILE'),
      optional(field('end_label', $.identifier)),
    ),

    repeat_statement: $ => seq(
      optional($._label),
      kw('REPEAT'),
      $._block_items,
      kw('UNTIL'),
      field('condition', $._expression),
      kws('END', 'REPEAT'),
      optional(field('end_label', $.identifier)),
    ),

    leave_statement: $ => seq(kw('LEAVE'), field('label', $.identifier)),

    iterate_statement: $ => seq(kw('ITERATE'), field('label', $.identifier)),

    return_statement: $ => seq(kw('RETURN'), $._expression),

    open_statement: $ => seq(kw('OPEN'), field('cursor', $.identifier)),

    close_statement: $ => seq(kw('CLOSE'), field('cursor', $.identifier)),

    fetch_statement: $ => seq(
      kw('FETCH'),
      optional(seq(optional(kw('NEXT')), kw('FROM'))),
      field('cursor', $.identifier),
      kw('INTO'),
      commaSep1(choice($.identifier, $.user_variable)),
    ),

    signal_statement: $ => seq(
      kw('SIGNAL'),
      choice($._sqlstate, $.identifier),
      optional($._signal_information),
    ),

    resignal_statement: $ => prec.right(seq(
      kw('RESIGNAL'),
      optional(choice($._sqlstate, $.identifier)),
      optional($._signal_information),
    )),

    _signal_information: $ => seq(
      kw('SET'),
      commaSep1(seq($.identifier, '=', $._expression)),
    ),

    // Accounts and privileges

    account: $ => choice(
      seq(
        field('user', choice($.identifier, $.string)),
        optional(seq('@', field('host', choice($.identifier, $.string)))),
      ),
      seq(kw('CURRENT_USER'), optional(seq('(', ')'))),
    ),

    // A role in GRANT and REVOKE is an account. A role that is one bare
    // identifier is a privilege, because the two cannot be told apart.
    _role: $ => choice(
      field('user', $.string),
      seq(
        field('user', choice($.identifier, $.string)),
        '@',
        field('host', choice($.identifier, $.string)),
      ),
    ),

    _authentication: $ => seq(
      kw('IDENTIFIED'),
      choice(
        seq(kw('BY'), $.string),
        seq(
          kw('WITH'),
          field('plugin', choice($.identifier, $.string)),
          optional(seq(choice(kw('BY'), kw('AS')), $.string)),
        ),
      ),
    ),

    create_user_statement: $ => seq(
      kws('CREATE', 'USER'),
      optional($._if_not_exists),
      commaSep1(seq($.account, optional($._authentication))),
      optional(seq(kws('DEFAULT', 'ROLE'), commaSep1($.account))),
    ),

    alter_user_statement: $ => seq(
      kws('ALTER', 'USER'),
      optional($._if_exists),
      commaSep1(seq($.account, optional($._authentication))),
    ),

    drop_user_statement: $ => seq(
      kws('DROP', 'USER'),
      optional($._if_exists),
      commaSep1($.account),
    ),

    grant_statement: $ => seq(
      kw('GRANT'),
      choice(
        seq(
          commaSep1(choice($.privilege, alias($._role, $.account))),
          optional(seq(kw('ON'), $._privilege_object)),
          kw('TO'),
          commaSep1($.account),
          optional(choice(kws('WITH', 'GRANT', 'OPTION'), kws('WITH', 'ADMIN', 'OPTION'))),
        ),
        seq(
          kws('PROXY', 'ON'),
          $.account,
          kw('TO'),
          commaSep1($.account),
          optional(kws('WITH', 'GRANT', 'OPTION')),
        ),
      ),
    ),

    revoke_statement: $ => seq(
      kw('REVOKE'),
      optional($._if_exists),
      choice(
        seq(
          commaSep1(choice($.privilege, alias($._role, $.account))),
          optional(seq(kw('ON'), $._privilege_object)),
          kw('FROM'),
          commaSep1($.account),
        ),
        seq(kws('PROXY', 'ON'), $.account, kw('FROM'), commaSep1($.account)),
      ),
    ),

    _privilege_object: $ => seq(
      optional(choice(kw('TABLE'), kw('FUNCTION'), kw('PROCEDURE'))),
      $.privilege_level,
    ),

    privilege: $ => seq(
      choice(
        seq(kw('ALL'), optional(kw('PRIVILEGES'))),
        seq(kw('ALTER'), optional(kw('ROUTINE'))),
        seq(kw('CREATE'), optional(choice(
          kw('ROUTINE'),
          kw('TABLESPACE'),
          kws('TEMPORARY', 'TABLES'),
          kw('USER'),
          kw('VIEW'),
          kw('ROLE'),
        ))),
        kw('DELETE'),
        seq(kw('DROP'), optional(kw('ROLE'))),
        kw('EVENT'),
        kw('EXECUTE'),
        kw('FILE'),
        kws('GRANT', 'OPTION'),
        kw('INDEX'),
        kw('INSERT'),
        kws('LOCK', 'TABLES'),
        kw('PROCESS'),
        kw('REFERENCES'),
        kw('RELOAD'),
        seq(kw('REPLICATION'), choice(kw('CLIENT'), kw('SLAVE'))),
        kw('SELECT'),
        seq(kw('SHOW'), choice(kw('DATABASES'), kw('VIEW'))),
        kw('SHUTDOWN'),
        kw('SUPER'),
        kw('TRIGGER'),
        kw('UPDATE'),
        kw('USAGE'),
        $.identifier,
      ),
      optional($.column_list),
    ),

    privilege_level: $ => choice(
      '*',
      seq('*', '.', '*'),
      seq(field('schema', $.identifier), '.', '*'),
      seq(field('schema', $.identifier), '.', field('name', $.identifier)),
      field('name', $.identifier),
    ),

    // Sessions and transactions

    use_statement: $ => seq(kw('USE'), field('name', $.identifier)),

    set_statement: $ => seq(
      kw('SET'),
      choice(
        commaSep1($.variable_assignment),
        seq(
          kw('NAMES'),
          choice(
            seq($._charset_name, optional(seq(kw('COLLATE'), choice($.identifier, $.string)))),
            kw('DEFAULT'),
          ),
        ),
        seq(choice(kws('CHARACTER', 'SET'), kw('CHARSET')), choice($._charset_name, kw('DEFAULT'))),
        seq(
          optional($._variable_scope),
          kw('TRANSACTION'),
          commaSep1($._transaction_characteristic),
        ),
        seq(
          kw('PASSWORD'),
          optional(seq(kw('FOR'), $.account)),
          '=',
          $.string,
        ),
      ),
    ),

    _variable_scope: _ => choice(
      kw('GLOBAL'),
      kw('SESSION'),
      kw('LOCAL'),
      kw('PERSIST'),
      kw('PERSIST_ONLY'),
    ),

    variable_assignment: $ => seq(
      choice(
        seq(optional($._variable_scope), field('left', $.column_reference)),
        field('left', choice($.user_variable, $.system_variable)),
      ),
      choice('=', ':='),
      field('right', choice($._expression, kw('DEFAULT'), kw('ON'), kw('ALL'))),
    ),

    _transaction_characteristic: _ => choice(
      seq(
        kws('ISOLATION', 'LEVEL'),
        choice(
          kws('REPEATABLE', 'READ'),
          kws('READ', 'COMMITTED'),
          kws('READ', 'UNCOMMITTED'),
          kw('SERIALIZABLE'),
        ),
      ),
      kws('READ', 'WRITE'),
      kws('READ', 'ONLY'),
    ),

    start_transaction_statement: _ => seq(
      kws('START', 'TRANSACTION'),
      optional(commaSep1(choice(
        kws('WITH', 'CONSISTENT', 'SNAPSHOT'),
        kws('READ', 'WRITE'),
        kws('READ', 'ONLY'),
      ))),
    ),

    _begin_work: _ => seq(kw('BEGIN'), optional(kw('WORK'))),

    commit_statement: $ => seq(
      kw('COMMIT'),
      optional(kw('WORK')),
      optional($._chain),
    ),

    rollback_statement: $ => seq(
      kw('ROLLBACK'),
      optional(kw('WORK')),
      choice(
        optional($._chain),
        seq(kw('TO'), optional(kw('SAVEPOINT')), field('savepoint', $.identifier)),
      ),
    ),

    _chain: _ => choice(
      seq(kw('AND'), optional(kw('NO')), kw('CHAIN'), optional(seq(optional(kw('NO')), kw('RELEASE')))),
      seq(optional(kw('NO')), kw('RELEASE')),
    ),

    savepoint_statement: $ => seq(kw('SAVEPOINT'), field('name', $.identifier)),

    release_savepoint_statement: $ => seq(
      kws('RELEASE', 'SAVEPOINT'),
      field('name', $.identifier),
    ),

    lock_tables_statement: $ => seq(
      kw('LOCK'),
      choice(kw('TABLE'), kw('TABLES')),
      commaSep1($.table_lock),
    ),

    table_lock: $ => seq(
      field('table', $.object_reference),
      optional(seq(optional(kw('AS')), field('alias', $.identifier))),
      choice(
        seq(kw('READ'), optional(kw('LOCAL'))),
        seq(optional(kw('LOW_PRIORITY')), kw('WRITE')),
      ),
    ),

    unlock_tables_statement: _ => seq(kw('UNLOCK'), choice(kw('TABLE'), kw('TABLES'))),

    prepare_statement: $ => seq(
      kw('PREPARE'),
      field('name', $.identifier),
      kw('FROM'),
      field('source', choice($.string, $.user_variable)),
    ),

    execute_statement: $ => seq(
      kw('EXECUTE'),
      field('name', $.identifier),
      optional(seq(kw('USING'), commaSep1($.user_variable))),
    ),

    deallocate_statement: $ => seq(
      choice(kw('DEALLOCATE'), kw('DROP')),
      kw('PREPARE'),
      field('name', $.identifier),
    ),

    call_statement: $ => seq(
      kw('CALL'),
      field('name', $.object_reference),
      optional(parens(optional(commaSep1($._expression)))),
    ),

    do_statement: $ => seq(kw('DO'), commaSep1($._expression)),

    // Information

    show_statement: $ => seq(
      kw('SHOW'),
      choice(
        seq(optional(kw('FULL')), kw('TABLES'), optional($._show_from), optional($._show_filter)),
        seq(choice(kw('DATABASES'), kw('SCHEMAS')), optional($._show_filter)),
        seq(
          optional(kw('EXTENDED')),
          optional(kw('FULL')),
          choice(kw('COLUMNS'), kw('FIELDS')),
          choice(kw('FROM'), kw('IN')),
          field('table', $.object_reference),
          optional($._show_from),
          optional($._show_filter),
        ),
        seq(
          optional(kw('EXTENDED')),
          choice(kw('INDEX'), kw('INDEXES'), kw('KEYS')),
          choice(kw('FROM'), kw('IN')),
          field('table', $.object_reference),
          optional($._show_from),
          optional($.where_clause),
        ),
        seq(
          kw('CREATE'),
          choice(
            seq(
              choice(kw('DATABASE'), kw('SCHEMA')),
              optional($._if_not_exists),
              field('name', $.identifier),
            ),
            seq(
              choice(kw('TABLE'), kw('VIEW'), kw('PROCEDURE'), kw('FUNCTION'), kw('TRIGGER'), kw('EVENT')),
              field('name', $.object_reference),
            ),
            seq(kw('USER'), $.account),
          ),
        ),
        seq(optional($._variable_scope), choice(kw('VARIABLES'), kw('STATUS')), optional($._show_filter)),
        seq(optional(kw('FULL')), kw('PROCESSLIST')),
        seq(kw('GRANTS'), optional(seq(kw('FOR'), $.account))),
        seq(choice(kw('WARNINGS'), kw('ERRORS')), optional($.limit_clause)),
        seq(kw('COUNT'), '(', '*', ')', choice(kw('WARNINGS'), kw('ERRORS'))),
        seq(optional(kw('STORAGE')), kw('ENGINES')),
        seq(kw('ENGINE'), field('engine', $.identifier), choice(kw('STATUS'), kw('MUTEX'))),
        seq(kws('TABLE', 'STATUS'), optional($._show_from), optional($._show_filter)),
        seq(kw('TRIGGERS'), optional($._show_from), optional($._show_filter)),
        seq(kw('EVENTS'), optional($._show_from), optional($._show_filter)),
        seq(kws('OPEN', 'TABLES'), optional($._show_from), optional($._show_filter)),
        seq(choice(kw('PROCEDURE'), kw('FUNCTION')), kw('STATUS'), optional($._show_filter)),
        seq(choice(kw('PROCEDURE'), kw('FUNCTION')), kw('CODE'), field('name', $.object_reference)),
        seq(choice(kws('CHARACTER', 'SET'), kw('CHARSET')), optional($._show_filter)),
        seq(kw('COLLATION'), optional($._show_filter)),
        kw('PLUGINS'),
        kw('PRIVILEGES'),
        seq(choice(kw('BINARY'), kw('MASTER')), kw('LOGS')),
        kws('BINARY', 'LOG', 'STATUS'),
        seq(choice(kw('MASTER'), kw('REPLICA'), kw('SLAVE')), kw('STATUS')),
        kw('REPLICAS'),
        kws('SLAVE', 'HOSTS'),
        kw('PROFILES'),
      ),
    ),

    _show_from: $ => seq(choice(kw('FROM'), kw('IN')), field('database', $.identifier)),

    _show_filter: $ => choice(
      seq(kw('LIKE'), field('pattern', $.string)),
      $.where_clause,
    ),

    describe_statement: $ => prec.right(seq(
      choice(kw('DESCRIBE'), kw('DESC'), kw('EXPLAIN')),
      field('table', $.object_reference),
      optional(field('column', choice($.identifier, $.string))),
    )),

    explain_statement: $ => seq(
      choice(kw('EXPLAIN'), kw('DESCRIBE'), kw('DESC')),
      optional(choice(kw('ANALYZE'), kw('EXTENDED'), kw('PARTITIONS'))),
      optional(seq(kw('FORMAT'), '=', field('format', $.identifier))),
      choice(
        $.select_statement,
        $.insert_statement,
        $.replace_statement,
        $.update_statement,
        $.delete_statement,
        seq(kws('FOR', 'CONNECTION'), $.number),
      ),
    ),

    // Expressions

    _expression: $ => choice(
      $._primary_expression,
      $.unary_expression,
      $.binary_expression,
      $.is_expression,
      $.in_expression,
      $.between_expression,
      $.like_expression,
      $.collate_expression,
      $.assignment_expression,
    ),

    _primary_expression: $ => choice(
      $._literal,
      $.temporal_literal,
      $.column_reference,
      $.function_call,
      $.user_variable,
      $.system_variable,
      $.parameter,
      $.parenthesized_expression,
      $.row_constructor,
      $.subquery,
      $.exists_expression,
      $.case_expression,
      $.interval_expression,
      $.match_expression,
    ),

    _literal: $ => choice(
      $.number,
      $.string,
      $.boolean,
      $.null,
    ),

    boolean: _ => choice(kw('TRUE'), kw('FALSE')),

    null: _ => kw('NULL'),

    // DATE, TIME and TIMESTAMP before a string give a literal of the type.
    temporal_literal: $ => prec(1, seq(
      field('type', choice(kw('DATE'), kw('TIME'), kw('TIMESTAMP'))),
      field('value', $.string),
    )),

    column_reference: $ => choice(
      field('name', choice($.identifier, $._keyword_name)),
      seq(field('table', $.identifier), '.', field('name', $.identifier)),
      seq(
        field('schema', $.identifier),
        '.',
        field('table', $.identifier),
        '.',
        field('name', $.identifier),
      ),
    ),

    // These keywords start an expression, and MySQL does not reserve them,
    // so they also name a column.
    _keyword_name: $ => keywordName(
      $,
      'DATE',
      'TIME',
      'TIMESTAMP',
      'POSITION',
      'TRIM',
      'SUBSTRING',
      'SUBSTR',
      'EXTRACT',
      'CAST',
    ),

    function_call: $ => choice(
      prec.right(seq(
        optional(seq(field('schema', $.identifier), '.')),
        field('name', $.identifier),
        $.arguments,
        optional($.over_clause),
      )),
      seq(
        field('name', keywordName($, 'DATE', 'TIME', 'TIMESTAMP')),
        $.arguments,
      ),
      prec.right(seq(
        field('name', keywordName($, 'CURRENT_TIMESTAMP', 'CURRENT_DATE', 'CURRENT_TIME', 'CURRENT_USER', 'LOCALTIME', 'LOCALTIMESTAMP', 'UTC_DATE', 'UTC_TIME', 'UTC_TIMESTAMP')),
        optional(parens(optional($.number))),
      )),
      seq(
        field('name', keywordName($, 'CAST')),
        parens(seq($._expression, kw('AS'), $.data_type)),
      ),
      seq(
        field('name', keywordName($, 'CONVERT')),
        parens(seq(
          $._expression,
          choice(seq(',', $.data_type), seq(kw('USING'), $._charset_name)),
        )),
      ),
      seq(
        field('name', keywordName($, 'EXTRACT')),
        parens(seq($._interval_unit, kw('FROM'), $._expression)),
      ),
      seq(
        field('name', keywordName($, 'TRIM')),
        parens(choice(
          $._expression,
          seq(
            optional(choice(kw('BOTH'), kw('LEADING'), kw('TRAILING'))),
            optional($._expression),
            kw('FROM'),
            $._expression,
          ),
        )),
      ),
      seq(
        field('name', keywordName($, 'SUBSTRING', 'SUBSTR')),
        parens(seq(
          $._expression,
          choice(
            seq(',', $._expression, optional(seq(',', $._expression))),
            seq(kw('FROM'), $._expression, optional(seq(kw('FOR'), $._expression))),
          ),
        )),
      ),
      seq(
        field('name', keywordName($, 'POSITION')),
        parens(seq($._primary_expression, kw('IN'), $._expression)),
      ),
    ),

    arguments: $ => parens(optional(choice(
      '*',
      seq(
        optional(choice(kw('DISTINCT'), kw('ALL'))),
        commaSep1($._expression),
        optional($.order_by_clause),
        optional(seq(kw('SEPARATOR'), $.string)),
      ),
    ))),

    over_clause: $ => seq(
      kw('OVER'),
      choice(field('window', $.identifier), $.window_specification),
    ),

    parenthesized_expression: $ => parens($._expression),

    row_constructor: $ => choice(
      seq(kw('ROW'), parens(commaSep1($._expression))),
      parens(seq($._expression, ',', commaSep1($._expression))),
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

    interval_expression: $ => seq(
      kw('INTERVAL'),
      field('value', $._expression),
      field('unit', $._interval_unit),
    ),

    _interval_unit: _ => choice(
      kw('MICROSECOND'),
      kw('SECOND'),
      kw('MINUTE'),
      kw('HOUR'),
      kw('DAY'),
      kw('WEEK'),
      kw('MONTH'),
      kw('QUARTER'),
      kw('YEAR'),
      kw('SECOND_MICROSECOND'),
      kw('MINUTE_MICROSECOND'),
      kw('MINUTE_SECOND'),
      kw('HOUR_MICROSECOND'),
      kw('HOUR_SECOND'),
      kw('HOUR_MINUTE'),
      kw('DAY_MICROSECOND'),
      kw('DAY_SECOND'),
      kw('DAY_MINUTE'),
      kw('DAY_HOUR'),
      kw('YEAR_MONTH'),
    ),

    match_expression: $ => seq(
      kw('MATCH'),
      parens(commaSep1($.column_reference)),
      kw('AGAINST'),
      parens(seq(
        $._expression,
        optional(choice(
          kws('IN', 'NATURAL', 'LANGUAGE', 'MODE'),
          kws('IN', 'NATURAL', 'LANGUAGE', 'MODE', 'WITH', 'QUERY', 'EXPANSION'),
          kws('IN', 'BOOLEAN', 'MODE'),
          kws('WITH', 'QUERY', 'EXPANSION'),
        )),
      )),
    ),

    unary_expression: $ => choice(
      prec(PREC.not, seq(field('operator', kw('NOT')), field('operand', $._expression))),
      prec(PREC.bang, seq(field('operator', '!'), field('operand', $._expression))),
      prec(PREC.unary, seq(field('operator', choice('-', '+', '~')), field('operand', $._expression))),
      prec.left(PREC.collate, seq(field('operator', kw('BINARY')), field('operand', $._expression))),
    ),

    binary_expression: $ => {
      /** @type {[number, RuleOrLiteral][]} */
      const table = [
        [PREC.or, choice('||', kw('OR'))],
        [PREC.xor, kw('XOR')],
        [PREC.and, choice('&&', kw('AND'))],
        [PREC.bitor, '|'],
        [PREC.bitand, '&'],
        [PREC.shift, choice('<<', '>>')],
        [PREC.add, choice('+', '-')],
        [PREC.multiply, choice('*', '/', '%', kw('DIV'), kw('MOD'))],
        [PREC.bitxor, '^'],
        [PREC.json, choice('->', '->>')],
      ];
      return choice(
        ...table.map(([p, op]) => prec.left(p, seq(
          field('left', $._expression),
          field('operator', op),
          field('right', $._expression),
        ))),
        prec.left(PREC.compare, seq(
          field('left', $._expression),
          field('operator', choice('=', '<=>', '>=', '>', '<=', '<', '<>', '!=')),
          field('right', choice($._expression, $.quantified_subquery)),
        )),
        prec.left(PREC.compare, seq(
          field('left', $._expression),
          field('operator', seq(kw('MEMBER'), optional(kw('OF')))),
          field('right', $.parenthesized_expression),
        )),
      );
    },

    quantified_subquery: $ => seq(
      field('quantifier', choice(kw('ANY'), kw('SOME'), kw('ALL'))),
      $.subquery,
    ),

    is_expression: $ => prec.left(PREC.compare, seq(
      field('left', $._expression),
      kw('IS'),
      optional(kw('NOT')),
      field('right', choice($.boolean, $.null, kw('UNKNOWN'))),
    )),

    in_expression: $ => prec.left(PREC.compare, seq(
      field('left', $._expression),
      optional(kw('NOT')),
      kw('IN'),
      field('right', choice($.subquery, $.list)),
    )),

    list: $ => parens(commaSep1($._expression)),

    between_expression: $ => prec.left(PREC.between, seq(
      field('left', $._expression),
      optional(kw('NOT')),
      kw('BETWEEN'),
      field('low', $._expression),
      kw('AND'),
      field('high', $._expression),
    )),

    like_expression: $ => prec.left(PREC.compare, seq(
      field('left', $._expression),
      optional(kw('NOT')),
      field('operator', choice(kw('LIKE'), kw('REGEXP'), kw('RLIKE'), kws('SOUNDS', 'LIKE'))),
      field('right', $._expression),
      optional(seq(kw('ESCAPE'), field('escape', $._expression))),
    )),

    collate_expression: $ => prec.left(PREC.collate, seq(
      field('value', $._expression),
      kw('COLLATE'),
      field('collation', choice($.identifier, $.string)),
    )),

    assignment_expression: $ => prec.right(PREC.assign, seq(
      field('left', $.user_variable),
      ':=',
      field('right', $._expression),
    )),

    // Names and literals

    object_reference: $ => seq(
      optional(seq(field('schema', $.identifier), '.')),
      field('name', $.identifier),
    ),

    identifier: $ => choice($._identifier, $._quoted_identifier),

    _identifier: _ => /[A-Za-z_\u0080-￿][A-Za-z0-9_$\u0080-￿]*/,

    _quoted_identifier: _ => /`([^`]|``)*`/,

    user_variable: _ => token(seq('@', choice(
      /[A-Za-z0-9_$.\u0080-￿]+/,
      /'([^'\\]|\\(.|\n)|'')*'/,
      /"([^"\\]|\\(.|\n)|"")*"/,
      /`([^`]|``)*`/,
    ))),

    system_variable: _ => token(seq('@@', /[A-Za-z_][A-Za-z0-9_$]*(\.[A-Za-z_][A-Za-z0-9_$]*)?/)),

    parameter: _ => '?',

    // A number is an integer, a decimal number, a number with an exponent,
    // or a hexadecimal or binary number in the form 0x1f or 0b101.
    number: _ => token(choice(
      /(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?/,
      /0x[0-9A-Fa-f]+/,
      /0b[01]+/,
    )),

    // A string is in single or double quotes, with backslash escapes and
    // doubled quotes. It can start with N, with a character set such as
    // _utf8mb4, or with X or B for a hexadecimal or binary string.
    string: _ => token(choice(
      /'([^'\\]|\\(.|\n)|'')*'/,
      /"([^"\\]|\\(.|\n)|"")*"/,
      /[nN]'([^'\\]|\\(.|\n)|'')*'/,
      /_[A-Za-z0-9]+'([^'\\]|\\(.|\n)|'')*'/,
      /_[A-Za-z0-9]+"([^"\\]|\\(.|\n)|"")*"/,
      /[xX]'[0-9A-Fa-f]*'/,
      /[bB]'[01]*'/,
    )),
  },
});

/**
 * Returns the rules after INSERT or REPLACE and its modifiers.
 *
 * @param {GrammarSymbols<string>} $
 */
function insertBody($) {
  return [
    optional(kw('INTO')),
    field('table', $.object_reference),
    optional($.partition_list),
    choice(
      seq(
        optional($.column_list),
        $.values_clause,
        optional(seq(kw('AS'), field('alias', $.identifier), optional($.column_list))),
      ),
      seq(kw('SET'), commaSep1($.assignment)),
      seq(optional($.column_list), $.select_statement),
    ),
  ];
}

/**
 * Returns the rules of a view after VIEW.
 *
 * @param {GrammarSymbols<string>} $
 */
function viewBody($) {
  return [
    field('name', $.object_reference),
    optional($.column_list),
    kw('AS'),
    $.select_statement,
    optional(seq(
      kw('WITH'),
      optional(choice(kw('CASCADED'), kw('LOCAL'))),
      kws('CHECK', 'OPTION'),
    )),
  ];
}
