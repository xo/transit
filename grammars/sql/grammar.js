/**
 * @file A generic grammar of SQL
 * @license MIT
 */

// xo writes this grammar for the SQL dialects that have no grammar of their
// own (D143). It reads the SQL of the standard that most of those dialects
// share, and the forms that several of them add, such as QUALIFY, PIVOT,
// LATERAL, the ARRAY and STRUCT types and values, the names in backticks,
// CREATE ... CLONE, MERGE and COPY INTO. The references are the SQL
// documentation of Snowflake, BigQuery and Databricks. The grammar does not
// try to read each dialect in full, and it copies no grammar of another
// project.
//
// The grammar is for a highlighter, so it is lenient (D143). It accepts
// more than each dialect does where that keeps the rules small, and a form
// that only one dialect has can parse as a generic form. A keyword is a
// token in any case, and it is an anonymous node with the name of the
// keyword in upper case, such as "SELECT".
//
// The token identifier is the word of the grammar, so a keyword that the
// grammar does not reserve is an identifier in each place where the grammar
// does not expect the keyword. Where a keyword and a name can both come,
// the keyword wins, so LIMIT after a table starts the clause LIMIT. The
// words that the three dialects reserve are in the reserved set global, so
// they are not identifiers, but for a label after AS or after a dot, where
// the grammar accepts every keyword.
//
// The grammar keeps its parse tables small (D122, D143). A parse state with
// more than 64 columns takes the large form, so the grammar keeps the
// columns of a state few. The operators of one level of precedence are one
// token, TRUE and FALSE are one token, the words such as CURRENT_DATE are
// one token, LIKE, ILIKE, RLIKE and REGEXP are one token, and NULL is a
// token of its own. The clauses of a query after SELECT, the clauses after
// the body of a query, the clauses after the arguments of a function and
// the clauses of a window come in any order, because a sequence of
// optional clauses costs parse states for each set of the clauses.
//
// The dialects have hundreds of options for their objects, such as the
// options of a table, of a stage or of COPY INTO. The grammar reads them as
// a list of options: words, strings, numbers, = and lists in parentheses.
// A word of such a list is an identifier that takes the reserved set label,
// so one token covers each word, and a reserved word such as BY is a word
// of the list too. The statements that the grammar does not read word by
// word, such as SHOW, GRANT and the CREATE of an object that is not a
// table, a view, an index or a function, read such a list too.
//
// The grammar needs no external scanner. A comment is -- , // or # to the
// end of the line, or /* ... */, which does not nest. A string is in single
// quotes, with a doubled quote or a backslash before a quote, and it can be
// in three quotes, have a prefix such as r or b, or be in $$. A statement
// ends at a semicolon or at the end of the input. The variables of usql
// belong to the usql grammar, which hands this grammar a placeholder of the
// same length in place of each variable (D101).

/// <reference types="tree-sitter-cli/dsl" />
// @ts-check

// The precedences of the operators, from the lowest to the highest. The
// arrow of a lambda binds the most loosely. NOT binds more loosely than a
// comparison, and the comparisons, IS, BETWEEN, IN and LIKE bind at one
// level. The bit operators bind more loosely than + and -, || binds as + and
// -, a sign binds more tightly than * and /, and COLLATE and AT TIME ZONE
// more tightly than a sign. The cast with ::, a subscript, the access with
// : and a dot bind the most tightly.
const PREC = {
  lambda: 1,
  or: 2,
  and: 3,
  not: 4,
  compare: 5,
  bitor: 6,
  bitand: 7,
  shift: 8,
  add: 9,
  multiply: 10,
  unary: 11,
  collate: 12,
  cast: 13,
  postfix: 14,
  field: 15,
};

// The words that Snowflake, BigQuery and Databricks all reserve, or that
// two of them reserve and that start a clause. A word is in the reserved
// set only when the grammar uses it as a keyword, because a reserved word
// must be a token of the grammar.
const RESERVED_WORDS = [
  'ALL', 'AND', 'ANY', 'AS', 'BETWEEN', 'BY', 'CASE', 'CREATE',
  'CROSS', 'DISTINCT', 'ELSE', 'END', 'EXCEPT', 'EXISTS', 'FALSE', 'FETCH',
  'FOR', 'FROM', 'FULL', 'GROUP', 'HAVING', 'IN', 'INNER', 'INTERSECT',
  'INTO', 'IS', 'JOIN', 'LATERAL', 'LEFT', 'LIKE', 'LIMIT', 'MINUS',
  'NATURAL', 'NOT', 'NULL', 'ON', 'OR', 'ORDER', 'OUTER', 'QUALIFY', 'RIGHT',
  'SELECT', 'SET', 'SOME', 'TABLE', 'THEN', 'TO', 'TRUE', 'UNION', 'USING',
  'WHEN', 'WHERE', 'WINDOW', 'WITH',
];

// The reserved words that name a function, which the set function_name
// leaves out, so that a function call can have such a name.
const FUNCTION_WORDS = ['LEFT', 'RIGHT', 'WINDOW'];

// The words of one token each, such as CURRENT_DATE, which take one column
// of each state where an expression starts.
const CURRENT_WORDS = [
  'CURRENT_DATE', 'CURRENT_TIME', 'CURRENT_TIMESTAMP', 'LOCALTIME',
  'LOCALTIMESTAMP', 'CURRENT_USER', 'CURRENT_ROLE', 'CURRENT_SCHEMA',
  'CURRENT_DATABASE', 'CURRENT_CATALOG', 'SESSION_USER',
];

// The units of an interval, in the singular and in the plural.
const INTERVAL_UNITS = [
  'YEAR', 'QUARTER', 'MONTH', 'WEEK', 'DAY', 'HOUR', 'MINUTE', 'SECOND',
  'MILLISECOND', 'MICROSECOND', 'NANOSECOND',
];

// The words that can come between CREATE and the kind of an object, such as
// TEMPORARY and TRANSIENT. They are one token, the node keyword.
const CREATE_MODIFIERS = [
  'TEMPORARY', 'TEMP', 'TRANSIENT', 'VOLATILE', 'SECURE', 'EXTERNAL',
  'GLOBAL', 'LOCAL', 'AGGREGATE', 'HYBRID', 'DYNAMIC', 'ICEBERG', 'STREAMING',
  'LIVE',
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
 * Returns the body of a list of options: an item that is not a list in
 * parentheses, and more items.
 *
 * @param {GrammarSymbols<string>} $
 */
function optionList($) {
  return prec.right(seq(
    choice(
      $._option_name,
      $.string,
      $.number,
      $.boolean,
      $.null,
      $.stage_reference,
    ),
    repeat($._option_item),
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
  name: 'sql',

  extras: $ => [
    /\s/,
    $.comment,
  ],

  word: $ => $.identifier,

  // The reserved set global holds the reserved words. The set
  // function_name leaves out the words that name a function, and the set
  // label is empty, because the grammar accepts every keyword as a label
  // after AS, after a dot and in a list of options. A state takes the last
  // set of its items, so the sets go from the strictest to the least
  // strict.
  reserved: {
    global: _ => RESERVED_WORDS.filter(w => KEYWORDS.has(w)).map(keyword),
    function_name: _ => RESERVED_WORDS.filter(w => KEYWORDS.has(w) && !FUNCTION_WORDS.includes(w)).map(keyword),
    label: _ => [],
  },

  // Each hidden rule that an expression can start with is a column of each
  // state where an expression starts, so these rules are inlined. The rules
  // _name, _label and _table_primary are inlined for the same reason, for
  // the states where a name or a table starts.
  inline: $ => [
    $._option_item,
    $._property_item,
    $._option_name,
    $._name,
    $._label,
    $._literal,
    $._function_name,
    $._qualifier,
    $._primary_expression,
    $._operand,
    $._table_primary,
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
      $.insert_statement,
      $.update_statement,
      $.delete_statement,
      $.merge_statement,
      $.truncate_statement,
      $.copy_statement,
      $.create_table_statement,
      $.create_view_statement,
      $.create_index_statement,
      $.create_function_statement,
      $.create_statement,
      $.alter_statement,
      $.drop_statement,
      $.comment_statement,
      $.grant_statement,
      $.revoke_statement,
      $.use_statement,
      $.set_statement,
      $.unset_statement,
      $.show_statement,
      $.describe_statement,
      $.explain_statement,
      $.call_statement,
      $.execute_statement,
      $.begin_statement,
      $.commit_statement,
      $.rollback_statement,
    ),

    // Changes of data

    // INSERT adds the rows of a query to a table. OVERWRITE replaces the
    // rows of the table. INTO and TABLE are optional, as in BigQuery and
    // Databricks. PARTITION names the partition of the table, BY NAME
    // matches the columns by their names, and REPLACE WHERE replaces the
    // rows that match a condition.
    insert_statement: $ => seq(
      kw('INSERT'),
      optional(kw('OVERWRITE')),
      choice(
        seq(
          optional(kw('INTO')),
          optional(kw('TABLE')),
          field('table', $.object_reference),
          optional($.partition_spec),
          optional($.column_list),
          repeat(choice(
            kws('BY', 'NAME'),
            seq(kws('REPLACE', 'WHERE'), field('condition', $._expression)),
            seq(kws('REPLACE', 'USING'), $.column_list),
          )),
          choice($.select_statement, kws('DEFAULT', 'VALUES')),
        ),
        $._multi_table_insert,
      ),
    ),

    // The multi-table INSERT of Snowflake puts each row of the query into
    // each table after INTO, or into the tables of the first WHEN or of
    // each WHEN whose condition holds.
    _multi_table_insert: $ => seq(
      choice(kw('ALL'), kw('FIRST')),
      choice(repeat1($.insert_into_clause), repeat1($.insert_when_clause)),
      optional(seq(kw('ELSE'), repeat1($.insert_into_clause))),
      $.select_statement,
    ),

    insert_when_clause: $ => prec.right(seq(
      kw('WHEN'),
      field('condition', $._expression),
      kw('THEN'),
      repeat1($.insert_into_clause),
    )),

    // The values of INSERT INTO are a row value in parentheses, which is
    // an expression.
    insert_into_clause: $ => prec.right(seq(
      kw('INTO'),
      field('table', $.object_reference),
      optional($.column_list),
      optional(seq(kw('VALUES'), parens(commaSep1(field('value', $._expression))))),
    )),

    // The partition of a table, with the values of its columns, as in
    // PARTITION (dt = '2024-01-01').
    partition_spec: $ => seq(
      kw('PARTITION'),
      parens(commaSep1(seq(
        field('column', $._name),
        optional(seq('=', field('value', $._expression))),
      ))),
    ),

    // UPDATE changes rows of a table. FROM joins other tables.
    update_statement: $ => seq(
      kw('UPDATE'),
      $._target_table,
      $._set_clause,
      repeat(choice($.from_clause, $.where_clause)),
    ),

    // The table that a statement changes, with its alias.
    _target_table: $ => seq(
      field('table', $.object_reference),
      optional($._alias),
    ),

    _set_clause: $ => seq(kw('SET'), commaSep1($.assignment)),

    // A column and its new value, or several columns and a row value.
    assignment: $ => seq(
      field('column', choice($.column_reference, $.column_list)),
      '=',
      field('value', choice($._expression, kw('DEFAULT'))),
    ),

    // DELETE deletes rows of a table. FROM is optional, as in BigQuery, and
    // USING joins other tables.
    delete_statement: $ => seq(
      kw('DELETE'),
      optional(kw('FROM')),
      $._target_table,
      repeat(choice(
        seq(kw('USING'), commaSep1($._table_reference)),
        $.where_clause,
      )),
    ),

    // MERGE joins a table with a source, and changes, inserts or deletes
    // rows of the table for each row of the source, as each WHEN says.
    merge_statement: $ => seq(
      kw('MERGE'),
      optional(kw('INTO')),
      $._target_table,
      kw('USING'),
      field('source', $._table_factor),
      kw('ON'),
      field('condition', $._expression),
      repeat1($.merge_when_clause),
    ),

    // WHEN MATCHED is for a row of the source that matches a row of the
    // table, and WHEN NOT MATCHED for a row with no match. BY SOURCE and BY
    // TARGET say which side has no match. * stands for each column, as in
    // UPDATE SET * and INSERT *, and INSERT ROW inserts the row of the
    // source, as in BigQuery.
    merge_when_clause: $ => seq(
      kw('WHEN'),
      optional(kw('NOT')),
      kw('MATCHED'),
      optional(seq(kw('BY'), choice(kw('SOURCE'), kw('TARGET')))),
      optional(seq(kw('AND'), field('condition', $._expression))),
      kw('THEN'),
      choice(
        seq(kw('UPDATE'), kw('SET'), choice(commaSep1($.assignment), '*')),
        kw('DELETE'),
        seq(
          kw('INSERT'),
          choice(
            '*',
            kw('ROW'),
            seq(
              optional($.column_list),
              kw('VALUES'),
              field('values', $._expression),
            ),
          ),
        ),
      ),
    ),

    // TRUNCATE deletes each row of a table.
    truncate_statement: $ => seq(
      kw('TRUNCATE'),
      optional(kw('TABLE')),
      optional($._if_exists),
      field('table', $.object_reference),
      optional($.partition_spec),
    ),

    // COPY INTO loads the files of a stage or of a path into a table, or
    // unloads a table or a query into files. The options after the source
    // are a list of options.
    copy_statement: $ => seq(
      kws('COPY', 'INTO'),
      field('target', choice($.object_reference, $.stage_reference, $.string)),
      optional($.column_list),
      optional(seq(
        kw('FROM'),
        field('source', choice($.object_reference, $.stage_reference, $.string, $.subquery)),
      )),
      optional($.options),
    ),

    // Changes of the schema

    // The words before the kind of the object that CREATE makes: OR
    // REPLACE, OR ALTER, and the words such as TEMPORARY.
    _create: $ => seq(
      kw('CREATE'),
      optional(seq(kw('OR'), choice(kw('REPLACE'), kw('ALTER'), kw('REFRESH')))),
      repeat(alias($._create_modifier, $.keyword)),
    ),

    _create_modifier: _ => token(choice(...CREATE_MODIFIERS.map(keyword))),

    _if_not_exists: _ => kws('IF', 'NOT', 'EXISTS'),

    _if_exists: _ => kws('IF', 'EXISTS'),

    // The name of the object that CREATE makes, with IF NOT EXISTS. It is a
    // hidden rule of its own, so that the parts after the name have one set
    // of parse states.
    _create_name: $ => seq(
      optional($._if_not_exists),
      field('name', $.object_reference),
    ),

    // CREATE TABLE gives the columns of a table, or it copies a table with
    // CLONE or LIKE. The options of the table, such as PARTITION BY,
    // CLUSTER BY, OPTIONS, USING and LOCATION, are a list of options, and
    // AS and a query give the rows of the table.
    create_table_statement: $ => seq(
      $._create,
      kw('TABLE'),
      $._create_name,
      optional(choice(
        parens(commaSep1(choice($.column_definition, $.table_constraint))),
        seq(kw('CLONE'), field('source', $.object_reference)),
        seq(alias($._like_operator, 'LIKE'), field('source', $.object_reference)),
      )),
      optional($.options),
      optional(seq(kw('AS'), $.select_statement)),
    ),

    // A column of a table, with its type and its constraints. A column of
    // CREATE TABLE AS can have no type. A word that the grammar does not
    // read as a constraint, such as MASKING POLICY, is a word of a list of
    // options.
    column_definition: $ => choice(
      field('name', $._name),
      seq(
        field('name', $._name),
        field('type', $.type),
        repeat($._column_constraint),
      ),
    ),

    _column_constraint: $ => choice(
      seq(kw('CONSTRAINT'), field('constraint', $._name)),
      seq(kw('NOT'), choice($.null, kw('ENFORCED'))),
      kws('PRIMARY', 'KEY'),
      kw('UNIQUE'),
      $._check,
      $._references,
      seq(kw('DEFAULT'), field('default', $._default_value)),
      seq(kw('COMMENT'), optional('='), field('comment', $.string)),
      seq(kw('COLLATE'), field('collation', choice($.string, $._name))),
      seq(
        optional(seq(
          kw('GENERATED'),
          choice(kw('ALWAYS'), kws('BY', 'DEFAULT')),
        )),
        kw('AS'),
        choice(
          parens(field('generated', $._expression)),
          kw('IDENTITY'),
        ),
      ),
      seq(kw('OPTIONS'), $.option_group),
      $._property_item,
    ),

    // The default value of a column is a literal, a function call or an
    // expression in parentheses, so that an option after it is not a part
    // of an expression. The parse states that end an expression take each
    // token that can come after an expression in any place, and a state
    // with more than 64 columns takes the large form. So the forms that end
    // before the words of a list of options, such as this value and CHECK,
    // have their own parentheses and their own call, and not the rules of
    // an expression.
    _default_value: $ => choice(
      $._literal,
      seq(alias($._sign, $.operator), $.number),
      $.current_value,
      alias($._option_call, $.function_call),
      alias($._default_column, $.column_reference),
      alias($._default_name, $.column_reference),
      parens($._expression),
    ),

    // A call of a function that ends before a list of options, as in
    // DEFAULT CURRENT_TIMESTAMP() or FOR SYSTEM_TIME AS OF f(x).
    _option_call: $ => prec(1, seq(
      optional($._qualifier),
      field('name', $._function_name),
      parens(optional(commaSep1($._expression))),
    )),

    // A column of a table, or a value of a sequence, as in seq1.nextval.
    _default_column: $ => seq(field('table', $._name), '.', field('name', $._label)),

    _default_name: $ => field('name', $._name),

    _check: $ => seq(kw('CHECK'), parens(field('check', $._expression))),

    _references: $ => prec.right(seq(
      kw('REFERENCES'),
      field('references', $.object_reference),
      optional($.column_list),
    )),

    // A constraint of a table, with its name or not. The words after it,
    // such as NOT ENFORCED and RELY, are a list of options.
    table_constraint: $ => seq(
      optional(seq(kw('CONSTRAINT'), field('name', $._name))),
      choice(
        seq(kws('PRIMARY', 'KEY'), $.column_list),
        seq(kw('UNIQUE'), $.column_list),
        seq(kws('FOREIGN', 'KEY'), $.column_list, $._references),
        $._check,
      ),
      optional($.options),
    ),

    // CREATE VIEW. RECURSIVE makes a view that reads itself. A view of
    // Databricks can have no query, as in CREATE TEMPORARY VIEW v USING
    // CSV, or a body in a string, as a metric view has. The columns
    // can have comments, and the options of the view, such as COMMENT and
    // OPTIONS, are a list of options.
    create_view_statement: $ => seq(
      $._create,
      optional(kw('RECURSIVE')),
      optional(kw('MATERIALIZED')),
      kw('VIEW'),
      $._create_name,
      optional(parens(commaSep1($.view_column))),
      optional($.options),
      optional(seq(kw('AS'), choice($.select_statement, field('body', $.string)))),
    ),

    view_column: $ => seq(
      field('name', $._name),
      optional($.options),
    ),

    // CREATE INDEX makes an index of the columns of a table.
    create_index_statement: $ => seq(
      $._create,
      optional(kw('UNIQUE')),
      kw('INDEX'),
      $._create_name,
      kw('ON'),
      field('table', $.object_reference),
      parens(commaSep1($.index_column)),
      optional($.options),
    ),

    index_column: $ => seq(
      field('name', $._name),
      optional(choice(kw('ASC'), kw('DESC'))),
    ),

    // CREATE FUNCTION and CREATE PROCEDURE make a function or a procedure.
    // TABLE FUNCTION makes a function that gives rows, as in BigQuery. The
    // characteristics, such as LANGUAGE, HANDLER and IMMUTABLE, are a list
    // of options. The body is a string, an expression in parentheses or a
    // query after AS, or an expression or a query after RETURN.
    create_function_statement: $ => seq(
      $._create,
      optional(kw('TABLE')),
      choice(kw('FUNCTION'), kw('PROCEDURE')),
      $._create_name,
      choice(
        $._function_definition,
        seq(kw('AS'), field('class', $.string), optional($.options)),
      ),
    ),

    // The parameters, the characteristics and the body of a function. A
    // function of Databricks with no parameters can name the class of its
    // code after AS, as in CREATE FUNCTION f AS 'com.example.F'.
    _function_definition: $ => seq(
      $.parameter_list,
      repeat(choice(
        seq(kw('RETURNS'), field('return_type', choice(
          $.type,
          prec.right(seq(kw('TABLE'), optional(parens(commaSep1($.column_definition))))),
        ))),
        seq(kws('EXECUTE', 'AS'), $._option_name),
        $._property_item,
      )),
      optional(choice(
        seq(kw('AS'), field('body', choice($.string, $.parenthesized_expression, $.select_statement))),
        seq(kw('RETURN'), field('body', choice($._expression, $.select_statement))),
      )),
    ),

    parameter_list: $ => parens(optional(commaSep1($.parameter_declaration))),

    // A parameter, with its mode, its type and its default value.
    parameter_declaration: $ => seq(
      optional(choice(kw('IN'), kw('OUT'), kw('INOUT'))),
      field('name', $._label),
      field('type', $.type),
      optional(seq(choice(kw('DEFAULT'), '='), field('default', $._default_value))),
      optional($.options),
    ),

    // The CREATE of another kind of object, such as a schema, a database, a
    // sequence, a stage or a warehouse. The kind is one word, and the
    // options after the name are a list of options, as in CREATE STAGE s
    // URL = 's3://b/'. A kind of two words, such as FILE FORMAT, parses as
    // a kind and a name, and the real name is a word of the list. AS and a
    // statement give the statement of a task or a pipe.
    create_statement: $ => seq(
      $._create,
      field('kind', choice(
        kw('SCHEMA'),
        kw('DATABASE'),
        kw('SEQUENCE'),
        alias($.identifier, $.keyword),
      )),
      $._create_name,
      optional($.options),
      optional(seq(kw('AS'), $._statement)),
    ),

    // ALTER changes an object. The changes have commas between them. A
    // change that the grammar does not read word by word, such as SWAP WITH
    // t or SUSPEND, is a list of options.
    alter_statement: $ => seq(
      kw('ALTER'),
      field('kind', $._object_kind),
      choice(
        seq(
          optional($._if_exists),
          field('name', $.object_reference),
          optional(choice(
            commaSep1($._alter_action),
            seq(kw('AS'), $.select_statement),
          )),
        ),
        seq(choice(kw('SET'), kw('UNSET')), $.options),
      ),
    ),

    // The kind of an object of ALTER and DROP: a keyword, or a word.
    _object_kind: $ => choice(
      kw('TABLE'),
      kw('VIEW'),
      kws('MATERIALIZED', 'VIEW'),
      kw('SCHEMA'),
      kw('DATABASE'),
      kw('INDEX'),
      kw('SEQUENCE'),
      kw('FUNCTION'),
      kw('PROCEDURE'),
      alias($.identifier, $.keyword),
    ),

    _alter_action: $ => choice(
      $.add_column,
      $.drop_column,
      $.rename_column,
      $.rename_to,
      $.alter_column,
      seq(kw('ADD'), $.table_constraint),
      seq(choice(kw('SET'), kw('UNSET')), $.options),
      $.options,
    ),

    add_column: $ => seq(
      kw('ADD'),
      optional(choice(kw('COLUMN'), kw('COLUMNS'))),
      optional($._if_not_exists),
      choice(
        $.column_definition,
        parens(commaSep1($.column_definition)),
      ),
    ),

    // DROP deletes a column, or another part of a table, such as a
    // constraint or a partition, whose name and words are a list of
    // options.
    drop_column: $ => prec.right(seq(
      kw('DROP'),
      optional(kw('COLUMN')),
      optional($._if_exists),
      field('column', $.column_reference),
      optional(alias($._column_options, $.options)),
    )),

    rename_column: $ => seq(
      kws('RENAME', 'COLUMN'),
      optional($._if_exists),
      field('column', $.column_reference),
      kw('TO'),
      field('new_name', $._name),
    ),

    rename_to: $ => seq(
      kws('RENAME', 'TO'),
      field('new_name', $.object_reference),
    ),

    // ALTER COLUMN changes a column. The change is a list of options, as in
    // SET DATA TYPE INT or DROP NOT NULL.
    alter_column: $ => seq(
      choice(kw('ALTER'), kw('MODIFY'), kw('CHANGE')),
      optional(kw('COLUMN')),
      field('column', $.column_reference),
      optional($.options),
    ),

    // DROP deletes objects. DROP FUNCTION can name the types of the
    // parameters. The words after the names, such as CASCADE and PURGE,
    // are a list of options.
    drop_statement: $ => seq(
      kw('DROP'),
      field('kind', $._object_kind),
      optional($._if_exists),
      commaSep1(field('name', $.object_reference)),
      optional(parens(optional(commaSep1($.type)))),
      optional($.options),
    ),

    // COMMENT ON gives the comment of an object. The kind and the name of
    // the object are a list of options.
    comment_statement: $ => seq(
      kw('COMMENT'),
      optional($._if_exists),
      kw('ON'),
      alias($._command_options, $.options),
      kw('IS'),
      field('comment', choice($.string, $.null)),
    ),

    // Access control

    // GRANT gives privileges on an object, or roles, to users and roles.
    // The privileges, the object and the users are lists of options.
    grant_statement: $ => seq(
      kw('GRANT'),
      $._grant_list,
      optional(seq(kw('ON'), $._grant_list)),
      kw('TO'),
      $._grant_list,
    ),

    revoke_statement: $ => seq(
      kw('REVOKE'),
      $._grant_list,
      optional(seq(kw('ON'), $._grant_list)),
      kw('FROM'),
      $._grant_list,
    ),

    _grant_list: $ => commaSep1(alias($._grant_options, $.options)),

    // The session

    // USE sets the database, the schema, the catalog, the warehouse or the
    // role of the session.
    use_statement: $ => seq(
      kw('USE'),
      optional(choice(
        kw('DATABASE'),
        kw('SCHEMA'),
        kw('CATALOG'),
        kw('WAREHOUSE'),
        kw('ROLE'),
        kws('SECONDARY', 'ROLES'),
      )),
      choice(field('name', $.object_reference), kw('ALL'), kw('NONE')),
    ),

    // SET gives a variable or a parameter of the session a value, as in
    // SET x = 1, SET (a, b) = (1, 2) or SET TIME ZONE 'UTC'. SET with a name
    // and no value shows the value, as in Databricks.
    set_statement: $ => seq(
      kw('SET'),
      optional(choice(kw('SESSION'), kw('VAR'), kw('VARIABLE'), alias($._create_modifier, $.keyword))),
      optional(choice(
        seq(
          field('name', choice(alias($._dotted_name, $.object_reference), $.column_list)),
          optional(seq(choice('=', kw('TO')), field('value', choice($._expression, kw('DEFAULT'))))),
        ),
        seq(kws('TIME', 'ZONE'), field('value', choice($._expression, alias($._create_modifier, $.keyword)))),
      )),
    ),

    // A name with any number of parts, such as spark.sql.shuffle.partitions.
    _dotted_name: $ => seq($._name, repeat(seq('.', $._label))),

    // UNSET and RESET take a variable or a parameter back.
    unset_statement: $ => seq(
      choice(kw('UNSET'), kw('RESET')),
      optional(choice(
        commaSep1(field('name', alias($._dotted_name, $.object_reference))),
        $.column_list,
      )),
    ),

    // SHOW lists objects, as in SHOW TABLES LIKE 'a%' IN SCHEMA s. The
    // words after SHOW are a list of options.
    show_statement: $ => seq(kw('SHOW'), optional(alias($._command_options, $.options))),

    // DESCRIBE gives the columns of a table or the details of an object.
    // The words after DESCRIBE are a list of options.
    describe_statement: $ => seq(
      choice(kw('DESCRIBE'), kw('DESC')),
      alias($._command_options, $.options),
    ),

    // EXPLAIN gives the plan of a statement, with the words of its options
    // before the statement, as in EXPLAIN USING JSON or EXPLAIN EXTENDED.
    explain_statement: $ => seq(
      kw('EXPLAIN'),
      repeat(alias(reserved('label', $.identifier), $.keyword)),
      $._statement,
    ),

    // CALL runs a procedure.
    call_statement: $ => seq(
      kw('CALL'),
      field('procedure', $.object_reference),
      $.arguments,
    ),

    // EXECUTE IMMEDIATE runs the statement in a string, with the values
    // after USING or INTO.
    execute_statement: $ => seq(
      kws('EXECUTE', 'IMMEDIATE'),
      field('statement', $._expression),
      repeat(seq(choice(kw('USING'), kw('INTO')), commaSep1($._expression))),
    ),

    // Transactions

    // The words after BEGIN TRANSACTION, such as NAME t of Snowflake, are a
    // list of options.
    begin_statement: $ => seq(
      choice(kw('BEGIN'), kw('START')),
      optional(choice(kw('TRANSACTION'), kw('WORK'))),
      optional(alias($._command_options, $.options)),
    ),

    commit_statement: _ => seq(kw('COMMIT'), optional(choice(kw('TRANSACTION'), kw('WORK')))),

    rollback_statement: $ => seq(
      kw('ROLLBACK'),
      optional(choice(kw('TRANSACTION'), kw('WORK'))),
      optional(seq(kw('TO'), optional(kw('SAVEPOINT')), field('savepoint', $._name))),
    ),

    // Lists of options

    // A list of options: words, strings, numbers, = and lists in
    // parentheses. A list in parentheses comes after another item, and not
    // at the start, where ( can start the columns of a table. The parse
    // states of a list take each keyword that can come after the list, so
    // the statements whose lists end before other keywords, such as GRANT,
    // have rules of their own, which are the node options too.
    options: $ => optionList($),

    // The privileges, the object and the users of GRANT and REVOKE.
    _grant_options: $ => optionList($),

    // The words after SHOW, DESCRIBE and COMMENT ON.
    _command_options: $ => optionList($),

    // The words after a column of ALTER TABLE, which can start with a list
    // in parentheses, as in DROP IF EXISTS PARTITION (a = 1).
    _column_options: $ => prec.right(repeat1($._option_item)),

    // An item of a list of options after a name, which is not a dot, so
    // that the name ends before it.
    _property_item: $ => choice(
      $._option_name,
      $.string,
      $.number,
      $.boolean,
      $.null,
      '=',
      $.option_group,
    ),

    _option_item: $ => choice(
      $._option_name,
      '.',
      $.string,
      $.number,
      $.boolean,
      $.null,
      $.stage_reference,
      '=',
      '=>',
      '<',
      '>',
      ':',
      '::',
      '*',
      '+',
      '-',
      '/',
      '||',
      $.option_group,
    ),

    // A list of options in parentheses, with commas between its items, as
    // in OPTIONS (description = 'a', labels = [('k', 'v')]).
    option_group: $ => choice(
      parens(optional(commaSep1(repeat1($._option_item)))),
      seq('[', optional(commaSep1(repeat1($._option_item))), ']'),
    ),

    // A word of a list of options. It takes the reserved set label, so a
    // reserved word is a word of the list too, as in PARTITION BY, but
    // where the state also expects the word as a keyword, as AS in CREATE
    // TABLE t CLUSTER BY (a) AS SELECT. One identifier covers all the
    // words, so that the states of the list are few and small. Each word is
    // a keyword in the tree, but a quoted identifier stays an identifier.
    _option_name: $ => choice(
      alias(reserved('label', $.identifier), $.keyword),
      alias($._quoted_identifier, $.identifier),
    ),

    // Queries

    select_statement: $ => $._query,

    // WITH comes before the body of a query, and ORDER BY, LIMIT, OFFSET
    // and FETCH after it, in any order.
    _query: $ => prec.right(seq(
      optional($.with_clause),
      $._query_body,
      repeat(choice(
        $.order_by_clause,
        $.limit_clause,
        $.offset_clause,
        $.fetch_clause,
        $.sort_by_clause,
      )),
    )),

    // SORT BY, CLUSTER BY and DISTRIBUTE BY of Databricks and Hive say how
    // the rows are sorted and split among the workers.
    // The words are one token, so that they take one column of each state
    // after a table.
    _sort_word: _ => token(choice(keyword('SORT'), keyword('CLUSTER'), keyword('DISTRIBUTE'))),

    sort_by_clause: $ => seq(
      alias($._sort_word, $.keyword),
      kw('BY'),
      commaSep1($.order_by_item),
    ),

    // WITH names the queries that the query reads, after RECURSIVE when
    // such a query reads itself.
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
    ),

    column_list: $ => parens(commaSep1($._name)),

    // VALUES in parentheses is a subquery, and not a table in a
    // parenthesized join.
    _query_body: $ => choice(
      $.select,
      prec(1, $.values),
      $.table_query,
      $.set_operation,
      $.parenthesized_query,
    ),

    parenthesized_query: $ => parens($._query),

    // INTERSECT binds more tightly than UNION, EXCEPT and MINUS. BY NAME
    // matches the columns of the two sides by their names.
    set_operation: $ => choice(
      prec.left(1, seq(
        field('left', $._query_body),
        field('operator', alias($._set_operator, $.keyword)),
        optional($._set_quantifier),
        optional(kws('BY', 'NAME')),
        field('right', $._query_body),
      )),
      prec.left(2, seq(
        field('left', $._query_body),
        field('operator', kw('INTERSECT')),
        optional($._set_quantifier),
        optional(kws('BY', 'NAME')),
        field('right', $._query_body),
      )),
    ),

    // UNION, EXCEPT and MINUS are one token, so that they take one column of
    // each state after a table or an expression.
    _set_operator: _ => token(choice(keyword('UNION'), keyword('EXCEPT'), keyword('MINUS'))),

    _set_quantifier: _ => choice(kw('ALL'), kw('DISTINCT')),

    // The clauses after SELECT come in any order. A comma can come after
    // the last item, as in Snowflake and BigQuery. TOP takes the first rows,
    // as in Snowflake, and AS STRUCT and AS VALUE give a STRUCT or a value
    // for each row, as in BigQuery.
    select: $ => prec.right(seq(
      kw('SELECT'),
      optional($._set_quantifier),
      optional($.top_clause),
      optional(seq(kw('AS'), choice(kw('STRUCT'), kw('VALUE')))),
      commaSep1($._select_item),
      optional(','),
      repeat(choice(
        $.from_clause,
        $.where_clause,
        $.group_by_clause,
        $.having_clause,
        $.window_clause,
        $.qualify_clause,
      )),
    )),

    // TOP and its count are one token, so that a column can have the name
    // top.
    top_clause: _ => token(seq(keyword('TOP'), /\s+/, /[0-9]+/)),

    _select_item: $ => choice(
      $._expression,
      $.aliased_expression,
      $.star_expression,
    ),

    // An alias comes after AS or after the expression with no AS.
    aliased_expression: $ => choice(
      seq(field('value', $._expression), kw('AS'), field('alias', $._label)),
      seq(field('value', $._expression), field('alias', $._name)),
    ),

    // * gives all the columns, and t.* all the columns of a table. EXCEPT
    // and EXCLUDE leave out columns, REPLACE gives columns new values,
    // RENAME gives them new names, and ILIKE keeps the columns whose names
    // match a pattern.
    star_expression: $ => prec.right(seq(
      optional(seq(
        optional(seq(field('schema', $._name), '.')),
        field('table', $._name),
        '.',
      )),
      '*',
      repeat($._star_modifier),
    )),

    _star_modifier: $ => prec(1, choice(
      seq(
        choice(alias($._set_operator, $.keyword), kw('EXCLUDE')),
        choice(parens(commaSep1(field('column', $.column_reference))), field('column', $._name)),
      ),
      seq(
        choice(kw('REPLACE'), kw('RENAME')),
        choice(parens(commaSep1($.aliased_expression)), $.aliased_expression),
      ),
      seq(alias($._like_operator, 'ILIKE'), field('pattern', $.string)),
    )),

    // VALUES gives rows, in place of a SELECT. A row is an expression, and
    // a row of more than one column is a row value in parentheses.
    // The rows can have an alias, as in VALUES (1), (2) AS t(a) of
    // Databricks, and VALUES with no parentheses is a table in FROM, as in
    // Snowflake.
    values: $ => prec.right(seq(
      kw('VALUES'),
      commaSep1($._expression),
      optional($._alias),
    )),

    // TABLE t reads each row of a table.
    table_query: $ => seq(kw('TABLE'), field('table', $.object_reference)),

    from_clause: $ => seq(kw('FROM'), commaSep1($._table_reference)),

    _table_reference: $ => choice(
      $._table_factor,
      $.join,
    ),

    // The right side of a join is a table factor: a join on the right side
    // must be in parentheses.
    _table_factor: $ => choice(
      $._table_primary,
      $.lateral_view,
      $.pivot_table,
      $.unpivot_table,
      $.sampled_table,
    ),

    _table_primary: $ => choice(
      $.table_reference,
      $.values,
      $.derived_table,
      $.unnest_table,
      $.function_table,
      $.stage_table,
      $.parenthesized_join,
    ),

    // A table. The clauses after its name read an earlier version of the
    // table, as FOR SYSTEM_TIME AS OF of BigQuery, AT and BEFORE of
    // Snowflake, and VERSION AS OF and TIMESTAMP AS OF of Databricks do.
    table_reference: $ => prec.right(seq(
      field('table', $.object_reference),
      repeat($.time_travel),
      optional($._alias),
    )),

    time_travel: $ => choice(
      seq(
        choice(kws('FOR', 'SYSTEM_TIME'), alias($._version_word, $.keyword)),
        kws('AS', 'OF'),
        field('value', $._default_value),
      ),
      seq(alias($._at_word, $.keyword), $._named_arguments),
    ),

    // Arguments with names in parentheses, which end before an alias of a
    // table, as in AT(OFFSET => -60).
    _named_arguments: $ => parens(commaSep1($.named_argument)),

    // The words of a version are one token, and so are AT, BEFORE and
    // CHANGES, so that they take few columns of each state after a table.
    // AT TIME ZONE takes the token of AT too.
    _version_word: _ => token(choice(keyword('TIMESTAMP'), keyword('VERSION'))),

    _at_word: _ => token(choice(keyword('AT'), keyword('BEFORE'), keyword('CHANGES'))),

    // An alias of a table, with the names of its columns.
    _alias: $ => seq(
      choice(
        seq(kw('AS'), field('alias', $._label)),
        field('alias', $._name),
      ),
      optional($.column_list),
    ),

    // A subquery in FROM. LATERAL lets it read the tables before it.
    derived_table: $ => prec.right(seq(
      optional(kw('LATERAL')),
      $.subquery,
      optional($._alias),
    )),

    // UNNEST turns arrays into rows. WITH ORDINALITY and WITH OFFSET add
    // the number of each row, and WITH OFFSET comes after the alias, as in
    // BigQuery.
    unnest_table: $ => prec.right(seq(
      kw('UNNEST'),
      parens(commaSep1($._expression)),
      optional(kws('WITH', 'ORDINALITY')),
      optional($._alias),
      optional(seq(
        kws('WITH', 'OFFSET'),
        optional(seq(optional(kw('AS')), field('offset', $._name))),
      )),
    )),

    // A function that gives rows, as in LATERAL FLATTEN(input => v) or
    // TABLE(GENERATOR(ROWCOUNT => 10)).
    function_table: $ => prec.right(seq(
      optional(kw('LATERAL')),
      choice(
        $.function_call,
        seq(kw('TABLE'), parens($.function_call)),
      ),
      optional($._alias),
    )),

    // The files of a stage, with the options in parentheses, as in
    // Snowflake.
    stage_table: $ => prec.right(seq(
      field('stage', $.stage_reference),
      optional($._named_arguments),
      optional($._alias),
    )),

    // A list of tables and joins in parentheses.
    parenthesized_join: $ => prec.right(seq(
      parens($._table_reference),
      optional($._alias),
    )),

    // TABLESAMPLE and SAMPLE take a sample of the rows of a table: a
    // percent, a number of rows or a bucket, with the method and the seed.
    sampled_table: $ => prec.left(seq(
      field('table', $._table_factor),
      alias(token(choice(keyword('TABLESAMPLE'), keyword('SAMPLE'))), $.keyword),
      optional(field('method', choice(kw('BERNOULLI'), kw('SYSTEM'), kw('ROW'), kw('BLOCK')))),
      parens(choice(
        seq(field('size', $._expression), optional(choice(kw('PERCENT'), kw('ROWS')))),
        seq(kw('BUCKET'), $.number, kws('OUT', 'OF'), $.number),
      )),
      optional(seq(choice(kw('REPEATABLE'), kw('SEED')), parens(field('seed', $.number)))),
    )),

    // A join. ON and USING are optional, because some dialects take a join
    // with no condition. APPLY is a lateral join, as in CROSS APPLY and
    // OUTER APPLY. MATCH_CONDITION is the condition of ASOF JOIN of
    // Snowflake.
    join: $ => prec.left(seq(
      field('left', $._table_reference),
      $._join_operator,
      field('right', $._table_factor),
      optional(seq(kw('MATCH_CONDITION'), parens(field('match_condition', $._expression)))),
      optional(choice(
        seq(kw('ON'), field('condition', $._expression)),
        seq(kw('USING'), $.column_list),
      )),
    )),

    // The words before JOIN come in any order. The words that only come
    // before JOIN, such as INNER, CROSS and ASOF, are one token, the node
    // keyword, so that they take one column of each state after a table
    // and after an expression.
    _join_operator: $ => seq(
      repeat(choice(
        alias($._join_word, $.keyword),
        kw('LEFT'),
        kw('RIGHT'),
        kw('FULL'),
        kw('OUTER'),
      )),
      choice(kw('JOIN'), kw('APPLY')),
    ),

    _join_word: _ => token(choice(...['NATURAL', 'INNER', 'CROSS', 'SEMI', 'ANTI', 'ASOF', 'DIRECTED'].map(keyword))),

    // LATERAL VIEW of Databricks and Hive joins each row with the rows of a
    // function, with the names of the columns after AS.
    lateral_view: $ => prec.left(seq(
      field('table', $._table_factor),
      kws('LATERAL', 'VIEW'),
      optional(kw('OUTER')),
      field('function', $.function_call),
      optional(field('alias', $._name)),
      optional(seq(kw('AS'), commaSep1(field('column', $._label)))),
    )),

    // PIVOT turns the values of the column after FOR into columns, with the
    // aggregates before FOR. The values can be a list, ANY or a query.
    pivot_table: $ => prec.left(seq(
      field('table', $._table_factor),
      kw('PIVOT'),
      parens(seq(
        commaSep1(choice($._expression, $.aliased_expression)),
        kw('FOR'),
        field('columns', choice($.column_reference, $.column_list)),
        kw('IN'),
        parens(choice(
          commaSep1(choice($._expression, $.aliased_expression)),
          seq(kw('ANY'), optional($.order_by_clause)),
          $._query,
        )),
        optional(seq(kws('DEFAULT', 'ON'), $.null, field('default', $.parenthesized_expression))),
      )),
      optional($._alias),
    )),

    // UNPIVOT turns columns into rows: the values into the column before
    // FOR, and the names into the column after FOR.
    unpivot_table: $ => prec.left(seq(
      field('table', $._table_factor),
      kw('UNPIVOT'),
      optional(seq(choice(kw('INCLUDE'), kw('EXCLUDE')), kw('NULLS'))),
      parens(seq(
        field('value', choice($._name, $.column_list)),
        kw('FOR'),
        field('name', $._name),
        kw('IN'),
        parens(commaSep1(choice($._expression, $.aliased_expression))),
      )),
      optional($._alias),
    )),

    where_clause: $ => seq(kw('WHERE'), field('condition', $._expression)),

    // GROUP BY ALL groups by each column of SELECT that is not an
    // aggregate.
    group_by_clause: $ => prec.right(seq(
      kws('GROUP', 'BY'),
      optional($._set_quantifier),
      optional(commaSep1($._grouping_element)),
      optional(seq(kw('WITH'), choice(kw('ROLLUP'), kw('CUBE')))),
    )),

    _grouping_element: $ => choice(
      $._expression,
      $.empty_grouping_set,
      $.rollup,
      $.cube,
      $.grouping_sets,
    ),

    empty_grouping_set: _ => seq('(', ')'),

    rollup: $ => seq(kw('ROLLUP'), parens(commaSep1($._grouping_set))),

    cube: $ => seq(kw('CUBE'), parens(commaSep1($._grouping_set))),

    grouping_sets: $ => seq(
      kws('GROUPING', 'SETS'),
      parens(commaSep1(choice($._grouping_set, $.rollup, $.cube))),
    ),

    _grouping_set: $ => choice($._expression, $.empty_grouping_set),

    having_clause: $ => seq(kw('HAVING'), field('condition', $._expression)),

    window_clause: $ => seq(kw('WINDOW'), commaSep1($.window_definition)),

    window_definition: $ => seq(
      field('name', $._name),
      kw('AS'),
      choice($.window_specification, field('window', $._name)),
    ),

    // QUALIFY filters the rows after the window functions.
    qualify_clause: $ => seq(kw('QUALIFY'), field('condition', $._expression)),

    // A window can start with the name of a window of the WINDOW clause.
    // The grammar takes its clauses in any order.
    window_specification: $ => parens(seq(
      optional(field('name', $._name)),
      repeat(choice(
        $.partition_by_clause,
        $.order_by_clause,
        $.frame_clause,
      )),
    )),

    partition_by_clause: $ => seq(kws('PARTITION', 'BY'), commaSep1($._expression)),

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

    // ORDER BY ALL orders by each column of SELECT.
    order_by_item: $ => seq(
      field('value', choice($._expression, kw('ALL'))),
      optional(choice(kw('ASC'), kw('DESC'))),
      optional(seq(kw('NULLS'), choice(kw('FIRST'), kw('LAST')))),
    ),

    // LIMIT takes a count, or a start and a count with a comma between
    // them.
    limit_clause: $ => seq(
      kw('LIMIT'),
      choice(
        field('count', $._expression),
        seq(field('start', $._expression), ',', field('count', $._expression)),
        kw('ALL'),
      ),
    ),

    offset_clause: $ => seq(
      kw('OFFSET'),
      field('start', $._expression),
      optional(choice(kw('ROW'), kw('ROWS'))),
    ),

    // FETCH FIRST takes the first rows, and WITH TIES the rows that tie
    // with the last of them.
    fetch_clause: $ => seq(
      kw('FETCH'),
      choice(kw('FIRST'), kw('NEXT')),
      optional(field('count', $._expression)),
      choice(kw('ROW'), kw('ROWS')),
      choice(kw('ONLY'), kws('WITH', 'TIES')),
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
      $.collate_expression,
      $.at_time_zone_expression,
      $.lambda_expression,
    ),

    _primary_expression: $ => choice(
      $._literal,
      $.typed_literal,
      $.interval_literal,
      $.current_value,
      $.column_reference,
      $.parameter,
      $.function_call,
      $.cast_expression,
      $.cast,
      $.subscript_expression,
      $.json_access,
      $.field_selection,
      $.parenthesized_expression,
      $.row_constructor,
      $.array,
      $.struct,
      $.object_constant,
      $.subquery,
      $.exists_expression,
      $.case_expression,
    ),

    _literal: $ => choice(
      $.number,
      $.string,
      $.boolean,
      $.null,
    ),

    // TRUE and FALSE are one token, so that they take one column of each
    // state where an expression starts.
    boolean: _ => token(choice(keyword('TRUE'), keyword('FALSE'))),

    // NULL is a token of its own, so that it takes one column of each state
    // where an expression starts.
    null: _ => (KEYWORDS.add('NULL'), keyword('NULL')),

    // The words that give a value of the session, such as CURRENT_DATE,
    // are one token. CURRENT_TIMESTAMP(3) is a function call with the
    // token as its name.
    current_value: _ => token(choice(...CURRENT_WORDS.map(keyword))),

    // A type name before a string gives a constant of the type, such as
    // DATE '2024-03-01' or JSON '{"a": 1}'. The name is an identifier, so
    // date also names a column.
    typed_literal: $ => seq(
      field('type', alias($.identifier, $.type)),
      field('value', $.string),
    ),

    // An interval: INTERVAL, a value and its units, as in INTERVAL 1 DAY,
    // INTERVAL '1-2' YEAR TO MONTH, INTERVAL '1 day' or INTERVAL 1 DAY 2
    // HOURS.
    interval_literal: $ => prec.right(seq(
      kw('INTERVAL'),
      field('value', choice(
        $.string,
        $.number,
        seq(alias($._sign, $.operator), choice($.string, $.number)),
        parens($._expression),
      )),
      repeat(choice(
        $.interval_unit,
        seq(kw('TO'), $.interval_unit),
        $.number,
        $.string,
      )),
    )),

    // The units of an interval are one token, so that they take one column
    // of each state after the value of an interval.
    interval_unit: _ => token(choice(...INTERVAL_UNITS.flatMap(u => [keyword(u), keyword(u + 'S')]))),

    // A column, with its table, its schema and its catalog. The grammar
    // cannot tell a field of a STRUCT from a column, so s.a is a column a
    // of a table s.
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
        field('catalog', $._name),
        '.',
        field('schema', $._label),
        '.',
        field('table', $._label),
        '.',
        field('name', $._label),
      ),
    )),

    // A parameter or a variable: ?, :name, :1, @name, @@name, $1, $name
    // and ${name}.
    parameter: _ => token(choice(
      '?',
      /:[A-Za-z_][A-Za-z0-9_]*/,
      /:[0-9]+/,
      /@@?[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*/,
      /\$[0-9]+/,
      /\$[A-Za-z_][A-Za-z0-9_]*/,
      /\$\{[^}]*\}/,
    )),

    // A stage of Snowflake, with a path after it, as in @s/data/ or
    // @~/f.csv.
    stage_reference: _ => token(seq(
      '@',
      choice(
        /[~%]?[A-Za-z_][A-Za-z0-9_$]*(\.[A-Za-z_][A-Za-z0-9_$]*)*/,
        '~',
        /"([^"]|"")*"/,
      ),
      optional(/\/[^\s;,()'"]*/),
    )),

    // A function call. The arguments of an aggregate function can have
    // DISTINCT, ORDER BY, LIMIT and IGNORE NULLS. WITHIN GROUP, FILTER,
    // IGNORE NULLS and OVER come after the arguments. The grammar takes
    // them in any order.
    function_call: $ => prec.right(1, seq(
      optional($._qualifier),
      field('name', $._function_name),
      $.arguments,
      repeat(choice(
        $.within_group_clause,
        $.filter_clause,
        seq(alias($._nulls_word, $.keyword), kw('NULLS')),
        $.over_clause,
      )),
    )),

    // A function name can be a word that the grammar reserves for a
    // function, such as LEFT, or a word such as CURRENT_TIMESTAMP, which
    // takes a precision in parentheses.
    _function_name: $ => choice(
      reserved('function_name', $.identifier),
      alias($._quoted_identifier, $.identifier),
      alias($.current_value, $.identifier),
    ),

    arguments: $ => parens(optional(seq(
      optional($._set_quantifier),
      $._argument_list,
      repeat(choice(
        $.order_by_clause,
        $.limit_clause,
        seq(alias($._nulls_word, $.keyword), kw('NULLS')),
      )),
    ))),

    // The arguments of a function. Most functions take a list of
    // expressions with commas between them. The functions that the standard
    // gives a syntax of their own have keywords between their arguments,
    // such as substring(s FROM 2 FOR 3), trim(BOTH 'x' FROM s) and
    // extract(year FROM d). The grammar accepts each of these keywords
    // between two arguments of any function, so that each argument is in
    // the same parse states. position(a IN b) is a call with the IN of an
    // expression.
    _argument_list: $ => seq(
      optional(seq(choice(kw('BOTH'), kw('LEADING'), kw('TRAILING')), optional(kw('FROM')))),
      $._argument,
      repeat(seq(choice(',', kw('FROM'), kw('FOR')), $._argument)),
    ),

    _argument: $ => choice(
      $._expression,
      $.named_argument,
      $.star_expression,
    ),

    // An argument with a name, as in FLATTEN(input => v, outer => true).
    // The name can be any keyword.
    named_argument: $ => seq(
      field('name', $._label),
      '=>',
      field('value', $._expression),
    ),

    // IGNORE and RESPECT are one token.
    _nulls_word: _ => token(choice(keyword('IGNORE'), keyword('RESPECT'))),

    within_group_clause: $ => seq(
      kws('WITHIN', 'GROUP'),
      parens($.order_by_clause),
    ),

    filter_clause: $ => seq(
      kw('FILTER'),
      parens(seq(kw('WHERE'), field('condition', $._expression))),
    ),

    over_clause: $ => seq(
      kw('OVER'),
      choice(field('window', $._name), $.window_specification),
    ),

    // CAST(x AS type), and TRY_CAST and SAFE_CAST, which give NULL in place
    // of an error. FORMAT gives the format of a date or a number, as in
    // BigQuery.
    cast_expression: $ => seq(
      field('name', alias($._cast_name, $.identifier)),
      parens(seq(
        field('value', $._expression),
        kw('AS'),
        field('type', $.type),
        optional(seq(kw('FORMAT'), field('format', $._expression))),
      )),
    ),

    _cast_name: _ => (token(choice(keyword('CAST'), keyword('TRY_CAST'), keyword('SAFE_CAST')))),

    // A value and a type with :: between them, as in x::INT.
    cast: $ => prec.left(PREC.cast, seq(
      field('value', $._primary_expression),
      '::',
      field('type', $.type),
    )),

    // An element of an array or a map, as in a[1], m['key'] or
    // a[OFFSET(1)].
    subscript_expression: $ => prec(PREC.postfix, seq(
      field('value', $._primary_expression),
      '[',
      field('index', $._expression),
      ']',
    )),

    // A field of a semi-structured value with : before it, as in v:name or
    // v:"Name" of Snowflake and Databricks. A dot and a subscript after it
    // give a deeper field, as in v:a.b[0].
    json_access: $ => prec(PREC.postfix, seq(
      field('value', $._primary_expression),
      ':',
      field('field', choice($._label, $.string)),
    )),

    // A field of a value, such as (r).a, f(x).b or a[1].c. A column
    // reference with more than four parts is a field of a value, as in
    // c.s.t.r.f. .* gives each field.
    field_selection: $ => prec(PREC.field, seq(
      field('value', choice(
        $.column_reference,
        $.parenthesized_expression,
        $.subscript_expression,
        $.json_access,
        $.function_call,
        $.cast_expression,
        $.field_selection,
      )),
      '.',
      choice(field('field', $._label), '*'),
    )),

    parenthesized_expression: $ => parens($._expression),

    // A row value: a list of two or more expressions in parentheses.
    row_constructor: $ => parens(seq($._expression, ',', commaSep1($._expression))),

    // An array, as in ARRAY[1, 2], [1, 2], ARRAY<INT64>[1, 2], ARRAY(1, 2)
    // or ARRAY(SELECT x FROM t).
    array: $ => choice(
      seq(
        optional(seq(kw('ARRAY'), optional(seq('<', field('type', $.type), '>')))),
        '[',
        optional(commaSep1($._expression)),
        ']',
      ),
      seq(
        kw('ARRAY'),
        parens(choice($._query, optional(commaSep1($._expression)))),
      ),
    ),

    // A STRUCT value, as in STRUCT(1 AS a, 'x' AS b) or STRUCT<a INT64>(1).
    // The fields can have names after AS.
    struct: $ => seq(
      kw('STRUCT'),
      optional(seq('<', commaSep1($.field_definition), '>')),
      parens(optional(commaSep1(choice($._expression, $.aliased_argument)))),
    ),

    aliased_argument: $ => seq(field('value', $._expression), kw('AS'), field('alias', $._label)),

    // An object constant of Snowflake, as in {'a': 1, 'b': 'x'}.
    object_constant: $ => seq(
      '{',
      optional(commaSep1(seq(field('key', $.string), ':', field('value', $._expression)))),
      '}',
    ),

    // EXISTS tests the rows of a subquery. exists(a, x -> x > 1) of
    // Databricks is a call with the same keyword.
    exists_expression: $ => seq(
      kw('EXISTS'),
      parens(choice($._query, commaSep1($._expression))),
    ),

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

    // A lambda: x -> x + 1 or (x, y) -> x + y.
    lambda_expression: $ => prec.right(PREC.lambda, seq(
      field('parameters', $._expression),
      '->',
      field('body', $._expression),
    )),

    // A choice of operators flattens into one production for each
    // operator, and each production has parse states of its own. So each
    // level of precedence takes its operators from one token, which is the
    // node operator.
    unary_expression: $ => choice(
      prec.right(PREC.not, seq(field('operator', kw('NOT')), field('operand', $._expression))),
      prec.right(PREC.unary, seq(field('operator', alias($._unary_operator, $.operator)), field('operand', $._expression))),
    ),

    _sign: _ => token(choice('-', '+')),

    _unary_operator: _ => token(choice('-', '+', '~')),

    binary_expression: $ => {
      /** @type {[number, RuleOrLiteral, RuleOrLiteral][]} */
      const table = [
        [PREC.or, kw('OR'), $._expression],
        [PREC.and, kw('AND'), $._expression],
        [PREC.compare, alias($._comparison_operator, $.operator), $._operand],
        [PREC.bitor, alias(token(choice('|', '^')), $.operator), $._expression],
        [PREC.bitand, alias('&', $.operator), $._expression],
        [PREC.shift, alias(token(choice('<<', '>>')), $.operator), $._expression],
        [PREC.add, alias(token(choice('+', '-', '||')), $.operator), $._expression],
        [PREC.multiply, alias($._multiplicative_operator, $.operator), $._expression],
      ];
      return choice(...table.map(([p, op, right]) => prec.left(p, seq(
        field('left', $._expression),
        field('operator', op),
        field('right', right),
      ))));
    },

    _comparison_operator: _ => token(choice('=', '==', '<', '>', '<=', '>=', '<>', '!=', '<=>')),

    _multiplicative_operator: _ => token(choice('*', '/', '%')),

    _operand: $ => choice($._expression, $.quantified_expression),

    // ANY, SOME or ALL after a comparison or LIKE compares the left side
    // with each row of a subquery or each value of a list.
    quantified_expression: $ => seq(
      field('quantifier', choice(kw('ANY'), kw('SOME'), kw('ALL'))),
      $._primary_expression,
    ),

    // IS NULL, IS TRUE, IS DISTINCT FROM and the others.
    is_expression: $ => prec.left(PREC.compare, seq(
      field('left', $._expression),
      kw('IS'),
      optional(kw('NOT')),
      field('right', choice(
        $.boolean,
        $.null,
        kw('UNKNOWN'),
        $.distinct_from,
      )),
    )),

    distinct_from: $ => seq(kws('DISTINCT', 'FROM'), $._expression),

    between_expression: $ => prec.left(PREC.compare, seq(
      field('left', $._expression),
      optional(kw('NOT')),
      kw('BETWEEN'),
      optional(choice(kw('SYMMETRIC'), kw('ASYMMETRIC'))),
      field('low', $._expression),
      kw('AND'),
      field('high', $._expression),
    )),

    // The right side of IN is a list in parentheses, which is a row value,
    // a subquery, or another expression, as in x IN UNNEST(a) of BigQuery.
    in_expression: $ => prec.left(PREC.compare, seq(
      field('left', $._expression),
      optional(kw('NOT')),
      kw('IN'),
      field('right', $._expression),
    )),

    // LIKE, ILIKE, RLIKE and REGEXP are one token, the node operator.
    like_expression: $ => prec.left(PREC.compare, seq(
      field('left', $._expression),
      optional(kw('NOT')),
      field('operator', alias($._like_operator, $.operator)),
      field('right', $._operand),
      optional(seq(kw('ESCAPE'), field('escape', $._expression))),
    )),

    _like_operator: _ => (token(choice(...['LIKE', 'ILIKE', 'RLIKE', 'REGEXP'].map(keyword)))),

    collate_expression: $ => prec.left(PREC.collate, seq(
      field('value', $._expression),
      kw('COLLATE'),
      field('collation', choice($.string, $._name)),
    )),

    at_time_zone_expression: $ => prec.left(PREC.collate, seq(
      field('value', $._expression),
      alias($._at_word, $.keyword),
      kws('TIME', 'ZONE'),
      field('zone', $._primary_expression),
    )),

    // Types

    // A type: a name with its parameters in parentheses, as in
    // DECIMAL(10, 2) or VARCHAR(100), a time with WITH TIME ZONE, or the
    // types ARRAY<T>, STRUCT<a T, b U> and MAP<K, V>. ARRAY alone is the
    // semi-structured array of Snowflake. [] after a type gives an array.
    type: $ => prec.right(choice(
      seq(
        optional($._qualifier),
        field('name', $._label),
        optional($.type_modifiers),
        optional(seq(choice(kw('WITH'), kw('WITHOUT')), kws('TIME', 'ZONE'))),
        optional(seq('[', ']')),
      ),
      seq(field('name', alias(kw('ARRAY'), $.identifier)), optional(seq('<', $.type, '>'))),
      seq(field('name', alias(kw('STRUCT'), $.identifier)), '<', commaSep1($.field_definition), '>'),
      seq(field('name', alias(kw('MAP'), $.identifier)), '<', $.type, ',', $.type, '>'),
      seq(field('name', alias(kw('TABLE'), $.identifier)), '<', commaSep1($.field_definition), '>'),
    )),

    // The parameters of a type: numbers, types and words, as in
    // VARCHAR(10 CHAR).
    type_modifiers: $ => parens(commaSep1(choice(
      $.number,
      $.type,
    ))),

    // A field of a STRUCT type, with its name or not, a colon after the
    // name, as in Databricks, and the options after the type, such as NOT
    // NULL and COMMENT.
    field_definition: $ => choice(
      $.type,
      seq(
        field('name', $._name),
        optional(':'),
        field('type', $.type),
        repeat(choice(
          seq(kw('NOT'), $.null),
          seq(kw('COMMENT'), field('comment', $.string)),
          seq(kw('OPTIONS'), $.option_group),
        )),
      ),
    ),

    // Names and literals

    // A name of a table, with its schema and its catalog.
    object_reference: $ => choice(
      field('name', $._name),
      seq(field('schema', $._name), '.', field('name', $._label)),
      seq(
        field('catalog', $._name),
        '.',
        field('schema', $._label),
        '.',
        field('name', $._label),
      ),
    ),

    // The schema before the name of a function or a type, with its
    // catalog.
    _qualifier: $ => choice(
      seq(field('schema', $._name), '.'),
      seq(field('catalog', $._name), '.', field('schema', $._label), '.'),
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

    // An identifier starts with a letter or _, and holds letters, digits, _
    // and $. A name that starts with _ is also the placeholder of a variable
    // of usql (D101).
    identifier: _ => /[A-Za-z_][A-Za-z0-9_$]*/,

    // A quoted identifier is in double quotes, with "" for a ", or in
    // backticks, with `` for a `. A name in backticks can hold dots, as in
    // `project.dataset.table` of BigQuery.
    _quoted_identifier: _ => token(choice(
      /"([^"]|"")*"/,
      /`([^`]|``)*`/,
    )),

    // A number is an integer, a decimal number or a number with an
    // exponent, with a suffix of its type such as L or BD, as in
    // Databricks. An integer can be hexadecimal, as in 0x1F.
    number: _ => token(choice(
      /[0-9]+(\.[0-9]*)?([eE][+-]?[0-9]+)?([lLsSyYdDfF]|[bB][dD])?/,
      /\.[0-9]+([eE][+-]?[0-9]+)?([dDfF]|[bB][dD])?/,
      /0[xX][0-9a-fA-F]+/,
    )),

    // A string is in single quotes, with '' or \' for a quote, or in three
    // single or double quotes, as in BigQuery. It can have a prefix: r or R
    // for a raw string, b or B for bytes, n or N, e or E, and x or X. A
    // string in $$ is the string of Snowflake, which is also the body of a
    // function.
    string: _ => token(choice(
      seq(optional(/[rRbBnNeExX]|[rR][bB]|[bB][rR]/), /'([^'\\]|\\(.|\n)|'')*'/),
      seq(optional(/[rRbB]|[rR][bB]|[bB][rR]/), /'''([^'\\]|\\(.|\n)|'[^']|''[^'])*'''/),
      seq(optional(/[rRbB]|[rR][bB]|[bB][rR]/), prec(1, /"""([^"\\]|\\(.|\n)|"[^"]|""[^"])*"""/)),
      /\$\$([^$]|\$[^$])*\$\$/,
    )),

    comment: _ => token(choice(
      seq('--', /[^\n]*/),
      seq('//', /[^\n]*/),
      seq('#', /[^\n]*/),
      seq('/*', /[^*]*\*+([^/*][^*]*\*+)*/, '/'),
    )),
  },
});
