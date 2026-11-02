/**
 * @file The grammar of the SQL of Trino
 * @license MIT
 */

// xo writes this grammar from the SQL documentation of the current release
// of Trino, at https://trino.io/docs/current/sql.html (D127). It does not
// copy the ANTLR grammar of Trino. The grammar also serves the SQL of
// Presto and of Athena, which usql maps to it, but it does not read HiveQL
// or the syntax that only Presto has.
//
// The grammar is for a highlighter and for completion, so it accepts more
// than Trino does where that keeps the rules small. A keyword is a token in
// any case, and it is an anonymous node with the name of the keyword in
// upper case, such as "SELECT".
//
// The token identifier is the word of the grammar, so a keyword that Trino
// does not reserve is an identifier in each place where the grammar does
// not expect the keyword. Where a keyword and a name can both come, the
// keyword wins, so LIMIT after a table starts the clause LIMIT. The
// reserved words of Trino are in the reserved set global, so they are not
// identifiers, but for a label after AS or after a dot, where the grammar
// accepts every keyword. The words that name a function with a syntax of
// its own, such as TRIM, EXTRACT, NORMALIZE and LISTAGG, are not keywords,
// so their calls are function calls, and the keywords between their
// arguments, such as FROM, are accepted between the arguments of any
// function.
//
// The grammar keeps its parse tables small (D122, D127). A parse state with
// more than 64 columns takes the large form, so the grammar keeps the
// columns of a state few. The operators of one level of precedence are one
// token, TRUE and FALSE are one token, the words such as CURRENT_DATE are
// one token, the names of the functions of JSON are one token, and NULL is
// a token of its own. The clauses of a query after
// FROM, the clauses after the body of a query, the clauses after the
// arguments of a function and the clauses of a window come in any order,
// because a sequence of optional clauses costs parse states for each set of
// the clauses.
//
// The grammar needs no external scanner. A comment is -- to the end of the
// line, or /* ... */, which does not nest. A statement ends at a semicolon
// or at the end of the input. The variables of usql belong to the usql
// grammar, which hands this grammar a placeholder of the same length in
// place of each variable (D101).

/// <reference types="tree-sitter-cli/dsl" />
// @ts-check

// The precedences of the operators, from the lowest to the highest, as the
// grammar of Trino orders them. The arrow of a lambda binds the most
// loosely. The predicate MATCH binds more loosely than OR in the grammar,
// so that MATCH after the condition of WHERE in NEAREST starts the clause
// MATCH. NOT binds more loosely than a comparison, and the comparisons, IS,
// BETWEEN, IN and LIKE bind at one level. || binds more loosely than + and
// -, a sign binds more tightly than * and /, and AT TIME ZONE more tightly
// than a sign. A subscript and a dot bind the most tightly.
const PREC = {
  lambda: 1,
  match: 2,
  or: 3,
  and: 4,
  not: 5,
  compare: 6,
  concat: 7,
  add: 8,
  multiply: 9,
  unary: 10,
  at: 11,
  postfix: 12,
  field: 13,
};

// The words that Trino reserves, from the page Keywords and identifiers of
// the documentation. A word is in the reserved set only when the grammar
// uses it as a keyword, because a reserved word must be a token of the
// grammar.
const RESERVED_WORDS = [
  'ALTER', 'AND', 'AS', 'AUTO', 'BETWEEN', 'BY', 'CASE', 'CAST', 'CONSTRAINT',
  'CREATE', 'CROSS', 'CUBE', 'CURRENT_CATALOG', 'CURRENT_DATE',
  'CURRENT_PATH', 'CURRENT_ROLE', 'CURRENT_SCHEMA', 'CURRENT_TIME',
  'CURRENT_TIMESTAMP', 'CURRENT_USER', 'DEALLOCATE', 'DELETE', 'DESCRIBE',
  'DISTINCT', 'DROP', 'ELSE', 'END', 'ESCAPE', 'EXCEPT', 'EXISTS', 'EXTRACT',
  'FALSE', 'FOR', 'FROM', 'FULL', 'GROUP', 'GROUPING', 'HAVING', 'IN',
  'INNER', 'INSERT', 'INTERSECT', 'INTO', 'IS', 'JOIN', 'JSON_ARRAY',
  'JSON_EXISTS', 'JSON_OBJECT', 'JSON_QUERY', 'JSON_SERIALIZE', 'JSON_TABLE',
  'JSON_VALUE', 'LEFT', 'LIKE', 'LISTAGG', 'LOCALTIME', 'LOCALTIMESTAMP',
  'NATURAL', 'NORMALIZE', 'NOT', 'NULL', 'ON', 'OR', 'ORDER', 'OUTER',
  'OVERLAPS', 'PREPARE', 'RECURSIVE', 'RIGHT', 'ROLLUP', 'SELECT', 'SKIP',
  'TABLE', 'THEN', 'TRIM', 'TRUE', 'UESCAPE', 'UNION', 'UNNEST', 'USING',
  'VALUES', 'WHEN', 'WHERE', 'WITH',
];

// The reserved words that name a function, which the set function_name
// leaves out, so that a function call can have such a name.
const FUNCTION_WORDS = ['GROUPING'];

// The words of one token each, such as CURRENT_DATE, which take one column
// of each state where an expression starts.
const CURRENT_WORDS = [
  'CURRENT_DATE', 'CURRENT_TIME', 'CURRENT_TIMESTAMP', 'LOCALTIME',
  'LOCALTIMESTAMP', 'CURRENT_USER', 'CURRENT_CATALOG', 'CURRENT_SCHEMA',
  'CURRENT_PATH',
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
  name: 'trino',

  extras: $ => [
    /\s/,
    $.comment,
  ],

  word: $ => $.identifier,

  // The reserved set global holds the words that Trino reserves. The set
  // function_name leaves out the words that name a function, and the set
  // label is empty, because the grammar accepts every keyword as a label
  // after AS and after a dot. A state takes the last set of its items, so
  // the sets go from the strictest to the least strict.
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
    $._name,
    $._label,
    $._literal,
    $._function_name,
    $._qualifier,
    $._static_type,
    $._primary_expression,
    $._operand,
    $._table_primary,
  ],

  conflicts: $ => [
    // WITH after the type of a column starts WITH TIME ZONE, or the
    // properties of the column, as in a time WITH (k = 1). The parser tries
    // both, and the word after WITH tells them apart.
    [$.type],
    // A parenthesized query can be the left side of a set operation, or a
    // subquery in an expression or in FROM.
    [$.parenthesized_query, $.subquery],
    // The ORDER BY of a table argument takes its items in parentheses or
    // not, and the items in parentheses can be a row value.
    [$.order_by_item, $.row_constructor],
    [$.order_by_item, $.parenthesized_expression],
    // AS after the fields of a row value, as in (r).*, gives the fields new
    // names, or it gives the expression an alias.
    [$.aliased_expression, $._expression],
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
      $.create_table_statement,
      $.create_view_statement,
      $.create_materialized_view_statement,
      $.create_schema_statement,
      $.create_catalog_statement,
      $.create_role_statement,
      $.create_branch_statement,
      $.create_function_statement,
      $.alter_table_statement,
      $.alter_view_statement,
      $.alter_materialized_view_statement,
      $.alter_schema_statement,
      $.alter_branch_statement,
      $.drop_statement,
      $.comment_statement,
      $.refresh_statement,
      $.analyze_statement,
      $.grant_statement,
      $.revoke_statement,
      $.deny_statement,
      $.set_statement,
      $.reset_statement,
      $.use_statement,
      $.prepare_statement,
      $.execute_statement,
      $.deallocate_statement,
      $.describe_statement,
      $.show_statement,
      $.explain_statement,
      $.call_statement,
      $.start_transaction_statement,
      $.commit_statement,
      $.rollback_statement,
    ),

    // Changes of data

    // INSERT adds the rows of a query to a table.
    insert_statement: $ => seq(
      kws('INSERT', 'INTO'),
      $._target_table,
      optional($.column_list),
      $.select_statement,
    ),

    // The table that a statement changes. @ names a branch of the table, as
    // the connector of Iceberg has.
    _target_table: $ => seq(
      field('table', $.object_reference),
      optional(seq('@', field('branch', $._name))),
    ),

    update_statement: $ => seq(
      kw('UPDATE'),
      $._target_table,
      $._set_clause,
      optional($.where_clause),
    ),

    _set_clause: $ => seq(kw('SET'), commaSep1($.assignment)),

    assignment: $ => seq(
      field('column', $._name),
      '=',
      field('value', $._expression),
    ),

    delete_statement: $ => seq(
      kws('DELETE', 'FROM'),
      $._target_table,
      optional($.where_clause),
    ),

    // MERGE INTO joins a table with a source, and changes, inserts or
    // deletes rows of the table for each row of the source, as each WHEN
    // says.
    merge_statement: $ => seq(
      kws('MERGE', 'INTO'),
      $._target_table,
      optional($._alias),
      kw('USING'),
      field('source', $._table_factor),
      kw('ON'),
      field('condition', $._expression),
      repeat1($.merge_when_clause),
    ),

    // WHEN MATCHED is for a row of the source that matches a row of the
    // table, and WHEN NOT MATCHED for a row of the source with no match.
    // The values of INSERT are a row value in parentheses, which is an
    // expression, so that they take the parse states of an expression. A
    // list of expressions of its own cost about 40 states.
    merge_when_clause: $ => seq(
      kw('WHEN'),
      optional(kw('NOT')),
      kw('MATCHED'),
      optional(seq(kw('AND'), field('condition', $._expression))),
      kw('THEN'),
      choice(
        seq(kw('UPDATE'), $._set_clause),
        kw('DELETE'),
        seq(
          kw('INSERT'),
          optional($.column_list),
          kw('VALUES'),
          field('values', $._expression),
        ),
      ),
    ),

    // TRUNCATE deletes each row of a table.
    truncate_statement: $ => seq(
      kws('TRUNCATE', 'TABLE'),
      field('table', $.object_reference),
    ),

    // Changes of the schema

    _create: _ => seq(kw('CREATE'), optional(kws('OR', 'REPLACE'))),

    _if_not_exists: _ => kws('IF', 'NOT', 'EXISTS'),

    _if_exists: _ => kws('IF', 'EXISTS'),

    // The name of the object that CREATE makes, with IF NOT EXISTS. It is a
    // hidden rule of its own, so that the parts after the name have one set
    // of parse states.
    _create_name: $ => seq(
      optional($._if_not_exists),
      field('name', $.object_reference),
    ),

    _comment: $ => seq(kw('COMMENT'), field('comment', $.string)),

    _with_properties: $ => seq(kw('WITH'), $.property_list),

    // CREATE TABLE gives the columns of a table and the tables whose columns
    // it copies, or it takes the columns from a query, with the names of the
    // columns before AS. The clauses after the columns come in any order.
    create_table_statement: $ => seq(
      $._create,
      kw('TABLE'),
      $._create_name,
      optional(choice(
        parens(commaSep1(choice($.column_definition, $.like_clause))),
        $.column_list,
      )),
      repeat(choice($._comment, $._with_properties)),
      optional(seq(kw('AS'), $.select_statement)),
    ),

    // A column of CREATE TABLE and of ALTER TABLE ADD COLUMN. A column of
    // ALTER TABLE can be a field of a row column, with the names of the row
    // columns before it, as in r.f INTEGER. DEFAULT ends its value before
    // an operator, so that NOT NULL after the value is a part of the column.
    // A value with an operator must be in parentheses.
    column_definition: $ => seq(
      repeat(seq(field('parent', $._name), '.')),
      field('name', $._name),
      field('type', $.type),
      repeat(choice(
        prec(PREC.field + 1, seq(kw('DEFAULT'), field('default', $._expression))),
        seq(kw('NOT'), $.null),
        $._comment,
        $._with_properties,
      )),
    ),

    // LIKE copies the columns of a table, and INCLUDING PROPERTIES its
    // properties too.
    like_clause: $ => seq(
      kw('LIKE'),
      field('table', $.object_reference),
      optional(seq(choice(kw('INCLUDING'), kw('EXCLUDING')), kw('PROPERTIES'))),
    ),

    // CREATE VIEW. SECURITY DEFINER runs the query of the view as the owner
    // of the view, and SECURITY INVOKER as the user who reads it.
    create_view_statement: $ => seq(
      $._create,
      kw('VIEW'),
      $._create_name,
      repeat(choice(
        $._comment,
        seq(kw('SECURITY'), choice(kw('DEFINER'), kw('INVOKER'))),
        $._with_properties,
      )),
      kw('AS'),
      $.select_statement,
    ),

    // CREATE MATERIALIZED VIEW. GRACE PERIOD says how long the data can be
    // stale, and WHEN STALE what a query does after that time.
    create_materialized_view_statement: $ => seq(
      $._create,
      kws('MATERIALIZED', 'VIEW'),
      $._create_name,
      repeat(choice(
        seq(kws('GRACE', 'PERIOD'), field('grace_period', $.interval_literal)),
        seq(kws('WHEN', 'STALE'), choice(kw('INLINE'), kw('FAIL'))),
        $._comment,
        $._with_properties,
      )),
      kw('AS'),
      $.select_statement,
    ),

    create_schema_statement: $ => seq(
      $._create,
      kw('SCHEMA'),
      optional($._if_not_exists),
      field('name', $.schema_reference),
      repeat(choice($._authorization, $._with_properties)),
    ),

    _authorization: $ => seq(kw('AUTHORIZATION'), $.principal),

    // CREATE CATALOG adds a catalog with the connector after USING.
    create_catalog_statement: $ => seq(
      $._create,
      kw('CATALOG'),
      optional($._if_not_exists),
      field('name', $._name),
      kw('USING'),
      field('connector', $._name),
      repeat(choice($._comment, $._authorization, $._with_properties)),
    ),

    create_role_statement: $ => seq(
      $._create,
      kw('ROLE'),
      field('name', $._name),
      repeat(choice(
        seq(kws('WITH', 'ADMIN'), $._grantor),
        $._in_catalog,
      )),
    ),

    _in_catalog: $ => seq(kw('IN'), field('catalog', $._name)),

    // CREATE BRANCH makes a branch of a table, from another branch or not.
    create_branch_statement: $ => seq(
      $._create,
      kw('BRANCH'),
      optional($._if_not_exists),
      field('name', $._name),
      optional($._with_properties),
      $._in_table,
      optional(seq(kw('FROM'), field('source', $._name))),
    ),

    _in_table: $ => seq(kws('IN', 'TABLE'), field('table', $.object_reference)),

    // CREATE FUNCTION stores a SQL routine in a catalog.
    create_function_statement: $ => seq(
      $._create,
      $.function_definition,
    ),

    // ALTER TABLE takes one change.
    alter_table_statement: $ => seq(
      kws('ALTER', 'TABLE'),
      $._alter_name,
      choice(
        $._rename_to,
        seq(
          kws('ADD', 'COLUMN'),
          optional($._if_not_exists),
          $.column_definition,
          optional(choice(kw('FIRST'), kw('LAST'), seq(kw('AFTER'), field('after', $._name)))),
        ),
        seq(
          kws('DROP', 'COLUMN'),
          optional($._if_exists),
          field('column', $.column_reference),
        ),
        seq(
          kws('RENAME', 'COLUMN'),
          optional($._if_exists),
          field('column', $.column_reference),
          kw('TO'),
          field('new_name', $._name),
        ),
        seq(
          kws('ALTER', 'COLUMN'),
          field('column', $.column_reference),
          choice(
            seq(kws('SET', 'DEFAULT'), field('default', $._expression)),
            kws('DROP', 'DEFAULT'),
            seq(kws('SET', 'DATA', 'TYPE'), field('type', $.type)),
            seq(kws('DROP', 'NOT'), $.null),
          ),
        ),
        $._set_authorization,
        $._set_properties,
        $._execute_procedure,
      ),
    ),

    _alter_name: $ => seq(
      optional($._if_exists),
      field('name', $.object_reference),
    ),

    _rename_to: $ => seq(kws('RENAME', 'TO'), field('new_name', $.object_reference)),

    _set_authorization: $ => seq(kws('SET', 'AUTHORIZATION'), $.principal),

    _set_properties: $ => seq(kws('SET', 'PROPERTIES'), commaSep1($.property)),

    // EXECUTE runs a procedure of the connector on the table, such as
    // optimize, with its arguments and a WHERE.
    _execute_procedure: $ => seq(
      kw('EXECUTE'),
      field('procedure', $._name),
      optional($.arguments),
      optional($.where_clause),
    ),

    // ALTER VIEW REFRESH updates the columns of a view from its query.
    alter_view_statement: $ => seq(
      kws('ALTER', 'VIEW'),
      field('name', $.object_reference),
      choice($._rename_to, kw('REFRESH'), $._set_authorization),
    ),

    alter_materialized_view_statement: $ => seq(
      kws('ALTER', 'MATERIALIZED', 'VIEW'),
      $._alter_name,
      choice(
        $._rename_to,
        $._set_properties,
        $._set_authorization,
        $._execute_procedure,
      ),
    ),

    alter_schema_statement: $ => seq(
      kws('ALTER', 'SCHEMA'),
      field('name', $.schema_reference),
      choice(
        seq(kws('RENAME', 'TO'), field('new_name', $._name)),
        $._set_authorization,
      ),
    ),

    // ALTER BRANCH FAST FORWARD moves a branch to the head of another
    // branch.
    alter_branch_statement: $ => seq(
      kws('ALTER', 'BRANCH'),
      field('name', $._name),
      $._in_table,
      kws('FAST', 'FORWARD', 'TO'),
      field('target', $._name),
    ),

    // DROP deletes an object. CASCADE also deletes the objects of a schema
    // or a catalog. DROP FUNCTION names the types of the parameters, because
    // two functions can have the same name.
    drop_statement: $ => seq(
      kw('DROP'),
      choice(
        seq(
          choice(kw('TABLE'), kw('VIEW'), kws('MATERIALIZED', 'VIEW')),
          $._alter_name,
        ),
        seq(
          kw('SCHEMA'),
          optional($._if_exists),
          field('name', $.schema_reference),
          optional($._drop_behavior),
        ),
        seq(
          kw('CATALOG'),
          optional($._if_exists),
          field('name', $._name),
          optional($._drop_behavior),
        ),
        seq(
          kw('ROLE'),
          optional($._if_exists),
          field('name', $._name),
          optional($._in_catalog),
        ),
        seq(
          kw('BRANCH'),
          optional($._if_exists),
          field('name', $._name),
          $._in_table,
        ),
        seq(
          kw('FUNCTION'),
          $._alter_name,
          $.parameter_list,
        ),
      ),
    ),

    _drop_behavior: _ => choice(kw('CASCADE'), kw('RESTRICT')),

    // COMMENT ON gives the comment of a table, a view or a column. NULL
    // deletes the comment.
    comment_statement: $ => seq(
      kws('COMMENT', 'ON'),
      choice(
        seq(
          choice(kw('TABLE'), kw('VIEW'), kws('MATERIALIZED', 'VIEW')),
          field('name', $.object_reference),
        ),
        seq(kw('COLUMN'), field('name', $.column_reference)),
      ),
      kw('IS'),
      field('comment', choice($.string, $.null)),
    ),

    // REFRESH MATERIALIZED VIEW runs the query of a materialized view again.
    refresh_statement: $ => seq(
      kws('REFRESH', 'MATERIALIZED', 'VIEW'),
      field('name', $.object_reference),
    ),

    // ANALYZE collects the statistics of a table.
    analyze_statement: $ => seq(
      kw('ANALYZE'),
      field('table', $.object_reference),
      optional($._with_properties),
    ),

    // Access control

    // GRANT gives privileges on an object, or roles, to users and roles.
    // The grammar takes both forms with one rule, and the clauses after the
    // users in any order.
    grant_statement: $ => seq(
      kw('GRANT'),
      $._privileges,
      optional($._grant_object),
      kw('TO'),
      commaSep1($.principal),
      repeat(choice(
        seq(kw('WITH'), choice(kw('GRANT'), kw('ADMIN')), kw('OPTION')),
        $._granted_by,
        $._in_catalog,
      )),
    ),

    revoke_statement: $ => seq(
      kw('REVOKE'),
      optional(seq(choice(kw('GRANT'), kw('ADMIN')), kws('OPTION', 'FOR'))),
      $._privileges,
      optional($._grant_object),
      kw('FROM'),
      commaSep1($.principal),
      repeat(choice($._granted_by, $._in_catalog)),
    ),

    deny_statement: $ => seq(
      kw('DENY'),
      $._privileges,
      $._grant_object,
      kw('TO'),
      $.principal,
    ),

    _privileges: $ => choice(
      commaSep1($.privilege),
      kws('ALL', 'PRIVILEGES'),
    ),

    // A privilege, or a role. ADMIN is also the name of a role, as in
    // REVOKE admin FROM u.
    privilege: $ => choice(
      kw('SELECT'),
      kw('INSERT'),
      kw('UPDATE'),
      kw('DELETE'),
      seq(kw('CREATE'), optional(kw('BRANCH'))),
      $._name,
      alias(keyword('ADMIN'), $.identifier),
    ),

    // The object of a privilege: a table, a schema, or a branch of a table.
    _grant_object: $ => seq(
      kw('ON'),
      optional(seq(kw('BRANCH'), field('branch', $._name), kw('IN'))),
      choice(
        seq(optional(kw('TABLE')), field('object', $.object_reference)),
        seq(kw('SCHEMA'), field('object', $.schema_reference)),
      ),
    ),

    // A user or a role. A name with no USER or ROLE is either.
    principal: $ => seq(
      optional(choice(kw('USER'), kw('ROLE'))),
      field('name', $._name),
    ),

    _granted_by: $ => seq(kws('GRANTED', 'BY'), $._grantor),

    _grantor: $ => choice(
      $.principal,
      $.current_value,
      kw('CURRENT_ROLE'),
    ),

    // The session

    // SET changes the role, a session property, the authorization, the path
    // of the functions or the time zone of the session.
    set_statement: $ => seq(
      kw('SET'),
      choice(
        seq(
          kw('ROLE'),
          choice(kw('ALL'), kw('NONE'), field('role', $._name)),
          optional($._in_catalog),
        ),
        seq(kws('SESSION', 'AUTHORIZATION'), field('user', choice($._name, $.string))),
        seq(
          kw('SESSION'),
          field('name', $.object_reference),
          '=',
          field('value', $._expression),
        ),
        seq(kw('PATH'), commaSep1(field('path', $.schema_reference))),
        seq(kws('TIME', 'ZONE'), choice(kw('LOCAL'), field('value', $._expression))),
      ),
    ),

    reset_statement: $ => seq(
      kws('RESET', 'SESSION'),
      choice(kw('AUTHORIZATION'), field('name', $.object_reference)),
    ),

    // USE sets the schema of the session, with its catalog.
    use_statement: $ => seq(kw('USE'), field('schema', $.schema_reference)),

    // A schema, with its catalog.
    schema_reference: $ => choice(
      field('schema', $._name),
      seq(field('catalog', $._name), '.', field('schema', $._label)),
    ),

    // Prepared statements

    // PREPARE names a statement with parameters, EXECUTE runs it with the
    // values of the parameters after USING, and DEALLOCATE PREPARE deletes
    // it. EXECUTE IMMEDIATE runs the statement in a string.
    prepare_statement: $ => seq(
      kw('PREPARE'),
      field('name', $._name),
      kw('FROM'),
      $._statement,
    ),

    execute_statement: $ => seq(
      kw('EXECUTE'),
      choice(
        field('name', $._name),
        seq(kw('IMMEDIATE'), field('statement', $.string)),
      ),
      optional(seq(kw('USING'), commaSep1($._expression))),
    ),

    deallocate_statement: $ => seq(
      kws('DEALLOCATE', 'PREPARE'),
      field('name', $._name),
    ),

    // DESCRIBE gives the columns of a table, DESCRIBE INPUT the parameters
    // of a prepared statement, and DESCRIBE OUTPUT the columns of a
    // prepared statement or of a query.
    describe_statement: $ => seq(
      choice(kw('DESCRIBE'), kw('DESC')),
      choice(
        field('table', $.object_reference),
        seq(kw('INPUT'), field('name', $._name)),
        seq(kw('OUTPUT'), choice(field('name', $._name), parens($.select_statement))),
      ),
    ),

    // SHOW gives the catalogs, the schemas, the tables and the other objects,
    // with a pattern after LIKE, and the statement that creates an object.
    show_statement: $ => seq(
      kw('SHOW'),
      choice(
        seq(
          choice(kw('CATALOGS'), kw('SESSION')),
          optional($._like_pattern),
        ),
        seq(
          choice(kw('SCHEMAS'), kw('TABLES'), kw('FUNCTIONS')),
          optional(seq(choice(kw('FROM'), kw('IN')), field('schema', $.schema_reference))),
          optional($._like_pattern),
        ),
        seq(
          kw('COLUMNS'),
          choice(kw('FROM'), kw('IN')),
          field('table', $.object_reference),
          optional($._like_pattern),
        ),
        seq(
          kws('STATS', 'FOR'),
          choice(field('table', $.object_reference), parens($.select_statement)),
        ),
        seq(kw('GRANTS'), optional($._grant_object)),
        seq(
          choice(seq(optional(kw('CURRENT')), kw('ROLES')), kws('ROLE', 'GRANTS')),
          optional(seq(choice(kw('FROM'), kw('IN')), field('catalog', $._name))),
        ),
        seq(kw('BRANCHES'), choice(kw('FROM'), kw('IN')), kw('TABLE'), field('table', $.object_reference)),
        seq(
          kw('CREATE'),
          choice(
            seq(
              choice(kw('TABLE'), kw('VIEW'), kws('MATERIALIZED', 'VIEW'), kw('FUNCTION')),
              field('name', $.object_reference),
            ),
            seq(kw('SCHEMA'), field('name', $.schema_reference)),
          ),
        ),
      ),
    ),

    _like_pattern: $ => seq(
      kw('LIKE'),
      field('pattern', $.string),
      optional(seq(kw('ESCAPE'), field('escape', $.string))),
    ),

    // EXPLAIN gives the plan of a statement, with the options in
    // parentheses, and EXPLAIN ANALYZE runs the statement and gives the plan
    // with the costs of each step.
    explain_statement: $ => seq(
      kw('EXPLAIN'),
      optional($._explain_options),
      $._statement,
    ),

    // The options of EXPLAIN are a hidden rule of their own, so that the
    // states where the statement after the options starts are one set.
    _explain_options: $ => choice(
      seq(kw('ANALYZE'), optional(kw('VERBOSE'))),
      parens(commaSep1($.explain_option)),
    ),

    explain_option: _ => choice(
      seq(kw('FORMAT'), choice(kw('TEXT'), kw('GRAPHVIZ'), kw('JSON'))),
      seq(kw('TYPE'), choice(kw('LOGICAL'), kw('DISTRIBUTED'), kw('VALIDATE'), kw('IO'))),
    ),

    // CALL runs a procedure, with arguments by position or by name.
    call_statement: $ => seq(
      kw('CALL'),
      field('procedure', $.object_reference),
      $.arguments,
    ),

    // Transactions

    start_transaction_statement: $ => seq(
      kws('START', 'TRANSACTION'),
      optional(commaSep1($.transaction_mode)),
    ),

    transaction_mode: _ => choice(
      seq(
        kws('ISOLATION', 'LEVEL'),
        choice(
          kws('READ', 'UNCOMMITTED'),
          kws('READ', 'COMMITTED'),
          kws('REPEATABLE', 'READ'),
          kw('SERIALIZABLE'),
        ),
      ),
      seq(kw('READ'), choice(kw('ONLY'), kw('WRITE'))),
    ),

    commit_statement: _ => seq(kw('COMMIT'), optional(kw('WORK'))),

    rollback_statement: _ => seq(kw('ROLLBACK'), optional(kw('WORK'))),

    // Queries

    select_statement: $ => $._query,

    // WITH, WITH SESSION and WITH FUNCTION come before the body of a query,
    // and ORDER BY, OFFSET, LIMIT and FETCH after it, in any order.
    _query: $ => prec.right(seq(
      repeat($.with_clause),
      $._query_body,
      repeat(choice(
        $.order_by_clause,
        $.offset_clause,
        $.limit_clause,
        $.fetch_clause,
        $._with_data,
      )),
    )),

    // WITH DATA and WITH NO DATA come after the query of CREATE TABLE AS.
    // The grammar takes them after any query. After the query of CREATE
    // TABLE only, they cost about 730 parse states, because the states after
    // an expression where WITH can come do not merge with the states where
    // WITHIN can come. After any query, they cost about 440 states.
    _with_data: _ => seq(kw('WITH'), optional(kw('NO')), kw('DATA')),

    // WITH names the queries that the query reads, after RECURSIVE when
    // such a query reads itself. WITH FUNCTION defines the functions that
    // the query calls, and WITH SESSION sets session properties for the
    // query.
    with_clause: $ => seq(
      kw('WITH'),
      choice(
        seq(kw('SESSION'), commaSep1($.session_property)),
        seq(optional(kw('RECURSIVE')), commaSep1(choice($.cte, $.function_definition))),
      ),
    ),

    // The value of a session property is a literal, a typed literal, a
    // CAST, a name or a signed number. Any expression cost about 180 parse
    // states, because the body of the query can start with ( after it.
    session_property: $ => seq(
      field('name', $.object_reference),
      '=',
      field('value', choice($._literal, $.typed_literal, $.cast_expression, $._name, seq(alias($._sign, $.operator), $.number))),
    ),

    cte: $ => seq(
      field('name', $._name),
      optional($.column_list),
      kw('AS'),
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

    // INTERSECT binds more tightly than UNION and EXCEPT. CORRESPONDING
    // matches the columns of the two sides by their names.
    set_operation: $ => choice(
      prec.left(1, seq(
        field('left', $._query_body),
        field('operator', choice(kw('UNION'), kw('EXCEPT'))),
        optional($._set_quantifier),
        optional($.corresponding_clause),
        field('right', $._query_body),
      )),
      prec.left(2, seq(
        field('left', $._query_body),
        field('operator', kw('INTERSECT')),
        optional($._set_quantifier),
        optional($.corresponding_clause),
        field('right', $._query_body),
      )),
    ),

    _set_quantifier: _ => choice(kw('ALL'), kw('DISTINCT')),

    corresponding_clause: $ => seq(
      kw('CORRESPONDING'),
      optional(seq(kw('BY'), $.column_list)),
    ),

    // The clauses after SELECT come in any order.
    select: $ => prec.right(seq(
      kw('SELECT'),
      optional($._set_quantifier),
      commaSep1($._select_item),
      repeat(choice(
        $.from_clause,
        $.where_clause,
        $.group_by_clause,
        $.having_clause,
        $.window_clause,
      )),
    )),

    _select_item: $ => choice(
      $._expression,
      $.aliased_expression,
      $.star_expression,
    ),

    // An alias comes after AS or after the expression with no AS.
    aliased_expression: $ => choice(
      seq(field('value', $._expression), kw('AS'), field('alias', $._label)),
      seq(field('value', $._expression), field('alias', $._name)),
      seq(field('value', $.field_selection), kw('AS'), $.column_list),
    ),

    // * gives all the columns, and t.* all the columns of a table. AS
    // gives new names to the columns of t.*. The fields of a row value are
    // field_selection, as in (x).*, and AS after them is aliased_expression.
    star_expression: $ => prec.right(seq(
      optional(seq(
        optional(seq(
          optional(seq(field('catalog', $._name), '.')),
          field('schema', $._name),
          '.',
        )),
        field('table', $._name),
        '.',
      )),
      '*',
      optional(seq(kw('AS'), $.column_list)),
    )),

    // VALUES gives rows, in place of a SELECT. A row is an expression, and
    // a row of more than one column is a row value in parentheses.
    values: $ => prec.right(seq(kw('VALUES'), commaSep1($._expression))),

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
      $.match_recognize_table,
      $.pivot_table,
      $.sampled_table,
    ),

    _table_primary: $ => choice(
      $.table_reference,
      $.derived_table,
      $.unnest_table,
      $.function_table,
      $.json_table,
      $.nearest_table,
      $.parenthesized_join,
    ),

    // A table. FOR TIMESTAMP AS OF and FOR VERSION AS OF read an earlier
    // version of the table, as the connectors of Iceberg and Delta Lake do.
    // The version is a literal, a CAST, a value such as CURRENT_TIMESTAMP,
    // a parameter or an expression in parentheses. Any
    // expression cost about 260 parse states, because an alias can come
    // after it.
    table_reference: $ => prec.right(seq(
      field('table', $.object_reference),
      optional($.query_period),
      optional($._alias),
    )),

    query_period: $ => seq(
      kw('FOR'),
      choice(kw('TIMESTAMP'), kw('VERSION')),
      kws('AS', 'OF'),
      field('value', choice($._literal, $.typed_literal, $.cast_expression, $.current_value, $.parameter, $.parenthesized_expression)),
    ),

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

    // UNNEST turns arrays and maps into rows. WITH ORDINALITY adds the
    // number of each row.
    unnest_table: $ => prec.right(seq(
      kw('UNNEST'),
      parens(commaSep1($._expression)),
      optional(kws('WITH', 'ORDINALITY')),
      optional($._alias),
    )),

    // TABLE gives the rows of a table function.
    function_table: $ => prec.right(seq(
      kw('TABLE'),
      parens(field('function', $.table_function_call)),
      optional($._alias),
    )),

    table_function_call: $ => seq(
      optional($._qualifier),
      field('name', $._function_name),
      parens(optional(commaSep1($._table_function_argument))),
    ),

    _table_function_argument: $ => choice(
      $._table_function_value,
      $.named_table_argument,
    ),

    named_table_argument: $ => seq(
      field('name', $._name),
      '=>',
      field('value', $._table_function_value),
    ),

    _table_function_value: $ => choice(
      $._expression,
      $.table_argument,
      $.descriptor,
    ),

    // A table as the argument of a table function, with the way that the
    // function splits and orders its rows.
    table_argument: $ => prec.right(seq(
      kw('TABLE'),
      parens(choice(field('table', $.object_reference), $._query)),
      optional($._alias),
      repeat(choice(
        seq(kws('PARTITION', 'BY'), choice($.empty_grouping_set, $._expression)),
        seq(choice(kw('PRUNE'), kw('KEEP')), kws('WHEN', 'EMPTY')),
        seq(kws('ORDER', 'BY'), choice(
          parens(commaSep1($.order_by_item)),
          $.order_by_item,
        )),
      )),
    )),

    // DESCRIPTOR names columns, with their types or not.
    descriptor: $ => seq(
      kw('DESCRIPTOR'),
      parens(commaSep1($.descriptor_field)),
    ),

    descriptor_field: $ => seq(field('name', $._name), optional(field('type', $.type))),

    // JSON_TABLE turns a JSON value into rows, with the columns after
    // COLUMNS.
    json_table: $ => prec.right(seq(
      kw('JSON_TABLE'),
      parens(seq(
        $._json_argument,
        ',',
        $._json_argument,
        repeat($._json_clause),
        $.json_table_columns,
        repeat(choice(
          $.json_table_plan,
          $._json_clause,
        )),
      )),
      optional($._alias),
    )),

    json_table_columns: $ => seq(
      kw('COLUMNS'),
      parens(commaSep1($.json_table_column)),
    ),

    // A column of JSON_TABLE: the number of the row, a value with its type
    // and its path, or the columns of a nested path.
    json_table_column: $ => choice(
      seq(field('name', $._name), kws('FOR', 'ORDINALITY')),
      seq(
        field('name', $._name),
        field('type', $.type),
        repeat(choice(
          seq(kw('PATH'), field('path', $.string)),
          $._format_json,
          $.json_wrapper,
          $.json_quotes,
          $.json_behavior,
        )),
      ),
      seq(
        kw('NESTED'),
        optional(kw('PATH')),
        field('path', $.string),
        optional(seq(kw('AS'), field('name', $._label))),
        $.json_table_columns,
      ),
    ),

    // PLAN joins the paths of the nested columns. The grammar takes its
    // names and words in any order.
    json_table_plan: $ => seq(
      kw('PLAN'),
      optional(kw('DEFAULT')),
      parens($._plan_items),
    ),

    _plan_items: $ => repeat1(choice(
      $._name,
      kw('OUTER'),
      kw('INNER'),
      kw('UNION'),
      kw('CROSS'),
      ',',
      parens($._plan_items),
    )),

    // NEAREST picks the row of a relation that is the nearest to each row
    // of the left side of a join, as MATCH says.
    nearest_table: $ => prec.right(seq(
      kw('NEAREST'),
      parens(seq(
        $.from_clause,
        optional(alias($._nearest_where, $.where_clause)),
        kw('MATCH'),
        field('match', $._expression),
      )),
      optional($._alias),
    )),

    // The condition of WHERE in NEAREST has the precedence of the predicate
    // MATCH, so that MATCH after it starts the clause MATCH.
    _nearest_where: $ => prec.left(PREC.match, seq(kw('WHERE'), field('condition', $._expression))),

    // A list of tables and joins in parentheses.
    parenthesized_join: $ => prec.right(seq(
      parens($._table_reference),
      optional($._alias),
    )),

    // TABLESAMPLE takes a sample of the rows of a table, with the method
    // BERNOULLI or SYSTEM and the percent of the rows.
    sampled_table: $ => prec.left(seq(
      field('table', $._table_factor),
      kw('TABLESAMPLE'),
      field('method', choice(kw('BERNOULLI'), kw('SYSTEM'))),
      parens(field('percent', $._expression)),
    )),

    // A join. A regular join needs ON or USING after its right side, and
    // CROSS JOIN and NATURAL JOIN take neither. The words before JOIN are
    // keywords that Trino reserves.
    join: $ => prec.left(seq(
      field('left', $._table_reference),
      choice(
        seq(
          optional($._join_type),
          kw('JOIN'),
          field('right', $._table_factor),
          choice(
            seq(kw('ON'), field('condition', $._expression)),
            seq(kw('USING'), $.column_list),
          ),
        ),
        seq(
          choice(kw('CROSS'), seq(kw('NATURAL'), optional($._join_type))),
          kw('JOIN'),
          field('right', $._table_factor),
        ),
      ),
    )),

    _join_type: _ => choice(
      kw('INNER'),
      seq(choice(kw('LEFT'), kw('RIGHT'), kw('FULL')), optional(kw('OUTER'))),
    ),

    // MATCH_RECOGNIZE finds the sequences of rows that match a pattern.
    match_recognize_table: $ => prec.left(seq(
      field('table', $._table_factor),
      kw('MATCH_RECOGNIZE'),
      parens(repeat(choice(
        $.partition_by_clause,
        $.order_by_clause,
        $._pattern_clause,
        $.rows_per_match,
      ))),
      optional($._alias),
    )),

    // The clauses of a row pattern, which MATCH_RECOGNIZE and a window take.
    // The grammar takes them in any order.
    _pattern_clause: $ => choice(
      $.measures_clause,
      $.after_match_clause,
      kw('INITIAL'),
      kw('SEEK'),
      $.pattern_clause,
      $.subset_clause,
      $.define_clause,
    ),

    measures_clause: $ => seq(kw('MEASURES'), commaSep1($.measure_definition)),

    // RUNNING and FINAL say which rows a measure reads. The grammar takes
    // them at the start of a measure and of a condition of DEFINE.
    measure_definition: $ => seq(
      optional(choice(kw('RUNNING'), kw('FINAL'))),
      field('value', $._expression),
      kw('AS'),
      field('name', $._label),
    ),

    rows_per_match: $ => choice(
      kws('ONE', 'ROW', 'PER', 'MATCH'),
      seq(
        kws('ALL', 'ROWS', 'PER', 'MATCH'),
        optional(choice(
          seq(choice(kw('SHOW'), kw('OMIT')), kws('EMPTY', 'MATCHES')),
          kws('WITH', 'UNMATCHED', 'ROWS'),
        )),
      ),
    ),

    after_match_clause: $ => seq(
      kws('AFTER', 'MATCH', 'SKIP'),
      choice(
        kws('TO', 'NEXT', 'ROW'),
        kws('PAST', 'LAST', 'ROW'),
        seq(kw('TO'), optional(choice(kw('FIRST'), kw('LAST'))), field('variable', $._name)),
      ),
    ),

    pattern_clause: $ => seq(kw('PATTERN'), parens($.row_pattern)),

    // A row pattern: the variables, with quantifiers such as * and {2,},
    // in sequences and alternatives.
    row_pattern: $ => choice(
      $._pattern_term,
      $.pattern_alternation,
      $.pattern_concatenation,
    ),

    pattern_alternation: $ => prec.left(1, seq($.row_pattern, '|', $.row_pattern)),

    pattern_concatenation: $ => prec.left(2, seq($.row_pattern, $._pattern_term)),

    _pattern_term: $ => choice(
      $._pattern_primary,
      $.quantified_pattern,
    ),

    quantified_pattern: $ => seq(
      $._pattern_primary,
      field('quantifier', $.pattern_quantifier),
    ),

    _pattern_primary: $ => choice(
      alias($._name, $.pattern_variable),
      $.pattern_group,
      $.pattern_permutation,
      $.pattern_exclusion,
      $.pattern_anchor,
    ),

    pattern_group: $ => seq('(', optional($.row_pattern), ')'),

    pattern_permutation: $ => seq(kw('PERMUTE'), parens(commaSep1($.row_pattern))),

    pattern_exclusion: $ => seq('{-', $.row_pattern, '-}'),

    pattern_anchor: _ => choice('^', '$'),

    // A quantifier, with ? after it when it is reluctant.
    pattern_quantifier: _ => token(seq(
      choice('*', '+', '?', /\{[0-9]*(,[0-9]*)?\}/),
      optional('?'),
    )),

    subset_clause: $ => seq(kw('SUBSET'), commaSep1($.subset_definition)),

    subset_definition: $ => seq(
      field('name', $._name),
      '=',
      parens(commaSep1(field('variable', $._name))),
    ),

    define_clause: $ => seq(kw('DEFINE'), commaSep1($.variable_definition)),

    variable_definition: $ => seq(
      field('variable', $._name),
      kw('AS'),
      optional(choice(kw('RUNNING'), kw('FINAL'))),
      field('condition', $._expression),
    ),

    // PIVOT turns the values of the columns after FOR into columns, with
    // the aggregates before FOR.
    pivot_table: $ => prec.left(seq(
      field('table', $._table_factor),
      kw('PIVOT'),
      parens(seq(
        commaSep1(choice($._expression, $.aliased_expression)),
        kw('FOR'),
        field('columns', choice($.column_reference, $.column_list)),
        kw('IN'),
        parens(commaSep1(choice($._expression, $.aliased_expression))),
        optional($.group_by_clause),
      )),
      optional($._alias),
    )),

    where_clause: $ => seq(kw('WHERE'), field('condition', $._expression)),

    // GROUP BY AUTO groups by each column of SELECT that is not an
    // aggregate.
    group_by_clause: $ => seq(
      kws('GROUP', 'BY'),
      optional($._set_quantifier),
      commaSep1($._grouping_element),
    ),

    _grouping_element: $ => choice(
      $._expression,
      $.empty_grouping_set,
      kw('AUTO'),
      $.rollup,
      $.cube,
      $.grouping_sets,
    ),

    empty_grouping_set: _ => seq('(', ')'),

    rollup: $ => seq(kw('ROLLUP'), parens(optional(commaSep1($._grouping_set)))),

    cube: $ => seq(kw('CUBE'), parens(optional(commaSep1($._grouping_set)))),

    grouping_sets: $ => seq(
      kws('GROUPING', 'SETS'),
      parens(commaSep1($._grouping_set)),
    ),

    _grouping_set: $ => choice($._expression, $.empty_grouping_set),

    having_clause: $ => seq(kw('HAVING'), field('condition', $._expression)),

    window_clause: $ => seq(kw('WINDOW'), commaSep1($.window_definition)),

    window_definition: $ => seq(
      field('name', $._name),
      kw('AS'),
      $.window_specification,
    ),

    // A window can start with the name of a window of the WINDOW clause.
    // The grammar takes its clauses in any order, with the clauses of a row
    // pattern.
    window_specification: $ => parens(seq(
      optional(field('name', $._name)),
      repeat(choice(
        $.partition_by_clause,
        $.order_by_clause,
        $.frame_clause,
        $._pattern_clause,
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

    order_by_item: $ => seq(
      field('value', $._expression),
      optional(choice(kw('ASC'), kw('DESC'))),
      optional(seq(kw('NULLS'), choice(kw('FIRST'), kw('LAST')))),
    ),

    offset_clause: $ => seq(
      kw('OFFSET'),
      field('start', $._expression),
      optional(choice(kw('ROW'), kw('ROWS'))),
    ),

    limit_clause: $ => seq(
      kw('LIMIT'),
      choice(field('count', $._expression), kw('ALL')),
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

    // SQL routines

    // A function of WITH FUNCTION and of CREATE FUNCTION. Its body is a
    // statement of a SQL routine, or a string in $$ for another language.
    // The characteristics can come before RETURNS, as an example of the
    // documentation shows, so the grammar takes RETURNS and the
    // characteristics in any order.
    function_definition: $ => seq(
      kw('FUNCTION'),
      field('name', $.object_reference),
      $.parameter_list,
      repeat1(choice(
        seq(kw('RETURNS'), field('return_type', $.type)),
        $._routine_characteristic,
      )),
      choice(
        field('body', $._routine_statement),
        seq(kw('AS'), field('body', $.dollar_string)),
      ),
    ),

    parameter_list: $ => parens(optional(commaSep1($.parameter_declaration))),

    parameter_declaration: $ => seq(
      optional(field('name', $._name)),
      field('type', $.type),
    ),

    _routine_characteristic: $ => choice(
      seq(kw('LANGUAGE'), field('language', $._name)),
      seq(optional(kw('NOT')), kw('DETERMINISTIC')),
      seq(kw('RETURNS'), $.null, kw('ON'), $.null, kw('INPUT')),
      seq(kws('CALLED', 'ON'), $.null, kw('INPUT')),
      seq(kw('SECURITY'), choice(kw('DEFINER'), kw('INVOKER'))),
      $._comment,
      $._with_properties,
    ),

    // The properties of an object, as in WITH (format = 'ORC').
    property_list: $ => parens(commaSep1($.property)),

    property: $ => seq(
      field('name', $._name),
      '=',
      field('value', choice($._expression, kw('DEFAULT'))),
    ),

    // The statements of the body of a SQL routine.
    _routine_statement: $ => choice(
      $.return_statement,
      $.assignment_statement,
      $.compound_statement,
      $.if_statement,
      $.case_statement,
      $.loop_statement,
      $.while_statement,
      $.repeat_statement,
      $.iterate_statement,
      $.leave_statement,
    ),

    _routine_statements: $ => repeat1(seq($._routine_statement, ';')),

    return_statement: $ => seq(kw('RETURN'), field('value', $._expression)),

    assignment_statement: $ => seq(
      kw('SET'),
      field('variable', $._name),
      '=',
      field('value', $._expression),
    ),

    // BEGIN ... END holds the variables of DECLARE and the statements.
    compound_statement: $ => seq(
      kw('BEGIN'),
      repeat(seq($.declare_statement, ';')),
      optional($._routine_statements),
      kw('END'),
    ),

    declare_statement: $ => seq(
      kw('DECLARE'),
      commaSep1(field('variable', $._name)),
      field('type', $.type),
      optional(seq(kw('DEFAULT'), field('default', $._expression))),
    ),

    if_statement: $ => seq(
      kw('IF'),
      field('condition', $._expression),
      kw('THEN'),
      $._routine_statements,
      repeat($.elseif_clause),
      optional($.else_clause),
      kws('END', 'IF'),
    ),

    elseif_clause: $ => seq(
      kw('ELSEIF'),
      field('condition', $._expression),
      kw('THEN'),
      $._routine_statements,
    ),

    else_clause: $ => seq(kw('ELSE'), $._routine_statements),

    case_statement: $ => seq(
      kw('CASE'),
      optional(field('value', $._expression)),
      repeat1($.case_statement_when),
      optional($.else_clause),
      kws('END', 'CASE'),
    ),

    case_statement_when: $ => seq(
      kw('WHEN'),
      field('condition', $._expression),
      kw('THEN'),
      $._routine_statements,
    ),

    // A loop can have a label, which ITERATE and LEAVE name.
    _label_prefix: $ => seq(field('label', $._name), ':'),

    loop_statement: $ => seq(
      optional($._label_prefix),
      kw('LOOP'),
      $._routine_statements,
      kws('END', 'LOOP'),
    ),

    while_statement: $ => seq(
      optional($._label_prefix),
      kw('WHILE'),
      field('condition', $._expression),
      kw('DO'),
      $._routine_statements,
      kws('END', 'WHILE'),
    ),

    repeat_statement: $ => seq(
      optional($._label_prefix),
      kw('REPEAT'),
      $._routine_statements,
      kw('UNTIL'),
      field('condition', $._expression),
      kws('END', 'REPEAT'),
    ),

    iterate_statement: $ => seq(kw('ITERATE'), field('label', $._name)),

    leave_statement: $ => seq(kw('LEAVE'), field('label', $._name)),

    // Expressions

    _expression: $ => choice(
      $._primary_expression,
      $.unary_expression,
      $.binary_expression,
      $.is_expression,
      $.between_expression,
      $.in_expression,
      $.like_expression,
      $.match_expression,
      $.at_time_zone_expression,
      $.lambda_expression,
    ),

    _primary_expression: $ => choice(
      $._literal,
      $.typed_literal,
      $.interval_literal,
      $.interval_expression,
      $.current_value,
      $.column_reference,
      $.parameter,
      $.function_call,
      $.measure_reference,
      $.method_call,
      $.json_function,
      $.cast_expression,
      $.subscript_expression,
      $.field_selection,
      $.parenthesized_expression,
      $.row_constructor,
      $.array,
      $.subquery,
      $.exists_expression,
      $.case_expression,
    ),

    _literal: $ => choice(
      $.number,
      $.string,
      $.unicode_string,
      $.binary_string,
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
    // DATE '2024-03-01' or TIMESTAMP '2024-03-01 10:00 UTC'. The name is an
    // identifier, so date also names a column.
    typed_literal: $ => seq(
      field('type', alias($.identifier, $.type)),
      field('value', $.string),
    ),

    // An interval: INTERVAL, a sign, a string and the unit, as in INTERVAL
    // '2' DAY or INTERVAL '1-6' YEAR TO MONTH.
    interval_literal: $ => seq(
      kw('INTERVAL'),
      optional(alias($._sign, $.operator)),
      field('value', $.string),
      $.interval_qualifier,
    ),

    // An expression in parentheses and a unit give an interval, as in
    // (a - b) DAY TO SECOND.
    interval_expression: $ => seq(
      field('value', $.parenthesized_expression),
      $.interval_qualifier,
    ),

    // The unit of an interval, or two units with TO between them. A unit
    // can have its precision, as in DAY(3).
    interval_qualifier: $ => prec.right(seq(
      $._interval_field,
      optional(seq(kw('TO'), $._interval_field)),
    )),

    _interval_field: $ => prec.right(seq(
      choice(kw('YEAR'), kw('MONTH'), kw('DAY'), kw('HOUR'), kw('MINUTE'), kw('SECOND')),
      optional($.type_modifiers),
    )),

    // A column, with its table, its schema and its catalog. The grammar
    // cannot tell a field of a row from a column, so r.a is a column a of a
    // table r.
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

    // A parameter of a prepared statement.
    parameter: _ => '?',

    // A function call. The arguments of an aggregate function can have
    // DISTINCT and ORDER BY, and those of LISTAGG ON OVERFLOW. WITHIN
    // GROUP, FILTER, IGNORE NULLS and OVER come after the arguments. The
    // grammar takes them in any order.
    function_call: $ => prec.right(1, seq(
      optional(choice($._qualifier, $._static_type)),
      field('name', $._function_name),
      $.arguments,
      repeat(choice(
        $.within_group_clause,
        $.filter_clause,
        seq(choice(kw('IGNORE'), kw('RESPECT')), kw('NULLS')),
        $.over_clause,
      )),
    )),

    // A function of a type has :: between the type and its name, as in
    // varchar::chr(65) or ROW::fields(r). The type is not a rule of its
    // own, so that it takes no column of the states where an expression
    // starts.
    _static_type: $ => seq(
      field('type', choice(alias($.identifier, $.type), alias(kw('ROW'), $.type))),
      '::',
    ),

    // A function name can be a word that Trino reserves for its own
    // functions, such as grouping, or a word such as CURRENT_TIMESTAMP,
    // which takes a precision in parentheses.
    _function_name: $ => choice(
      reserved('function_name', $.identifier),
      alias($._quoted_identifier, $.identifier),
      alias($.current_value, $.identifier),
    ),

    arguments: $ => parens(optional(seq(
      optional($._set_quantifier),
      $._argument_list,
      optional($.order_by_clause),
      optional($.overflow_clause),
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

    // ROW(1 AS a) gives the field a of a row its name. Only ROW takes
    // such an argument, because AS after any argument of any function cost
    // about 370 parse states.
    aliased_argument: $ => seq(field('value', $._expression), kw('AS'), field('alias', $._label)),

    named_argument: $ => seq(
      field('name', $._name),
      '=>',
      field('value', $._expression),
    ),

    // ON OVERFLOW says what LISTAGG does with a result that is too long.
    overflow_clause: $ => seq(
      kws('ON', 'OVERFLOW'),
      choice(
        kw('ERROR'),
        seq(
          kw('TRUNCATE'),
          optional(field('filler', $.string)),
          choice(kw('WITH'), kw('WITHOUT')),
          kw('COUNT'),
        ),
      ),
    ),

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

    // A measure of the row pattern of a window, with the window after
    // OVER, as in m OVER w.
    measure_reference: $ => seq(field('measure', $._name), $.over_clause),

    // The dot after a value calls a method of the value, as in
    // 'Hello'.length(). A name before the dot gives a function call with a
    // schema, because the grammar cannot tell a column from a schema.
    method_call: $ => prec(PREC.field + 1, seq(
      field('value', choice(
        $.string,
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

    // The functions of JSON of the SQL standard, such as JSON_VALUE and
    // JSON_OBJECT. Their arguments are expressions with FORMAT JSON, a path
    // with its name after AS, or the members of an object, with KEY and
    // VALUE or with a colon. The clauses after the arguments, such as
    // PASSING, RETURNING and the behaviors ON EMPTY and ON ERROR, come in
    // any order.
    json_function: $ => seq(
      field('name', alias($._json_function_name, $.identifier)),
      parens(optional(seq(
        commaSep1(choice($._json_argument, $.json_member)),
        repeat($._json_clause),
      ))),
    ),

    // The names of the functions are one token, so that they take one
    // column of each state where an expression starts.
    _json_function_name: _ => token(choice(
      ...['JSON_EXISTS', 'JSON_VALUE', 'JSON_QUERY', 'JSON_OBJECT', 'JSON_ARRAY', 'JSON_SERIALIZE'].map(keyword),
    )),

    _json_argument: $ => choice(
      $._expression,
      $.json_format,
      $.json_path,
    ),

    json_format: $ => seq(
      field('value', $._expression),
      $._format_json,
    ),

    _format_json: $ => seq(
      kws('FORMAT', 'JSON'),
      optional(seq(kw('ENCODING'), field('encoding', $._name))),
    ),

    json_path: $ => seq(
      field('path', $.string),
      kw('AS'),
      field('name', $._label),
    ),

    json_member: $ => choice(
      seq(optional(kw('KEY')), field('key', $._expression), kw('VALUE'), field('value', $._json_argument)),
      seq(field('key', $._expression), ':', field('value', $._json_argument)),
    ),

    _json_clause: $ => choice(
      $.json_passing,
      $.json_returning,
      $.json_behavior,
      $.json_wrapper,
      $.json_quotes,
      $.json_unique_keys,
    ),

    json_passing: $ => prec.right(seq(
      kw('PASSING'),
      commaSep1(choice($.aliased_expression, $.json_format_alias)),
    )),

    json_format_alias: $ => seq(
      field('value', $._expression),
      $._format_json,
      kw('AS'),
      field('alias', $._label),
    ),

    json_returning: $ => prec.right(seq(
      kw('RETURNING'),
      field('type', $.type),
      optional($._format_json),
    )),

    // What a function does with a missing value, an error or NULL, as in
    // DEFAULT 0 ON EMPTY, EMPTY ARRAY ON ERROR or ABSENT ON NULL.
    json_behavior: $ => seq(
      choice(
        kw('ERROR'),
        $.null,
        $.boolean,
        kw('UNKNOWN'),
        kw('ABSENT'),
        seq(kw('EMPTY'), optional(choice(kw('ARRAY'), kw('OBJECT')))),
        seq(kw('DEFAULT'), field('default', $._expression)),
      ),
      kw('ON'),
      choice(kw('EMPTY'), kw('ERROR'), $.null),
    ),

    json_wrapper: _ => seq(
      choice(
        kw('WITHOUT'),
        seq(kw('WITH'), optional(choice(kw('CONDITIONAL'), kw('UNCONDITIONAL')))),
      ),
      optional(kw('ARRAY')),
      kw('WRAPPER'),
    ),

    json_quotes: _ => seq(
      choice(kw('KEEP'), kw('OMIT')),
      kw('QUOTES'),
      optional(kws('ON', 'SCALAR', 'STRING')),
    ),

    json_unique_keys: _ => seq(
      choice(kw('WITH'), kw('WITHOUT')),
      kw('UNIQUE'),
      optional(kw('KEYS')),
    ),

    // CAST(x AS type), and TRY_CAST(x AS type), which gives NULL in place
    // of an error.
    cast_expression: $ => seq(
      choice(kw('CAST'), kw('TRY_CAST')),
      parens(seq(field('value', $._expression), kw('AS'), field('type', $.type))),
    ),

    // An element of an array or a map, as in a[1] or m['key']. [*] gives
    // each element of a JSON array.
    subscript_expression: $ => prec(PREC.postfix, seq(
      field('value', $._primary_expression),
      '[',
      field('index', choice($._expression, '*')),
      ']',
    )),

    // A field of a row, such as (r).a, f(x).b or a[1].c. A column reference
    // with more than four parts is a field of a row, as in c.s.t.r.f. .*
    // gives each field.
    field_selection: $ => prec(PREC.field, seq(
      field('value', choice(
        $.column_reference,
        $.cast_expression,
        $.parenthesized_expression,
        $.row_constructor,
        $.subquery,
        $.subscript_expression,
        $.function_call,
        $.method_call,
        $.field_selection,
      )),
      '.',
      choice(field('field', $._label), '*'),
    )),

    parenthesized_expression: $ => parens($._expression),

    // A row value: a list of two or more expressions in parentheses, or ROW
    // and the fields, which can have names after AS.
    row_constructor: $ => choice(
      parens(seq($._expression, ',', commaSep1($._expression))),
      seq(kw('ROW'), parens(commaSep1(choice($._expression, $.aliased_argument)))),
    ),

    // An array, as in ARRAY[1, 2, 3] or [1, 2, 3].
    array: $ => seq(
      optional(kw('ARRAY')),
      '[',
      optional(commaSep1($._expression)),
      ']',
    ),

    // EXISTS and UNIQUE test the rows of a subquery.
    exists_expression: $ => seq(choice(kw('EXISTS'), kw('UNIQUE')), $.subquery),

    case_expression: $ => seq(
      kw('CASE'),
      optional(field('value', $._expression)),
      repeat1($.when_clause),
      optional(seq(kw('ELSE'), field('else', $._expression))),
      kw('END'),
    ),

    // A WHEN of the simple form can hold the right side of a predicate,
    // as in WHEN > 100 or WHEN IS NULL, whose left side is the value after
    // CASE.
    when_clause: $ => seq(
      kw('WHEN'),
      field('condition', choice($._expression, $.predicate_fragment)),
      kw('THEN'),
      field('result', $._expression),
    ),

    predicate_fragment: $ => choice(
      seq(alias($._comparison_operator, $.operator), $._operand),
      seq(
        optional(kw('NOT')),
        kw('BETWEEN'),
        optional(choice(kw('SYMMETRIC'), kw('ASYMMETRIC'))),
        field('low', $._expression),
        kw('AND'),
        field('high', $._expression),
      ),
      seq(optional(kw('NOT')), kw('IN'), field('right', $._expression)),
      seq(
        optional(kw('NOT')),
        kw('LIKE'),
        field('right', $._expression),
        optional(seq(kw('ESCAPE'), field('escape', $._expression))),
      ),
      seq(kw('IS'), optional(kw('NOT')), choice($.boolean, $.null, kw('UNKNOWN'), $.distinct_from)),
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
      prec.right(PREC.unary, seq(field('operator', alias($._sign, $.operator)), field('operand', $._expression))),
    ),

    _sign: _ => token(choice('-', '+')),

    binary_expression: $ => {
      /** @type {[number, RuleOrLiteral, RuleOrLiteral][]} */
      const table = [
        [PREC.or, kw('OR'), $._expression],
        [PREC.and, kw('AND'), $._expression],
        [PREC.compare, alias($._comparison_operator, $.operator), $._operand],
        [PREC.concat, alias('||', $.operator), $._expression],
        [PREC.add, alias($._sign, $.operator), $._expression],
        [PREC.multiply, alias($._multiplicative_operator, $.operator), $._expression],
      ];
      return choice(...table.map(([p, op, right]) => prec.left(p, seq(
        field('left', $._expression),
        field('operator', op),
        field('right', right),
      ))));
    },

    _comparison_operator: _ => token(choice('=', '<', '>', '<=', '>=', '<>', '!=')),

    _multiplicative_operator: _ => token(choice('*', '/', '%')),

    _operand: $ => choice($._expression, $.quantified_expression),

    // ANY, SOME or ALL after a comparison compares the left side with each
    // row of a subquery.
    quantified_expression: $ => seq(
      field('quantifier', choice(kw('ANY'), kw('SOME'), kw('ALL'))),
      $.subquery,
    ),

    // IS NULL, IS TRUE, IS UNKNOWN, IS DISTINCT FROM and the others.
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
    // or a subquery. position(a IN b) is a function call with such an
    // expression too.
    in_expression: $ => prec.left(PREC.compare, seq(
      field('left', $._expression),
      optional(kw('NOT')),
      kw('IN'),
      field('right', $._expression),
    )),

    like_expression: $ => prec.left(PREC.compare, seq(
      field('left', $._expression),
      optional(kw('NOT')),
      kw('LIKE'),
      field('right', $._expression),
      optional(seq(kw('ESCAPE'), field('escape', $._expression))),
    )),

    // MATCH tests whether a row value is a row of a subquery.
    match_expression: $ => prec.left(PREC.match, seq(
      field('left', $._expression),
      kw('MATCH'),
      optional(kw('UNIQUE')),
      optional(choice(kw('SIMPLE'), kw('PARTIAL'), kw('FULL'))),
      field('right', $.subquery),
    )),

    // AT TIME ZONE gives a time in a time zone, and AT LOCAL in the time
    // zone of the session.
    at_time_zone_expression: $ => prec.left(PREC.at, seq(
      field('value', $._expression),
      kw('AT'),
      choice(
        seq(kws('TIME', 'ZONE'), field('zone', choice($.string, $.interval_literal, $.column_reference))),
        kw('LOCAL'),
      ),
    )),

    // Types

    // A type: a name with its parameters in parentheses, as in
    // DECIMAL(10, 2), ARRAY(INTEGER), MAP(VARCHAR, BIGINT) or ROW(a INTEGER,
    // b VARCHAR), a time with WITH TIME ZONE, or an interval with its
    // units. WITH and WITHOUT after a type can also start the properties of
    // a column, a characteristic of a routine or the wrapper of JSON_QUERY,
    // so the parser tries both at run time, as the conflicts say.
    type: $ => choice(
      seq(
        optional($._qualifier),
        field('name', $._label),
        optional($.type_modifiers),
        optional($._time_zone),
      ),
      seq(kw('INTERVAL'), $.interval_qualifier),
    ),

    _time_zone: _ => seq(choice(kw('WITH'), kw('WITHOUT')), kws('TIME', 'ZONE')),

    // The parameters of a type: numbers, types, and fields, which have a
    // name and a type.
    type_modifiers: $ => parens(commaSep1(choice(
      $.number,
      $.type,
      $.field_definition,
    ))),

    field_definition: $ => seq(
      field('name', $._name),
      field('type', $.type),
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

    // An identifier starts with a letter or _, and holds letters, digits
    // and _. A name that starts with _ is also the placeholder of a
    // variable of usql (D101).
    identifier: _ => /[A-Za-z_][A-Za-z0-9_]*/,

    // A quoted identifier is in double quotes, with "" for a ".
    _quoted_identifier: _ => /"([^"]|"")*"/,

    // A number is an integer, a decimal number or a number with an
    // exponent. An underscore can separate two digits. An integer can be
    // hexadecimal, octal or binary, as in 0x1F, 0o17 and 0b101.
    number: _ => token(choice(
      /[0-9](_?[0-9])*(\.([0-9](_?[0-9])*)?)?([eE][+-]?[0-9]+)?/,
      /\.[0-9](_?[0-9])*([eE][+-]?[0-9]+)?/,
      /0[xX](_?[0-9a-fA-F])+/,
      /0[oO](_?[0-7])+/,
      /0[bB](_?[01])+/,
    )),

    // A string is in single quotes, with a quote doubled.
    string: _ => /'([^']|'')*'/,

    // A Unicode string, as in U&'caf\00E9', with UESCAPE and the escape
    // character after it, or not. The string and UESCAPE are one token.
    unicode_string: _ => token(seq(
      /[uU]&'([^']|'')*'/,
      optional(seq(/\s+/, keyword('UESCAPE'), /\s+/, /'([^']|'')*'/)),
    )),

    // A binary string, as in X'CAFE'.
    binary_string: _ => /[xX]'[^']*'/,

    // A string in $$, the body of a function in another language than SQL.
    dollar_string: _ => /\$\$([^$]|\$[^$])*\$\$/,

    comment: _ => token(choice(
      seq('--', /[^\n]*/),
      seq('/*', /[^*]*\*+([^/*][^*]*\*+)*/, '/'),
    )),
  },
});
