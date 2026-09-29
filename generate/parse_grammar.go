package generate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// This file ports crates/generate/src/parse_grammar.rs, which reads
// grammar.json into an InputGrammar.
//
// Upstream decodes the objects rules and reserved with the feature
// preserve_order of serde_json, so they keep the order of the document. The
// first rule is the start rule, and the order of interning follows the order of
// the rules, so the port decodes both objects in order too. See orderedObject.

// ruleJSON is a rule of grammar.json. The field type names the kind of rule,
// and each kind uses some of the other fields.
//
// ruleJSON is RuleJSON, an enum that serde tags with the field type.
type ruleJSON struct {
	Type        string            `json:"type"`
	Content     json.RawMessage   `json:"content"`
	Members     []json.RawMessage `json:"members"`
	Named       *bool             `json:"named"`
	Value       json.RawMessage   `json:"value"`
	Flags       *string           `json:"flags"`
	Name        *string           `json:"name"`
	ContextName *string           `json:"context_name"`
}

// grammarJSON is grammar.json.
//
// grammarJSON is GrammarJSON.
type grammarJSON struct {
	Name        *string             `json:"name"`
	Rules       json.RawMessage     `json:"rules"`
	Precedences [][]json.RawMessage `json:"precedences"`
	Conflicts   [][]string          `json:"conflicts"`
	Externals   []json.RawMessage   `json:"externals"`
	Extras      []json.RawMessage   `json:"extras"`
	Inline      []string            `json:"inline"`
	Supertypes  []string            `json:"supertypes"`
	Word        *string             `json:"word"`
	Reserved    json.RawMessage     `json:"reserved"`
}

// ParseGrammarErrorKind is the kind of an error of ParseGrammar.
type ParseGrammarErrorKind uint8

// The kinds of error, in the order of upstream.
const (
	ParseGrammarSerialization ParseGrammarErrorKind = iota
	ParseGrammarInvalidExtra
	ParseGrammarUnexpected
	ParseGrammarInvalidReservedWordSet
	ParseGrammarUnexpectedRule
)

// ParseGrammarError is an error of ParseGrammar. Its text is the text of
// upstream, except for a JSON error, whose text comes from encoding/json.
//
// ParseGrammarError is ParseGrammarError.
type ParseGrammarError struct {
	Kind ParseGrammarErrorKind
	// Detail is the text of a JSON error, or the name of the rule of
	// ParseGrammarUnexpectedRule.
	Detail string
}

// Error returns the text of the error.
func (e *ParseGrammarError) Error() string {
	switch e.Kind {
	case ParseGrammarInvalidExtra:
		return "Rules in the `extras` array must not contain empty strings"
	case ParseGrammarUnexpected:
		return "Invalid rule in precedences array. Only strings and symbols are allowed"
	case ParseGrammarInvalidReservedWordSet:
		return "Reserved word sets must be arrays"
	case ParseGrammarUnexpectedRule:
		return "Grammar Error: Unexpected rule `" + e.Detail + "` in `token()` call"
	}
	return e.Detail
}

// serializationError returns a ParseGrammarError for a JSON error.
func serializationError(err error) error {
	return &ParseGrammarError{Kind: ParseGrammarSerialization, Detail: err.Error()}
}

// normalize drops the rules that nothing uses, and removes each reference to
// them from conflicts, supertypes, inline, extras, externals and precedences.
//
// A variable is used when it is the start rule, the word token, named in
// extras or externals, or reached from any of these through references.
//
// normalize is InputGrammar::normalize.
func (g *InputGrammar) normalize(diagnostics *[]Diagnostic) {
	p := g.Pool
	// Find the used set with a walk from the roots: the start rule, the word,
	// and the references in extras and externals. A symbol that extras names
	// at its top counts as a use, so that naming a rule in extras keeps it. An
	// entry of externals does not, because the entry is the rule itself.
	byName := make(map[StrID]RuleID, len(g.Variables))
	for _, v := range g.Variables {
		byName[v.Name] = v.Root
	}
	used := map[StrID]bool{}
	var stack []StrID
	if len(g.Variables) > 0 {
		stack = append(stack, g.Variables[0].Name)
	}
	if g.WordName != 0 {
		stack = append(stack, g.WordName)
	}
	for _, root := range g.ExtraRoots {
		p.CollectReferencedIDs(root, false, &stack)
	}
	for _, root := range g.ExternalRoots {
		p.CollectReferencedIDs(root, true, &stack)
	}
	for _, set := range g.ReservedSets {
		for _, root := range set.Roots {
			p.CollectReferencedIDs(root, false, &stack)
		}
	}
	for len(stack) > 0 {
		name := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if used[name] {
			continue
		}
		used[name] = true
		if root, ok := byName[name]; ok {
			p.CollectReferencedIDs(root, false, &stack)
		}
	}

	for _, v := range g.Variables {
		if !used[v.Name] {
			continue
		}
		if !slices.ContainsFunc(g.ExtraRoots, func(r RuleID) bool { return p.RuleIsReferenced(r, v.Name, false) }) {
			continue
		}
		inner := v.Root
		if n := p.Node(v.Root); n.Kind == RuleMetadata {
			inner = n.Child
		}
		var matchesEmpty bool
		switch n := p.Node(inner); n.Kind {
		case RuleString:
			matchesEmpty = p.Resolve(n.Str) == ""
		case RulePattern:
			// Upstream compiles the pattern with the Rust crate regex. The
			// result only decides a warning, and Go's regexp stands in for it.
			if re, err := regexp.Compile(p.Resolve(n.Str)); err == nil {
				matchesEmpty = re.MatchString("")
			}
		}
		if matchesEmpty {
			*diagnostics = append(*diagnostics, Diagnostic{Kind: DiagnosticEmptyStringMatch, Name: p.Resolve(v.Name)})
		}
	}

	// Drop the unused variables, and remove the references to them.
	var dropped []StrID
	for _, v := range g.Variables {
		if !used[v.Name] {
			dropped = append(dropped, v.Name)
		}
	}
	g.Variables = slices.DeleteFunc(g.Variables, func(v Variable) bool { return !used[v.Name] })
	for _, name := range dropped {
		g.ConflictNames = slices.DeleteFunc(g.ConflictNames, func(c []StrID) bool { return slices.Contains(c, name) })
		g.SupertypeNames = slices.DeleteFunc(g.SupertypeNames, func(s StrID) bool { return s == name })
		g.InlineNames = slices.DeleteFunc(g.InlineNames, func(s StrID) bool { return s == name })
		g.ExtraRoots = slices.DeleteFunc(g.ExtraRoots, func(r RuleID) bool { return p.RuleIsReferenced(r, name, true) })
		g.ExternalRoots = slices.DeleteFunc(g.ExternalRoots, func(r RuleID) bool { return p.RuleIsReferenced(r, name, true) })
		g.PrecedenceOrderings = slices.DeleteFunc(g.PrecedenceOrderings, func(o []PrecedenceEntry) bool {
			return slices.ContainsFunc(o, func(e PrecedenceEntry) bool { return e.Kind == PrecedenceEntrySymbol && e.Value == name })
		})
		// Prune the entries, and keep the context: an empty context of
		// reserved words is a marker that a rule can name.
		for i := range g.ReservedSets {
			g.ReservedSets[i].Roots = slices.DeleteFunc(g.ReservedSets[i].Roots, func(r RuleID) bool { return p.RuleIsReferenced(r, name, false) })
		}
	}
}

// ParseGrammar reads grammar.json into an InputGrammar, and appends the
// warnings that it finds to diagnostics.
//
// ParseGrammar is parse_grammar.
func ParseGrammar(input []byte, diagnostics *[]Diagnostic) (*InputGrammar, error) {
	var gj grammarJSON
	if err := json.Unmarshal(input, &gj); err != nil {
		return nil, serializationError(err)
	}
	if gj.Name == nil {
		return nil, serializationError(errors.New("missing field `name`"))
	}
	if gj.Rules == nil {
		return nil, serializationError(errors.New("missing field `rules`"))
	}
	p := NewRulePool()

	var extraRoots []RuleID
	for _, item := range gj.Extras {
		root, err := p.parseRuleJSON(item, false, diagnostics)
		if err != nil {
			return nil, err
		}
		if n := p.Node(root); n.Kind == RuleString && p.Resolve(n.Str) == "" {
			return nil, &ParseGrammarError{Kind: ParseGrammarInvalidExtra}
		}
		extraRoots = append(extraRoots, root)
	}

	var externalRoots []RuleID
	for _, e := range gj.Externals {
		root, err := p.parseRuleJSON(e, false, diagnostics)
		if err != nil {
			return nil, err
		}
		externalRoots = append(externalRoots, root)
	}

	orderings := make([][]PrecedenceEntry, 0, len(gj.Precedences))
	for _, list := range gj.Precedences {
		ordering := make([]PrecedenceEntry, 0, len(list))
		for _, raw := range list {
			r, err := decodeRule(raw)
			if err != nil {
				return nil, err
			}
			switch {
			case r.Type == "STRING" && r.Value != nil:
				var s string
				if err := json.Unmarshal(r.Value, &s); err != nil {
					return nil, serializationError(err)
				}
				ordering = append(ordering, PrecedenceEntry{Kind: PrecedenceEntryName, Value: p.Intern(s)})
			case r.Type == "SYMBOL" && r.Name != nil:
				ordering = append(ordering, PrecedenceEntry{Kind: PrecedenceEntrySymbol, Value: p.Intern(*r.Name)})
			default:
				return nil, &ParseGrammarError{Kind: ParseGrammarUnexpected}
			}
		}
		orderings = append(orderings, ordering)
	}

	rules, err := orderedObject(gj.Rules)
	if err != nil {
		return nil, serializationError(err)
	}
	variables := make([]Variable, 0, len(rules))
	for _, r := range rules {
		name := p.Intern(r.key)
		root, err := p.parseRuleJSON(r.value, false, diagnostics)
		if err != nil {
			return nil, err
		}
		variables = append(variables, Variable{Name: name, Root: root})
	}

	var reservedSets []ReservedWordContext
	if gj.Reserved != nil {
		sets, err := orderedObject(gj.Reserved)
		if err != nil {
			return nil, serializationError(err)
		}
		for _, set := range sets {
			var values []json.RawMessage
			if !bytes.HasPrefix(bytes.TrimSpace(set.value), []byte("[")) {
				return nil, &ParseGrammarError{Kind: ParseGrammarInvalidReservedWordSet}
			}
			if err := json.Unmarshal(set.value, &values); err != nil {
				return nil, serializationError(err)
			}
			name := p.Intern(set.key)
			roots := make([]RuleID, 0, len(values))
			for _, v := range values {
				root, err := p.parseRuleJSON(v, false, diagnostics)
				if err != nil {
					return nil, err
				}
				roots = append(roots, root)
			}
			reservedSets = append(reservedSets, ReservedWordContext{Name: name, Roots: roots})
		}
	}

	supertypes := make([]StrID, 0, len(gj.Supertypes))
	for _, s := range gj.Supertypes {
		supertypes = append(supertypes, p.Intern(s))
	}
	conflicts := make([][]StrID, 0, len(gj.Conflicts))
	for _, c := range gj.Conflicts {
		names := make([]StrID, 0, len(c))
		for _, n := range c {
			names = append(names, p.Intern(n))
		}
		conflicts = append(conflicts, names)
	}
	inline := make([]StrID, 0, len(gj.Inline))
	for _, n := range gj.Inline {
		inline = append(inline, p.Intern(n))
	}
	var word StrID
	if gj.Word != nil {
		word = p.Intern(*gj.Word)
	}
	name := p.Intern(*gj.Name)

	g := &InputGrammar{
		Pool:                p,
		Name:                name,
		Variables:           variables,
		ExternalRoots:       externalRoots,
		ExtraRoots:          extraRoots,
		ReservedSets:        reservedSets,
		SupertypeNames:      supertypes,
		ConflictNames:       conflicts,
		InlineNames:         inline,
		WordName:            word,
		PrecedenceOrderings: orderings,
	}
	g.normalize(diagnostics)
	return g, nil
}

// decodeRule decodes one rule of grammar.json, without its children.
func decodeRule(raw json.RawMessage) (ruleJSON, error) {
	var r ruleJSON
	if err := json.Unmarshal(raw, &r); err != nil {
		return r, serializationError(err)
	}
	if r.Type == "" {
		return r, serializationError(errors.New("missing field `type`"))
	}
	return r, nil
}

// parseRuleJSON decodes one rule of grammar.json and adds it to the pool.
func (p *RulePool) parseRuleJSON(raw json.RawMessage, isToken bool, diagnostics *[]Diagnostic) (RuleID, error) {
	r, err := decodeRule(raw)
	if err != nil {
		return 0, err
	}
	return p.parseRule(r, isToken, diagnostics)
}

// missing returns the error of serde for a missing field of a kind of rule.
func missing(field string) error {
	return serializationError(fmt.Errorf("missing field `%s`", field))
}

// parseRule adds one rule to the pool, and its children first. Inside a token,
// isToken is true, and a rule may not name another rule.
//
// parseRule is RulePool::parse_rule.
func (p *RulePool) parseRule(r ruleJSON, isToken bool, diagnostics *[]Diagnostic) (RuleID, error) {
	content := func() (RuleID, error) {
		if r.Content == nil {
			return 0, missing("content")
		}
		return p.parseRuleJSON(r.Content, isToken, diagnostics)
	}
	switch r.Type {
	case "ALIAS":
		c, err := content()
		if err != nil {
			return 0, err
		}
		if r.Named == nil {
			return 0, missing("named")
		}
		value, err := stringValue(r.Value)
		if err != nil {
			return 0, err
		}
		return p.Alias(c, p.Intern(value), *r.Named), nil
	case "BLANK":
		return p.Blank(), nil
	case "PATTERN":
		value, err := stringValue(r.Value)
		if err != nil {
			return 0, err
		}
		var kept strings.Builder
		if r.Flags != nil {
			for _, c := range *r.Flags {
				switch c {
				case 'i':
					kept.WriteRune(c)
				case 'u', 'v':
					// the unicode flags are ignored with no warning
				default:
					*diagnostics = append(*diagnostics, Diagnostic{Kind: DiagnosticUnsupportedRegexFlag, Flag: c, Pattern: value})
				}
			}
		}
		v := p.Intern(value)
		f := p.Intern(kept.String())
		return p.Pattern(v, f), nil
	case "SYMBOL":
		if r.Name == nil {
			return 0, missing("name")
		}
		if isToken {
			return 0, &ParseGrammarError{Kind: ParseGrammarUnexpectedRule, Detail: *r.Name}
		}
		return p.NamedSymbol(p.Intern(*r.Name)), nil
	case "CHOICE":
		if r.Members == nil {
			return 0, missing("members")
		}
		members := make([]RuleID, 0, len(r.Members))
		for _, m := range r.Members {
			id, err := p.parseRuleJSON(m, isToken, diagnostics)
			if err != nil {
				return 0, err
			}
			members = append(members, id)
		}
		return p.Choice(members), nil
	case "SEQ":
		if r.Members == nil {
			return 0, missing("members")
		}
		return TrySeq(p, r.Members, func(p *RulePool, m json.RawMessage) (RuleID, error) {
			return p.parseRuleJSON(m, isToken, diagnostics)
		})
	case "FIELD":
		c, err := content()
		if err != nil {
			return 0, err
		}
		if r.Name == nil {
			return 0, missing("name")
		}
		return p.Field(p.Intern(*r.Name), c), nil
	case "REPEAT":
		c, err := content()
		if err != nil {
			return 0, err
		}
		repeat := p.Repeat(c)
		blank := p.Blank()
		// Neither member is a Choice, and they differ, so the flattening of
		// Choice is not needed.
		children := p.PushChildren([]RuleID{repeat, blank})
		return p.PushNode(Rule{Kind: RuleChoice, Children: children}), nil
	case "REPEAT1":
		c, err := content()
		if err != nil {
			return 0, err
		}
		return p.Repeat(c), nil
	case "PREC", "PREC_LEFT", "PREC_RIGHT":
		c, err := content()
		if err != nil {
			return 0, err
		}
		value, err := p.precedenceValue(r.Value)
		if err != nil {
			return 0, err
		}
		switch r.Type {
		case "PREC_LEFT":
			return p.PrecLeft(value, c), nil
		case "PREC_RIGHT":
			return p.PrecRight(value, c), nil
		}
		return p.Prec(value, c), nil
	case "PREC_DYNAMIC":
		c, err := content()
		if err != nil {
			return 0, err
		}
		n, err := int32Value(r.Value)
		if err != nil {
			return 0, err
		}
		return p.PrecDynamic(n, c), nil
	case "RESERVED":
		c, err := content()
		if err != nil {
			return 0, err
		}
		if r.ContextName == nil {
			return 0, missing("context_name")
		}
		return p.Reserved(c, p.Intern(*r.ContextName)), nil
	case "TOKEN":
		if r.Content == nil {
			return 0, missing("content")
		}
		c, err := p.parseRuleJSON(r.Content, true, diagnostics)
		if err != nil {
			return 0, err
		}
		return p.Token(c), nil
	case "IMMEDIATE_TOKEN":
		if r.Content == nil {
			return 0, missing("content")
		}
		c, err := p.parseRuleJSON(r.Content, true, diagnostics)
		if err != nil {
			return 0, err
		}
		return p.ImmediateToken(c), nil
	case "STRING":
		value, err := stringValue(r.Value)
		if err != nil {
			return 0, err
		}
		return p.String(p.Intern(value)), nil
	case "EOF":
		return p.EOF(), nil
	}
	return 0, serializationError(fmt.Errorf("unknown variant `%s`", r.Type))
}

// precedenceValue decodes the value of a precedence, which is a number or a
// name.
//
// precedenceValue is PrecedenceValueJSON::intern, with the untagged decoding
// of serde: a number that fits an int32 first, then a string.
func (p *RulePool) precedenceValue(raw json.RawMessage) (Precedence, error) {
	if raw == nil {
		return Precedence{}, missing("value")
	}
	if n, err := int32Value(raw); err == nil {
		return Precedence{Kind: PrecedenceInteger, Integer: n}, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return Precedence{}, serializationError(errors.New("data did not match any variant of untagged enum PrecedenceValueJSON"))
	}
	return Precedence{Kind: PrecedenceName, Name: p.Intern(s)}, nil
}

// stringValue decodes a string field.
func stringValue(raw json.RawMessage) (string, error) {
	if raw == nil {
		return "", missing("value")
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", serializationError(err)
	}
	return s, nil
}

// int32Value decodes a number that fits an int32, as serde decodes an i32.
func int32Value(raw json.RawMessage) (int32, error) {
	if raw == nil {
		return 0, missing("value")
	}
	n, err := strconv.ParseInt(string(bytes.TrimSpace(raw)), 10, 32)
	if err != nil {
		return 0, serializationError(fmt.Errorf("invalid value %s, expected i32", raw))
	}
	return int32(n), nil
}

// objectEntry is one key and value of a JSON object.
type objectEntry struct {
	key   string
	value json.RawMessage
}

// orderedObject decodes a JSON object into its entries, in the order of the
// document. A key that comes twice keeps its first place and takes its last
// value, as the IndexMap of serde_json does.
func orderedObject(raw json.RawMessage) ([]objectEntry, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	tok, err := d.Token()
	if err != nil {
		return nil, fmt.Errorf("decoding an object: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("decoding an object: found %v", tok)
	}
	var entries []objectEntry
	index := map[string]int{}
	for d.More() {
		tok, err := d.Token()
		if err != nil {
			return nil, fmt.Errorf("decoding a key: %w", err)
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("decoding a key: found %v", tok)
		}
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return nil, fmt.Errorf("decoding the value of %q: %w", key, err)
		}
		if i, ok := index[key]; ok {
			entries[i].value = value
			continue
		}
		index[key] = len(entries)
		entries = append(entries, objectEntry{key: key, value: value})
	}
	return entries, nil
}
