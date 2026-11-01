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
// If each word that starts a statement were a lookahead after each
// expression, each state after an expression would take the large form. So
// the external scanner gives _end, a token that holds no characters, where
// the grammar can end a statement and the next word starts a statement, or
// is ELSE or END. A simple statement in a batch or a block ends at a
// semicolon or at _end. A statement that holds statements, such as IF or
// a block, ends with the last statement in it. The scanner gives _end
// also after the condition of IF and WHILE. A word that starts a
// statement and that SQL Server does not reserve, such as THROW, starts a
// statement after another statement, where the server reads it as an
// alias.
//
// The procedural statements are written inline, as in the Oracle grammar:
// BEGIN ... END, IF, WHILE, TRY ... CATCH, DECLARE and the statements of
// cursors. The words that only these statements use, such as TRY and
// CATCH, are not reserved.
//
// The external scanner in src/scanner.c reads the comments, which nest, a
// line with only GO and a count, which ends a batch in sqlcmd and in SQL
// Server Management Studio, and _end. GO is not Transact-SQL, but a script
// holds it, so the grammar takes it as the end of a batch. The other
// commands of sqlcmd, such as :r and :setvar, belong to usql. The variables
// of usql belong to the usql grammar, which hands this grammar a
// placeholder of the same length in place of each variable (D101).

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
    $._end,
    $._error_sentinel,
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
    // The first statement of a batch can be the call of a procedure with no
    // EXECUTE before it, as in sp_who.
    source_file: $ => seq(
      optional($._batch_start),
      repeat($._item),
    ),

    _batch_start: $ => seq(alias($._procedure_call, $.execute_statement), $._terminator),

    // A statement in a batch or a block ends at a semicolon, or where the
    // external scanner gives _end, which holds no characters: before a word
    // that starts a statement, before ELSE, END, a label or a line with
    // GO, and at the end of the input. A statement that holds statements,
    // such as IF, ends with the last statement in it.
    _item: $ => choice(
      seq($._simple_statement, $._terminator),
      $._compound_statement,
      $.label,
      seq($.go_statement, optional($._batch_start)),
      ';',
    ),

    _terminator: $ => choice(';', $._end),

    // The statement after IF, ELSE or WHILE. A block can end with a
    // semicolon before ELSE, as in IF a = 1 BEGIN PRINT 'a' END; ELSE PRINT
    // 'b'.
    _body: $ => choice(
      seq($._simple_statement, $._terminator),
      prec.right(seq(choice($.block, $.try_statement), optional(';'))),
      $.if_statement,
      $.while_statement,
    ),

    _compound_statement: $ => choice(
      $.block,
      $.if_statement,
      $.while_statement,
      $.try_statement,
    ),

    // GO and a count, alone on its line, ends a batch, and runs it count
    // times. The grammar takes GO in each place where a statement can
    // start, also in a block, where the server does not take it. Then the
    // states after a statement in a block and after a statement of the
    // batch are the same states.
    go_statement: $ => prec.right(seq(
      alias($._go, 'GO'),
      optional(field('count', $.number)),
    )),

    _simple_statement: $ => choice(
      $.select_statement,
      $.insert_statement,
      $.update_statement,
      $.delete_statement,
      $.merge_statement,
      $.bulk_insert_statement,
      $.truncate_statement,
      $.readtext_statement,
      $.writetext_statement,
      $.updatetext_statement,
      $.begin_transaction_statement,
      $.commit_statement,
      $.rollback_statement,
      $.save_transaction_statement,
      $.set_statement,
      $.use_statement,
      $.execute_statement,
      $.execute_as_statement,
      $.revert_statement,
      $.print_statement,
      $.raiserror_statement,
      $.throw_statement,
      $.break_statement,
      $.continue_statement,
      $.goto_statement,
      $.return_statement,
      $.waitfor_statement,
      $.declare_statement,
      $.cursor_declaration,
      $.open_statement,
      $.fetch_statement,
      $.close_statement,
      $.deallocate_statement,
    ),

    // Changes of data

    // The clauses after the first part of a statement that changes data
    // come in any order, as the clauses of a query do. INSERT takes rows
    // from VALUES, a query, EXECUTE, or DEFAULT VALUES.
    insert_statement: $ => seq(
      optional($.with_clause),
      kw('INSERT'),
      optional($.top_clause),
      optional(kw('INTO')),
      $._dml_table,
      optional($.column_list),
      repeat($.output_clause),
      choice(
        $.values_clause,
        $._query,
        $.execute_statement,
        kws('DEFAULT', 'VALUES'),
      ),
    ),

    values_clause: $ => seq(kw('VALUES'), commaSep1($.value_list)),

    // An UPDATE sets columns, variables, the properties of a value of a CLR
    // type, as in SET c.X = 1, or calls a method, as in SET
    // c.WRITE(N'x', 0, NULL).
    update_statement: $ => seq(
      optional($.with_clause),
      kw('UPDATE'),
      optional($.top_clause),
      $._dml_table,
      $._set_clause,
      repeat($._dml_clause),
    ),

    _set_clause: $ => seq(
      kw('SET'),
      commaSep1(choice(
        alias($._update_assignment, $.assignment),
        $.function_call,
        $.field_expression,
      )),
    ),

    _update_assignment: $ => seq(
      field('left', choice($.column_reference, $.variable, $.field_expression)),
      field('operator', choice('=', alias($._compound_operator, $.operator))),
      field('right', choice($._expression, kw('DEFAULT'))),
    ),

    delete_statement: $ => seq(
      optional($.with_clause),
      kw('DELETE'),
      optional($.top_clause),
      optional(kw('FROM')),
      $._dml_table,
      repeat($._dml_clause),
    ),

    _dml_clause: $ => choice(
      $.output_clause,
      $.from_clause,
      $.where_clause,
      $.current_of_clause,
      $.option_clause,
    ),

    // The table of INSERT, UPDATE, DELETE and MERGE: a table, a table
    // variable or a rowset function, with its hints after WITH.
    _dml_table: $ => prec.right(seq(
      field('table', choice(
        $.object_reference,
        $.variable,
        alias($._rowset_function, $.function_call),
      )),
      optional(alias($._with_table_hints, $.table_hints)),
    )),

    _with_table_hints: $ => seq(kw('WITH'), $._hint_list),

    // WHERE CURRENT OF changes the row of a cursor.
    current_of_clause: $ => seq(
      kws('WHERE', 'CURRENT', 'OF'),
      optional(kw('GLOBAL')),
      field('cursor', choice($._name, $.variable)),
    ),

    // OUTPUT gives the rows that the statement changed, with INSERTED,
    // DELETED and $action, and INTO puts them in a table.
    output_clause: $ => prec.right(seq(
      kw('OUTPUT'),
      commaSep1($._select_item),
      optional(seq(
        kw('INTO'),
        field('table', choice($.object_reference, $.variable)),
        optional($.column_list),
      )),
    )),

    // MERGE requires a semicolon at its end in the server, but the grammar
    // does not.
    merge_statement: $ => seq(
      optional($.with_clause),
      kw('MERGE'),
      optional($.top_clause),
      optional(kw('INTO')),
      $._dml_table,
      optional($._alias),
      kw('USING'),
      field('source', $._table_reference),
      kw('ON'),
      field('condition', $._expression),
      repeat(choice(
        $.merge_when_clause,
        $.output_clause,
        $.option_clause,
      )),
    ),

    merge_when_clause: $ => seq(
      kw('WHEN'),
      optional(kw('NOT')),
      kw('MATCHED'),
      optional(seq(kw('BY'), choice(kw('TARGET'), kw('SOURCE')))),
      optional(seq(kw('AND'), field('condition', $._expression))),
      kw('THEN'),
      choice(
        seq(kw('UPDATE'), $._set_clause),
        kw('DELETE'),
        seq(
          kw('INSERT'),
          optional($.column_list),
          choice($.values_clause, kws('DEFAULT', 'VALUES')),
        ),
      ),
    ),

    // BULK INSERT reads a file into a table. Its options are a list of
    // words, as the hints of a table are.
    bulk_insert_statement: $ => prec.right(seq(
      kws('BULK', 'INSERT'),
      field('table', $.object_reference),
      kw('FROM'),
      field('file', $.string),
      optional(seq(kw('WITH'), $._hint_list)),
    )),

    // TRUNCATE TABLE, with the partitions in WITH (PARTITIONS (1, 3 TO 5)).
    truncate_statement: $ => prec.right(seq(
      kws('TRUNCATE', 'TABLE'),
      field('table', $.object_reference),
      optional(seq(kw('WITH'), $._option_group)),
    )),

    // READTEXT, WRITETEXT and UPDATETEXT read and change a value of text,
    // ntext or image with a text pointer.
    readtext_statement: $ => seq(
      kw('READTEXT'),
      field('column', $.column_reference),
      field('pointer', $._text_value),
      field('offset', $._text_value),
      field('size', $._text_value),
      optional(kw('HOLDLOCK')),
    ),

    writetext_statement: $ => seq(
      kw('WRITETEXT'),
      optional(kw('BULK')),
      field('column', $.column_reference),
      field('pointer', $._text_value),
      optional(kws('WITH', 'LOG')),
      field('data', $._text_value),
    ),

    updatetext_statement: $ => prec.right(seq(
      kw('UPDATETEXT'),
      optional(kw('BULK')),
      field('column', $.column_reference),
      field('pointer', $._text_value),
      field('offset', $._text_value),
      field('length', $._text_value),
      optional(kws('WITH', 'LOG')),
      optional(choice(
        field('data', $._text_value),
        seq(field('source', $.column_reference), field('source_pointer', $._text_value)),
      )),
    )),

    _text_value: $ => choice($.variable, $._literal, $._negative_number),

    // A number with a sign, where a constant can come.
    _negative_number: $ => seq(alias($._unary_operator, $.operator), choice($.number, $.money)),

    // Transactions

    // BEGIN TRANSACTION, with its name and WITH MARK.
    begin_transaction_statement: $ => prec.right(seq(
      kw('BEGIN'),
      optional(kw('DISTRIBUTED')),
      $._transaction,
      optional(seq(
        field('name', choice($._name, $.variable)),
        optional(seq(kws('WITH', 'MARK'), optional(field('description', $.string)))),
      )),
    )),

    _transaction: _ => choice(kw('TRAN'), kw('TRANSACTION')),

    // COMMIT, with the delayed durability of the transaction.
    commit_statement: $ => prec.right(seq(
      kw('COMMIT'),
      optional(choice(
        kw('WORK'),
        seq($._transaction, optional(field('name', choice($._name, $.variable)))),
      )),
      optional(seq(kw('WITH'), $._option_group)),
    )),

    rollback_statement: $ => prec.right(seq(
      kw('ROLLBACK'),
      optional(choice(
        kw('WORK'),
        seq($._transaction, optional(field('name', choice($._name, $.variable)))),
      )),
    )),

    save_transaction_statement: $ => seq(
      kw('SAVE'),
      $._transaction,
      field('name', choice($._name, $.variable)),
    ),

    // SET sets a variable, the isolation level of the transactions, or an
    // option of the session. The options that take ON or OFF, a number or a
    // string are a list of words, as in SET ANSI_NULLS, QUOTED_IDENTIFIER
    // ON or SET STATISTICS IO ON. The options that take a word have rules
    // of their own, because the list of words cannot end at a word.
    set_statement: $ => seq(
      kw('SET'),
      choice(
        seq(
          field('left', choice($.variable, $._variable_member)),
          field('operator', choice('=', alias($._compound_operator, $.operator))),
          field('right', choice($._expression, $.cursor_definition)),
        ),
        $._variable_member,
        seq(
          kws('TRANSACTION', 'ISOLATION', 'LEVEL'),
          choice(
            seq(kw('READ'), choice(kw('UNCOMMITTED'), kw('COMMITTED'))),
            kws('REPEATABLE', 'READ'),
            kw('SNAPSHOT'),
            kw('SERIALIZABLE'),
          ),
        ),
        seq(
          choice(kw('DATEFORMAT'), kw('LANGUAGE'), kw('DEADLOCK_PRIORITY')),
          field('value', choice($._name, $.variable, $._signed_number, $.string)),
        ),
        seq(
          $._option_word,
          repeat(choice($._option_word, ',', '.')),
          field('value', choice(
            kw('ON'),
            kw('OFF'),
            $._signed_number,
            $.string,
            $.binary,
            $.variable,
          )),
        ),
      ),
    ),

    // A property or a method of a variable of a CLR type, as in SET @p.X
    // = 1 or SET @p.SetXY(1, 2).
    _variable_member: $ => alias(prec.right(seq(
      field('value', $.variable),
      '.',
      field('field', $._label),
      optional($.arguments),
    )), $.field_expression),

    use_statement: $ => seq(kw('USE'), field('database', $._name)),

    // EXECUTE

    // EXECUTE runs a procedure, a string of Transact-SQL, or a command on a
    // linked server.
    execute_statement: $ => prec.right(seq(
      choice(kw('EXEC'), kw('EXECUTE')),
      choice(
        $._procedure_call,
        seq(
          optional(seq(field('status', $.variable), '=')),
          field('procedure', $.variable),
          optional($._execute_arguments),
          optional($._execute_options),
        ),
        seq(
          parens(commaSep1($._execute_string_item)),
          repeat(choice(
            seq(kw('AS'), choice(kw('LOGIN'), $._user), '=', field('principal', $.string)),
            seq(kw('AT'), optional(kw('DATA_SOURCE')), field('server', $._name)),
          )),
        ),
      ),
    )),

    // The call of a procedure, with the variable that takes its status.
    _procedure_call: $ => prec.right(seq(
      optional(seq(field('status', $.variable), '=')),
      field('procedure', $.object_reference),
      optional($._execute_arguments),
      optional($._execute_options),
    )),

    _execute_arguments: $ => commaSep1($.execute_argument),

    // An argument is a constant, a variable, a name, which is a string with
    // no quotes, or DEFAULT. A variable can take a value back, with OUTPUT.
    execute_argument: $ => prec.right(seq(
      optional(seq(field('name', $.variable), '=')),
      field('value', choice(
        $._literal,
        $._negative_number,
        $.variable,
        $.global_variable,
        $._name,
        kw('DEFAULT'),
      )),
      optional(choice(kw('OUTPUT'), kw('OUT'))),
    )),

    _execute_string_item: $ => seq(
      $._expression,
      optional(choice(kw('OUTPUT'), kw('OUT'))),
    ),

    _execute_options: $ => seq(kw('WITH'), commaSep1($.execute_option)),

    // RECOMPILE, or the result sets of the procedure.
    execute_option: $ => choice(
      kw('RECOMPILE'),
      seq(
        kws('RESULT', 'SETS'),
        choice(
          kw('UNDEFINED'),
          kw('NONE'),
          parens(commaSep1($.result_set)),
        ),
      ),
    ),

    result_set: $ => choice(
      parens(commaSep1($.column_definition)),
      seq(kw('AS'), choice(kw('OBJECT'), kw('TYPE')), field('name', $.object_reference)),
      kws('AS', 'FOR', 'XML'),
    ),

    // USER is a niladic function, so its token is the token of the niladic
    // functions.
    _user: $ => alias($.niladic_function, 'USER'),

    // EXECUTE AS changes the user of the session, and REVERT changes it
    // back.
    execute_as_statement: $ => prec.right(seq(
      choice(kw('EXEC'), kw('EXECUTE')),
      kw('AS'),
      choice(
        seq(choice(kw('LOGIN'), $._user), '=', field('principal', $.string)),
        kw('CALLER'),
      ),
      optional(seq(
        kw('WITH'),
        choice(
          kws('NO', 'REVERT'),
          seq(kws('COOKIE', 'INTO'), field('cookie', $.variable)),
        ),
      )),
    )),

    revert_statement: $ => prec.right(seq(
      kw('REVERT'),
      optional(seq(kws('WITH', 'COOKIE'), '=', field('cookie', $.variable))),
    )),

    // Messages and errors

    print_statement: $ => seq(kw('PRINT'), $._expression),

    // RAISERROR takes a message, a severity, a state and the arguments of
    // the message.
    raiserror_statement: $ => prec.right(seq(
      kw('RAISERROR'),
      $.arguments,
      optional(seq(kw('WITH'), commaSep1(choice(kw('LOG'), kw('NOWAIT'), kw('SETERROR'))))),
    )),

    // THROW with no arguments raises again the error of a CATCH block.
    throw_statement: $ => prec.right(seq(
      kw('THROW'),
      optional(seq(
        field('number', $._expression),
        ',',
        field('message', $._expression),
        ',',
        field('state', $._expression),
      )),
    )),

    // The control-of-flow language

    // A block holds statements between BEGIN and END.
    block: $ => seq(
      kw('BEGIN'),
      repeat($._item),
      kw('END'),
    ),

    // The scanner gives _end after the condition of IF and WHILE, before
    // the word that starts the statement after it.
    if_statement: $ => prec.right(seq(
      kw('IF'),
      field('condition', $._expression),
      $._end,
      field('consequence', $._body),
      optional(seq(kw('ELSE'), field('alternative', $._body))),
    )),

    while_statement: $ => seq(
      kw('WHILE'),
      field('condition', $._expression),
      $._end,
      field('body', $._body),
    ),

    break_statement: _ => kw('BREAK'),

    continue_statement: _ => kw('CONTINUE'),

    goto_statement: $ => seq(kw('GOTO'), field('label', $._name)),

    // A label is a name and a colon, as in retry:, which GOTO names. It is
    // one token, so that a name at the end of a statement, such as the
    // alias of a table, does not take it.
    label: _ => token(seq(
      /[A-Za-z_#\u0080-\u{10FFFF}][A-Za-z0-9_@$#\u0080-\u{10FFFF}]*/u,
      ':',
    )),

    return_statement: $ => prec.right(seq(kw('RETURN'), optional(field('value', $._expression)))),

    // WAITFOR waits for a time of day, or for a delay.
    waitfor_statement: $ => seq(
      kw('WAITFOR'),
      choice(kw('DELAY'), kw('TIME')),
      field('time', $._expression),
    ),

    // BEGIN TRY ... END TRY BEGIN CATCH ... END CATCH.
    try_statement: $ => seq(
      kws('BEGIN', 'TRY'),
      repeat($._item),
      kws('END', 'TRY'),
      $.catch_block,
    ),

    catch_block: $ => seq(
      kws('BEGIN', 'CATCH'),
      repeat($._item),
      kws('END', 'CATCH'),
    ),

    // Variables and cursors

    // DECLARE declares variables, with a value, cursor variables and table
    // variables.
    declare_statement: $ => seq(
      kw('DECLARE'),
      commaSep1($.variable_declaration),
    ),

    // A variable of a natively compiled module can be NOT NULL.
    variable_declaration: $ => seq(
      field('name', $.variable),
      optional(kw('AS')),
      choice(
        seq(
          field('type', $.type),
          optional(seq(kw('NOT'), $.null)),
          optional(seq('=', field('value', $._expression))),
        ),
        kw('CURSOR'),
        seq(kw('TABLE'), $.table_definition),
      ),
    ),

    // The columns, the constraints and the indexes of a table variable.
    table_definition: $ => parens(commaSep1(choice(
      $.column_definition,
      $.table_constraint,
      $.table_index,
    ))),

    // A constraint of a table, as in PRIMARY KEY (a, b DESC).
    table_constraint: $ => seq(
      optional(seq(kw('CONSTRAINT'), field('name', $._name))),
      choice(
        seq(
          choice(kws('PRIMARY', 'KEY'), kw('UNIQUE')),
          optional(choice(kw('CLUSTERED'), kw('NONCLUSTERED'))),
          $.index_columns,
          optional(seq(kw('WITH'), $._hint_list)),
        ),
        seq(kw('CHECK'), $.parenthesized_expression),
      ),
    ),

    table_index: $ => seq(
      kw('INDEX'),
      field('name', $._name),
      optional(kw('UNIQUE')),
      optional(choice(kw('CLUSTERED'), kw('NONCLUSTERED'))),
      $.index_columns,
    ),

    index_columns: $ => parens(commaSep1($.order_by_item)),

    // DECLARE CURSOR, in the form of ISO, with INSENSITIVE and SCROLL
    // before CURSOR, or in the form of Transact-SQL, with the options
    // after CURSOR.
    cursor_declaration: $ => seq(
      kw('DECLARE'),
      field('name', $._name),
      repeat(alias($.identifier, $.keyword)),
      $.cursor_definition,
    ),

    // The options of a cursor are words, such as LOCAL and FAST_FORWARD.
    // FOR UPDATE and FOR READ ONLY at the end are clauses of the query.
    cursor_definition: $ => seq(
      kw('CURSOR'),
      repeat(alias($.identifier, $.keyword)),
      kw('FOR'),
      $._query,
    ),

    open_statement: $ => seq(kw('OPEN'), $._cursor),

    // FETCH takes a row of a cursor into variables.
    fetch_statement: $ => seq(
      kw('FETCH'),
      optional(seq(
        optional(choice(
          kw('NEXT'),
          kw('PRIOR'),
          kw('FIRST'),
          kw('LAST'),
          seq(
            choice(kw('ABSOLUTE'), kw('RELATIVE')),
            field('offset', choice($._signed_number, $.variable)),
          ),
        )),
        kw('FROM'),
      )),
      $._cursor,
      optional(seq(kw('INTO'), commaSep1($.variable))),
    ),

    close_statement: $ => seq(kw('CLOSE'), $._cursor),

    deallocate_statement: $ => seq(kw('DEALLOCATE'), $._cursor),

    _cursor: $ => seq(
      optional(kw('GLOBAL')),
      field('cursor', choice($._name, $.variable)),
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

    // The columns of a common table expression, an alias or an INSERT. The
    // columns of an edge of a graph, $from_id and $to_id, are
    // pseudocolumns.
    column_list: $ => parens(commaSep1(choice($._name, $.pseudocolumn))),

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
      $._hint_list,
    ),

    _hint_list: $ => parens(seq($.hint, repeat(seq(optional(','), $.hint)))),

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

    // A column of a table variable, of the result set of EXECUTE, or of a
    // rowset function. A column of OPENJSON, OPENXML and OPENROWSET can have
    // a path, as in id int '$.id', and AS JSON. A computed column has an
    // expression in place of its type.
    column_definition: $ => prec.right(seq(
      field('name', $._name),
      choice(
        field('type', $.type),
        seq(kw('AS'), field('value', $._expression)),
      ),
      repeat(choice(
        seq(kw('COLLATE'), field('collation', $._name)),
        field('path', choice($.string, $.number)),
        kws('AS', 'JSON'),
        $._column_constraint,
      )),
    )),

    // The constraints and the properties of a column. DEFAULT takes a
    // constant, a function or an expression in parentheses, so that NOT
    // after it starts NOT NULL.
    _column_constraint: $ => choice(
      $.null,
      seq(kw('NOT'), $.null),
      kw('PERSISTED'),
      kw('ROWGUIDCOL'),
      seq(
        alias($._function_word, $.keyword),
        optional(parens(seq(field('seed', $._signed_number), ',', field('increment', $._signed_number)))),
      ),
      seq(
        optional(seq(kw('CONSTRAINT'), field('constraint', $._name))),
        choice(
          seq(
            choice(kws('PRIMARY', 'KEY'), kw('UNIQUE')),
            optional(choice(kw('CLUSTERED'), kw('NONCLUSTERED'))),
          ),
          seq(kw('CHECK'), $.parenthesized_expression),
          seq(kw('DEFAULT'), field('default', choice(
            $._literal,
            $._negative_number,
            $.niladic_function,
            $.function_call,
            $.parenthesized_expression,
          ))),
        ),
      ),
      seq(
        kw('INDEX'),
        field('index', $._name),
        optional(choice(kw('CLUSTERED'), kw('NONCLUSTERED'))),
      ),
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
        kws('READ', 'ONLY'),
        seq(kw('UPDATE'), optional(seq(kw('OF'), commaSep1($.column_reference)))),
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
    // empty, as in db..t. A pseudocolumn can follow a table, as in
    // p.$node_id.
    column_reference: $ => prec.right(choice(
      field('name', $._name),
      seq(field('table', $._name), '.', field('name', choice($._label, $.pseudocolumn))),
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
      // The default value of an argument of a table-valued function.
      kw('DEFAULT'),
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
