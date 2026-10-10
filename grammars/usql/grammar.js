/**
 * @file The grammar of the input of usql
 * @license MIT
 */

// The input of usql holds SQL statements, meta commands such as \d and \g,
// and variables such as :name (D13). The grammar is one language for every
// SQL dialect (D108). The options of the dialect change only the external
// scanner, which reads them from USQL_OPTIONS in src/scanner.c. The scanner
// finds the end of a statement, the strings, the dollar quotes, the
// comments, the depth of the parentheses and the end of the arguments of a
// meta command, so the rules here stay small (D102).
//
// Each SQL statement is one node, and queries/injections.scm injects it
// with the language sql. A meta command is one node, meta_command, with the
// field name. Its arguments are its children. The commands \if, \elif,
// \else and \endif are meta commands like the others, and the tree does not
// nest the text between them.

/// <reference types="tree-sitter-cli/dsl" />
// @ts-check

module.exports = grammar({
  name: 'usql',

  // The scanner skips the white space, because it decides what a new line
  // ends. The comments come from the scanner too.
  extras: $ => [/\s/, $.comment],

  // The order of the external tokens is the order of enum TokenType in
  // src/scanner.c.
  externals: $ => [
    $._sql_text,
    $.string,
    $.quoted_identifier,
    $.dollar_string,
    $.comment,
    $._semicolon,
    $._backslash,
    $._backslash_options,
    $._command_base,
    $._shell_command_name,
    $._separator_command_name,
    $.modifier,
    $._command_end,
    $.word,
    $._pipe,
    $.shell_command,
    $._backtick_open,
    $._backtick_text,
    $._backtick_close,
    $._option_open,
    $._option_close,
    $._option_equals,
    $._option_word,
    $._sigil_plain,
    $._sigil_single_quote,
    $._sigil_double_quote,
    $._sigil_brace,
    $.variable_name,
    $._close_single_quote,
    $._close_double_quote,
    $._close_brace,
    $._variable_check,
    $._error_sentinel,
  ],

  rules: {
    source_file: $ => repeat(choice($.statement, $.meta_command)),

    // A statement ends at a semicolon outside parentheses, at a meta
    // command or at the end of the input.
    statement: $ => choice(
      prec.right(seq(repeat1($._piece), optional(alias($._semicolon, ';')))),
      alias($._semicolon, ';'),
    ),

    // The scanner returns _variable_check, a token with no characters, before
    // :' and :". Then it knows whether a variable or text and a string come
    // next.
    _piece: $ => choice(
      $._sql_text,
      $.string,
      $.quoted_identifier,
      $.dollar_string,
      $.variable,
      $._variable_check,
    ),

    meta_command: $ => choice(
      seq(
        field('name', alias($._command_name, $.command_name)),
        repeat($.modifier),
        repeat($._argument),
        $._command_end,
      ),
      seq(
        field('name', alias($._options_command_name, $.command_name)),
        repeat($.modifier),
        repeat(choice($._argument, $.option_list)),
        $._command_end,
      ),
      seq(
        field('name', alias($._shell_command_name, $.command_name)),
        optional($.shell_command),
        $._command_end,
      ),
      field('name', alias($._separator_command_name, $.command_name)),
    ),

    // A backslash with no name after it is a meta command with an empty
    // name.
    _command_name: $ => seq($._backslash, optional($._command_base)),

    _options_command_name: $ => seq($._backslash_options, $._command_base),

    _argument: $ => choice(
      $.word,
      $.string,
      $.quoted_identifier,
      $.variable,
      $.backtick_command,
      $.pipe,
      $._variable_check,
    ),

    pipe: $ => seq(alias($._pipe, '|'), optional($.shell_command)),

    // A backtick with no end runs to the end of the line.
    backtick_command: $ => seq(
      alias($._backtick_open, '`'),
      optional(alias($._backtick_text, $.shell_command)),
      optional(alias($._backtick_close, '`')),
    ),

    option_list: $ => seq(
      alias($._option_open, '('),
      repeat($.option),
      alias($._option_close, ')'),
    ),

    option: $ => seq(
      field('key', alias($._option_word, $.word)),
      optional(seq(
        alias($._option_equals, '='),
        optional($._variable_check),
        field('value', choice(
          alias($._option_word, $.word),
          $.string,
          $.quoted_identifier,
          $.variable,
        )),
      )),
    ),

    variable: $ => choice(
      seq(alias($._sigil_plain, ':'), field('name', $.variable_name)),
      seq(
        alias($._sigil_single_quote, ':\''),
        field('name', $.variable_name),
        alias($._close_single_quote, '\''),
      ),
      seq(
        alias($._sigil_double_quote, ':"'),
        field('name', $.variable_name),
        alias($._close_double_quote, '"'),
      ),
      seq(
        alias($._sigil_brace, ':{?'),
        field('name', $.variable_name),
        alias($._close_brace, '}'),
      ),
    ),
  },
});
