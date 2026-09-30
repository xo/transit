package html

import "bytes"

// This file ports src/tag.h of tree-sitter-html at v0.23.2
// (5a5ca8551a179998360b4a4ca2c0f366a35acc03). A String of C is an
// Array(char), and a Go slice of bytes here. tag_free has no port, because
// it only frees memory.

// tagType is TagType. The C compiler gives the enum the type unsigned int,
// because it has no negative value, so a tagType is a uint32.
type tagType uint32

const (
	tagArea tagType = iota
	tagBase
	tagBasefont
	tagBgsound
	tagBr
	tagCol
	tagCommand
	tagEmbed
	tagFrame
	tagHr
	tagImage
	tagImg
	tagInput
	tagIsindex
	tagKeygen
	tagLink
	tagMenuitem
	tagMeta
	tagNextid
	tagParam
	tagSource
	tagTrack
	tagWbr
	endOfVoidTags

	tagA
	tagAbbr
	tagAddress
	tagArticle
	tagAside
	tagAudio
	tagB
	tagBdi
	tagBdo
	tagBlockquote
	tagBody
	tagButton
	tagCanvas
	tagCaption
	tagCite
	tagCode
	tagColgroup
	tagData
	tagDatalist
	tagDd
	tagDel
	tagDetails
	tagDfn
	tagDialog
	tagDiv
	tagDl
	tagDt
	tagEm
	tagFieldset
	tagFigcaption
	tagFigure
	tagFooter
	tagForm
	tagH1
	tagH2
	tagH3
	tagH4
	tagH5
	tagH6
	tagHead
	tagHeader
	tagHgroup
	tagHTML
	tagI
	tagIframe
	tagIns
	tagKbd
	tagLabel
	tagLegend
	tagLi
	tagMain
	tagMap
	tagMark
	tagMath
	tagMenu
	tagMeter
	tagNav
	tagNoscript
	tagObject
	tagOl
	tagOptgroup
	tagOption
	tagOutput
	tagP
	tagPicture
	tagPre
	tagProgress
	tagQ
	tagRb
	tagRp
	tagRt
	tagRtc
	tagRuby
	tagS
	tagSamp
	tagScript
	tagSection
	tagSelect
	tagSlot
	tagSmall
	tagSpan
	tagStrong
	tagStyle
	tagSub
	tagSummary
	tagSup
	tagSvg
	tagTable
	tagTbody
	tagTd
	tagTemplate
	tagTextarea
	tagTfoot
	tagTh
	tagThead
	tagTime
	tagTitle
	tagTr
	tagU
	tagUl
	tagVar
	tagVideo

	tagCustom

	tagEnd
)

// tagMapEntry is TagMapEntry.
type tagMapEntry struct {
	tagName string
	tagType tagType
}

// tag is Tag. The field type of C is typ, because type is a keyword of Go.
type tag struct {
	typ           tagType
	customTagName []byte
}

// tagTypesByTagName is TAG_TYPES_BY_TAG_NAME.
var tagTypesByTagName = [126]tagMapEntry{
	{"AREA", tagArea},
	{"BASE", tagBase},
	{"BASEFONT", tagBasefont},
	{"BGSOUND", tagBgsound},
	{"BR", tagBr},
	{"COL", tagCol},
	{"COMMAND", tagCommand},
	{"EMBED", tagEmbed},
	{"FRAME", tagFrame},
	{"HR", tagHr},
	{"IMAGE", tagImage},
	{"IMG", tagImg},
	{"INPUT", tagInput},
	{"ISINDEX", tagIsindex},
	{"KEYGEN", tagKeygen},
	{"LINK", tagLink},
	{"MENUITEM", tagMenuitem},
	{"META", tagMeta},
	{"NEXTID", tagNextid},
	{"PARAM", tagParam},
	{"SOURCE", tagSource},
	{"TRACK", tagTrack},
	{"WBR", tagWbr},
	{"A", tagA},
	{"ABBR", tagAbbr},
	{"ADDRESS", tagAddress},
	{"ARTICLE", tagArticle},
	{"ASIDE", tagAside},
	{"AUDIO", tagAudio},
	{"B", tagB},
	{"BDI", tagBdi},
	{"BDO", tagBdo},
	{"BLOCKQUOTE", tagBlockquote},
	{"BODY", tagBody},
	{"BUTTON", tagButton},
	{"CANVAS", tagCanvas},
	{"CAPTION", tagCaption},
	{"CITE", tagCite},
	{"CODE", tagCode},
	{"COLGROUP", tagColgroup},
	{"DATA", tagData},
	{"DATALIST", tagDatalist},
	{"DD", tagDd},
	{"DEL", tagDel},
	{"DETAILS", tagDetails},
	{"DFN", tagDfn},
	{"DIALOG", tagDialog},
	{"DIV", tagDiv},
	{"DL", tagDl},
	{"DT", tagDt},
	{"EM", tagEm},
	{"FIELDSET", tagFieldset},
	{"FIGCAPTION", tagFigcaption},
	{"FIGURE", tagFigure},
	{"FOOTER", tagFooter},
	{"FORM", tagForm},
	{"H1", tagH1},
	{"H2", tagH2},
	{"H3", tagH3},
	{"H4", tagH4},
	{"H5", tagH5},
	{"H6", tagH6},
	{"HEAD", tagHead},
	{"HEADER", tagHeader},
	{"HGROUP", tagHgroup},
	{"HTML", tagHTML},
	{"I", tagI},
	{"IFRAME", tagIframe},
	{"INS", tagIns},
	{"KBD", tagKbd},
	{"LABEL", tagLabel},
	{"LEGEND", tagLegend},
	{"LI", tagLi},
	{"MAIN", tagMain},
	{"MAP", tagMap},
	{"MARK", tagMark},
	{"MATH", tagMath},
	{"MENU", tagMenu},
	{"METER", tagMeter},
	{"NAV", tagNav},
	{"NOSCRIPT", tagNoscript},
	{"OBJECT", tagObject},
	{"OL", tagOl},
	{"OPTGROUP", tagOptgroup},
	{"OPTION", tagOption},
	{"OUTPUT", tagOutput},
	{"P", tagP},
	{"PICTURE", tagPicture},
	{"PRE", tagPre},
	{"PROGRESS", tagProgress},
	{"Q", tagQ},
	{"RB", tagRb},
	{"RP", tagRp},
	{"RT", tagRt},
	{"RTC", tagRtc},
	{"RUBY", tagRuby},
	{"S", tagS},
	{"SAMP", tagSamp},
	{"SCRIPT", tagScript},
	{"SECTION", tagSection},
	{"SELECT", tagSelect},
	{"SLOT", tagSlot},
	{"SMALL", tagSmall},
	{"SPAN", tagSpan},
	{"STRONG", tagStrong},
	{"STYLE", tagStyle},
	{"SUB", tagSub},
	{"SUMMARY", tagSummary},
	{"SUP", tagSup},
	{"SVG", tagSvg},
	{"TABLE", tagTable},
	{"TBODY", tagTbody},
	{"TD", tagTd},
	{"TEMPLATE", tagTemplate},
	{"TEXTAREA", tagTextarea},
	{"TFOOT", tagTfoot},
	{"TH", tagTh},
	{"THEAD", tagThead},
	{"TIME", tagTime},
	{"TITLE", tagTitle},
	{"TR", tagTr},
	{"U", tagU},
	{"UL", tagUl},
	{"VAR", tagVar},
	{"VIDEO", tagVideo},
	{"CUSTOM", tagCustom},
}

// tagTypesNotAllowedInParagraphs is TAG_TYPES_NOT_ALLOWED_IN_PARAGRAPHS.
var tagTypesNotAllowedInParagraphs = [...]tagType{
	tagAddress, tagArticle, tagAside, tagBlockquote, tagDetails, tagDiv,
	tagDl, tagFieldset, tagFigcaption, tagFigure, tagFooter, tagForm, tagH1,
	tagH2, tagH3, tagH4, tagH5, tagH6, tagHeader, tagHr, tagMain, tagNav,
	tagOl, tagP, tagPre, tagSection,
}

// tagTypeForName is tag_type_for_name.
func tagTypeForName(tagName []byte) tagType {
	for i := range 126 {
		entry := &tagTypesByTagName[i]
		if len(entry.tagName) == len(tagName) &&
			entry.tagName == string(tagName) {
			return entry.tagType
		}
	}
	return tagCustom
}

// tagNew is tag_new.
func tagNew() tag {
	var t tag
	t.typ = tagEnd
	t.customTagName = nil
	return t
}

// tagForName is tag_for_name.
func tagForName(name []byte) tag {
	t := tagNew()
	t.typ = tagTypeForName(name)
	if t.typ == tagCustom {
		t.customTagName = name
	}
	return t
}

// tagIsVoid is tag_is_void.
func tagIsVoid(self *tag) bool {
	return self.typ < endOfVoidTags
}

// tagEq is tag_eq.
func tagEq(self, other *tag) bool {
	if self.typ != other.typ {
		return false
	}
	if self.typ == tagCustom {
		if len(self.customTagName) != len(other.customTagName) {
			return false
		}
		if !bytes.Equal(self.customTagName, other.customTagName) {
			return false
		}
	}
	return true
}

// tagCanContain is tag_can_contain.
func tagCanContain(self, other *tag) bool {
	child := other.typ

	switch self.typ {
	case tagLi:
		return child != tagLi

	case tagDt, tagDd:
		return child != tagDt && child != tagDd

	case tagP:
		for i := range 26 {
			if child == tagTypesNotAllowedInParagraphs[i] {
				return false
			}
		}
		return true

	case tagColgroup:
		return child == tagCol

	case tagRb, tagRt, tagRp:
		return child != tagRb && child != tagRt && child != tagRp

	case tagOptgroup:
		return child != tagOptgroup

	case tagTr:
		return child != tagTr

	case tagTd, tagTh:
		return child != tagTd && child != tagTh && child != tagTr

	default:
		return true
	}
}
