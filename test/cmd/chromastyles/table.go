package main

import "strings"

// captureTable gives the token type of chroma for a capture name or for a
// prefix of one, one name on each line. captureType takes the longest prefix
// of a name, by dots, that the table holds. The names follow the standard capture names of
// Neovim and Helix, the older names of Neovim and of upstream tree-sitter,
// and the names that the queries of the grammar set use. The types follow
// the token types that the lexers of Pygments give the same text.
const captureTable = `
annotation                 NameDecorator
array                      Text
assignvalue                Text
attribute                  NameDecorator
attribute.builtin          NameDecorator
boolean                    KeywordConstant
char                       LiteralStringChar
character                  LiteralStringChar
character.escape           LiteralStringEscape
character.special          LiteralStringEscape
class                      NameClass
clean                      Text
cmd                        Text
comment                    Comment
comment.block              CommentMultiline
comment.doc                CommentSpecial
comment.documentation      CommentSpecial
comment.error              CommentSpecial
comment.hint               CommentSpecial
comment.line               CommentSingle
comment.note               CommentSpecial
comment.todo               CommentSpecial
comment.warning            CommentSpecial
conceal                    Text
conditional                Keyword
constant                   NameConstant
constant.builtin           KeywordConstant
constant.character         LiteralStringChar
constant.character.escape  LiteralStringEscape
constant.language          KeywordConstant
constant.null              KeywordConstant
constant.numeric           LiteralNumber
constant.numeric.float     LiteralNumberFloat
constant.numeric.integer   LiteralNumberInteger
constructor                NameClass
constructor.builtin        NameBuiltin
custom_directive           CommentPreproc
decorator                  NameDecorator
delimiter                  Punctuation
diff                       Generic
diff.delta                 GenericSubheading
diff.minus                 GenericDeleted
diff.plus                  GenericInserted
embedded                   Text
enum                       NameClass
enumMember                 NameConstant
error                      Error
escape                     LiteralStringEscape
exception                  Keyword
field                      NameProperty
float                      LiteralNumberFloat
function                   NameFunction
function.builtin           NameBuiltin
function.defaultLibrary    NameBuiltin
function.macro             NameFunctionMagic
function.method.builtin    NameBuiltin
function.special           NameFunctionMagic
identifier                 Name
include                    KeywordNamespace
interface                  NameClass
keyword                    Keyword
keyword.control.import     KeywordNamespace
keyword.directive          CommentPreproc
keyword.function           KeywordDeclaration
keyword.import             KeywordNamespace
keyword.module             KeywordNamespace
keyword.operator           OperatorWord
keyword.storage            KeywordDeclaration
keyword.type               KeywordDeclaration
keyword.variable           KeywordDeclaration
label                      NameLabel
local                      Name
markup                     Generic
markup.heading             GenericHeading
markup.italic              GenericEmph
markup.link                NameTag
markup.link.url            NameAttribute
markup.list                Keyword
markup.quote               GenericEmph
markup.raw                 LiteralStringBacktick
markup.strikethrough       GenericDeleted
markup.strong              GenericStrong
markup.underline           GenericUnderline
meta                       Text
method                     NameFunction
method.defaultLibrary      NameBuiltin
module                     NameNamespace
module.builtin             NameBuiltin
namespace                  NameNamespace
none                       Text
nospell                    Text
number                     LiteralNumber
number.float               LiteralNumberFloat
operator                   Operator
parameter                  NameVariable
parameter.builtin          NameBuiltinPseudo
preproc                    CommentPreproc
property                   NameProperty
punctuation                Punctuation
punctuation.special        LiteralStringInterpol
repeat                     Keyword
source                     Text
special                    NameEntity
spell                      Text
storage                    KeywordDeclaration
storageclass               Keyword
string                     LiteralString
string.documentation       LiteralStringDoc
string.escape              LiteralStringEscape
string.regex               LiteralStringRegex
string.regexp              LiteralStringRegex
string.special             LiteralStringOther
string.special.regex       LiteralStringRegex
string.special.symbol      LiteralStringSymbol
symbol                     LiteralStringSymbol
tag                        NameTag
tag.attribute              NameAttribute
tag.delimiter              Punctuation
tag.error                  Error
text                       Text
text.danger                CommentSpecial
text.emphasis              GenericEmph
text.literal               LiteralStringBacktick
text.note                  CommentSpecial
text.reference             NameTag
text.strong                GenericStrong
text.title                 GenericHeading
text.underline             GenericUnderline
text.uri                   NameAttribute
text.warning               CommentSpecial
type                       KeywordType
type.builtin               KeywordType
type.definition            NameClass
type.enum.variant          NameConstant
type.qualifier             Keyword
union                      NameClass
variable                   NameVariable
variable.builtin           NameBuiltinPseudo
variable.defaultLibrary    NameBuiltin
variable.global            NameVariableGlobal
variable.member            NameProperty
variable.other.member      NameProperty
variable.parameter         NameVariable
variable.parameter.builtin NameBuiltinPseudo
warning                    CommentSpecial
`

// captureTypes holds captureTable as a map.
var captureTypes = func() map[string]string {
	m := map[string]string{}
	for line := range strings.Lines(captureTable) {
		if f := strings.Fields(line); len(f) == 2 {
			m[f[0]] = f[1]
		}
	}
	return m
}()

// captureType returns the token type of chroma for a capture name: the type
// of the longest prefix of the name, by dots, that captureTypes holds.
func captureType(capture string) (tokenType, bool) {
	name := capture
	for {
		if t, ok := captureTypes[name]; ok {
			return tokenTypes[t], true
		}
		i := strings.LastIndexByte(name, '.')
		if i < 0 {
			return 0, false
		}
		name = name[:i]
	}
}

// fromPygments holds the styles that chroma took from Pygments. Commit
// d12529a of chroma, "HTML formatter + import all Pygments styles", added
// them with the script _tools/style.py, which reads a style of Pygments.
// default.go became pygments.xml, and the commit made borland.go and
// trac.go again from Pygments. The other styles of chroma came later, from
// other sources.
var fromPygments = map[string]bool{
	"abap":          true,
	"algol":         true,
	"algol_nu":      true,
	"arduino":       true,
	"autumn":        true,
	"borland":       true,
	"bw":            true,
	"colorful":      true,
	"emacs":         true,
	"friendly":      true,
	"fruity":        true,
	"igor":          true,
	"lovelace":      true,
	"manni":         true,
	"monokai":       true,
	"murphy":        true,
	"native":        true,
	"paraiso-dark":  true,
	"paraiso-light": true,
	"pastie":        true,
	"perldoc":       true,
	"pygments":      true,
	"rainbow_dash":  true,
	"rrt":           true,
	"tango":         true,
	"trac":          true,
	"vim":           true,
	"vs":            true,
	"xcode":         true,
}
