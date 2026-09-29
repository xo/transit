package generate

import (
	"slices"
	"strconv"
)

// This file ports crates/generate/src/prepare_grammar/expand_repeats.rs: the
// pass that replaces each repeat with an auxiliary rule of a binary tree.

// repeatExpander replaces the repeats of the rules with auxiliary rules.
//
// repeatExpander is Expander.
type repeatExpander struct {
	// preceding is the number of variables before the auxiliary rules.
	preceding int
	aux       []Variable
	// memo holds the content and the symbol of each auxiliary rule, by the
	// hash of the content. The hash only narrows the search, and
	// SubtreeEqual decides.
	memo      map[uint64][]repeatMemo
	stack     []repeatTask
	zeroWidth *zeroWidth
}

// repeatMemo is the content of an auxiliary rule and its symbol.
type repeatMemo struct {
	node   RuleID
	symbol Symbol
}

// repeatTask is a step of the walk of expandRoot: a visit of a node, or the
// expansion of a repeat after its content.
//
// repeatTask is Task, an enum with data upstream. content is used only when
// expand is true.
type repeatTask struct {
	id      RuleID
	content RuleID
	expand  bool
}

// ExpandRepeatsError is the error of a repeat that can match the empty
// string at the end of the input.
//
// ExpandRepeatsError is ExpandRepeatsError.
type ExpandRepeatsError struct {
	Rule string
}

// Error returns the text of the error.
func (e *ExpandRepeatsError) Error() string {
	return "Rule `" + e.Rule + "` contains a repetition that can match the empty string at end of input"
}

// expandRoot expands the repeats of one rule, in post-order, so that the
// children expand first. It does not go into a reserved node.
//
// expandRoot is Expander::expand_root.
func (e *repeatExpander) expandRoot(pool *RulePool, root RuleID, varName StrID, auxRepeatCounter *uint32) error {
	e.stack = append(e.stack[:0], repeatTask{id: root})
walk:
	for len(e.stack) > 0 {
		task := e.stack[len(e.stack)-1]
		e.stack = e.stack[:len(e.stack)-1]
		if !task.expand {
			n := pool.Node(task.id)
			switch n.Kind {
			case RuleRepeat:
				e.stack = append(e.stack, repeatTask{id: task.id, content: n.Child, expand: true}, repeatTask{id: n.Child})
			case RuleSeq, RuleChoice:
				// go into the children of a choice, a seq or a metadata node,
				// and replace the repeats in them
				base := len(e.stack)
				for _, c := range pool.ChildSlice(n.Children) {
					e.stack = append(e.stack, repeatTask{id: c})
				}
				slices.Reverse(e.stack[base:])
			case RuleMetadata:
				e.stack = append(e.stack, repeatTask{id: n.Child})
			}
			// a primitive rule stays as it is
			continue
		}

		id, content := task.id, task.content
		width := e.zeroWidth.eval(pool, content)
		if width.eofNullable {
			return &ExpandRepeatsError{Rule: pool.Resolve(varName)}
		}
		// A repeat becomes an auxiliary rule that holds the repeated content,
		// or a binary tree of itself.
		hash := pool.SubtreeHash(content)
		for _, m := range e.memo[hash] {
			if pool.SubtreeEqual(m.node, content) {
				pool.SetNode(id, Rule{Kind: RuleSym, Sym: m.symbol})
				continue walk
			}
		}
		*auxRepeatCounter++
		name := pool.Intern(pool.Resolve(varName) + "_repeat" + strconv.FormatUint(uint64(*auxRepeatCounter), 10))
		// The auxiliary rules come after the original variables, so their
		// indices start at preceding. The auxiliary rule stands for
		// Repeat(content), which matches zero width exactly when content
		// does.
		symbol := NonTerminalSymbol(e.preceding + len(e.aux))
		e.zeroWidth.pushVariable(width)
		if e.memo == nil {
			e.memo = map[uint64][]repeatMemo{}
		}
		e.memo[hash] = append(e.memo[hash], repeatMemo{node: content, symbol: symbol})
		auxRoot := wrapInBinaryTree(pool, symbol, content)
		e.aux = append(e.aux, Variable{Name: name, Root: auxRoot})
		pool.SetNode(id, Rule{Kind: RuleSym, Sym: symbol})
	}
	return nil
}

// width is whether a rule can match zero characters, without an eof() and
// through one.
//
// width is Width.
type width struct {
	nullable    bool
	eofNullable bool
}

// merge joins other into w, and reports whether that made a field true.
//
// merge is Width::merge.
func (w *width) merge(other width) bool {
	before := *w
	w.nullable = w.nullable || other.nullable
	w.eofNullable = w.eofNullable || other.eofNullable
	return *w != before
}

// zeroWidthVisit is a step of the walk of eval: the entry into a node, or the
// exit from it.
//
// zeroWidthVisit is Visit, an enum with data upstream.
type zeroWidthVisit struct {
	id   RuleID
	exit bool
}

// zeroWidth finds the rules that match zero width. byVariable starts from a
// fixed point over the whole grammar before any expansion, so the results do
// not depend on the order of the rules.
//
// zeroWidth is ZeroWidth.
type zeroWidth struct {
	byVariable []width
	stack      []zeroWidthVisit
	values     []width
}

// newZeroWidth returns the widths of every rule, as a least fixed point. The
// rules can refer to each other in a cycle, so every rule starts as "matches
// nothing", and every rule is evaluated again until a pass changes nothing.
// A rule that reaches itself reads "matches nothing", so only a path that
// matches zero width sets a field.
//
// newZeroWidth is ZeroWidth::new.
func newZeroWidth(pool *RulePool, variables []Variable) *zeroWidth {
	z := &zeroWidth{byVariable: make([]width, len(variables))}
	for {
		changed := false
		for i, v := range variables {
			w := z.eval(pool, v.Root)
			if z.byVariable[i].merge(w) {
				changed = true
			}
		}
		if !changed {
			return z
		}
	}
}

// pushVariable records the width of a new auxiliary rule.
//
// pushVariable is ZeroWidth::push_variable.
func (z *zeroWidth) pushVariable(w width) {
	z.byVariable = append(z.byVariable, w)
}

// eval returns the width of the rule at root. It walks the rule in
// post-order, and joins the widths of the children into their parent.
//
// eval is ZeroWidth::eval.
func (z *zeroWidth) eval(pool *RulePool, root RuleID) width {
	z.stack = append(z.stack[:0], zeroWidthVisit{id: root})
	z.values = z.values[:0]
	for len(z.stack) > 0 {
		step := z.stack[len(z.stack)-1]
		z.stack = z.stack[:len(z.stack)-1]
		n := pool.Node(step.id)
		if !step.exit {
			switch n.Kind {
			case RuleSeq, RuleChoice:
				z.stack = append(z.stack, zeroWidthVisit{id: step.id, exit: true})
				for _, c := range pool.ChildSlice(n.Children) {
					z.stack = append(z.stack, zeroWidthVisit{id: c})
				}
			case RuleRepeat, RuleMetadata, RuleReserved:
				z.stack = append(z.stack, zeroWidthVisit{id: step.id, exit: true}, zeroWidthVisit{id: n.Child})
			case RuleBlank:
				z.values = append(z.values, width{nullable: true})
			case RuleString:
				z.values = append(z.values, width{nullable: n.Str == EmptyStrID})
			case RuleEOF:
				z.values = append(z.values, width{eofNullable: true})
			case RuleSym:
				var w width
				switch n.Sym.Kind() {
				case SymbolEnd:
					w = width{eofNullable: true}
				case SymbolNonTerminal:
					if index, _ := n.Sym.NonTerminalIndex(); int(index) < len(z.byVariable) {
						w = z.byVariable[index]
					}
				case SymbolExternal:
					// An external scanner decides at run time how far to
					// advance, and it can return a token of zero width, so
					// an external token is nullable. A scanner can check
					// lexer->eof, but nothing here can know that, and to
					// assume it would reject every grammar that repeats an
					// external token.
					w = width{nullable: true}
				case SymbolTerminal:
					// expand_tokens rejects a token that matches the empty
					// string
				case SymbolEndOfNonTerminalExtra:
					// build_parse_table adds this marker for a non-terminal
					// extra after this pass
					panic("generate: an end of a non-terminal extra before the parse table")
				}
				z.values = append(z.values, w)
			default:
				// extract_tokens moves every pattern to the lexical grammar,
				// and intern_symbols resolves every named symbol, so the
				// walk meets neither
				panic("generate: a pattern or a named symbol after the tokens are extracted")
			}
			continue
		}

		switch n.Kind {
		case RuleChoice:
			base := len(z.values) - int(n.Children.Len)
			var w width
			for _, child := range z.values[base:] {
				w.nullable = w.nullable || child.nullable
				w.eofNullable = w.eofNullable || child.eofNullable
			}
			z.values = append(z.values[:base], w)
		case RuleSeq:
			base := len(z.values) - int(n.Children.Len)
			// A seq matches the empty string when every element does, and it
			// matches through eof() when an element does and every element
			// matches the empty string or eof().
			nullable, anyEOF, allZeroWidth := true, false, true
			for _, child := range z.values[base:] {
				nullable = nullable && child.nullable
				anyEOF = anyEOF || child.eofNullable
				allZeroWidth = allZeroWidth && (child.nullable || child.eofNullable)
			}
			z.values = append(z.values[:base], width{nullable: nullable, eofNullable: allZeroWidth && anyEOF})
		case RuleRepeat, RuleMetadata, RuleReserved:
			// Each wraps one child, whose width is on the stack and is the
			// width of the wrapper. A repeat is one or more, and a repeat of
			// zero or more comes as Choice(Repeat, Blank) from parse_grammar.
			// A metadata node changes no width, and a reserved node only
			// names a set of reserved words.
		default:
			// any other rule is a leaf, and its entry pushed its width
			panic("generate: the exit from a leaf rule")
		}
	}
	if len(z.values) == 0 {
		return width{}
	}
	w := z.values[len(z.values)-1]
	z.values = z.values[:len(z.values)-1]
	return w
}

// wrapInBinaryTree returns the body of an auxiliary rule of a repeat,
// choice(seq(symbol, symbol), inner). The choices of inner are flattened
// into it, and an element equal to an earlier one is dropped.
//
// wrapInBinaryTree is wrap_in_binary_tree.
func wrapInBinaryTree(pool *RulePool, symbol Symbol, inner RuleID) RuleID {
	s1 := pool.PushNode(Rule{Kind: RuleSym, Sym: symbol})
	s2 := pool.PushNode(Rule{Kind: RuleSym, Sym: symbol})
	seq := pool.PushNode(Rule{Kind: RuleSeq, Children: pool.PushChildren([]RuleID{s1, s2})})
	elements := []RuleID{seq}
	stack := []RuleID{inner}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n := pool.Node(id); n.Kind == RuleChoice {
			base := len(stack)
			stack = append(stack, pool.ChildSlice(n.Children)...)
			slices.Reverse(stack[base:])
		} else if !slices.ContainsFunc(elements, func(e RuleID) bool { return pool.SubtreeEqual(e, id) }) {
			elements = append(elements, id)
		}
	}
	if len(elements) == 1 {
		return elements[0]
	}
	return pool.PushNode(Rule{Kind: RuleChoice, Children: pool.PushChildren(elements)})
}

// expandRepeats replaces each repeat of the grammar with a symbol of an
// auxiliary rule, which it adds after the variables.
//
// expandRepeats is expand_repeats.
func expandRepeats(g *InputGrammar, meta *extractedGrammarMeta) error {
	expander := &repeatExpander{
		preceding: len(g.Variables),
		zeroWidth: newZeroWidth(g.Pool, g.Variables),
	}
	for i := range g.Variables {
		name, root := g.Variables[i].Name, g.Variables[i].Root
		var auxRepeatCount uint32

		// A hidden variable whose rule is a repeat becomes its own binary
		// tree, and gains no auxiliary rule. It can no longer be inlined.
		if n := g.Pool.Node(root); meta.kinds[i] == VariableHidden && n.Kind == RuleRepeat {
			content := n.Child
			if expander.zeroWidth.eval(g.Pool, content).eofNullable {
				return &ExpandRepeatsError{Rule: g.Pool.Resolve(name)}
			}
			if err := expander.expandRoot(g.Pool, content, name, &auxRepeatCount); err != nil {
				return err
			}
			g.Variables[i].Root = wrapInBinaryTree(g.Pool, NonTerminalSymbol(i), content)
			meta.kinds[i] = VariableAuxiliary
			meta.inline = slices.DeleteFunc(meta.inline, func(s Symbol) bool { return s == NonTerminalSymbol(i) })
			continue
		}

		if err := expander.expandRoot(g.Pool, root, name, &auxRepeatCount); err != nil {
			return err
		}
	}
	for _, v := range expander.aux {
		g.Variables = append(g.Variables, v)
		meta.kinds = append(meta.kinds, VariableAuxiliary)
	}
	return nil
}
