/**
 * @file The grammar of the Transact-SQL of SQL Server 2025
 * @license MIT
 */

// xo writes this grammar from the Transact-SQL reference of SQL Server 2025
// (D124). It does not come from the grammar of Crary-Systems/tree-sitter-tsql.
//
// The grammar is for a highlighter and for completion, so it accepts more
// than the server does where that keeps the rules small. A keyword is a
// token in any case, and it is an anonymous node with the name of the
// keyword in upper case, such as "SELECT". The token identifier is the word
// of the grammar, so a keyword that SQL Server does not reserve is an
// identifier in each place where the grammar does not expect the keyword.
// The reserved keywords of Transact-SQL are in the reserved set global, so
// they are not identifiers, but for a name after a dot and a word of a list
// of options, where the grammar accepts every keyword.
//
// The grammar keeps its parse tables small (D122, D124). A parse state with
// more than 64 columns takes the large form, so the grammar keeps the
// columns of a state few. The clauses of a query after FROM, and the
// clauses of a table after its name, come in any order, because a sequence
// of optional clauses costs parse states for each set of the clauses. The
// hints of a table and of a query, and the options of FOR XML and FOR JSON,
// are a list of words. A word of such a list is an identifier that takes
// the reserved set label, so that one token covers each word of the list.
//
// A statement of Transact-SQL can end at a semicolon, but it does not have
// to, so two statements can follow each other with nothing between them.
// The reserved keywords end the statement before them, as they do in the
// server. A keyword that SQL Server does not reserve, such as THROW, can be
// the alias of a table at the end of the statement before it, as in the
// server, which needs a semicolon before such a statement.
//
// The external scanner in src/scanner.c reads the comments, which nest, and
// a line with only GO and a count, which ends a batch in sqlcmd and in SQL
// Server Management Studio. GO is not Transact-SQL, but a script holds it,
// so the grammar takes it as the end of a batch. The other commands of
// sqlcmd, such as :r and :setvar, belong to usql. The variables of usql
// belong to the usql grammar, which hands this grammar a placeholder of the
// same length in place of each variable (D101).

/// <reference types="tree-sitter-cli/dsl" />
// @ts-check

// The precedences of the operators, from the lowest to the highest. The
// section "Operator precedence" of the reference puts ALL, ANY, BETWEEN,
// IN, LIKE, OR and SOME on one level below AND. SQL Server reads a
// condition such as a LIKE b AND c as (a LIKE b) AND c, so the conditions
// bind more tightly than NOT, AND and OR here. The bitwise operators &, ^
// and | have the precedence of + and -, and ~ has the highest, as the
// reference says.
const PREC = {
  or: 1,
  and: 2,
  not: 3,
  condition: 4,
  compare: 5,
  add: 6,
  multiply: 7,
  unary: 8,
  at: 9,
  collate: 10,
  field: 11,
};

// The reserved keywords of Transact-SQL, from the page "Reserved keywords"
// of the reference. WITHIN GROUP is two words, and WITHIN is not reserved
// alone. PRECISION is left out: the grammar reads it only after DOUBLE, and
// scripts read the column precision of sys.columns with no brackets, which
// the server accepts.
const RESERVED_WORDS = [
  'ADD', 'ALL', 'ALTER', 'AND', 'ANY', 'AS', 'ASC', 'AUTHORIZATION', 'BACKUP',
  'BEGIN', 'BETWEEN', 'BREAK', 'BROWSE', 'BULK', 'BY', 'CASCADE', 'CASE',
  'CHECK', 'CHECKPOINT', 'CLOSE', 'CLUSTERED', 'COALESCE', 'COLLATE', 'COLUMN',
  'COMMIT', 'COMPUTE', 'CONSTRAINT', 'CONTAINS', 'CONTAINSTABLE', 'CONTINUE',
  'CONVERT', 'CREATE', 'CROSS', 'CURRENT', 'CURRENT_DATE', 'CURRENT_TIME',
  'CURRENT_TIMESTAMP', 'CURRENT_USER', 'CURSOR', 'DATABASE', 'DBCC',
  'DEALLOCATE', 'DECLARE', 'DEFAULT', 'DELETE', 'DENY', 'DESC', 'DISK',
  'DISTINCT', 'DISTRIBUTED', 'DOUBLE', 'DROP', 'DUMP', 'ELSE', 'END', 'ERRLVL',
  'ESCAPE', 'EXCEPT', 'EXEC', 'EXECUTE', 'EXISTS', 'EXIT', 'EXTERNAL', 'FETCH',
  'FILE', 'FILLFACTOR', 'FOR', 'FOREIGN', 'FREETEXT', 'FREETEXTTABLE', 'FROM',
  'FULL', 'FUNCTION', 'GOTO', 'GRANT', 'GROUP', 'HAVING', 'HOLDLOCK', 'IDENTITY',
  'IDENTITY_INSERT', 'IDENTITYCOL', 'IF', 'IN', 'INDEX', 'INNER', 'INSERT',
  'INTERSECT', 'INTO', 'IS', 'JOIN', 'KEY', 'KILL', 'LEFT', 'LIKE', 'LINENO',
  'LOAD', 'MERGE', 'NATIONAL', 'NOCHECK', 'NONCLUSTERED', 'NOT', 'NULL',
  'NULLIF', 'OF', 'OFF', 'OFFSETS', 'ON', 'OPEN', 'OPENDATASOURCE', 'OPENQUERY',
  'OPENROWSET', 'OPENXML', 'OPTION', 'OR', 'ORDER', 'OUTER', 'OVER', 'PERCENT',
  'PIVOT', 'PLAN', 'PRIMARY', 'PRINT', 'PROC', 'PROCEDURE',
  'PUBLIC', 'RAISERROR', 'READ', 'READTEXT', 'RECONFIGURE', 'REFERENCES',
  'REPLICATION', 'RESTORE', 'RESTRICT', 'RETURN', 'REVERT', 'REVOKE', 'RIGHT',
  'ROLLBACK', 'ROWCOUNT', 'ROWGUIDCOL', 'RULE', 'SAVE', 'SCHEMA',
  'SECURITYAUDIT', 'SELECT', 'SEMANTICKEYPHRASETABLE',
  'SEMANTICSIMILARITYDETAILSTABLE', 'SEMANTICSIMILARITYTABLE', 'SESSION_USER',
  'SET', 'SETUSER', 'SHUTDOWN', 'SOME', 'STATISTICS', 'SYSTEM_USER', 'TABLE',
  'TABLESAMPLE', 'TEXTSIZE', 'THEN', 'TO', 'TOP', 'TRAN', 'TRANSACTION',
  'TRIGGER', 'TRUNCATE', 'TRY_CONVERT', 'TSEQUAL', 'UNION', 'UNIQUE', 'UNPIVOT',
  'UPDATE', 'UPDATETEXT', 'USE', 'USER', 'VALUES', 'VARYING', 'VIEW', 'WAITFOR',
  'WHEN', 'WHERE', 'WHILE', 'WITH', 'WRITETEXT',
];

// The words that the rules use as keywords. The rules run before the
// reserved sets, so kw fills it before the reserved sets read it. A
// reserved word must be a token of the grammar, so the reserved set holds
// only the reserved words that are in it.
const KEYWORDS = new Set();

// The reserved words that are functions with no arguments and no
// parentheses, which the reference calls niladic functions. They are one
// token, and not keywords, so that they take one column of each state
// where an expression starts.
const NILADIC_FUNCTIONS = [
  'CURRENT_DATE', 'CURRENT_TIMESTAMP', 'CURRENT_USER', 'SESSION_USER',
  'SYSTEM_USER', 'USER',
];

// The reserved words that are also the names of functions, as in
// LEFT(s, 2) and CONTAINS(c, 'x'). They are one token, the node identifier,
// for the same reason.
const FUNCTION_WORDS = [
  'COALESCE', 'CONTAINS', 'FREETEXT', 'IDENTITY', 'LEFT', 'NULLIF', 'RIGHT',
];

// The reserved words that are the names of rowset functions, which give
// rows in FROM, as in FREETEXTTABLE(t, c, 'x'). They are not in the states
// where an expression starts, so that those states stay small.
const ROWSET_FUNCTION_WORDS = [
  'CONTAINSTABLE', 'FREETEXTTABLE', 'OPENDATASOURCE', 'OPENQUERY', 'OPENROWSET',
  'OPENXML', 'SEMANTICKEYPHRASETABLE', 'SEMANTICSIMILARITYDETAILSTABLE',
  'SEMANTICSIMILARITYTABLE',
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
  name: 'sqlserver',

  extras: $ => [
    /\s/,
    $.comment,
  ],

  // The order of the external tokens is the order of enum TokenType in
  // src/scanner.c.
  externals: $ => [
    $.comment,
    $._go,
  ],

  word: $ => $.identifier,

  // The reserved set global holds the reserved keywords of Transact-SQL
  // that the grammar uses as keywords. The set label is empty, because the
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
    // ( after the name of a table in FROM can start the arguments of a
    // table-valued function or the hints of the table in the old form, as
    // in t (NOLOCK).
    [$.function_call, $.object_reference],
  ],

  rules: {
    // A batch ends at a line with only GO. A statement can end at a
    // semicolon, and two statements can follow each other with nothing
    // between them.
    source_file: $ => repeat(choice(
      $._statement,
      ';',
      $.go_statement,
    )),

    // GO and a count, alone on its line, ends a batch, and runs it count
    // times.
    go_statement: $ => seq(
      alias($._go, 'GO'),
      optional(field('count', $.number)),
    ),

    _statement: $ => choice(
      $.select_statement,
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
        $.for_clause,
        $.option_clause,
      )),
    )),

    // A common table expression can name itself, which makes it
    // recursive. WITH XMLNAMESPACES comes before the common table
    // expressions.
    with_clause: $ => seq(
      kw('WITH'),
      choice(
        seq($.xmlnamespaces, optional(seq(',', commaSep1($.cte)))),
        commaSep1($.cte),
      ),
    ),

    xmlnamespaces: $ => seq(
      kw('XMLNAMESPACES'),
      parens(commaSep1(choice(
        seq(field('uri', $.string), kw('AS'), field('prefix', $._name)),
        seq(kw('DEFAULT'), field('uri', $.string)),
      ))),
    ),

    // A common table expression.
    cte: $ => seq(
      field('name', $._name),
      optional($.column_list),
      kw('AS'),
      $.subquery,
    ),

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
        optional(kw('ALL')),
        field('right', $._query_body),
      )),
      prec.left(2, seq(
        field('left', $._query_body),
        field('operator', kw('INTERSECT')),
        field('right', $._query_body),
      )),
    ),

    // The clauses after FROM come in any order.
    select: $ => prec.right(seq(
      kw('SELECT'),
      optional(choice(kw('ALL'), kw('DISTINCT'))),
      optional($.top_clause),
      commaSep1($._select_item),
      optional($.into_clause),
      optional($.from_clause),
      repeat(choice(
        $.where_clause,
        $.group_by_clause,
        $.having_clause,
        $.window_clause,
      )),
    )),

    // TOP takes an expression in parentheses. A number or a variable with
    // no parentheses is the old form.
    top_clause: $ => prec.right(seq(
      kw('TOP'),
      field('count', choice($.parenthesized_expression, $.number, $.variable)),
      optional(kw('PERCENT')),
      optional(kws('WITH', 'TIES')),
    )),

    _select_item: $ => choice(
      $._expression,
      $.aliased_expression,
      $.all_columns,
      $.assignment,
    ),

    // An alias after an expression, with AS or without it, or before the
    // expression with =, as in SELECT n = COUNT(*). The alias can be a
    // string. An expression a = b in a select list is an alias and a value,
    // not a comparison.
    aliased_expression: $ => choice(
      seq(
        field('value', $._expression),
        optional(kw('AS')),
        field('alias', choice($._name, $.string)),
      ),
      prec.dynamic(1, prec(PREC.compare + 1, seq(
        field('alias', choice($._name, $.string)),
        '=',
        field('value', $._expression),
      ))),
    ),

    // SELECT @v = expression sets a variable, and a compound assignment
    // operator, such as +=, changes it.
    assignment: $ => prec(PREC.compare + 1, seq(
      field('left', $.variable),
      field('operator', choice('=', alias($._compound_operator, $.operator))),
      field('right', $._expression),
    )),

    _compound_operator: _ => token(choice('+=', '-=', '*=', '/=', '%=', '&=', '^=', '|=')),

    all_columns: $ => choice(
      '*',
      seq(field('table', $._name), '.', '*'),
      seq(field('schema', $._name), '.', field('table', $._label), '.', '*'),
      seq(
        field('database', $._name),
        '.',
        optional(field('schema', $._label)),
        '.',
        field('table', $._label),
        '.',
        '*',
      ),
    ),

    // SELECT INTO makes a table. ON names its filegroup.
    into_clause: $ => seq(
      kw('INTO'),
      field('table', $.object_reference),
      optional(seq(kw('ON'), field('filegroup', $._name))),
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

    // The clauses of a table after its name come in any order. The hints
    // in parentheses without WITH are the old form, as in t (NOLOCK).
    table_reference: $ => prec.right(seq(
      field('table', choice($.object_reference, $.variable)),
      repeat(choice(
        $.system_time_clause,
        $.tablesample_clause,
        $.table_hints,
        $._alias,
        $._table_operation,
      )),
    )),

    _alias: $ => seq(optional(kw('AS')), field('alias', $._name)),

    // PIVOT and UNPIVOT come after a table, and an alias after them names
    // the result.
    _table_operation: $ => choice($.pivot_clause, $.unpivot_clause),

    // FOR SYSTEM_TIME reads a temporal table at a time or in a period.
    system_time_clause: $ => seq(
      alias($._for_system_time, 'FOR SYSTEM_TIME'),
      choice(
        seq(kws('AS', 'OF'), field('time', $._expression)),
        seq(kw('FROM'), field('start', $._expression), kw('TO'), field('end', $._expression)),
        seq(kw('BETWEEN'), field('start', $._expression), kw('AND'), field('end', $._expression)),
        seq(kws('CONTAINED', 'IN'), parens(seq(field('start', $._expression), ',', field('end', $._expression)))),
        kw('ALL'),
      ),
    ),

    // FOR SYSTEM_TIME is one token, so that FOR after a table can also
    // start FOR XML, FOR JSON or FOR BROWSE of the query.
    _for_system_time: _ => token(seq(keyword('FOR'), /\s+/, keyword('SYSTEM_TIME'))),

    tablesample_clause: $ => prec.right(seq(
      kw('TABLESAMPLE'),
      optional(kw('SYSTEM')),
      parens(seq(field('size', $._expression), optional(choice(kw('PERCENT'), kw('ROWS'))))),
      optional(seq(kw('REPEATABLE'), parens(field('seed', $._expression)))),
    )),

    // The hints of a table, as in WITH (NOLOCK, INDEX(ix)), or in the old
    // form with no WITH. A comma between two hints is optional.
    table_hints: $ => seq(
      optional(kw('WITH')),
      parens(seq($.hint, repeat(seq(optional(','), $.hint)))),
    ),

    // A hint is a word, with a value after = or a list in parentheses, as
    // in INDEX(ix1, ix2), INDEX = ix and FORCESEEK(ix (a, b)).
    hint: $ => prec.right(seq(
      $._option_word,
      optional(choice(
        seq('=', $._option_value),
        parens(commaSep1(choice($._option_value, seq($._option_value, $.column_list)))),
      )),
    )),

    _option_value: $ => choice(
      $._option_word,
      $.string,
      $._signed_number,
      $.variable,
    ),

    // A word of a list of options. It takes the reserved set label, so a
    // reserved word such as INDEX or HOLDLOCK is a word of the list too.
    // One identifier covers all the words, so that the states of the list
    // are few and small. Each word is a keyword in the tree, but a
    // delimited identifier stays an identifier.
    _option_word: $ => choice(
      alias(reserved('label', $.identifier), $.keyword),
      alias($._delimited_identifier, $.identifier),
    ),

    derived_table: $ => prec.right(seq(
      choice($.subquery, $.values),
      repeat(choice(
        $._alias,
        $.column_list,
        $._table_operation,
      )),
    )),

    // A table value constructor in FROM, as in (VALUES (1, 'a'), (2, 'b'))
    // AS v (n, s).
    values: $ => parens(seq(kw('VALUES'), commaSep1($.value_list))),

    value_list: $ => parens(commaSep1(choice($._expression, kw('DEFAULT')))),

    // A table-valued function, a rowset function such as OPENJSON or
    // OPENROWSET, or a method that gives rows, as in @x.nodes('/a') AS
    // t(c). WITH gives the columns of OPENJSON, OPENXML and OPENROWSET.
    function_table: $ => prec.right(seq(
      choice($.function_call, alias($._rowset_function, $.function_call), $.field_expression),
      repeat(choice(
        $.with_columns,
        $._alias,
        $.column_list,
        $._table_operation,
      )),
    )),

    // OPENDATASOURCE names a table of its server after it, as in
    // OPENDATASOURCE('MSOLEDBSQL', 'Data Source=s').db.dbo.t.
    _rowset_function: $ => prec.right(seq(
      field('name', choice(...ROWSET_FUNCTION_WORDS.map(kw))),
      $.arguments,
      optional(seq('.', field('table', $.object_reference))),
    )),

    // The columns of a rowset function, as in WITH (id int '$.id', doc
    // nvarchar(max) AS JSON), or a table that gives them, as in WITH t.
    with_columns: $ => seq(
      kw('WITH'),
      choice(
        parens(commaSep1($.column_definition)),
        field('table', $.object_reference),
      ),
    ),

    column_definition: $ => seq(
      field('name', $._name),
      field('type', $.type),
      optional(seq(kw('COLLATE'), field('collation', $._name))),
      optional(field('path', choice($.string, $.number))),
      optional(kws('AS', 'JSON')),
    ),

    parenthesized_join: $ => prec.right(seq(
      parens(choice($.join, $.parenthesized_join)),
      optional($._alias),
    )),

    // The words of a join before JOIN, such as LEFT OUTER HASH JOIN, are
    // one token with JOIN, so that they take one column of a parse state.
    // It is the keyword JOIN in the tree. CROSS APPLY and OUTER APPLY are
    // the keyword APPLY.
    join: $ => prec.left(seq(
      field('left', $._table_reference),
      choice(
        kw('JOIN'),
        alias($._join, 'JOIN'),
        alias($._apply, 'APPLY'),
      ),
      field('right', $._table_factor),
      optional(seq(kw('ON'), field('condition', $._expression))),
    )),

    _join: _ => token(seq(
      choice(
        keyword('INNER'),
        keyword('CROSS'),
        seq(
          choice(keyword('LEFT'), keyword('RIGHT'), keyword('FULL')),
          optional(seq(/\s+/, keyword('OUTER'))),
        ),
      ),
      optional(seq(/\s+/, choice(
        keyword('LOOP'),
        keyword('HASH'),
        keyword('MERGE'),
        keyword('REMOTE'),
      ))),
      /\s+/,
      keyword('JOIN'),
    )),

    _apply: _ => token(seq(
      choice(keyword('CROSS'), keyword('OUTER')),
      /\s+/,
      keyword('APPLY'),
    )),

    pivot_clause: $ => seq(
      kw('PIVOT'),
      '(',
      field('value', $.function_call),
      kw('FOR'),
      field('for', $.column_reference),
      kw('IN'),
      parens(commaSep1($._name)),
      ')',
    ),

    unpivot_clause: $ => seq(
      kw('UNPIVOT'),
      '(',
      field('value', $._name),
      kw('FOR'),
      field('for', $._name),
      kw('IN'),
      parens(commaSep1($._name)),
      ')',
    ),

    where_clause: $ => seq(kw('WHERE'), $._expression),

    // GROUP BY ALL and WITH ROLLUP and WITH CUBE after the list are the old
    // forms.
    group_by_clause: $ => prec.right(seq(
      kws('GROUP', 'BY'),
      optional(kw('ALL')),
      commaSep1($._grouping_element),
      optional(seq(kw('WITH'), choice(kw('ROLLUP'), kw('CUBE')))),
    )),

    _grouping_element: $ => choice(
      $._expression,
      $.list,
      $.empty_grouping_set,
      $.rollup,
      $.cube,
      $.grouping_sets,
    ),

    empty_grouping_set: _ => seq('(', ')'),

    rollup: $ => seq(kw('ROLLUP'), parens(commaSep1(choice($._expression, $.list)))),

    cube: $ => seq(kw('CUBE'), parens(commaSep1(choice($._expression, $.list)))),

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

    // The clauses of a window come in the order of the reference, but the
    // grammar takes them in any order. A window can start with the name of
    // a window of the WINDOW clause.
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

    order_by_item: $ => seq(
      field('value', $._expression),
      optional(choice(kw('ASC'), kw('DESC'))),
    ),

    offset_clause: $ => seq(
      kw('OFFSET'),
      field('start', $._expression),
      choice(kw('ROW'), kw('ROWS')),
    ),

    fetch_clause: $ => seq(
      kw('FETCH'),
      choice(kw('FIRST'), kw('NEXT')),
      field('count', $._expression),
      choice(kw('ROW'), kw('ROWS')),
      kw('ONLY'),
    ),

    // FOR XML, FOR JSON and FOR BROWSE. The options of FOR XML and FOR JSON
    // are words, with a value in parentheses, as in FOR XML PATH('r'),
    // ROOT('x'), TYPE, ELEMENTS XSINIL.
    for_clause: $ => prec.right(seq(
      kw('FOR'),
      choice(
        kw('BROWSE'),
        seq(
          choice(kw('XML'), kw('JSON')),
          commaSep1($._for_option),
        ),
      ),
    )),

    _for_option: $ => prec.right(seq(
      repeat1($._option_word),
      optional(parens($._option_value)),
    )),

    // OPTION gives the hints of a query, as in OPTION (RECOMPILE, MAXDOP
    // 1, OPTIMIZE FOR (@p = 1)). The hints are a list of words.
    option_clause: $ => seq(
      kw('OPTION'),
      $._option_group,
    ),

    _option_group: $ => parens(repeat($._option)),

    _option: $ => choice(
      $._option_value,
      '=',
      ',',
      $._option_group,
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
      $.column_reference,
      $.variable,
      $.global_variable,
      $.pseudocolumn,
      $.niladic_function,
      $.function_call,
      $.convert_expression,
      $.next_value_for,
      $.field_expression,
      $.parenthesized_expression,
      $.subquery,
      $.exists_expression,
      $.case_expression,
      $.odbc_escape,
    ),

    _literal: $ => choice(
      $.number,
      $.money,
      $.binary,
      $.string,
      $.null,
    ),

    null: _ => kw('NULL'),

    // A column, with its table, its schema and its database. A part can be
    // empty, as in db..t.
    column_reference: $ => prec.right(choice(
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
        optional(field('schema', $._label)),
        '.',
        field('table', $._label),
        '.',
        field('name', $._label),
      ),
    )),

    // A local variable or a parameter, as in @name.
    variable: _ => /@[A-Za-z0-9_$#\u0080-\u{10FFFF}][A-Za-z0-9_@$#\u0080-\u{10FFFF}]*/u,

    // A system function with @@, as in @@ROWCOUNT.
    global_variable: _ => /@@[A-Za-z0-9_@$#\u0080-\u{10FFFF}]+/u,

    // A column name that starts with $, such as $IDENTITY, $ROWGUID, the
    // columns of a graph table, such as $node_id, and $PARTITION.
    pseudocolumn: _ => /\$[A-Za-z_][A-Za-z0-9_]*/,

    // The token wins over identifier where both can come.
    niladic_function: _ => token(prec(1, choice(...NILADIC_FUNCTIONS.map(keyword)))),

    _function_word: _ => token(prec(1, choice(...FUNCTION_WORDS.map(keyword)))),

    // A function call, with its schema and its database, or a static
    // method of a type, as in geography::Point(1, 2, 4326). The clauses
    // after the arguments come in any order.
    function_call: $ => prec.right(seq(
      choice(
        field('name', choice($._name, alias($._function_word, $.identifier))),
        seq(field('schema', $._name), '.', field('name', $._label)),
        seq(
          field('database', $._name),
          '.',
          optional(field('schema', $._label)),
          '.',
          field('name', $._label),
        ),
        seq(
          optional(seq(field('schema', $._name), '.')),
          field('type', $._name),
          '::',
          field('name', $._label),
        ),
        // The old form of a system table-valued function, as in
        // ::fn_helpcollations().
        seq('::', field('name', $._name)),
      ),
      $.arguments,
      repeat(choice(
        $.within_group_clause,
        $.null_treatment,
        $.over_clause,
      )),
    )),

    arguments: $ => parens(optional(seq(
      optional(choice(kw('DISTINCT'), kw('ALL'))),
      $._argument_list,
    ))),

    // The arguments of a function. Most functions take a list of
    // expressions with commas between them. Some functions have keywords
    // between their arguments, such as TRIM(BOTH 'x' FROM s), PARSE(s AS
    // date USING 'en-US') and JSON_OBJECT('a' : 1). The grammar accepts each
    // of these keywords between two arguments of any function, so that each
    // argument is in the same parse states. The clauses of the JSON
    // functions, such as RETURNING and NULL ON NULL, can come after each
    // argument.
    _argument_list: $ => seq(
      optional(seq(choice(kw('BOTH'), kw('LEADING'), kw('TRAILING')), optional(kw('FROM')))),
      $._argument_item,
      repeat(seq($._argument_separator, $._argument_item)),
    ),

    _argument_separator: _ => choice(
      ',',
      ':',
      kw('FROM'),
      kw('USING'),
    ),

    _argument_item: $ => seq($._argument, repeat($._function_clause)),

    _argument: $ => choice(
      $._expression,
      // CAST(x AS type), TRY_CAST and PARSE.
      seq($._expression, kw('AS'), $.type),
      // COUNT(*) and CHECKSUM(t.*).
      $.all_columns,
      // The columns of FREETEXT and CONTAINS, as in FREETEXT((a, b), 'x').
      $.list,
      // OPENROWSET(BULK 'file', SINGLE_CLOB).
      seq(kw('BULK'), $._expression),
      // LANGUAGE of CONTAINSTABLE and FREETEXTTABLE, as in LANGUAGE 1033.
      // The word is not a keyword, so that language can name a column.
      seq(alias($.identifier, $.keyword), choice($.number, $.string, $.variable)),
    ),

    // The clauses of the JSON functions and of the AI functions of SQL
    // Server 2025, after an argument.
    _function_clause: $ => choice(
      seq(kw('RETURNING'), $.type),
      seq(choice(kw('NULL'), kw('ABSENT')), kws('ON', 'NULL')),
      seq(kw('WITH'), optional(kw('ARRAY')), kw('WRAPPER')),
      $.order_by_clause,
      seq(kws('USE', 'MODEL'), field('model', $._name)),
    ),

    within_group_clause: $ => seq(
      kws('WITHIN', 'GROUP'),
      parens($.order_by_clause),
    ),

    null_treatment: _ => seq(choice(kw('IGNORE'), kw('RESPECT')), kw('NULLS')),

    over_clause: $ => seq(
      kw('OVER'),
      choice(field('window', $._name), $.window_specification),
    ),

    // CONVERT(type, value, style) and TRY_CONVERT take a type first.
    convert_expression: $ => seq(
      choice(kw('CONVERT'), kw('TRY_CONVERT')),
      '(',
      field('type', $.type),
      ',',
      field('value', $._expression),
      optional(seq(',', field('style', $._expression))),
      ')',
    ),

    // NEXT VALUE FOR gives the next value of a sequence. The three words are
    // one token, so that next can name a column.
    next_value_for: $ => prec.right(seq(
      alias($._next_value_for, 'NEXT VALUE FOR'),
      field('sequence', $.object_reference),
      optional($.over_clause),
    )),

    _next_value_for: _ => token(seq(keyword('NEXT'), /\s+/, keyword('VALUE'), /\s+/, keyword('FOR'))),

    // A property or a method of a value of a CLR type, such as a spatial
    // type, hierarchyid or xml, as in @p.X, (c).Lat, @x.value('/a', 'int')
    // or CAST(s AS xml).query('.'). A method of a column has the form of a
    // function call with a schema, as in c.STArea().
    field_expression: $ => prec.right(PREC.field, seq(
      field('value', choice(
        $.variable,
        $.pseudocolumn,
        $.function_call,
        $.convert_expression,
        $.parenthesized_expression,
        $.subquery,
        $.field_expression,
      )),
      '.',
      field('field', $._label),
      optional($.arguments),
    )),

    parenthesized_expression: $ => parens($._expression),

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

    // An escape of ODBC, as in {fn UCASE(s)}, {d '2026-11-01'} or {ts
    // '2026-11-01 10:00:00'}. The first word is not a keyword, so that it
    // can name a column elsewhere.
    odbc_escape: $ => seq(
      '{',
      alias($.identifier, $.keyword),
      choice($.function_call, $.string),
      '}',
    ),

    // A choice of operators flattens into one production for each
    // operator, and each production has parse states of its own. So each
    // level of precedence takes its operators from one hidden rule.
    unary_expression: $ => choice(
      prec.right(PREC.not, seq(field('operator', kw('NOT')), field('operand', $._expression))),
      prec.right(PREC.unary, seq(field('operator', alias($._unary_operator, $.operator)), field('operand', $._expression))),
    ),

    _unary_operator: _ => token(choice('-', '+', '~')),

    binary_expression: $ => {
      /** @type {[number, RuleOrLiteral, RuleOrLiteral][]} */
      const table = [
        [PREC.or, kw('OR'), $._expression],
        [PREC.and, kw('AND'), $._expression],
        [PREC.multiply, alias($._multiplicative_operator, $.operator), $._expression],
        [PREC.add, alias($._additive_operator, $.operator), $._expression],
        [PREC.compare, alias($._comparison_operator, $.operator), $._operand],
      ];
      return choice(...table.map(([p, op, right]) => prec.left(p, seq(
        field('left', $._expression),
        field('operator', op),
        field('right', right),
      ))));
    },

    _multiplicative_operator: _ => token(choice('*', '/', '%')),

    // + adds or concatenates, and &, ^ and | are the bitwise operators,
    // with the precedence of + and -.
    _additive_operator: _ => token(choice('+', '-', '&', '^', '|')),

    _comparison_operator: _ => token(choice(
      '=', '<', '>', '<=', '>=', '<>', '!=', '!<', '!>',
    )),

    _operand: $ => choice($._expression, $.quantified_expression),

    // ANY, SOME or ALL after a comparison operator compares the left side
    // with each row of a subquery.
    quantified_expression: $ => seq(
      field('quantifier', choice(kw('ANY'), kw('SOME'), kw('ALL'))),
      $.subquery,
    ),

    // Each condition takes its operator from one hidden rule, so that it
    // has one production, and NOT before the operator gives no other.
    is_expression: $ => prec.left(PREC.condition, seq(
      field('left', $._expression),
      $._is_operator,
      field('right', choice(
        $.null,
        seq(kws('DISTINCT', 'FROM'), $._expression),
      )),
    )),

    _is_operator: _ => seq(kw('IS'), optional(kw('NOT'))),

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
      field('right', choice($.subquery, $.parenthesized_expression, $.list)),
    )),

    _in_operator: _ => seq(optional(kw('NOT')), kw('IN')),

    // A list of two or more expressions, as in a IN (1, 2, 3).
    list: $ => parens(seq($._expression, ',', commaSep1($._expression))),

    like_expression: $ => prec.left(PREC.condition, seq(
      field('left', $._expression),
      field('operator', $._like_operator),
      field('right', $._expression),
      optional(seq(kw('ESCAPE'), field('escape', $._expression))),
    )),

    _like_operator: _ => seq(optional(kw('NOT')), kw('LIKE')),

    at_time_zone_expression: $ => prec.left(PREC.at, seq(
      field('value', $._expression),
      kws('AT', 'TIME', 'ZONE'),
      field('zone', $._expression),
    )),

    collate_expression: $ => prec.left(PREC.collate, seq(
      field('value', $._expression),
      kw('COLLATE'),
      field('collation', $._name),
    )),

    // Types

    // A type, with its schema, as in int, nvarchar(max), decimal(10, 2),
    // dbo.my_type, xml(CONTENT dbo.s) or vector(3, float16). The names of
    // the types are not reserved, so they are identifiers.
    type: $ => prec.right(choice(
      seq(
        choice(
          field('name', $._name),
          seq(field('schema', $._name), '.', field('name', $._label)),
        ),
        optional(kw('VARYING')),
        optional($.type_modifiers),
      ),
      kws('DOUBLE', 'PRECISION'),
      seq(
        kw('NATIONAL'),
        field('name', $._name),
        optional(kw('VARYING')),
        optional($.type_modifiers),
      ),
    )),

    // The length, the precision and the scale of a type, as in
    // varchar(max) or decimal(10, 2), the schema collection of xml, as in
    // xml(DOCUMENT dbo.s), and the dimensions and the base type of vector.
    type_modifiers: $ => parens(commaSep1(choice(
      $._signed_number,
      seq(optional(choice(kw('CONTENT'), kw('DOCUMENT'))), $.object_reference),
    ))),

    _signed_number: $ => seq(optional(alias($._unary_operator, $.operator)), $.number),

    // Names and literals

    // A name of an object, with its schema, its database and its server. A
    // part can be empty, as in db..t.
    object_reference: $ => choice(
      field('name', $._name),
      seq(field('schema', $._name), '.', field('name', $._label)),
      seq(
        field('database', $._name),
        '.',
        optional(field('schema', $._label)),
        '.',
        field('name', $._label),
      ),
      seq(
        field('server', $._name),
        '.',
        optional(field('database', $._label)),
        '.',
        optional(field('schema', $._label)),
        '.',
        field('name', $._label),
      ),
    ),

    // A name is an identifier or a delimited identifier, and both are the
    // node identifier.
    _name: $ => choice(
      $.identifier,
      alias($._delimited_identifier, $.identifier),
    ),

    // A name after a dot can be any keyword.
    _label: $ => choice(
      reserved('label', $.identifier),
      alias($._delimited_identifier, $.identifier),
    ),

    // An identifier starts with a letter, _ or #. A name of a temporary
    // table starts with # or ##. The grammar takes an identifier that
    // starts with _, which is also the placeholder of a variable of usql
    // (D101). A currency symbol starts a constant of money, not an
    // identifier.
    identifier: _ => /[A-Za-z_#\u0080-¡¦-₟⃐-\u{10FFFF}][A-Za-z0-9_@$#\u0080-\u{10FFFF}]*/u,

    // A delimited identifier is in brackets, with ]] for a ], or in double
    // quotes, with "" for a ".
    _delimited_identifier: _ => token(choice(
      /\[([^\]]|\]\])*\]/,
      /"([^"]|"")*"/,
    )),

    // A number is an integer, a decimal number, which can end with its
    // dot, or a float with an exponent.
    number: _ => token(choice(
      /\d+(\.\d*)?([eE][+-]?\d+)?/,
      /\.\d+([eE][+-]?\d+)?/,
    )),

    // A constant of money is a number after a currency symbol, as in
    // $12.50 or $-1.
    money: _ => token(seq(
      /[$¢-¥₠-⃏]/,
      optional(/[+-]/),
      choice(/\d+(\.\d*)?/, /\.\d+/),
    )),

    // A binary constant, as in 0x1F. 0x alone is an empty value.
    binary: _ => /0[xX][0-9A-Fa-f]*/,

    // A string is in single quotes, with a quote doubled. N before it gives
    // a Unicode string.
    string: _ => /[nN]?'([^']|'')*'/,
  },
});
