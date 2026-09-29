package transit

import (
	"context"
	"fmt"
	"io"
	"math"
	"slices"

	"github.com/xo/transit/internal/abi"
)

// This file ports lib/src/parser.c, and lib/src/error_costs.h and
// lib/src/reduce_action.h, which the parser uses most.
//
// A context replaces the progress callback of TSParseOptions. The parser
// reads the error of the context at the points where C calls the callback,
// once in each OP_COUNT_PER_PARSER_CALLBACK_CHECK operations (D25). A parse
// that the context stops keeps its state, and the next parse goes on from
// it, as a parse that the callback stops does in C.
//
// These parts have no Go form. The WebAssembly store and has_scanner_error,
// which only the store sets, are not ported (D1). ts_parser_delete frees
// memory. TSParseState holds the values for the progress callback.
// ts_parser_parse_with_options and ts_parser_parse_string_encoding are
// ParseInput with a context (docs/API.md).

// The costs of the error recovery, and the error state.
const (
	// errorState is ERROR_STATE.
	errorState StateID = 0
	// errorCostPerRecovery is ERROR_COST_PER_RECOVERY.
	errorCostPerRecovery = 500
	// errorCostPerMissingTree is ERROR_COST_PER_MISSING_TREE.
	errorCostPerMissingTree = 110
	// errorCostPerSkippedTree is ERROR_COST_PER_SKIPPED_TREE.
	errorCostPerSkippedTree = 100
	// errorCostPerSkippedLine is ERROR_COST_PER_SKIPPED_LINE.
	errorCostPerSkippedLine = 30
	// errorCostPerSkippedChar is ERROR_COST_PER_SKIPPED_CHAR.
	errorCostPerSkippedChar = 1
)

// reduceAction is ReduceAction.
type reduceAction struct {
	count             uint32
	symbol            Symbol
	dynamicPrecedence int32
	productionID      uint16
}

// reduceActionSet is ReduceActionSet.
type reduceActionSet []reduceAction

// add is ts_reduce_action_set_add.
func (s *reduceActionSet) add(newAction reduceAction) {
	for _, action := range *s {
		if action.symbol == newAction.symbol && action.count == newAction.count {
			return
		}
	}
	*s = append(*s, newAction)
}

// The versions of the language that the parser accepts.
const (
	// languageVersion is TREE_SITTER_LANGUAGE_VERSION.
	languageVersion = 15
	// minCompatibleLanguageVersion is
	// TREE_SITTER_MIN_COMPATIBLE_LANGUAGE_VERSION.
	minCompatibleLanguageVersion = 13
)

// Error is an error of the runtime.
type Error string

// Error returns the text of the error.
func (e Error) Error() string {
	return string(e)
}

// The errors of the runtime.
const (
	// ErrIncompatibleLanguage is the error of SetLanguage for a language
	// whose ABI version the runtime does not accept.
	ErrIncompatibleLanguage Error = "incompatible language version"
	// ErrInvalidRanges is the error of SetIncludedRanges for ranges that
	// overlap or are not in order.
	ErrInvalidRanges Error = "invalid included ranges"
	// ErrNoLanguage is the error of a parse before SetLanguage.
	ErrNoLanguage Error = "parser has no language"
	// ErrLanguageMismatch is the error of a parse with an old tree of
	// another language.
	ErrLanguageMismatch Error = "old tree has another language"
	// ErrInvalidInput is the error of a parse with no input or with an
	// encoding that the runtime does not know.
	ErrInvalidInput Error = "invalid input"
)

// maxVersionCount is MAX_VERSION_COUNT.
const maxVersionCount = 6

// maxVersionCountOverflow is MAX_VERSION_COUNT_OVERFLOW.
const maxVersionCountOverflow = 4

// maxSummaryDepth is MAX_SUMMARY_DEPTH.
const maxSummaryDepth = 16

// maxCostDifference is MAX_COST_DIFFERENCE.
const maxCostDifference = 18 * errorCostPerSkippedTree

// opCountPerParserCallbackCheck is OP_COUNT_PER_PARSER_CALLBACK_CHECK.
const opCountPerParserCallbackCheck = 100

// tokenCache is TokenCache.
type tokenCache struct {
	token             subtree
	lastExternalToken subtree
	byteIndex         uint32
}

// Parser builds trees. It belongs to one goroutine at a time.
//
// Parser is TSParser.
type Parser struct {
	lexer                        lexer
	stack                        *stack
	treePool                     subtreePool
	language                     *Language
	reduceActions                reduceActionSet
	finishedTree                 subtree
	trailingExtras               subtreeArray
	trailingExtras2              subtreeArray
	scratchTrees                 subtreeArray
	tokenCache                   tokenCache
	reusableNode                 reusableNode
	externalScannerPayload       abi.Scanner
	dotGraphFile                 io.Writer
	acceptCount                  uint32
	operationCount               uint32
	oldTree                      subtree
	includedRangeDifferences     []textRange
	includedRangeDifferenceIndex uint32
	canceledBalancing            bool
	hasError                     bool
}

// errorStatus is ErrorStatus.
type errorStatus struct {
	cost              uint32
	nodeCount         uint32
	dynamicPrecedence int
	isInError         bool
}

// errorComparison is ErrorComparison.
type errorComparison uint8

// The results of a comparison of two stack versions.
const (
	// errorComparisonTakeLeft is ErrorComparisonTakeLeft.
	errorComparisonTakeLeft errorComparison = iota
	// errorComparisonPreferLeft is ErrorComparisonPreferLeft.
	errorComparisonPreferLeft
	// errorComparisonNone is ErrorComparisonNone.
	errorComparisonNone
	// errorComparisonPreferRight is ErrorComparisonPreferRight.
	errorComparisonPreferRight
	// errorComparisonTakeRight is ErrorComparisonTakeRight.
	errorComparisonTakeRight
)

// String returns the name of the comparison.
func (c errorComparison) String() string {
	switch c {
	case errorComparisonTakeLeft:
		return "take left"
	case errorComparisonPreferLeft:
		return "prefer left"
	case errorComparisonNone:
		return noneName
	case errorComparisonPreferRight:
		return "prefer right"
	case errorComparisonTakeRight:
		return "take right"
	}
	return unknownName
}

// stringInput is TSStringInput, the input of a parse of a byte slice.
type stringInput []byte

// ReadAt is ts_string_input_read.
func (s stringInput) ReadAt(offset int, _ Point) []byte {
	if offset >= len(s) {
		return nil
	}
	return s[offset:]
}

// logging reports whether the parser logs. It is the condition of the
// macros LOG and LOG_LOOKAHEAD, and each call site tests it, as the macros
// do, so that the arguments of a message are made only when the parser logs.
func (p *Parser) logging() bool {
	return p.lexer.logger != nil || p.dotGraphFile != nil
}

// logf is the macro LOG, after its condition. It writes the message to the
// buffer of the lexer, as snprintf does, and logs it.
func (p *Parser) logf(format string, args ...any) {
	p.writeDebugBuffer(fmt.Sprintf(format, args...))
	p.log()
}

// writeDebugBuffer writes a message to the buffer of the lexer as snprintf
// writes it in C: at most TREE_SITTER_SERIALIZATION_BUFFER_SIZE - 1 bytes and
// a NUL byte.
func (p *Parser) writeDebugBuffer(message string) {
	buf := p.lexer.debugBuffer[:]
	n := copy(buf[:len(buf)-1], message)
	buf[n] = 0
}

// logLookahead is the macro LOG_LOOKAHEAD, after its condition. C writes
// the escaped name byte by byte, and it can write past the end of the buffer
// for a name of about 1,000 bytes. Go cuts the message at the size of the
// buffer.
func (p *Parser) logLookahead(symbolName string, size uint32) {
	b := make([]byte, 0, 64)
	b = append(b, "lexed_lookahead sym:"...)
	for i := 0; i < len(symbolName) && symbolName[i] != 0; i++ {
		switch symbolName[i] {
		case '\t':
			b = append(b, '\\', 't')
		case '\n':
			b = append(b, '\\', 'n')
		case '\v':
			b = append(b, '\\', 'v')
		case '\f':
			b = append(b, '\\', 'f')
		case '\r':
			b = append(b, '\\', 'r')
		case '\\':
			b = append(b, '\\', '\\')
		default:
			b = append(b, symbolName[i])
		}
	}
	b = fmt.Appendf(b, ", size:%d", size)
	p.writeDebugBuffer(string(b))
	p.log()
}

// logStack is the macro LOG_STACK.
func (p *Parser) logStack() {
	if p.dotGraphFile != nil {
		p.stack.printDotGraph(p.language, p.dotGraphFile)
		fprintf(p.dotGraphFile, "\n\n")
	}
}

// logTree is the macro LOG_TREE.
func (p *Parser) logTree(tree subtree) {
	if p.dotGraphFile != nil {
		tree.printDotGraph(p.language, p.dotGraphFile)
		fprintf(p.dotGraphFile, "\n")
	}
}

// symName is the macro SYM_NAME.
func (p *Parser) symName(symbol Symbol) string {
	return p.language.SymbolName(symbol)
}

// treeName is the macro TREE_NAME.
func (p *Parser) treeName(tree subtree) string {
	return p.symName(tree.symbol())
}

// log is ts_parser__log. It logs the message in the buffer of the lexer.
func (p *Parser) log() {
	buf := p.lexer.debugBuffer[:]
	n := 0
	for n < len(buf) && buf[n] != 0 {
		n++
	}
	if p.lexer.logger != nil {
		p.lexer.logger(LogParse, string(buf[:n]))
	}

	if p.dotGraphFile != nil {
		fprintf(p.dotGraphFile, "graph {\nlabel=\"")
		label := make([]byte, 0, n+8)
		for _, chr := range buf[:n] {
			if chr == '"' || chr == '\\' {
				label = append(label, '\\')
			}
			label = append(label, chr)
		}
		_, _ = p.dotGraphFile.Write(label)
		fprintf(p.dotGraphFile, "\"\n}\n\n")
	}
}

// breakdownTopOfStack is ts_parser__breakdown_top_of_stack.
func (p *Parser) breakdownTopOfStack(version stackVersion) bool {
	didBreakDown := false
	var pending bool

	for {
		pop := p.stack.popPending(version)
		if len(pop) == 0 {
			break
		}

		didBreakDown = true
		pending = false
		for i := range pop {
			slice := pop[i]
			state := p.stack.state(slice.version)
			parent := slice.subtrees[0]

			for j, n := uint32(0), parent.childCount(); j < n; j++ {
				child := parent.ptr.children[j]
				pending = child.childCount() > 0

				if child.isError() {
					state = errorState
				} else if !child.extra() {
					state = p.language.NextState(state, child.symbol())
				}

				child.retain()
				p.stack.push(slice.version, child, pending, state)
			}

			for j := 1; j < len(slice.subtrees); j++ {
				tree := slice.subtrees[j]
				p.stack.push(slice.version, tree, false, state)
			}

			parent.release(&p.treePool)

			if p.logging() {
				p.logf("breakdown_top_of_stack tree:%s", p.treeName(parent))
			}
			p.logStack()
		}
		if !pending {
			break
		}
	}

	return didBreakDown
}

// breakdownLookahead is ts_parser__breakdown_lookahead.
func (p *Parser) breakdownLookahead(
	lookahead *subtree,
	state StateID,
	reusableNode *reusableNode,
) {
	didDescend := false
	tree := reusableNode.tree()
	for tree.childCount() > 0 && tree.parseState() != state {
		if p.logging() {
			p.logf("state_mismatch sym:%s", p.treeName(tree))
		}
		reusableNode.descend()
		tree = reusableNode.tree()
		didDescend = true
	}

	if didDescend {
		lookahead.release(&p.treePool)
		*lookahead = tree
		lookahead.retain()
	}
}

// compareVersions is ts_parser__compare_versions.
func (p *Parser) compareVersions(a, b errorStatus) errorComparison {
	if !a.isInError && b.isInError {
		if a.cost < b.cost {
			return errorComparisonTakeLeft
		}
		return errorComparisonPreferLeft
	}

	if a.isInError && !b.isInError {
		if b.cost < a.cost {
			return errorComparisonTakeRight
		}
		return errorComparisonPreferRight
	}

	if a.cost < b.cost {
		if (b.cost-a.cost)*(1+a.nodeCount) > maxCostDifference {
			return errorComparisonTakeLeft
		}
		return errorComparisonPreferLeft
	}

	if b.cost < a.cost {
		if (a.cost-b.cost)*(1+b.nodeCount) > maxCostDifference {
			return errorComparisonTakeRight
		}
		return errorComparisonPreferRight
	}

	if a.dynamicPrecedence > b.dynamicPrecedence {
		return errorComparisonPreferLeft
	}
	if b.dynamicPrecedence > a.dynamicPrecedence {
		return errorComparisonPreferRight
	}
	return errorComparisonNone
}

// versionStatus is ts_parser__version_status.
func (p *Parser) versionStatus(version stackVersion) errorStatus {
	cost := p.stack.errorCost(version)
	isPaused := p.stack.isPaused(version)
	if isPaused {
		cost += errorCostPerSkippedTree
	}
	return errorStatus{
		cost:              cost,
		nodeCount:         p.stack.nodeCountSinceError(version),
		dynamicPrecedence: p.stack.dynamicPrecedence(version),
		isInError:         isPaused || p.stack.state(version) == errorState,
	}
}

// betterVersionExists is ts_parser__better_version_exists.
func (p *Parser) betterVersionExists(
	version stackVersion,
	isInError bool,
	cost uint32,
) bool {
	if p.finishedTree.ptr != nil && p.finishedTree.errorCost() <= cost {
		return true
	}

	position := p.stack.position(version)
	status := errorStatus{
		cost:              cost,
		isInError:         isInError,
		dynamicPrecedence: p.stack.dynamicPrecedence(version),
		nodeCount:         p.stack.nodeCountSinceError(version),
	}

	for i, n := stackVersion(0), stackVersion(p.stack.versionCount()); i < n; i++ {
		if i == version ||
			!p.stack.isActive(i) ||
			p.stack.position(i).bytes < position.bytes {
			continue
		}
		statusI := p.versionStatus(i)
		switch p.compareVersions(status, statusI) {
		case errorComparisonTakeRight:
			return true
		case errorComparisonPreferRight:
			if p.stack.canMerge(i, version) {
				return true
			}
		}
	}

	return false
}

// callMainLexFn is ts_parser__call_main_lex_fn.
func (p *Parser) callMainLexFn(lexMode abi.LexerMode) bool {
	return p.language.tables.LexFn(&p.lexer.data, lexMode.LexState)
}

// callKeywordLexFn is ts_parser__call_keyword_lex_fn.
func (p *Parser) callKeywordLexFn() bool {
	return p.language.tables.KeywordLexFn(&p.lexer.data, 0)
}

// externalScannerCreate is ts_parser__external_scanner_create.
func (p *Parser) externalScannerCreate() {
	if p.language != nil && p.language.tables.ExternalScanner.States != nil {
		if p.language.tables.ExternalScanner.Create != nil {
			p.externalScannerPayload = p.language.tables.ExternalScanner.Create()
		}
	}
}

// externalScannerDestroy is ts_parser__external_scanner_destroy. The garbage
// collector frees the scanner, so Go only drops it.
func (p *Parser) externalScannerDestroy() {
	p.externalScannerPayload = nil
}

// externalScannerSerialize is ts_parser__external_scanner_serialize. The
// scanner writes its state to the buffer of the lexer.
func (p *Parser) externalScannerSerialize() uint32 {
	length := uint32(p.externalScannerPayload.Serialize(p.lexer.debugBuffer[:]))
	assert(length <= abi.SerializationBufferSize)
	return length
}

// externalScannerDeserialize is ts_parser__external_scanner_deserialize.
func (p *Parser) externalScannerDeserialize(externalToken subtree) {
	var data []byte
	if externalToken.ptr != nil {
		data = externalToken.ptr.externalScannerState.data()
	}

	p.externalScannerPayload.Deserialize(data)
}

// externalScannerScan is ts_parser__external_scanner_scan.
func (p *Parser) externalScannerScan(externalLexState uint16) bool {
	validExternalTokens := p.language.enabledExternalTokens(uint32(externalLexState))
	return p.externalScannerPayload.Scan(&p.lexer.data, validExternalTokens)
}

// canReuseFirstLeaf is ts_parser__can_reuse_first_leaf.
func (p *Parser) canReuseFirstLeaf(
	state StateID,
	tree subtree,
	tableEntry *tableEntry,
) bool {
	leafSymbol := tree.leafSymbol()
	leafState := tree.leafParseState()
	currentLexMode := p.language.lexModeForState(state)
	leafLexMode := p.language.lexModeForState(leafState)

	// At the end of a non-terminal extra node, the lexer normally returns
	// NULL, which indicates that the parser should look for a reduce action
	// at symbol `0`. Avoid reusing tokens in this situation to ensure that
	// the same thing happens when incrementally reparsing.
	if currentLexMode.LexState == math.MaxUint16 {
		return false
	}

	// If the token was created in a state with the same set of lookaheads, it is reusable.
	if len(tableEntry.actions) > 0 &&
		leafLexMode == currentLexMode &&
		(leafSymbol != Symbol(p.language.tables.KeywordCaptureToken) ||
			(!tree.isKeyword() && tree.parseState() == state)) {
		return true
	}

	// Empty tokens are not reusable in states with different lookaheads.
	if tree.size().bytes == 0 && leafSymbol != builtinSymEnd {
		return false
	}

	// If the current state allows external tokens or other tokens that conflict with this
	// token, this token is not reusable.
	return currentLexMode.ExternalLexState == 0 && tableEntry.isReusable
}

// lex is ts_parser__lex.
func (p *Parser) lex(
	version stackVersion,
	parseState StateID,
) subtree {
	lexMode := p.language.lexModeForState(parseState)
	if lexMode.LexState == math.MaxUint16 {
		if p.logging() {
			p.logf("no_lookahead_after_non_terminal_extra")
		}
		return subtree{}
	}

	startPosition := p.stack.position(version)
	externalToken := p.stack.lastExternalToken(version)

	foundExternalToken := false
	errorMode := parseState == errorState
	skippedError := false
	calledGetColumn := false
	firstErrorCharacter := int32(0)
	errorStartPosition := lengthZero()
	errorEndPosition := lengthZero()
	lookaheadEndByte := uint32(0)
	externalScannerStateLen := uint32(0)
	externalScannerStateChanged := false
	p.lexer.reset(startPosition)

	for {
		var foundToken bool
		currentPosition := p.lexer.currentPosition
		columnData := p.lexer.columnData

		if lexMode.ExternalLexState != 0 {
			if p.logging() {
				p.logf(
					"lex_external state:%d, row:%d, column:%d",
					lexMode.ExternalLexState,
					currentPosition.extent.row,
					currentPosition.extent.column,
				)
			}
			p.lexer.start()
			p.externalScannerDeserialize(externalToken)
			foundToken = p.externalScannerScan(lexMode.ExternalLexState)
			lookaheadEndByte = p.lexer.finish(lookaheadEndByte)

			if foundToken {
				externalScannerStateLen = p.externalScannerSerialize()
				externalScannerStateChanged = !externalToken.getExternalScannerState().eq(
					p.lexer.debugBuffer[:externalScannerStateLen],
				)

				// Avoid infinite loops caused by the external scanner returning empty tokens.
				// Empty tokens are needed in some circumstances, e.g. indent/dedent tokens
				// in Python. Ignore the following classes of empty tokens:
				//
				// * Tokens produced during error recovery. When recovering from an error,
				//   all tokens are allowed, so it's easy to accidentally return unwanted
				//   empty tokens.
				// * Tokens that are marked as 'extra' in the grammar. These don't change
				//   the parse state, so they would definitely cause an infinite loop.
				if p.lexer.tokenEndPosition.bytes <= currentPosition.bytes &&
					!externalScannerStateChanged {
					symbol := Symbol(p.language.tables.ExternalScanner.SymbolMap[p.lexer.data.ResultSymbol])
					nextParseState := p.language.NextState(parseState, symbol)
					tokenIsExtra := nextParseState == parseState
					if errorMode || !p.stack.hasAdvancedSinceError(version) || tokenIsExtra {
						if p.logging() {
							p.logf(
								"ignore_empty_external_token symbol:%s",
								p.symName(Symbol(p.language.tables.ExternalScanner.SymbolMap[p.lexer.data.ResultSymbol])),
							)
						}
						foundToken = false
					}
				}
			}

			if foundToken {
				foundExternalToken = true
				calledGetColumn = p.lexer.didGetColumn
				break
			}

			p.lexer.reset(currentPosition)
			p.lexer.columnData = columnData
		}

		if p.logging() {
			p.logf(
				"lex_internal state:%d, row:%d, column:%d",
				lexMode.LexState,
				currentPosition.extent.row,
				currentPosition.extent.column,
			)
		}
		p.lexer.start()
		foundToken = p.callMainLexFn(lexMode)
		lookaheadEndByte = p.lexer.finish(lookaheadEndByte)
		if foundToken {
			break
		}

		if !errorMode {
			errorMode = true
			lexMode = p.language.lexModeForState(errorState)
			p.lexer.reset(startPosition)
			continue
		}

		if !skippedError {
			if p.logging() {
				p.logf("skip_unrecognized_character")
			}
			skippedError = true
			errorStartPosition = p.lexer.tokenStartPosition
			errorEndPosition = p.lexer.tokenStartPosition
			firstErrorCharacter = p.lexer.data.Lookahead
		}

		if p.lexer.currentPosition.bytes == errorEndPosition.bytes {
			if p.lexer.data.EOF() {
				p.lexer.data.ResultSymbol = uint16(builtinSymError)
				break
			}
			p.lexer.data.Advance(false)
		}

		errorEndPosition = p.lexer.currentPosition
	}

	var result subtree
	if skippedError {
		padding := errorStartPosition.sub(startPosition)
		size := errorEndPosition.sub(errorStartPosition)
		lookaheadBytes := lookaheadEndByte - errorEndPosition.bytes
		result = newError(
			&p.treePool,
			firstErrorCharacter,
			padding,
			size,
			lookaheadBytes,
			parseState,
			p.language,
		)
	} else {
		isKeyword := false
		symbol := Symbol(p.lexer.data.ResultSymbol)
		padding := p.lexer.tokenStartPosition.sub(startPosition)
		size := p.lexer.tokenEndPosition.sub(p.lexer.tokenStartPosition)
		lookaheadBytes := lookaheadEndByte - p.lexer.tokenEndPosition.bytes

		if foundExternalToken {
			symbol = Symbol(p.language.tables.ExternalScanner.SymbolMap[symbol])
		} else if symbol == Symbol(p.language.tables.KeywordCaptureToken) && symbol != 0 {
			endByte := p.lexer.tokenEndPosition.bytes
			p.lexer.reset(p.lexer.tokenStartPosition)
			p.lexer.start()

			isKeyword = p.callKeywordLexFn()

			if isKeyword &&
				p.lexer.tokenEndPosition.bytes == endByte &&
				(p.language.hasActions(parseState, Symbol(p.lexer.data.ResultSymbol)) ||
					p.language.isReservedWord(parseState, Symbol(p.lexer.data.ResultSymbol))) {
				symbol = Symbol(p.lexer.data.ResultSymbol)
			}
		}

		result = newLeaf(
			&p.treePool,
			symbol,
			padding,
			size,
			lookaheadBytes,
			parseState,
			foundExternalToken,
			calledGetColumn,
			isKeyword,
			p.language,
		)

		if foundExternalToken {
			mutResult := result
			mutResult.ptr.externalScannerState.init(p.lexer.debugBuffer[:externalScannerStateLen])
			mutResult.ptr.hasExternalScannerStateChange = externalScannerStateChanged
		}
	}

	if p.logging() {
		p.logLookahead(
			p.symName(result.symbol()),
			result.totalSize().bytes,
		)
	}
	return result
}

// getCachedToken is ts_parser__get_cached_token.
func (p *Parser) getCachedToken(
	state StateID,
	position uint32,
	lastExternalToken subtree,
	tableEntry *tableEntry,
) subtree {
	cache := &p.tokenCache
	if cache.token.ptr != nil && cache.byteIndex == position &&
		cache.lastExternalToken.externalScannerStateEq(lastExternalToken) {
		*tableEntry = p.language.tableEntry(state, cache.token.symbol())
		if p.canReuseFirstLeaf(state, cache.token, tableEntry) {
			cache.token.retain()
			return cache.token
		}
	}
	return subtree{}
}

// setCachedToken is ts_parser__set_cached_token.
func (p *Parser) setCachedToken(
	byteIndex uint32,
	lastExternalToken subtree,
	token subtree,
) {
	cache := &p.tokenCache
	if token.ptr != nil {
		token.retain()
	}
	if lastExternalToken.ptr != nil {
		lastExternalToken.retain()
	}
	if cache.token.ptr != nil {
		cache.token.release(&p.treePool)
	}
	if cache.lastExternalToken.ptr != nil {
		cache.lastExternalToken.release(&p.treePool)
	}
	cache.token = token
	cache.byteIndex = byteIndex
	cache.lastExternalToken = lastExternalToken
}

// hasIncludedRangeDifference is ts_parser__has_included_range_difference.
func (p *Parser) hasIncludedRangeDifference(
	startPosition uint32,
	endPosition uint32,
) bool {
	return rangeArrayIntersects(
		p.includedRangeDifferences,
		p.includedRangeDifferenceIndex,
		startPosition,
		endPosition,
	)
}

// reuseNode is ts_parser__reuse_node.
func (p *Parser) reuseNode(
	version stackVersion,
	state *StateID,
	position uint32,
	lastExternalToken subtree,
	tableEntry *tableEntry,
) subtree {
	for {
		result := p.reusableNode.tree()
		if result.ptr == nil {
			break
		}
		byteOffset := p.reusableNode.byteOffset()
		endByteOffset := byteOffset + result.totalBytes()

		// Do not reuse an EOF node if the included ranges array has changes
		// later on in the file.
		if result.isEOF() {
			endByteOffset = math.MaxUint32
		}

		if byteOffset > position {
			if p.logging() {
				p.logf("before_reusable_node symbol:%s", p.treeName(result))
			}
			break
		}

		if byteOffset < position {
			if p.logging() {
				p.logf("past_reusable_node symbol:%s", p.treeName(result))
			}
			if endByteOffset <= position || !p.reusableNode.descend() {
				p.reusableNode.advance()
			}
			continue
		}

		if !p.reusableNode.lastExternalToken.externalScannerStateEq(lastExternalToken) {
			if p.logging() {
				p.logf("reusable_node_has_different_external_scanner_state symbol:%s", p.treeName(result))
			}
			p.reusableNode.advance()
			continue
		}

		reason := ""
		switch {
		case result.hasChanges():
			reason = "has_changes"
		case result.isError():
			reason = "is_error"
		case result.missing():
			reason = "is_missing"
		case result.isFragile():
			reason = "is_fragile"
		case p.hasIncludedRangeDifference(byteOffset, reuseEnd(result, endByteOffset)):
			reason = "contains_different_included_range"
		}

		if reason != "" {
			if p.logging() {
				p.logf("cant_reuse_node_%s tree:%s", reason, p.treeName(result))
			}
			if !p.reusableNode.descend() {
				p.reusableNode.advance()
				p.breakdownTopOfStack(version)
				*state = p.stack.state(version)
			}
			continue
		}

		leafSymbol := result.leafSymbol()
		*tableEntry = p.language.tableEntry(*state, leafSymbol)
		if !p.canReuseFirstLeaf(*state, result, tableEntry) {
			if p.logging() {
				p.logf(
					"cant_reuse_node symbol:%s, first_leaf_symbol:%s",
					p.treeName(result),
					p.symName(leafSymbol),
				)
			}
			p.reusableNode.advancePastLeaf()
			break
		}

		if p.logging() {
			p.logf("reuse_node symbol:%s", p.treeName(result))
		}
		result.retain()
		return result
	}

	return subtree{}
}

// reuseEnd returns the end of the range that ts_parser__reuse_node looks
// for a difference of the included ranges in: the end of an EOF node, or the
// end of the lookahead of another node.
func reuseEnd(result subtree, endByteOffset uint32) uint32 {
	if result.isEOF() {
		return endByteOffset
	}
	return endByteOffset + result.lookaheadBytes()
}

// selectTree is ts_parser__select_tree.
//
// Determine if a given tree should be replaced by an alternative tree.
//
// The decision is based on the trees' error costs (if any), their dynamic precedence,
// and finally, as a default, by a recursive comparison of the trees' symbols.
func (p *Parser) selectTree(left, right subtree) bool {
	if left.ptr == nil {
		return true
	}
	if right.ptr == nil {
		return false
	}

	if right.errorCost() < left.errorCost() {
		if p.logging() {
			p.logf("select_smaller_error symbol:%s, over_symbol:%s", p.treeName(right), p.treeName(left))
		}
		return true
	}

	if left.errorCost() < right.errorCost() {
		if p.logging() {
			p.logf("select_smaller_error symbol:%s, over_symbol:%s", p.treeName(left), p.treeName(right))
		}
		return false
	}

	if right.dynamicPrecedence() > left.dynamicPrecedence() {
		if p.logging() {
			p.logf("select_higher_precedence symbol:%s, prec:%d, over_symbol:%s, other_prec:%d",
				p.treeName(right), right.dynamicPrecedence(), p.treeName(left),
				left.dynamicPrecedence())
		}
		return true
	}

	if left.dynamicPrecedence() > right.dynamicPrecedence() {
		if p.logging() {
			p.logf("select_higher_precedence symbol:%s, prec:%d, over_symbol:%s, other_prec:%d",
				p.treeName(left), left.dynamicPrecedence(), p.treeName(right),
				right.dynamicPrecedence())
		}
		return false
	}

	if left.errorCost() > 0 {
		return true
	}

	comparison := compare(left, right, &p.treePool)
	switch comparison {
	case -1:
		if p.logging() {
			p.logf("select_earlier symbol:%s, over_symbol:%s", p.treeName(left), p.treeName(right))
		}
		return false
	case 1:
		if p.logging() {
			p.logf("select_earlier symbol:%s, over_symbol:%s", p.treeName(right), p.treeName(left))
		}
		return true
	default:
		if p.logging() {
			p.logf("select_existing symbol:%s, over_symbol:%s", p.treeName(left), p.treeName(right))
		}
		return false
	}
}

// selectChildren is ts_parser__select_children.
//
// Determine if a given tree's children should be replaced by an alternative
// array of children.
func (p *Parser) selectChildren(
	left subtree,
	children subtreeArray,
) bool {
	p.scratchTrees = append(p.scratchTrees[:0], children...)

	// Create a temporary subtree using the scratch trees array. This node does
	// not perform any allocation except for possibly growing the array to make
	// room for its own heap data. The scratch tree is never explicitly released,
	// so the same 'scratch trees' array can be reused again later.
	scratchTree := newNode(
		&p.treePool,
		left.symbol(),
		p.scratchTrees,
		0,
		p.language,
	)

	return p.selectTree(
		left,
		scratchTree,
	)
}

// shift is ts_parser__shift.
func (p *Parser) shift(
	version stackVersion,
	state StateID,
	lookahead subtree,
	extra bool,
) {
	isLeaf := lookahead.childCount() == 0
	subtreeToPush := lookahead
	if extra != lookahead.extra() && isLeaf {
		result := lookahead.makeMut(&p.treePool)
		result.setExtra(extra)
		subtreeToPush = result
	}

	p.stack.push(version, subtreeToPush, !isLeaf, state)
	if subtreeToPush.hasExternalTokens() {
		p.stack.setLastExternalToken(
			version, subtreeToPush.lastExternalToken(),
		)
	}
}

// reduce is ts_parser__reduce.
func (p *Parser) reduce(
	version stackVersion,
	symbol Symbol,
	count uint32,
	dynamicPrecedence int32,
	productionID uint16,
	isFragile bool,
	endOfNonTerminalExtra bool,
) stackVersion {
	initialVersionCount := p.stack.versionCount()

	// Pop the given number of nodes from the given version of the parse stack.
	// If stack versions have previously merged, then there may be more than one
	// path back through the stack. For each path, create a new parent node to
	// contain the popped children, and push it onto the stack in place of the
	// children.
	pop := p.stack.popCount(version, count)
	removedVersionCount := uint32(0)
	haltedVersionCount := p.stack.haltedVersionCount()
	for i := 0; i < len(pop); i++ {
		slice := pop[i]
		sliceVersion := slice.version - stackVersion(removedVersionCount)

		// This is where new versions are added to the parse stack. The versions
		// will all be sorted and truncated at the end of the outer parsing loop.
		// Allow the maximum version count to be temporarily exceeded, but only
		// by a limited threshold.
		if uint32(sliceVersion) > maxVersionCount+maxVersionCountOverflow+haltedVersionCount {
			p.stack.removeVersion(sliceVersion)
			slice.subtrees.delete(&p.treePool)
			removedVersionCount++
			for i+1 < len(pop) {
				if p.logging() {
					p.logf("aborting reduce with too many versions")
				}
				nextSlice := pop[i+1]
				if nextSlice.version != slice.version {
					break
				}
				nextSlice.subtrees.delete(&p.treePool)
				i++
			}
			continue
		}

		// Extra tokens on top of the stack should not be included in this new parent
		// node. They will be re-pushed onto the stack after the parent node is
		// created and pushed.
		children := slice.subtrees
		children.removeTrailingExtras(&p.trailingExtras)

		parent := newNode(
			&p.treePool, symbol, children, uint32(productionID), p.language,
		)

		// This pop operation may have caused multiple stack versions to collapse
		// into one, because they all diverged from a common state. In that case,
		// choose one of the arrays of trees to be the parent node's children, and
		// delete the rest of the tree arrays.
		for i+1 < len(pop) {
			nextSlice := pop[i+1]
			if nextSlice.version != slice.version {
				break
			}
			i++

			nextSliceChildren := nextSlice.subtrees
			nextSliceChildren.removeTrailingExtras(&p.trailingExtras2)

			if p.selectChildren(
				parent,
				nextSliceChildren,
			) {
				p.trailingExtras.clear(&p.treePool)
				parent.release(&p.treePool)
				p.trailingExtras, p.trailingExtras2 = p.trailingExtras2, p.trailingExtras
				parent = newNode(
					&p.treePool, symbol, nextSliceChildren, uint32(productionID), p.language,
				)
			} else {
				p.trailingExtras2 = p.trailingExtras2[:0]
				nextSlice.subtrees.delete(&p.treePool)
			}
		}

		state := p.stack.state(sliceVersion)
		nextState := p.language.NextState(state, symbol)
		if endOfNonTerminalExtra && nextState == state {
			parent.ptr.extra = true
		}
		if isFragile || len(pop) > 1 || initialVersionCount > 1 {
			parent.ptr.fragileLeft = true
			parent.ptr.fragileRight = true
			parent.ptr.parseState = tsTreeStateNone
		} else {
			parent.ptr.parseState = state
		}
		parent.ptr.dynamicPrecedence += dynamicPrecedence

		// Push the parent node onto the stack, along with any extra tokens that
		// were previously on top of the stack.
		p.stack.push(sliceVersion, parent, false, nextState)
		for _, extra := range p.trailingExtras {
			p.stack.push(sliceVersion, extra, false, nextState)
		}

		for j := range sliceVersion {
			if j == version {
				continue
			}
			if p.stack.merge(j, sliceVersion) {
				removedVersionCount++
				break
			}
		}
	}

	// Return the first new stack version that was created.
	if p.stack.versionCount() > initialVersionCount {
		return stackVersion(initialVersionCount)
	}
	return stackVersionNone
}

// accept is ts_parser__accept.
func (p *Parser) accept(
	version stackVersion,
	lookahead subtree,
) {
	assert(lookahead.isEOF())
	p.stack.push(version, lookahead, false, 1)

	pop := p.stack.popAll(version)
	for i := range pop {
		trees := pop[i].subtrees

		root := subtree{}
		for j, tree := range slices.Backward(trees) {
			if !tree.extra() {
				assert(!tree.ptr.isInline)
				children := tree.ptr.children
				for _, child := range children {
					child.retain()
				}
				trees = slices.Replace(trees, j, j+1, children...)
				root = newNode(
					&p.treePool,
					tree.symbol(),
					trees,
					uint32(tree.ptr.productionID),
					p.language,
				)
				tree.release(&p.treePool)
				break
			}
		}

		assert(root.ptr != nil)
		p.acceptCount++

		if p.finishedTree.ptr != nil {
			if p.selectTree(p.finishedTree, root) {
				p.finishedTree.release(&p.treePool)
				p.finishedTree = root
			} else {
				root.release(&p.treePool)
			}
		} else {
			p.finishedTree = root
		}
	}

	p.stack.removeVersion(pop[0].version)
	p.stack.halt(version)
}

// processCandidateRecoveryActions is
// ts_parser__process_candidate_recovery_actions.
func (p *Parser) processCandidateRecoveryActions(
	actions []abi.ParseActionEntry,
) bool {
	hasShiftAction := false
	for i := range actions {
		action := actions[i].Action
		switch action.Type {
		case abi.ParseActionTypeShift, abi.ParseActionTypeRecover:
			if !action.Shift.Extra && !action.Shift.Repetition {
				hasShiftAction = true
			}
		case abi.ParseActionTypeReduce:
			if action.Reduce.ChildCount > 0 {
				p.reduceActions.add(reduceAction{
					symbol:            Symbol(action.Reduce.Symbol),
					count:             uint32(action.Reduce.ChildCount),
					dynamicPrecedence: int32(action.Reduce.DynamicPrecedence),
					productionID:      action.Reduce.ProductionID,
				})
			}
		}
	}
	return hasShiftAction
}

// doAllPotentialReductions is ts_parser__do_all_potential_reductions.
func (p *Parser) doAllPotentialReductions(
	startingVersion stackVersion,
	lookaheadSymbol Symbol,
) bool {
	initialVersionCount := p.stack.versionCount()

	canShiftLookaheadSymbol := false
	version := startingVersion
	for i := uint32(0); ; i++ {
		versionCount := p.stack.versionCount()
		if uint32(version) >= versionCount {
			break
		}

		merged := false
		for j := stackVersion(initialVersionCount); j < version; j++ {
			if p.stack.merge(j, version) {
				merged = true
				break
			}
		}
		if merged {
			continue
		}

		state := p.stack.state(version)
		hasShiftAction := false
		p.reduceActions = p.reduceActions[:0]

		if lookaheadSymbol != 0 {
			entry := p.language.tableEntry(state, lookaheadSymbol)
			hasShiftAction = p.processCandidateRecoveryActions(entry.actions)
		} else {
			iter := p.language.lookaheads(state)
			for iter.next() {
				// only terminal tokens are valid lookaheads for reduction decisions
				if iter.symbol == builtinSymEnd || uint32(iter.symbol) >= p.language.tables.TokenCount {
					continue
				}
				if p.processCandidateRecoveryActions(iter.actions) {
					hasShiftAction = true
				}
			}

			// Sort reduce_actions by symbol descending to ensure deterministic
			// ordering. The LookaheadIterator may visit symbols in a different
			// order than the original linear scan (group order vs symbol order
			// for small parse states), which can produce different orderings.
			// Since reductions are applied sequentially and the last reduction
			// version survives, the order affects error recovery outcomes.
			for j := 1; j < len(p.reduceActions); j++ {
				key := p.reduceActions[j]
				k := j - 1
				for k >= 0 && p.reduceActions[k].symbol < key.symbol {
					p.reduceActions[k+1] = p.reduceActions[k]
					k--
				}
				p.reduceActions[k+1] = key
			}
		}

		reductionVersion := stackVersionNone
		for _, action := range p.reduceActions {
			reductionVersion = p.reduce(
				version, action.symbol, action.count,
				action.dynamicPrecedence, action.productionID,
				true, false,
			)
		}

		switch {
		case hasShiftAction:
			canShiftLookaheadSymbol = true
		case reductionVersion != stackVersionNone && i < maxVersionCount:
			p.stack.renumberVersion(reductionVersion, version)
			continue
		case lookaheadSymbol != 0:
			p.stack.removeVersion(version)
		}

		if version == startingVersion {
			version = stackVersion(versionCount)
		} else {
			version++
		}
	}

	return canShiftLookaheadSymbol
}

// recoverToState is ts_parser__recover_to_state.
func (p *Parser) recoverToState(
	version stackVersion,
	depth uint32,
	goalState StateID,
) bool {
	pop := p.stack.popCount(version, depth)
	previousVersion := stackVersionNone

	for i := 0; i < len(pop); i++ {
		slice := pop[i]

		if slice.version == previousVersion {
			slice.subtrees.delete(&p.treePool)
			pop = slices.Delete(pop, i, i+1)
			i--
			continue
		}

		if p.stack.state(slice.version) != goalState {
			p.stack.halt(slice.version)
			slice.subtrees.delete(&p.treePool)
			pop = slices.Delete(pop, i, i+1)
			i--
			continue
		}

		errorTrees := p.stack.popError(slice.version)
		if len(errorTrees) > 0 {
			assert(len(errorTrees) == 1)
			errorTree := errorTrees[0]
			errorChildCount := errorTree.childCount()
			if errorChildCount > 0 {
				nested := make(subtreeArray, 0, errorChildCount)
				for j := range errorChildCount {
					child := errorTree.ptr.children[j]
					child.retain()
					nested = append(nested, child)
				}
				nestedError := newNode(
					&p.treePool, builtinSymErrorRepeat, nested, 0, p.language,
				)
				slice.subtrees = slices.Insert(slice.subtrees, 0, nestedError)
			}
			errorTrees.delete(&p.treePool)
		}

		slice.subtrees.removeTrailingExtras(&p.trailingExtras)

		if len(slice.subtrees) > 0 {
			errorNode := newErrorNode(&p.treePool, slice.subtrees, true, p.language)
			p.stack.push(slice.version, errorNode, false, goalState)
		}

		for _, tree := range p.trailingExtras {
			p.stack.push(slice.version, tree, false, goalState)
		}

		previousVersion = slice.version
	}

	return previousVersion != stackVersionNone
}

// recover is ts_parser__recover.
func (p *Parser) recover(
	version stackVersion,
	lookahead subtree,
) {
	didRecover := false
	previousVersionCount := p.stack.versionCount()
	position := p.stack.position(version)
	summary := p.stack.getSummary(version)
	nodeCountSinceError := p.stack.nodeCountSinceError(version)
	currentErrorCost := p.stack.errorCost(version)

	// When the parser is in the error state, there are two strategies for recovering with a
	// given lookahead token:
	// 1. Find a previous state on the stack in which that lookahead token would be valid. Then,
	//    create a new stack version that is in that state again. This entails popping all of the
	//    subtrees that have been pushed onto the stack since that previous state, and wrapping
	//    them in an ERROR node.
	// 2. Wrap the lookahead token in an ERROR node, push that ERROR node onto the stack, and
	//    move on to the next lookahead token, remaining in the error state.
	//
	// First, try the strategy 1. Upon entering the error state, the parser recorded a summary
	// of the previous parse states and their depths. Look at each state in the summary, to see
	// if the current lookahead token would be valid in that state.
	if summary != nil && !lookahead.isError() {
		for _, entry := range *summary {
			if entry.state == errorState {
				continue
			}
			if entry.position.bytes == position.bytes {
				continue
			}
			depth := entry.depth
			if nodeCountSinceError > 0 {
				depth++
			}

			// Do not recover in ways that create redundant stack versions.
			wouldMerge := false
			for j := stackVersion(0); uint32(j) < previousVersionCount; j++ {
				if p.stack.state(j) == entry.state &&
					p.stack.position(j).bytes == position.bytes {
					wouldMerge = true
					break
				}
			}
			if wouldMerge {
				continue
			}

			// Do not recover if the result would clearly be worse than some existing stack version.
			newCost := currentErrorCost +
				entry.depth*errorCostPerSkippedTree +
				(position.bytes-entry.position.bytes)*errorCostPerSkippedChar +
				(position.extent.row-entry.position.extent.row)*errorCostPerSkippedLine
			if p.betterVersionExists(version, false, newCost) {
				break
			}

			// If the current lookahead token is valid in some previous state, recover to that state.
			// Then stop looking for further recoveries.
			if p.language.hasActions(entry.state, lookahead.symbol()) {
				if p.recoverToState(version, depth, entry.state) {
					didRecover = true
					if p.logging() {
						p.logf("recover_to_previous state:%d, depth:%d", entry.state, depth)
					}
					p.logStack()
					break
				}
			}
		}
	}

	// In the process of attempting to recover, some stack versions may have been created
	// and subsequently halted. Remove those versions.
	for i := stackVersion(previousVersionCount); uint32(i) < p.stack.versionCount(); i++ {
		if !p.stack.isActive(i) {
			if p.logging() {
				p.logf("removed paused version:%d", i)
			}
			p.stack.removeVersion(i)
			i--
			p.logStack()
		}
	}

	// If the parser is still in the error state at the end of the file, just wrap everything
	// in an ERROR node and terminate.
	if lookahead.isEOF() {
		if p.logging() {
			p.logf("recover_eof")
		}
		var children subtreeArray
		parent := newErrorNode(&p.treePool, children, false, p.language)
		p.stack.push(version, parent, false, 1)
		p.accept(version, lookahead)
		return
	}

	// If strategy 1 succeeded, a new stack version will have been created which is able to handle
	// the current lookahead token. Now, in addition, try strategy 2 described above: skip the
	// current lookahead token by wrapping it in an ERROR node.

	// Don't pursue this additional strategy if there are already too many stack versions.
	if didRecover && p.stack.versionCount() > maxVersionCount {
		p.stack.halt(version)
		lookahead.release(&p.treePool)
		return
	}

	if didRecover &&
		lookahead.hasExternalScannerStateChange() {
		p.stack.halt(version)
		lookahead.release(&p.treePool)
		return
	}

	// Do not recover if the result would clearly be worse than some existing stack version.
	newCost := currentErrorCost + errorCostPerSkippedTree +
		lookahead.totalBytes()*errorCostPerSkippedChar +
		lookahead.totalSize().extent.row*errorCostPerSkippedLine
	if p.betterVersionExists(version, false, newCost) {
		p.stack.halt(version)
		lookahead.release(&p.treePool)
		return
	}

	// If the current lookahead token is an extra token, mark it as extra. This means it won't
	// be counted in error cost calculations.
	actions := p.language.actions(1, lookahead.symbol())
	if n := len(actions); n > 0 && actions[n-1].Action.Type == abi.ParseActionTypeShift && actions[n-1].Action.Shift.Extra {
		mutableLookahead := lookahead.makeMut(&p.treePool)
		mutableLookahead.setExtra(true)
		lookahead = mutableLookahead
	}

	// Wrap the lookahead token in an ERROR.
	if p.logging() {
		p.logf("skip_token symbol:%s", p.treeName(lookahead))
	}
	children := make(subtreeArray, 0, 1)
	children = append(children, lookahead)
	errorRepeat := newNode(
		&p.treePool,
		builtinSymErrorRepeat,
		children,
		0,
		p.language,
	)

	// If other tokens have already been skipped, so there is already an ERROR at the top of the
	// stack, then pop that ERROR off the stack and wrap the two ERRORs together into one larger
	// ERROR.
	if nodeCountSinceError > 0 {
		pop := p.stack.popCount(version, 1)

		// TODO: Figure out how to make this condition occur.
		// See https://github.com/atom/atom/issues/18450#issuecomment-439579778
		// If multiple stack versions have merged at this point, just pick one of the errors
		// arbitrarily and discard the rest.
		if len(pop) > 1 {
			for i := 1; i < len(pop); i++ {
				pop[i].subtrees.delete(&p.treePool)
			}
			for p.stack.versionCount() > uint32(pop[0].version)+1 {
				p.stack.removeVersion(pop[0].version + 1)
			}
		}

		p.stack.renumberVersion(pop[0].version, version)
		pop[0].subtrees = append(pop[0].subtrees, errorRepeat)
		errorRepeat = newNode(
			&p.treePool,
			builtinSymErrorRepeat,
			pop[0].subtrees,
			0,
			p.language,
		)
	}

	// Push the new ERROR onto the stack.
	p.stack.push(version, errorRepeat, false, errorState)
	if lookahead.hasExternalTokens() {
		p.stack.setLastExternalToken(
			version, lookahead.lastExternalToken(),
		)
	}

	hasError := true
	for i := stackVersion(0); uint32(i) < p.stack.versionCount(); i++ {
		status := p.versionStatus(i)
		if !status.isInError {
			hasError = false
			break
		}
	}
	p.hasError = hasError
}

// handleError is ts_parser__handle_error.
func (p *Parser) handleError(
	version stackVersion,
	lookahead subtree,
) {
	previousVersionCount := p.stack.versionCount()

	// Perform any reductions that can happen in this state, regardless of the lookahead. After
	// skipping one or more invalid tokens, the parser might find a token that would have allowed
	// a reduction to take place.
	p.doAllPotentialReductions(version, 0)
	versionCount := p.stack.versionCount()
	position := p.stack.position(version)

	// Push a discontinuity onto the stack. Merge all of the stack versions that
	// were created in the previous step.
	didInsertMissingToken := false
	for v := version; uint32(v) < versionCount; {
		if !didInsertMissingToken {
			state := p.stack.state(v)
			for missingSymbol := Symbol(1); missingSymbol < Symbol(uint16(p.language.tables.TokenCount)); missingSymbol++ {
				stateAfterMissingSymbol := p.language.NextState(state, missingSymbol)
				if stateAfterMissingSymbol == 0 || stateAfterMissingSymbol == state {
					continue
				}

				if p.language.hasReduceAction(
					stateAfterMissingSymbol,
					lookahead.leafSymbol(),
				) {
					// In case the parser is currently outside of any included range, the lexer will
					// snap to the beginning of the next included range. The missing token's padding
					// must be assigned to position it within the next included range.
					p.lexer.reset(position)
					p.lexer.markEnd()
					padding := p.lexer.tokenEndPosition.sub(position)
					lookaheadBytes := lookahead.totalBytes() + lookahead.lookaheadBytes()

					versionWithMissingTree := p.stack.copyVersion(v)
					missingTree := newMissingLeaf(
						&p.treePool, missingSymbol, state,
						padding, lookaheadBytes,
						p.language,
					)
					p.stack.push(
						versionWithMissingTree,
						missingTree, false,
						stateAfterMissingSymbol,
					)

					if p.doAllPotentialReductions(
						versionWithMissingTree,
						lookahead.leafSymbol(),
					) {
						if p.logging() {
							p.logf(
								"recover_with_missing symbol:%s, state:%d",
								p.symName(missingSymbol),
								p.stack.state(versionWithMissingTree),
							)
						}
						didInsertMissingToken = true
						break
					}
				}
			}
		}

		p.stack.push(v, subtree{}, false, errorState)
		if v == version {
			v = stackVersion(previousVersionCount)
		} else {
			v++
		}
	}

	for i := previousVersionCount; i < versionCount; i++ {
		didMerge := p.stack.merge(version, stackVersion(previousVersionCount))
		assert(didMerge)
	}

	p.stack.recordSummary(version, maxSummaryDepth)

	// Begin recovery with the current lookahead node, rather than waiting for the
	// next turn of the parse loop. This ensures that the tree accounts for the
	// current lookahead token's "lookahead bytes" value, which describes how far
	// the lexer needed to look ahead beyond the content of the token in order to
	// recognize it.
	if lookahead.childCount() > 0 {
		p.breakdownLookahead(&lookahead, errorState, &p.reusableNode)
	}
	p.recover(version, lookahead)

	p.logStack()
}

// checkProgress is ts_parser__check_progress. The context replaces the
// progress callback, so Go does not record the position and hasError for
// it, and it reads the error of the context where C calls the callback.
func (p *Parser) checkProgress(ctx context.Context, lookahead *subtree, operations uint32) bool {
	p.operationCount += operations
	if p.operationCount >= opCountPerParserCallbackCheck {
		p.operationCount = 0
	}
	if p.operationCount == 0 && ctx.Err() != nil {
		if lookahead != nil && lookahead.ptr != nil {
			lookahead.release(&p.treePool)
		}
		return false
	}
	return true
}

// advance is ts_parser__advance.
func (p *Parser) advance(
	ctx context.Context,
	version stackVersion,
	allowNodeReuse bool,
) bool {
	state := p.stack.state(version)
	position := p.stack.position(version).bytes
	lastExternalToken := p.stack.lastExternalToken(version)

	didReuse := true
	lookahead := subtree{}
	var entry tableEntry

	// If possible, reuse a node from the previous syntax tree.
	if allowNodeReuse {
		lookahead = p.reuseNode(
			version, &state, position, lastExternalToken, &entry,
		)
	}

	// If no node from the previous syntax tree could be reused, then try to
	// reuse the token previously returned by the lexer.
	if lookahead.ptr == nil {
		didReuse = false
		lookahead = p.getCachedToken(
			state, position, lastExternalToken, &entry,
		)
	}

	needsLex := lookahead.ptr == nil
	for {
		// Otherwise, re-run the lexer.
		if needsLex {
			needsLex = false
			lookahead = p.lex(version, state)

			if lookahead.ptr != nil {
				p.setCachedToken(position, lastExternalToken, lookahead)
				entry = p.language.tableEntry(state, lookahead.symbol())
			} else {
				// When parsing a non-terminal extra, a null lookahead indicates the
				// end of the rule. The reduction is stored in the EOF table entry.
				// After the reduction, the lexer needs to be run again.
				entry = p.language.tableEntry(state, builtinSymEnd)
			}
		}

		// If a progress callback was provided, then check every
		// time a fixed number of parse actions has been processed.
		if !p.checkProgress(ctx, &lookahead, 1) {
			return false
		}

		// Process each parse action for the current lookahead token in
		// the current state. If there are multiple actions, then this is
		// an ambiguous state. REDUCE actions always create a new stack
		// version, whereas SHIFT actions update the existing stack version
		// and terminate this loop.
		didReduce := false
		lastReductionVersion := stackVersionNone
		for i := range entry.actions {
			action := entry.actions[i].Action

			switch action.Type {
			case abi.ParseActionTypeShift:
				if action.Shift.Repetition {
					break
				}
				var nextState StateID
				if action.Shift.Extra {
					nextState = state
					if p.logging() {
						p.logf("shift_extra")
					}
				} else {
					nextState = StateID(action.Shift.State)
					if p.logging() {
						p.logf("shift state:%d", nextState)
					}
				}

				if lookahead.childCount() > 0 {
					p.breakdownLookahead(&lookahead, state, &p.reusableNode)
					nextState = p.language.NextState(state, lookahead.symbol())
				}

				p.shift(version, nextState, lookahead, action.Shift.Extra)
				if didReuse {
					p.reusableNode.advance()
				}
				return true

			case abi.ParseActionTypeReduce:
				isFragile := len(entry.actions) > 1
				endOfNonTerminalExtra := lookahead.ptr == nil
				if p.logging() {
					p.logf("reduce sym:%s, child_count:%d", p.symName(Symbol(action.Reduce.Symbol)), action.Reduce.ChildCount)
				}
				reductionVersion := p.reduce(
					version, Symbol(action.Reduce.Symbol), uint32(action.Reduce.ChildCount),
					int32(action.Reduce.DynamicPrecedence), action.Reduce.ProductionID,
					isFragile, endOfNonTerminalExtra,
				)
				didReduce = true
				if reductionVersion != stackVersionNone {
					lastReductionVersion = reductionVersion
				}

			case abi.ParseActionTypeAccept:
				if p.logging() {
					p.logf("accept")
				}
				p.accept(version, lookahead)
				return true

			case abi.ParseActionTypeRecover:
				if lookahead.childCount() > 0 {
					p.breakdownLookahead(&lookahead, errorState, &p.reusableNode)
				}

				p.recover(version, lookahead)
				if didReuse {
					p.reusableNode.advance()
				}
				return true
			}
		}

		// If a reduction was performed, then replace the current stack version
		// with one of the stack versions created by a reduction, and continue
		// processing this version of the stack with the same lookahead symbol.
		if lastReductionVersion != stackVersionNone {
			p.stack.renumberVersion(lastReductionVersion, version)
			p.logStack()
			state = p.stack.state(version)

			// At the end of a non-terminal extra rule, the lexer will return a
			// null subtree, because the parser needs to perform a fixed reduction
			// regardless of the lookahead node. After performing that reduction,
			// (and completing the non-terminal extra rule) run the lexer again based
			// on the current parse state.
			if lookahead.ptr == nil {
				needsLex = true
			} else {
				entry = p.language.tableEntry(
					state,
					lookahead.leafSymbol(),
				)
			}

			continue
		}

		// A reduction was performed, but was merged into an existing stack version.
		// This version can be discarded.
		if didReduce {
			if lookahead.ptr != nil {
				lookahead.release(&p.treePool)
			}
			p.stack.halt(version)
			return true
		}

		// If the current lookahead token is a keyword that is not valid, but the
		// default word token *is* valid, then treat the lookahead token as the word
		// token instead.
		if lookahead.isKeyword() &&
			lookahead.symbol() != Symbol(p.language.tables.KeywordCaptureToken) &&
			!p.language.isReservedWord(state, lookahead.symbol()) {
			entry = p.language.tableEntry(
				state,
				Symbol(p.language.tables.KeywordCaptureToken),
			)
			if len(entry.actions) > 0 {
				if p.logging() {
					p.logf(
						"switch from_keyword:%s, to_word_token:%s",
						p.treeName(lookahead),
						p.symName(Symbol(p.language.tables.KeywordCaptureToken)),
					)
				}

				mutableLookahead := lookahead.makeMut(&p.treePool)
				mutableLookahead.setSymbol(Symbol(p.language.tables.KeywordCaptureToken), p.language)
				lookahead = mutableLookahead
				continue
			}
		}

		// If the current lookahead token is not valid and the parser is
		// already in the error state, restart the error recovery process.
		if state == errorState {
			p.recover(version, lookahead)
			return true
		}

		// If the current lookahead token is not valid and the previous subtree on
		// the stack was reused from an old tree, then it wasn't actually valid to
		// reuse that previous subtree. Remove it from the stack, and in its place,
		// push each of its children. Then try again to process the current lookahead.
		if p.breakdownTopOfStack(version) {
			state = p.stack.state(version)
			lookahead.release(&p.treePool)
			needsLex = true
			continue
		}

		// Otherwise, there is definitely an error in this version of the parse stack.
		// Mark this version as paused and continue processing any other stack
		// versions that exist. If some other version advances successfully, then
		// this version can simply be removed. But if all versions end up paused,
		// then error recovery is needed.
		if p.logging() {
			p.logf("detect_error lookahead:%s", p.treeName(lookahead))
		}
		p.stack.pause(version, lookahead)
		return true
	}
}

// condenseStack is ts_parser__condense_stack.
func (p *Parser) condenseStack() uint32 {
	madeChanges := false
	minErrorCost := uint32(math.MaxUint32)
	for i := stackVersion(0); uint32(i) < p.stack.versionCount(); i++ {
		// Prune any versions that have been marked for removal.
		if p.stack.isHalted(i) {
			p.stack.removeVersion(i)
			i--
			continue
		}

		// Keep track of the minimum error cost of any stack version so
		// that it can be returned.
		statusI := p.versionStatus(i)
		if !statusI.isInError && statusI.cost < minErrorCost {
			minErrorCost = statusI.cost
		}

		// Examine each pair of stack versions, removing any versions that
		// are clearly worse than another version. Ensure that the versions
		// are ordered from most promising to least promising.
		for j := stackVersion(0); j < i; j++ {
			statusJ := p.versionStatus(j)

			switch p.compareVersions(statusJ, statusI) {
			case errorComparisonTakeLeft:
				madeChanges = true
				p.stack.removeVersion(i)
				i--
				j = i

			case errorComparisonPreferLeft, errorComparisonNone:
				if p.stack.merge(j, i) {
					madeChanges = true
					i--
					j = i
				}

			case errorComparisonPreferRight:
				madeChanges = true
				if p.stack.merge(j, i) {
					i--
					j = i
				} else {
					p.stack.swapVersions(i, j)
				}

			case errorComparisonTakeRight:
				madeChanges = true
				p.stack.removeVersion(j)
				i--
				j--
			}
		}
	}

	// Enforce a hard upper bound on the number of stack versions by
	// discarding the least promising versions.
	for p.stack.versionCount() > maxVersionCount {
		p.stack.removeVersion(maxVersionCount)
		madeChanges = true
	}

	// If the best-performing stack version is currently paused, or all
	// versions are paused, then resume the best paused version and begin
	// the error recovery process. Otherwise, remove the paused versions.
	if p.stack.versionCount() > 0 {
		hasUnpausedVersion := false
		for i, n := stackVersion(0), stackVersion(p.stack.versionCount()); i < n; i++ {
			if p.stack.isPaused(i) {
				if !hasUnpausedVersion && p.acceptCount < maxVersionCount {
					if p.logging() {
						p.logf("resume version:%d", i)
					}
					minErrorCost = p.stack.errorCost(i)
					lookahead := p.stack.resume(i)
					p.handleError(i, lookahead)
					hasUnpausedVersion = true
				} else {
					p.stack.removeVersion(i)
					madeChanges = true
					i--
					n--
				}
			} else {
				hasUnpausedVersion = true
			}
		}
	}

	if madeChanges {
		if p.logging() {
			p.logf("condense")
		}
		p.logStack()
	}

	return minErrorCost
}

// balanceSubtree is ts_parser__balance_subtree.
func (p *Parser) balanceSubtree(ctx context.Context) bool {
	finishedTree := p.finishedTree

	// If we haven't canceled balancing in progress before, then we want to clear the tree stack and
	// push the initial finished tree onto it. Otherwise, if we're resuming balancing after a
	// cancellation, we don't want to clear the tree stack.
	if !p.canceledBalancing {
		p.treePool.treeStack = p.treePool.treeStack[:0]
		if finishedTree.childCount() > 0 && finishedTree.ptr.refCount.Load() == 1 {
			p.treePool.treeStack = append(p.treePool.treeStack, finishedTree)
		}
	}

	for len(p.treePool.treeStack) > 0 {
		if !p.checkProgress(ctx, nil, 1) {
			return false
		}

		tree := p.treePool.treeStack[len(p.treePool.treeStack)-1]

		if tree.ptr.repeatDepth > 0 {
			child1 := tree.ptr.children[0]
			child2 := tree.ptr.children[len(tree.ptr.children)-1]
			repeatDelta := int64(child1.repeatDepth()) - int64(child2.repeatDepth())
			if repeatDelta > 0 {
				n := uint32(repeatDelta)

				for i := n / 2; i > 0; i /= 2 {
					tree.compress(i, p.language, &p.treePool.treeStack)
					n -= i

					// We scale the operation count increment in `ts_parser__check_progress` proportionately to the compression
					// size since larger values of i take longer to process. Shifting by 4 empirically provides good check
					// intervals (e.g. 193 operations when i=3100) to prevent blocking during large compressions.
					operations := uint8(max(i>>4, 1))
					if !p.checkProgress(ctx, nil, uint32(operations)) {
						return false
					}
				}
			}
		}

		p.treePool.treeStack = p.treePool.treeStack[:len(p.treePool.treeStack)-1]

		for _, child := range tree.ptr.children {
			if child.childCount() > 0 && child.ptr.refCount.Load() == 1 {
				p.treePool.treeStack = append(p.treePool.treeStack, child)
			}
		}
	}

	return true
}

// hasOutstandingParse is ts_parser_has_outstanding_parse.
func (p *Parser) hasOutstandingParse() bool {
	return p.canceledBalancing ||
		p.externalScannerPayload != nil ||
		p.stack.state(0) != 1 ||
		p.stack.nodeCountSinceError(0) != 0
}

// NewParser returns a parser with no language.
//
// NewParser is ts_parser_new.
func NewParser() *Parser {
	p := &Parser{}
	p.lexer.init()
	p.reduceActions = make(reduceActionSet, 0, 4)
	p.treePool = newSubtreePool()
	p.stack = newStack(&p.treePool)
	p.finishedTree = subtree{}
	p.reusableNode = newReusableNode()
	p.dotGraphFile = nil
	p.language = nil
	p.hasError = false
	p.canceledBalancing = false
	p.externalScannerPayload = nil
	p.operationCount = 0
	p.oldTree = subtree{}
	p.includedRangeDifferences = nil
	p.includedRangeDifferenceIndex = 0
	p.setCachedToken(0, subtree{}, subtree{})
	return p
}

// Language returns the language of the parser, or nil.
//
// Language is ts_parser_language.
func (p *Parser) Language() *Language {
	return p.language
}

// SetLanguage sets the language of the parser, and resets the parser. It
// returns ErrIncompatibleLanguage for a language whose ABI version the
// runtime does not accept, or that has no lex function, and the parser then
// has no language. A nil language removes the language of the parser.
//
// SetLanguage is ts_parser_set_language.
func (p *Parser) SetLanguage(language *Language) error {
	p.Reset()
	p.language = nil

	if language != nil {
		if language.tables.ABIVersion > languageVersion ||
			language.tables.ABIVersion < minCompatibleLanguageVersion {
			return ErrIncompatibleLanguage
		}

		// ts_language_is_parseable
		if language.tables.LexFn == nil {
			return ErrIncompatibleLanguage
		}
	}

	p.language = language
	return nil
}

// SetLogger sets the function that gets the log messages of the parser and
// of the lexer. A nil function stops the log.
//
// SetLogger is ts_parser_set_logger.
func (p *Parser) SetLogger(fn func(LogType, string)) {
	p.lexer.logger = fn
}

// PrintDotGraphs writes a graph of the stack and of the trees to w in the
// DOT language of Graphviz, as the parser works. A nil writer stops the
// graphs.
//
// PrintDotGraphs is ts_parser_print_dot_graphs. The C function takes a file
// descriptor.
func (p *Parser) PrintDotGraphs(w io.Writer) {
	p.dotGraphFile = w
}

// SetIncludedRanges sets the ranges of the text that the parser reads. It
// returns ErrInvalidRanges for ranges that are not in order or that overlap.
// No ranges is the whole text.
//
// SetIncludedRanges is ts_parser_set_included_ranges.
func (p *Parser) SetIncludedRanges(ranges []Range) error {
	internal := make([]textRange, len(ranges))
	for i, r := range ranges {
		internal[i] = r.internal()
	}
	if !p.lexer.setIncludedRanges(internal) {
		return ErrInvalidRanges
	}
	return nil
}

// IncludedRanges returns a copy of the ranges of the text that the parser
// reads.
//
// IncludedRanges is ts_parser_included_ranges.
func (p *Parser) IncludedRanges() []Range {
	ranges := p.lexer.getIncludedRanges()
	out := make([]Range, len(ranges))
	for i, r := range ranges {
		out[i] = r.public()
	}
	return out
}

// Reset makes the next parse start from the start of the text, and not go
// on from a parse that a context stopped.
//
// Reset is ts_parser_reset.
func (p *Parser) Reset() {
	p.externalScannerDestroy()

	if p.oldTree.ptr != nil {
		p.oldTree.release(&p.treePool)
		p.oldTree = subtree{}
	}

	p.reusableNode.clear()
	p.lexer.reset(lengthZero())
	p.stack.clear()
	p.setCachedToken(0, subtree{}, subtree{})
	if p.finishedTree.ptr != nil {
		p.finishedTree.release(&p.treePool)
		p.finishedTree = subtree{}
	}
	p.acceptCount = 0
	p.hasError = false
	p.canceledBalancing = false
}

// ParseInput parses the text that in gives, in the encoding enc. When old is
// the tree of an earlier version of the text, after Tree.Edit, the parser
// reuses the parts of it that the edits did not change.
//
// When ctx ends, ParseInput returns its error, wrapped. The parser keeps its
// state, and the next call goes on from the same point with the same input,
// unless Reset or SetLanguage is called first.
//
// ParseInput is ts_parser_parse.
func (p *Parser) ParseInput(ctx context.Context, in Input, enc Encoding, old *Tree) (*Tree, error) {
	switch {
	case p.language == nil:
		return nil, ErrNoLanguage
	case in == nil || enc < EncodingUTF8 || enc > EncodingUTF16BE:
		return nil, ErrInvalidInput
	case old != nil && old.language != p.language:
		return nil, ErrLanguageMismatch
	}

	p.lexer.setInput(input{read: in, encoding: enc})
	p.includedRangeDifferences = p.includedRangeDifferences[:0]
	p.includedRangeDifferenceIndex = 0

	p.operationCount = 0

	// balance is the label balance of C: a parse whose balancing a context
	// stopped goes on with the balancing.
	balance := false
	if p.hasOutstandingParse() {
		if p.logging() {
			p.logf("resume_parsing")
		}
		balance = p.canceledBalancing
	} else {
		p.externalScannerCreate()

		if old != nil {
			old.root.retain()
			p.oldTree = old.root
			rangeArrayGetChangedRanges(
				old.includedRanges,
				p.lexer.includedRanges,
				&p.includedRangeDifferences,
			)
			p.reusableNode.reset(old.root)
			if p.logging() {
				p.logf("parse_after_edit")
			}
			p.logTree(p.oldTree)
			for _, r := range p.includedRangeDifferences {
				if p.logging() {
					p.logf("different_included_range %d - %d", r.startByte, r.endByte)
				}
			}
		} else {
			p.reusableNode.clear()
			if p.logging() {
				p.logf("new_parse")
			}
		}
	}

	if !balance {
		var position, lastPosition, versionCount uint32
		for {
			for version := stackVersion(0); ; version++ {
				versionCount = p.stack.versionCount()
				if uint32(version) >= versionCount {
					break
				}
				allowNodeReuse := versionCount == 1
				for p.stack.isActive(version) {
					if p.logging() {
						p.logf(
							"process version:%d, version_count:%d, state:%d, row:%d, col:%d",
							version,
							p.stack.versionCount(),
							p.stack.state(version),
							p.stack.position(version).extent.row,
							p.stack.position(version).extent.column,
						)
					}

					if !p.advance(ctx, version, allowNodeReuse) {
						return nil, fmt.Errorf("parsing: %w", ctx.Err())
					}

					p.logStack()

					position = p.stack.position(version).bytes
					if position > lastPosition || (version > 0 && position == lastPosition) {
						lastPosition = position
						break
					}
				}
			}

			// After advancing each version of the stack, re-sort the versions by their cost,
			// removing any versions that are no longer worth pursuing.
			minErrorCost := p.condenseStack()

			// If there's already a finished parse tree that's better than any in-progress version,
			// then terminate parsing. Clear the parse stack to remove any extra references to subtrees
			// within the finished tree, ensuring that these subtrees can be safely mutated in-place
			// for rebalancing.
			if p.finishedTree.ptr != nil && p.finishedTree.errorCost() < minErrorCost {
				p.stack.clear()
				break
			}

			for int(p.includedRangeDifferenceIndex) < len(p.includedRangeDifferences) {
				r := &p.includedRangeDifferences[p.includedRangeDifferenceIndex]
				if r.endByte <= position {
					p.includedRangeDifferenceIndex++
				} else {
					break
				}
			}

			if versionCount == 0 {
				break
			}
		}
	}

	assert(p.finishedTree.ptr != nil)
	if !p.balanceSubtree(ctx) {
		p.canceledBalancing = true
		return nil, fmt.Errorf("parsing: %w", ctx.Err())
	}
	p.canceledBalancing = false
	if p.logging() {
		p.logf("done")
	}
	p.logTree(p.finishedTree)

	result := newTree(
		p.finishedTree,
		p.language,
		p.lexer.includedRanges,
	)
	p.finishedTree = subtree{}

	p.Reset()
	return result, nil
}

// Parse parses src, in UTF-8. When old is the tree of an earlier version of
// the text, after Tree.Edit, the parser reuses the parts of it that the
// edits did not change. ParseInput holds the rest of the rules.
//
// Parse is ts_parser_parse_string.
func (p *Parser) Parse(ctx context.Context, src []byte, old *Tree) (*Tree, error) {
	return p.ParseInput(ctx, stringInput(src), EncodingUTF8, old)
}
