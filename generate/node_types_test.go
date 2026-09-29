package generate

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestNodeTypesSimple is test_node_types_simple in node_types.rs.
func TestNodeTypesSimple(t *testing.T) {
	pool := NewRulePool()
	v1 := func() RuleID {
		f1 := nodeTypesField(pool, "f1", named(pool, "v2"))
		f2 := nodeTypesField(pool, "f2", str(pool, ";"))
		return pool.Seq([]RuleID{f1, f2})
	}()
	v2 := str(pool, "x")
	v3 := str(pool, "y")

	v1Name := pool.Intern("v1")
	v2Name := pool.Intern("v2")
	semicolon := pool.Intern(";")
	f1Name := pool.Intern("f1")
	f2Name := pool.Intern("f2")
	nodeTypes := mustGetNodeTypes(t, &InputGrammar{
		Pool: pool,
		Variables: []Variable{
			{Name: pool.Intern("v1"), Root: v1},
			{Name: pool.Intern("v2"), Root: v2},
			// This rule is not reachable from the start symbol, so it is
			// not in the node types.
			{Name: pool.Intern("v3"), Root: v3},
		},
	})

	if len(nodeTypes) != 3 {
		t.Fatalf("expected 3 node types, got: %d", len(nodeTypes))
	}
	expectNodeInfo(t, nodeTypes[0], nodeInfoJSON{
		kind:  v1Name,
		named: true,
		root:  true,
		fields: map[StrID]*fieldInfoJSON{
			f1Name: {required: true, types: []nodeTypeRef{{kind: v2Name, named: true}}},
			f2Name: {required: true, types: []nodeTypeRef{{kind: semicolon, named: false}}},
		},
	})
	expectNodeInfo(t, nodeTypes[1], nodeInfoJSON{kind: semicolon, named: false})
	expectNodeInfo(t, nodeTypes[2], nodeInfoJSON{kind: v2Name, named: true})
}

// TestNodeTypesSimpleExtras is test_node_types_simple_extras in
// node_types.rs.
func TestNodeTypesSimpleExtras(t *testing.T) {
	pool := NewRulePool()
	v1 := func() RuleID {
		f1 := nodeTypesField(pool, "f1", named(pool, "v2"))
		f2 := nodeTypesField(pool, "f2", str(pool, ";"))
		return pool.Seq([]RuleID{f1, f2})
	}()
	v2 := str(pool, "x")
	v3 := str(pool, "y")
	extra := named(pool, "v3")
	v1Name := pool.Intern("v1")
	v2Name := pool.Intern("v2")
	v3Name := pool.Intern("v3")
	semicolon := pool.Intern(";")
	f1Name := pool.Intern("f1")
	f2Name := pool.Intern("f2")
	nodeTypes := mustGetNodeTypes(t, &InputGrammar{
		Pool:       pool,
		ExtraRoots: []RuleID{extra},
		Variables: []Variable{
			{Name: pool.Intern("v1"), Root: v1},
			{Name: pool.Intern("v2"), Root: v2},
			// This rule is not reachable from the start symbol, but it is
			// reachable from the extra symbols, so it is in the node types.
			// It is only a literal, so a lexical variable replaces it.
			{Name: pool.Intern("v3"), Root: v3},
		},
	})

	if len(nodeTypes) != 4 {
		t.Fatalf("expected 4 node types, got: %d", len(nodeTypes))
	}
	expectNodeInfo(t, nodeTypes[0], nodeInfoJSON{
		kind:  v1Name,
		named: true,
		root:  true,
		fields: map[StrID]*fieldInfoJSON{
			f1Name: {required: true, types: []nodeTypeRef{{kind: v2Name, named: true}}},
			f2Name: {required: true, types: []nodeTypeRef{{kind: semicolon, named: false}}},
		},
	})
	expectNodeInfo(t, nodeTypes[1], nodeInfoJSON{kind: semicolon, named: false})
	expectNodeInfo(t, nodeTypes[2], nodeInfoJSON{kind: v2Name, named: true})
	expectNodeInfo(t, nodeTypes[3], nodeInfoJSON{kind: v3Name, named: true, extra: true})
}

// TestNodeTypesDeeperExtras is test_node_types_deeper_extras in
// node_types.rs.
func TestNodeTypesDeeperExtras(t *testing.T) {
	pool := NewRulePool()
	v1 := func() RuleID {
		f1 := nodeTypesField(pool, "f1", named(pool, "v2"))
		f2 := nodeTypesField(pool, "f2", str(pool, ";"))
		return pool.Seq([]RuleID{f1, f2})
	}()
	v2 := str(pool, "x")
	v3 := func() RuleID {
		y := str(pool, "y")
		z := pool.Repeat(str(pool, "z"))
		return pool.Seq([]RuleID{y, z})
	}()
	extra := named(pool, "v3")
	v1Name := pool.Intern("v1")
	v2Name := pool.Intern("v2")
	v3Name := pool.Intern("v3")
	semicolon := pool.Intern(";")
	f1Name := pool.Intern("f1")
	f2Name := pool.Intern("f2")
	nodeTypes := mustGetNodeTypes(t, &InputGrammar{
		Pool:       pool,
		ExtraRoots: []RuleID{extra},
		Variables: []Variable{
			{Name: pool.Intern("v1"), Root: v1},
			{Name: pool.Intern("v2"), Root: v2},
			// This rule is not reachable from the start symbol, but it is
			// reachable from the extra symbols, so it is in the node types.
			// It is not only a literal, so no lexical variable replaces it.
			{Name: pool.Intern("v3"), Root: v3},
		},
	})

	if len(nodeTypes) != 6 {
		t.Fatalf("expected 6 node types, got: %d", len(nodeTypes))
	}
	expectNodeInfo(t, nodeTypes[0], nodeInfoJSON{
		kind:  v1Name,
		named: true,
		root:  true,
		fields: map[StrID]*fieldInfoJSON{
			f1Name: {required: true, types: []nodeTypeRef{{kind: v2Name, named: true}}},
			f2Name: {required: true, types: []nodeTypeRef{{kind: semicolon, named: false}}},
		},
	})
	expectNodeInfo(t, nodeTypes[1], nodeInfoJSON{
		kind:   v3Name,
		named:  true,
		extra:  true,
		fields: map[StrID]*fieldInfoJSON{},
	})
	expectNodeInfo(t, nodeTypes[2], nodeInfoJSON{kind: semicolon, named: false})
	expectNodeInfo(t, nodeTypes[3], nodeInfoJSON{kind: v2Name, named: true})
}

// TestNodeTypesWithSupertypes is test_node_types_with_supertypes in
// node_types.rs.
func TestNodeTypesWithSupertypes(t *testing.T) {
	pool := NewRulePool()
	v1 := nodeTypesField(pool, "f1", named(pool, "_v2"))
	v2 := func() RuleID {
		a, b, c := named(pool, "v3"), named(pool, "v4"), str(pool, "*")
		return pool.Choice([]RuleID{a, b, c})
	}()
	v3 := str(pool, "x")
	v4 := str(pool, "y")
	v1Name := pool.Intern("v1")
	v2Name := pool.Intern("_v2")
	v3Name := pool.Intern("v3")
	v4Name := pool.Intern("v4")
	asterisk := pool.Intern("*")
	f1Name := pool.Intern("f1")
	nodeTypes := mustGetNodeTypes(t, &InputGrammar{
		Pool:           pool,
		SupertypeNames: []StrID{pool.Intern("_v2")},
		Variables: []Variable{
			{Name: pool.Intern("v1"), Root: v1},
			{Name: pool.Intern("_v2"), Root: v2},
			{Name: pool.Intern("v3"), Root: v3},
			{Name: pool.Intern("v4"), Root: v4},
		},
	})

	expectNodeInfo(t, nodeTypes[0], nodeInfoJSON{
		kind:  v2Name,
		named: true,
		subtypes: []nodeTypeRef{
			{kind: asterisk, named: false},
			{kind: v3Name, named: true},
			{kind: v4Name, named: true},
		},
	})
	expectNodeInfo(t, nodeTypes[1], nodeInfoJSON{
		kind:  v1Name,
		named: true,
		root:  true,
		fields: map[StrID]*fieldInfoJSON{
			f1Name: {required: true, types: []nodeTypeRef{{kind: v2Name, named: true}}},
		},
	})
}

// TestNodeTypesWithAliasedSupertype is
// test_node_types_with_aliased_supertype in node_types.rs.
func TestNodeTypesWithAliasedSupertype(t *testing.T) {
	pool := NewRulePool()
	expression := named(pool, "_expression")
	document := nodeTypesAlias(pool, expression, "expression_target", true)
	expression = named(pool, "identifier")
	identifier := pat(pool, "[a-z]+")
	expressionName := pool.Intern("_expression")
	expressionTargetName := pool.Intern("expression_target")
	identifierName := pool.Intern("identifier")
	nodeTypes := mustGetNodeTypes(t, &InputGrammar{
		Pool:           pool,
		SupertypeNames: []StrID{expressionName},
		Variables: []Variable{
			{Name: pool.Intern("document"), Root: document},
			{Name: expressionName, Root: expression},
			{Name: identifierName, Root: identifier},
		},
	})

	i := slices.IndexFunc(nodeTypes, func(n nodeInfoJSON) bool { return n.kind == expressionTargetName })
	if i < 0 {
		t.Fatal("the aliased supertype must have its own node-types entry")
	}
	alias := nodeTypes[i]
	if !alias.named {
		t.Error("expected the alias to be named")
	}
	if alias.subtypes != nil {
		t.Errorf("expected no subtypes, got: %v", alias.subtypes)
	}
	if expected := []nodeTypeRef{{kind: identifierName, named: true}}; !slices.Equal(alias.children.types, expected) {
		t.Errorf("expected the children %v, got: %v", expected, alias.children.types)
	}
}

// TestNodeTypesDistinguishNamedAndAnonymousAliases is
// test_node_types_distinguish_named_and_anonymous_aliases in node_types.rs.
func TestNodeTypesDistinguishNamedAndAnonymousAliases(t *testing.T) {
	pool := NewRulePool()
	node := named(pool, "_node")
	namedAlias := nodeTypesAlias(pool, node, "same", true)
	anonymousAlias := nodeTypesAlias(pool, node, "same", false)
	document := pool.Choice([]RuleID{namedAlias, anonymousAlias})
	node = func() RuleID {
		value := named(pool, "value")
		suffix := str(pool, "!")
		return pool.Seq([]RuleID{value, suffix})
	}()
	value := pat(pool, "[a-z]+")
	sameName := pool.Intern("same")
	nodeTypes := mustGetNodeTypes(t, &InputGrammar{
		Pool: pool,
		Variables: []Variable{
			{Name: pool.Intern("document"), Root: document},
			{Name: pool.Intern("_node"), Root: node},
			{Name: pool.Intern("value"), Root: value},
		},
	})

	var namedness []bool
	for _, n := range nodeTypes {
		if n.kind == sameName {
			namedness = append(namedness, n.named)
		}
	}
	if expected := []bool{false, true}; !slices.Equal(namedness, expected) {
		t.Errorf("expected %v, got: %v", expected, namedness)
	}
}

// TestNodeTypesMergeAnonymousNodeAndTokenAliases is
// test_node_types_merge_anonymous_node_and_token_aliases in node_types.rs.
func TestNodeTypesMergeAnonymousNodeAndTokenAliases(t *testing.T) {
	pool := NewRulePool()
	node := named(pool, "_node")
	node = nodeTypesAlias(pool, node, "same", false)
	token := str(pool, "!")
	token = nodeTypesAlias(pool, token, "same", false)
	document := pool.Choice([]RuleID{node, token})
	node = func() RuleID {
		value1, value2 := named(pool, "value"), named(pool, "value")
		suffix := str(pool, "?")
		return pool.Seq([]RuleID{value1, value2, suffix})
	}()
	value := pat(pool, "[a-z]+")
	sameName := pool.Intern("same")
	valueName := pool.Intern("value")
	nodeTypes := mustGetNodeTypes(t, &InputGrammar{
		Pool: pool,
		Variables: []Variable{
			{Name: pool.Intern("document"), Root: document},
			{Name: pool.Intern("_node"), Root: node},
			{Name: valueName, Root: value},
		},
	})

	var aliases []nodeInfoJSON
	for _, n := range nodeTypes {
		if n.kind == sameName && !n.named {
			aliases = append(aliases, n)
		}
	}
	if len(aliases) != 1 {
		t.Fatalf("expected 1 alias, got: %d", len(aliases))
	}
	children := aliases[0].children
	if !children.multiple {
		t.Error("expected the children to be multiple")
	}
	if children.required {
		t.Error("expected the children to be optional")
	}
	if expected := []nodeTypeRef{{kind: valueName, named: true}}; !slices.Equal(children.types, expected) {
		t.Errorf("expected the children %v, got: %v", expected, children.types)
	}
}

// TestNodeTypesDistinguishExtraMetadataByNamedness is
// test_node_types_distinguish_extra_metadata_by_namedness in node_types.rs.
func TestNodeTypesDistinguishExtraMetadataByNamedness(t *testing.T) {
	pool := NewRulePool()
	node := named(pool, "_node")
	node = nodeTypesAlias(pool, node, "same", false)
	extra := named(pool, "_extra")
	extraAlias := nodeTypesAlias(pool, extra, "same", true)
	document := pool.Choice([]RuleID{node, extraAlias})
	node = func() RuleID {
		value := named(pool, "value")
		suffix := str(pool, "?")
		return pool.Seq([]RuleID{value, suffix})
	}()
	extraRule := func() RuleID {
		prefix := str(pool, "#")
		value := named(pool, "extra_value")
		return pool.Seq([]RuleID{prefix, value})
	}()
	value := pat(pool, "[a-z]+")
	extraValue := pat(pool, "[A-Z]+")
	sameName := pool.Intern("same")
	nodeTypes := mustGetNodeTypes(t, &InputGrammar{
		Pool:       pool,
		ExtraRoots: []RuleID{extra},
		Variables: []Variable{
			{Name: pool.Intern("document"), Root: document},
			{Name: pool.Intern("_node"), Root: node},
			{Name: pool.Intern("_extra"), Root: extraRule},
			{Name: pool.Intern("value"), Root: value},
			{Name: pool.Intern("extra_value"), Root: extraValue},
		},
	})

	var aliases []nodeInfoJSON
	for _, n := range nodeTypes {
		if n.kind == sameName {
			aliases = append(aliases, n)
		}
	}
	slices.SortFunc(aliases, func(a, b nodeInfoJSON) int { return compareBool(a.named, b.named) })
	if len(aliases) != 2 {
		t.Fatalf("expected 2 aliases, got: %d", len(aliases))
	}
	if aliases[0].named || aliases[0].extra {
		t.Errorf("expected the first alias to be anonymous and not extra, got: %+v", aliases[0])
	}
	if !aliases[1].named || !aliases[1].extra {
		t.Errorf("expected the second alias to be named and extra, got: %+v", aliases[1])
	}
}

// TestNodeTypesDistinguishSupertypeDependenciesByNamedness is
// test_node_types_distinguish_supertype_dependencies_by_namedness in
// node_types.rs.
func TestNodeTypesDistinguishSupertypeDependenciesByNamedness(t *testing.T) {
	pool := NewRulePool()
	document := named(pool, "_super")
	supertype := func() RuleID {
		item := named(pool, "item")
		item = nodeTypesAlias(pool, item, "_super", false)
		other := named(pool, "other")
		return pool.Choice([]RuleID{item, other})
	}()
	item := str(pool, "x")
	other := str(pool, "y")
	superName := pool.Intern("_super")
	nodeTypes := mustGetNodeTypes(t, &InputGrammar{
		Pool:           pool,
		SupertypeNames: []StrID{superName},
		Variables: []Variable{
			{Name: pool.Intern("document"), Root: document},
			{Name: superName, Root: supertype},
			{Name: pool.Intern("item"), Root: item},
			{Name: pool.Intern("other"), Root: other},
		},
	})

	var namedness []bool
	for _, n := range nodeTypes {
		if n.kind == superName {
			namedness = append(namedness, n.named)
		}
	}
	slices.SortFunc(namedness, compareBool)
	if expected := []bool{false, true}; !slices.Equal(namedness, expected) {
		t.Errorf("expected %v, got: %v", expected, namedness)
	}
}

// TestNodeTypesWithAliasMatchingCanonicalSupertype is
// test_node_types_with_alias_matching_canonical_supertype in node_types.rs.
func TestNodeTypesWithAliasMatchingCanonicalSupertype(t *testing.T) {
	pool := NewRulePool()
	document := func() RuleID {
		node := named(pool, "_node")
		node = nodeTypesAlias(pool, node, "_super", true)
		supertype := named(pool, "_super")
		return pool.Choice([]RuleID{node, supertype})
	}()
	supertype := func() RuleID {
		one, two := named(pool, "one"), named(pool, "two")
		return pool.Choice([]RuleID{one, two})
	}()
	node := func() RuleID {
		value := named(pool, "value")
		suffix := str(pool, "?")
		return pool.Seq([]RuleID{value, suffix})
	}()
	one := str(pool, "1")
	two := str(pool, "2")
	value := pat(pool, "[a-z]+")
	superName := pool.Intern("_super")
	grammar := &InputGrammar{
		Pool:           pool,
		SupertypeNames: []StrID{superName},
		Variables: []Variable{
			{Name: pool.Intern("document"), Root: document},
			{Name: superName, Root: supertype},
			{Name: pool.Intern("_node"), Root: node},
			{Name: pool.Intern("one"), Root: one},
			{Name: pool.Intern("two"), Root: two},
			{Name: pool.Intern("value"), Root: value},
		},
	}
	prepared := prepareForTest(t, grammar)
	_, err := GetVariableInfo(&prepared.SyntaxGrammar, &prepared.LexicalGrammar, prepared.DefaultAliases, prepared.StrPool)

	var collision *SupertypeAliasCollisionError
	if !errors.As(err, &collision) || collision.Name != "_super" {
		t.Fatalf("expected a SupertypeAliasCollisionError for _super, got: %v", err)
	}
}

// TestNodeTypesSupertypeWithOnlyHiddenChild is
// test_node_types_supertype_with_only_hidden_child in node_types.rs. A
// supertype whose only child is a hidden external token must not make the
// generator panic. The subtype map must skip an entry with no subtypes, so
// that the lookup in the topological sort does not fail.
func TestNodeTypesSupertypeWithOnlyHiddenChild(t *testing.T) {
	pool := NewRulePool()
	v1 := func() RuleID {
		a, b := named(pool, "_type_a"), named(pool, "_type_b")
		return pool.Seq([]RuleID{a, b})
	}()
	typeA := func() RuleID {
		a, b := named(pool, "v2"), named(pool, "v3")
		return pool.Choice([]RuleID{a, b})
	}()
	v2 := str(pool, "x")
	v3 := str(pool, "y")
	typeB := nodeTypesExternal(pool, 0)
	hiddenExt := named(pool, "_hidden_ext")
	_, err := getNodeTypes(t, &InputGrammar{
		Pool:           pool,
		SupertypeNames: []StrID{pool.Intern("_type_a"), pool.Intern("_type_b")},
		ExternalRoots:  []RuleID{hiddenExt},
		Variables: []Variable{
			{Name: pool.Intern("v1"), Root: v1},
			// Supertype A: a normal choice of named subtypes.
			{Name: pool.Intern("_type_a"), Root: typeA},
			{Name: pool.Intern("v2"), Root: v2},
			{Name: pool.Intern("v3"), Root: v3},
			// Supertype B: a hidden external token with no subtypes.
			{Name: pool.Intern("_type_b"), Root: typeB},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

// TestNodeTypesForChildrenWithoutFields is
// test_node_types_for_children_without_fields in node_types.rs.
func TestNodeTypesForChildrenWithoutFields(t *testing.T) {
	pool := NewRulePool()
	v1 := func() RuleID {
		a := named(pool, "v2")
		f1 := nodeTypesField(pool, "f1", named(pool, "v3"))
		c := named(pool, "v4")
		return pool.Seq([]RuleID{a, f1, c})
	}()
	v2 := func() RuleID {
		open := str(pool, "{")
		mid := func() RuleID {
			v3 := named(pool, "v3")
			blank := pool.Blank()
			return pool.Choice([]RuleID{v3, blank})
		}()
		closer := str(pool, "}")
		return pool.Seq([]RuleID{open, mid, closer})
	}()
	v3 := str(pool, "x")
	v4 := str(pool, "y")
	v1Name := pool.Intern("v1")
	v2Name := pool.Intern("v2")
	v3Name := pool.Intern("v3")
	v4Name := pool.Intern("v4")
	f1Name := pool.Intern("f1")
	nodeTypes := mustGetNodeTypes(t, &InputGrammar{
		Pool: pool,
		Variables: []Variable{
			{Name: pool.Intern("v1"), Root: v1},
			{Name: pool.Intern("v2"), Root: v2},
			{Name: pool.Intern("v3"), Root: v3},
			{Name: pool.Intern("v4"), Root: v4},
		},
	})

	expectNodeInfo(t, nodeTypes[0], nodeInfoJSON{
		kind:  v1Name,
		named: true,
		root:  true,
		children: &fieldInfoJSON{
			multiple: true,
			required: true,
			types:    []nodeTypeRef{{kind: v2Name, named: true}, {kind: v4Name, named: true}},
		},
		fields: map[StrID]*fieldInfoJSON{
			f1Name: {required: true, types: []nodeTypeRef{{kind: v3Name, named: true}}},
		},
	})
	expectNodeInfo(t, nodeTypes[1], nodeInfoJSON{
		kind:  v2Name,
		named: true,
		children: &fieldInfoJSON{
			types: []nodeTypeRef{{kind: v3Name, named: true}},
		},
		fields: map[StrID]*fieldInfoJSON{},
	})
}

// TestNodeTypesWithInlinedRules is test_node_types_with_inlined_rules in
// node_types.rs.
func TestNodeTypesWithInlinedRules(t *testing.T) {
	pool := NewRulePool()
	v1 := func() RuleID {
		a, b := named(pool, "v2"), named(pool, "v3")
		return pool.Seq([]RuleID{a, b})
	}()
	v2 := nodeTypesAlias(pool, str(pool, "a"), "x", true)
	v3 := str(pool, "b")
	v1Name := pool.Intern("v1")
	v3Name := pool.Intern("v3")
	xName := pool.Intern("x")
	nodeTypes := mustGetNodeTypes(t, &InputGrammar{
		Pool:        pool,
		InlineNames: []StrID{pool.Intern("v2")},
		Variables: []Variable{
			{Name: pool.Intern("v1"), Root: v1},
			// v2 is inlined, so it is not in the node types.
			{Name: pool.Intern("v2"), Root: v2},
			{Name: pool.Intern("v3"), Root: v3},
		},
	})

	expectNodeInfo(t, nodeTypes[0], nodeInfoJSON{
		kind:  v1Name,
		named: true,
		root:  true,
		children: &fieldInfoJSON{
			multiple: true,
			required: true,
			types:    []nodeTypeRef{{kind: v3Name, named: true}, {kind: xName, named: true}},
		},
		fields: map[StrID]*fieldInfoJSON{},
	})
}

// TestNodeTypesForAliasedNodes is test_node_types_for_aliased_nodes in
// node_types.rs.
func TestNodeTypesForAliasedNodes(t *testing.T) {
	pool := NewRulePool()
	thing := func() RuleID {
		a, b := named(pool, "type"), named(pool, "expression")
		return pool.Choice([]RuleID{a, b})
	}()
	ty := func() RuleID {
		id := nodeTypesAlias(pool, named(pool, "identifier"), "type_identifier", true)
		void := str(pool, "void")
		return pool.Choice([]RuleID{id, void})
	}()
	expression := func() RuleID {
		id := named(pool, "identifier")
		foo := nodeTypesAlias(pool, named(pool, "foo_identifier"), "identifier", true)
		return pool.Choice([]RuleID{id, foo})
	}()
	identifier := pat(pool, `\w+`)
	fooIdentifier := pat(pool, `[\w-]+`)
	identifierName := pool.Intern("identifier")
	fooIdentifierName := pool.Intern("foo_identifier")
	typeIdentifierName := pool.Intern("type_identifier")
	nodeTypes := mustGetNodeTypes(t, &InputGrammar{
		Pool: pool,
		Variables: []Variable{
			{Name: pool.Intern("thing"), Root: thing},
			{Name: pool.Intern("type"), Root: ty},
			{Name: pool.Intern("expression"), Root: expression},
			{Name: pool.Intern("identifier"), Root: identifier},
			{Name: pool.Intern("foo_identifier"), Root: fooIdentifier},
		},
	})

	find := func(kind StrID) (nodeInfoJSON, bool) {
		i := slices.IndexFunc(nodeTypes, func(n nodeInfoJSON) bool { return n.kind == kind })
		if i < 0 {
			return nodeInfoJSON{}, false
		}
		return nodeTypes[i], true
	}
	if n, ok := find(fooIdentifierName); ok {
		t.Errorf("expected no foo_identifier, got: %+v", n)
	}
	if n, ok := find(identifierName); !ok {
		t.Error("expected an identifier")
	} else {
		expectNodeInfo(t, n, nodeInfoJSON{kind: identifierName, named: true})
	}
	if n, ok := find(typeIdentifierName); !ok {
		t.Error("expected a type_identifier")
	} else {
		expectNodeInfo(t, n, nodeInfoJSON{kind: typeIdentifierName, named: true})
	}
}

// TestNodeTypesWithMultipleValuedFields is
// test_node_types_with_multiple_valued_fields in node_types.rs.
func TestNodeTypesWithMultipleValuedFields(t *testing.T) {
	pool := NewRulePool()
	a := func() RuleID {
		first := func() RuleID {
			blank := pool.Blank()
			rep := pool.Repeat(nodeTypesField(pool, "f1", named(pool, "b")))
			return pool.Choice([]RuleID{blank, rep})
		}()
		second := pool.Repeat(named(pool, "c"))
		return pool.Seq([]RuleID{first, second})
	}()
	b := str(pool, "b")
	c := str(pool, "c")
	aName := pool.Intern("a")
	bName := pool.Intern("b")
	cName := pool.Intern("c")
	f1Name := pool.Intern("f1")
	nodeTypes := mustGetNodeTypes(t, &InputGrammar{
		Pool: pool,
		Variables: []Variable{
			{Name: pool.Intern("a"), Root: a},
			{Name: pool.Intern("b"), Root: b},
			{Name: pool.Intern("c"), Root: c},
		},
	})

	expectNodeInfo(t, nodeTypes[0], nodeInfoJSON{
		kind:  aName,
		named: true,
		root:  true,
		children: &fieldInfoJSON{
			multiple: true,
			required: true,
			types:    []nodeTypeRef{{kind: cName, named: true}},
		},
		fields: map[StrID]*fieldInfoJSON{
			f1Name: {multiple: true, required: false, types: []nodeTypeRef{{kind: bName, named: true}}},
		},
	})
}

// TestNodeTypesWithFieldsOnHiddenTokens is
// test_node_types_with_fields_on_hidden_tokens in node_types.rs.
func TestNodeTypesWithFieldsOnHiddenTokens(t *testing.T) {
	pool := NewRulePool()
	script := func() RuleID {
		a := nodeTypesField(pool, "a", pat(pool, "hi"))
		b := nodeTypesField(pool, "b", pat(pool, "bye"))
		return pool.Seq([]RuleID{a, b})
	}()
	scriptName := pool.Intern("script")
	nodeTypes := mustGetNodeTypes(t, &InputGrammar{
		Pool:      pool,
		Variables: []Variable{{Name: pool.Intern("script"), Root: script}},
	})

	if len(nodeTypes) != 1 {
		t.Fatalf("expected 1 node type, got: %d", len(nodeTypes))
	}
	expectNodeInfo(t, nodeTypes[0], nodeInfoJSON{
		kind:   scriptName,
		named:  true,
		root:   true,
		fields: map[StrID]*fieldInfoJSON{},
	})
}

// TestNodeTypesWithMultipleRulesSameAliasName is
// test_node_types_with_multiple_rules_same_alias_name in node_types.rs.
func TestNodeTypesWithMultipleRulesSameAliasName(t *testing.T) {
	pool := NewRulePool()
	script := func() RuleID {
		a := named(pool, "a")
		b := nodeTypesAlias(pool, named(pool, "b"), "a", true)
		return pool.Choice([]RuleID{a, b})
	}()
	a := func() RuleID {
		f1 := nodeTypesField(pool, "f1", str(pool, "1"))
		f2 := nodeTypesField(pool, "f2", str(pool, "2"))
		return pool.Seq([]RuleID{f1, f2})
	}()
	b := func() RuleID {
		f2a := nodeTypesField(pool, "f2", str(pool, "22"))
		f2b := nodeTypesField(pool, "f2", str(pool, "222"))
		f3 := nodeTypesField(pool, "f3", str(pool, "3"))
		return pool.Seq([]RuleID{f2a, f2b, f3})
	}()
	aName := pool.Intern("a")
	scriptName := pool.Intern("script")
	name1 := pool.Intern("1")
	name2 := pool.Intern("2")
	name22 := pool.Intern("22")
	name222 := pool.Intern("222")
	name3 := pool.Intern("3")
	f1Name := pool.Intern("f1")
	f2Name := pool.Intern("f2")
	f3Name := pool.Intern("f3")
	nodeTypes := mustGetNodeTypes(t, &InputGrammar{
		Pool: pool,
		Variables: []Variable{
			{Name: pool.Intern("script"), Root: script},
			{Name: pool.Intern("a"), Root: a},
			{Name: pool.Intern("b"), Root: b},
		},
	})

	expectNodeKinds(t, nodeTypes, []StrID{aName, scriptName, name1, name2, name22, name222, name3})
	// A combination of the types of `a` and `b`.
	expectNodeInfo(t, nodeTypes[0], nodeInfoJSON{
		kind:  aName,
		named: true,
		fields: map[StrID]*fieldInfoJSON{
			f1Name: {required: false, types: []nodeTypeRef{{kind: name1, named: false}}},
			f2Name: {multiple: true, required: true, types: []nodeTypeRef{
				{kind: name2, named: false},
				{kind: name22, named: false},
				{kind: name222, named: false},
			}},
			f3Name: {required: false, types: []nodeTypeRef{{kind: name3, named: false}}},
		},
	})
	expectNodeInfo(t, nodeTypes[1], nodeInfoJSON{
		kind:  scriptName,
		named: true,
		root:  true,
		// Only one node.
		children: &fieldInfoJSON{
			required: true,
			types:    []nodeTypeRef{{kind: aName, named: true}},
		},
		fields: map[StrID]*fieldInfoJSON{},
	})
}

// TestNodeTypesWithTokensAliasedToMatchRules is
// test_node_types_with_tokens_aliased_to_match_rules in node_types.rs.
func TestNodeTypesWithTokensAliasedToMatchRules(t *testing.T) {
	pool := NewRulePool()
	a := func() RuleID {
		b, c := named(pool, "b"), named(pool, "c")
		return pool.Seq([]RuleID{b, c})
	}()
	b := func() RuleID {
		c1, mid, c2 := named(pool, "c"), str(pool, "B"), named(pool, "c")
		return pool.Seq([]RuleID{c1, mid, c2})
	}()
	c := func() RuleID {
		cc := str(pool, "C")
		// This token has the alias `b`, which makes a `b` node with no
		// children.
		d := nodeTypesAlias(pool, str(pool, "D"), "b", true)
		return pool.Choice([]RuleID{cc, d})
	}()

	nameA := pool.Intern("a")
	nameB := pool.Intern("b")
	nameC := pool.Intern("c")
	nameCapitalB := pool.Intern("B")
	nameCapitalC := pool.Intern("C")
	nodeTypes := mustGetNodeTypes(t, &InputGrammar{
		Pool: pool,
		Variables: []Variable{
			{Name: pool.Intern("a"), Root: a},
			// Usually, a `b` node has two named `c` children.
			{Name: pool.Intern("b"), Root: b},
			{Name: pool.Intern("c"), Root: c},
		},
	})

	expectNodeKinds(t, nodeTypes, []StrID{nameA, nameB, nameC, nameCapitalB, nameCapitalC})
	expectNodeInfo(t, nodeTypes[1], nodeInfoJSON{
		kind:  nameB,
		named: true,
		children: &fieldInfoJSON{
			multiple: true,
			required: false,
			types:    []nodeTypeRef{{kind: nameC, named: true}},
		},
		fields: map[StrID]*fieldInfoJSON{},
	})
}

// TestGetVariableInfo is test_get_variable_info in node_types.rs.
func TestGetVariableInfo(t *testing.T) {
	interner := NewStrPool()
	field1 := interner.Intern("field1")
	field2 := interner.Intern("field2")
	lexicalGrammar := nodeTypesLexicalGrammar(interner)
	grammar := nodeTypesSyntaxGrammar(interner, []nodeTypesVariable{
		// The required field `field1` has only one node type.
		{"rule0", VariableNamed, [][]ProductionStep{{
			nodeTypesStep(TerminalSymbol(0), 0),
			nodeTypesStep(NonTerminalSymbol(1), field1),
		}}},
		// A hidden node.
		{"_rule1", VariableHidden, [][]ProductionStep{{nodeTypesStep(TerminalSymbol(1), 0)}}},
		// The optional field `field2` can have two node types.
		{"rule2", VariableNamed, [][]ProductionStep{
			{nodeTypesStep(TerminalSymbol(0), 0)},
			{nodeTypesStep(TerminalSymbol(0), 0), nodeTypesStep(TerminalSymbol(2), field2)},
			{nodeTypesStep(TerminalSymbol(0), 0), nodeTypesStep(TerminalSymbol(3), field2)},
		}},
	}, nil)
	variableInfo, err := GetVariableInfo(grammar, lexicalGrammar, AliasMap{}, interner)
	if err != nil {
		t.Fatal(err)
	}

	expectFields(t, variableInfo[0].Fields, map[StrID]*FieldInfo{
		field1: {
			Quantity: ChildQuantity{exists: true, required: true, multiple: false},
			Types:    []ChildType{NormalChildType(TerminalSymbol(1))},
		},
	})
	expectFields(t, variableInfo[2].Fields, map[StrID]*FieldInfo{
		field2: {
			Quantity: ChildQuantity{exists: true, required: false, multiple: false},
			Types:    []ChildType{NormalChildType(TerminalSymbol(2)), NormalChildType(TerminalSymbol(3))},
		},
	})
}

// TestGetVariableInfoWithRepetitionsInsideFields is
// test_get_variable_info_with_repetitions_inside_fields in node_types.rs.
func TestGetVariableInfoWithRepetitionsInsideFields(t *testing.T) {
	interner := NewStrPool()
	field1 := interner.Intern("field1")
	lexicalGrammar := nodeTypesLexicalGrammar(interner)
	grammar := nodeTypesSyntaxGrammar(interner, []nodeTypesVariable{
		// A field of a repetition.
		{"rule0", VariableNamed, [][]ProductionStep{{nodeTypesStep(NonTerminalSymbol(1), field1)}, {}}},
		{"_rule0_repeat", VariableHidden, [][]ProductionStep{
			{nodeTypesStep(TerminalSymbol(1), 0)},
			{nodeTypesStep(NonTerminalSymbol(1), 0), nodeTypesStep(NonTerminalSymbol(1), 0)},
		}},
	}, nil)
	variableInfo, err := GetVariableInfo(grammar, lexicalGrammar, AliasMap{}, interner)
	if err != nil {
		t.Fatal(err)
	}

	expectFields(t, variableInfo[0].Fields, map[StrID]*FieldInfo{
		field1: {
			Quantity: ChildQuantity{exists: true, required: false, multiple: true},
			Types:    []ChildType{NormalChildType(TerminalSymbol(1))},
		},
	})
}

// TestGetVariableInfoWithInheritedFields is
// test_get_variable_info_with_inherited_fields in node_types.rs.
func TestGetVariableInfoWithInheritedFields(t *testing.T) {
	interner := NewStrPool()
	field1 := interner.Intern("field1")
	dot := interner.Intern(".")
	lexicalGrammar := nodeTypesLexicalGrammar(interner)
	grammar := nodeTypesSyntaxGrammar(interner, []nodeTypesVariable{
		{"rule0", VariableNamed, [][]ProductionStep{
			{
				nodeTypesStep(TerminalSymbol(0), 0),
				nodeTypesStep(NonTerminalSymbol(1), 0),
				nodeTypesStep(TerminalSymbol(1), 0),
			},
			{nodeTypesStep(NonTerminalSymbol(1), 0)},
		}},
		// A hidden node with fields.
		{"_rule1", VariableHidden, [][]ProductionStep{{
			PackProductionStep(TerminalSymbol(2), Precedence{}, AssociativityNone, Alias{Value: dot, IsNamed: false}, true, 0, NoReservedWords),
			nodeTypesStep(TerminalSymbol(3), field1),
		}}},
	}, nil)
	variableInfo, err := GetVariableInfo(grammar, lexicalGrammar, AliasMap{}, interner)
	if err != nil {
		t.Fatal(err)
	}

	expectFields(t, variableInfo[0].Fields, map[StrID]*FieldInfo{
		field1: {
			Quantity: ChildQuantity{exists: true, required: true, multiple: false},
			Types:    []ChildType{NormalChildType(TerminalSymbol(3))},
		},
	})
	expected := FieldInfo{
		Quantity: ChildQuantity{exists: true, required: false, multiple: true},
		Types:    []ChildType{NormalChildType(TerminalSymbol(0)), NormalChildType(TerminalSymbol(1))},
	}
	if !reflect.DeepEqual(variableInfo[0].ChildrenWithoutFields, expected) {
		t.Errorf("expected the children without fields %+v, got: %+v", expected, variableInfo[0].ChildrenWithoutFields)
	}
}

// TestGetVariableInfoWithSupertypes is test_get_variable_info_with_supertypes
// in node_types.rs.
func TestGetVariableInfoWithSupertypes(t *testing.T) {
	interner := NewStrPool()
	field1 := interner.Intern("field1")
	lexicalGrammar := nodeTypesLexicalGrammar(interner)
	grammar := nodeTypesSyntaxGrammar(interner, []nodeTypesVariable{
		{"rule0", VariableNamed, [][]ProductionStep{{
			nodeTypesStep(TerminalSymbol(0), 0),
			nodeTypesStep(NonTerminalSymbol(1), field1),
			nodeTypesStep(TerminalSymbol(1), 0),
		}}},
		{"_rule1", VariableHidden, [][]ProductionStep{
			{nodeTypesStep(TerminalSymbol(2), 0)},
			{nodeTypesStep(TerminalSymbol(3), 0)},
		}},
	},
		// _rule1 is a supertype.
		[]Symbol{NonTerminalSymbol(1)},
	)
	variableInfo, err := GetVariableInfo(grammar, lexicalGrammar, AliasMap{}, interner)
	if err != nil {
		t.Fatal(err)
	}

	expectFields(t, variableInfo[0].Fields, map[StrID]*FieldInfo{
		field1: {
			Quantity: ChildQuantity{exists: true, required: true, multiple: false},
			Types:    []ChildType{NormalChildType(NonTerminalSymbol(1))},
		},
	})
}

// TestNodeTypesSupertypeAndInlineConflict is
// test_supertype_and_inline_conflict in node_types.rs. The name differs
// because intern_symbols_test.go has a test of the same name.
//
//	v1: field("f1", _v2)
//	_v2: choice(v3, v4)
//	supertypes: [_v2]
//	inline: [_v2]
func TestNodeTypesSupertypeAndInlineConflict(t *testing.T) {
	pool := NewRulePool()
	v1Name := pool.Intern("v1")
	v2Name := pool.Intern("_v2")
	v3Name := pool.Intern("v3")
	v4Name := pool.Intern("v4")
	f1Name := pool.Intern("f1")
	v1Root := nodeTypesField(pool, "f1", pool.NamedSymbol(v2Name))
	v2Root := func() RuleID {
		v3 := named(pool, "v3")
		v4 := named(pool, "v4")
		return pool.Choice([]RuleID{v3, v4})
	}()
	v3Root := str(pool, "x")
	v4Root := str(pool, "y")
	grammar := &InputGrammar{
		Pool: pool,
		Variables: []Variable{
			{Name: v1Name, Root: v1Root},
			{Name: v2Name, Root: v2Root},
			{Name: v3Name, Root: v3Root},
			{Name: v4Name, Root: v4Root},
		},
		SupertypeNames: []StrID{v2Name},
		InlineNames:    []StrID{v2Name},
	}

	nodeTypes := mustGetNodeTypes(t, grammar)

	// The supertype entry of `_v2` is dropped, because the rule is inlined
	// away and can never appear in a tree. Before, a phantom `_v2` entry
	// with subtypes was in the output.
	if slices.ContainsFunc(nodeTypes, func(n nodeInfoJSON) bool { return n.kind == v2Name }) {
		t.Error("expected no entry of _v2")
	}
	i := slices.IndexFunc(nodeTypes, func(n nodeInfoJSON) bool { return n.kind == v1Name })
	if i < 0 {
		t.Fatal("expected an entry of v1")
	}
	expected := []nodeTypeRef{{kind: v3Name, named: true}, {kind: v4Name, named: true}}
	if actual := nodeTypes[i].fields[f1Name].types; !slices.Equal(actual, expected) {
		t.Errorf("expected the types %v, got: %v", expected, actual)
	}
}

// TestNodeTypesOnEveryTestGrammar builds node-types.json for each test
// grammar that PrepareGrammar accepts, and compares it byte for byte with
// every golden node-types.json of the grammar. Upstream has no such test.
//
// A grammar that the upstream tool rejects in build_tables has no golden
// node-types.json, because the tool writes the file only after parser.c.
// The test makes sure that exactly those grammars have none.
func TestNodeTypesOnEveryTestGrammar(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("testdata", "*", "grammar.json"))
	if err != nil {
		t.Fatal(err)
	}
	var noGolden []string
	compared := 0
	for _, f := range files {
		dir := filepath.Dir(f)
		name := filepath.Base(dir)
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		g, err := ParseGrammar(b, new([]Diagnostic))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		prepared, err := PrepareGrammar(g, new([]Diagnostic))
		if err != nil {
			// TestPrepareGrammarOnEveryTestGrammar checks these grammars.
			continue
		}
		variableInfo, err := GetVariableInfo(&prepared.SyntaxGrammar, &prepared.LexicalGrammar, prepared.DefaultAliases, prepared.StrPool)
		if err != nil {
			t.Errorf("%s: expected no error from GetVariableInfo, got: %v", name, err)
			continue
		}
		actual, err := NodeTypesJSON(&prepared.SyntaxGrammar, &prepared.LexicalGrammar, prepared.DefaultAliases, variableInfo, prepared.StrPool)
		if err != nil {
			t.Errorf("%s: expected no error from NodeTypesJSON, got: %v", name, err)
			continue
		}
		goldens, err := filepath.Glob(filepath.Join(dir, "*", "node-types.json"))
		if err != nil {
			t.Fatal(err)
		}
		if len(goldens) == 0 {
			noGolden = append(noGolden, name)
			continue
		}
		for _, golden := range goldens {
			expected, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			compared++
			if actual != string(expected) {
				t.Errorf("%s: node-types.json differs from the golden file:\n%s", golden, firstDifference(string(expected), actual))
			}
		}
	}
	expectedNoGolden := []string{
		"associativity_missing",
		"conflict_in_repeat_rule",
		"conflict_in_repeat_rule_after_external_token",
		"conflicting_precedence",
		"partially_resolved_conflict",
		"precedence_on_single_child_missing",
	}
	if !slices.Equal(noGolden, expectedNoGolden) {
		t.Errorf("expected no golden node-types.json for %v, got: %v", expectedNoGolden, noGolden)
	}
	if compared == 0 {
		t.Error("expected at least one golden file")
	}
}

// TestNodeTypesJSONEscapes checks that a string is escaped as serde_json
// escapes it. Upstream has no such test.
func TestNodeTypesJSONEscapes(t *testing.T) {
	var w prettyJSON
	w.writeString("a\"b\\c\n\t\r\b\f\x00\x1f\x7f/<>&é\u2028")
	expected := `"a\"b\\c\n\t\r\b\f\u0000\u001f` + "\x7f/<>&é\u2028" + `"`
	if actual := w.b.String(); actual != expected {
		t.Errorf("expected %q, got: %q", expected, actual)
	}
}

// TestSuperTypeCycleErrorText checks the text of SuperTypeCycleError.
// Upstream has no such test.
func TestSuperTypeCycleErrorText(t *testing.T) {
	for _, tt := range []struct {
		items    []string
		expected string
	}{
		{nil, "Dependency cycle detected in node types:"},
		{[]string{"a", "b"}, "Dependency cycle detected in node types: a, b"},
	} {
		if actual := (&SuperTypeCycleError{Items: tt.items}).Error(); actual != tt.expected {
			t.Errorf("expected %q, got: %q", tt.expected, actual)
		}
	}
}

// firstDifference returns the lines around the first line where two texts
// differ.
func firstDifference(expected, actual string) string {
	el, al := strings.Split(expected, "\n"), strings.Split(actual, "\n")
	for i := 0; i < len(el) || i < len(al); i++ {
		var e, a string
		if i < len(el) {
			e = el[i]
		}
		if i < len(al) {
			a = al[i]
		}
		if e != a || (i >= len(el)) != (i >= len(al)) {
			return fmt.Sprintf("line %d:\nexpected: %q\n     got: %q", i+1, e, a)
		}
	}
	return "the texts are equal"
}

// getNodeTypes prepares a grammar, computes the info of its variables, and
// returns its node types. It fails the test when the grammar does not
// prepare, or when the info has an error.
//
// getNodeTypes is get_node_types in node_types.rs.
func getNodeTypes(t *testing.T, grammar *InputGrammar) ([]nodeInfoJSON, error) {
	t.Helper()
	prepared := prepareForTest(t, grammar)
	variableInfo, err := GetVariableInfo(&prepared.SyntaxGrammar, &prepared.LexicalGrammar, prepared.DefaultAliases, prepared.StrPool)
	if err != nil {
		t.Fatalf("expected no error from GetVariableInfo, got: %v", err)
	}
	return generateNodeTypes(&prepared.SyntaxGrammar, &prepared.LexicalGrammar, prepared.DefaultAliases, variableInfo, prepared.StrPool)
}

// mustGetNodeTypes is getNodeTypes, and it fails the test on an error. It is
// the unwrap of the result of get_node_types in node_types.rs.
func mustGetNodeTypes(t *testing.T, grammar *InputGrammar) []nodeInfoJSON {
	t.Helper()
	nodeTypes, err := getNodeTypes(t, grammar)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return nodeTypes
}

// expectNodeInfo fails the test unless a node type is the expected one.
func expectNodeInfo(t *testing.T, actual, expected nodeInfoJSON) {
	t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("expected the node type\n%s\ngot:\n%s", formatNodeInfo(expected), formatNodeInfo(actual))
	}
}

// formatNodeInfo returns a node type as text, with the fields sorted by id.
func formatNodeInfo(n nodeInfoJSON) string {
	var b strings.Builder
	fmt.Fprintf(&b, "{kind:%d named:%t root:%t extra:%t", n.kind, n.named, n.root, n.extra)
	if n.fields != nil {
		b.WriteString(" fields:")
		b.WriteString(formatFields(n.fields))
	}
	if n.children != nil {
		fmt.Fprintf(&b, " children:%+v", *n.children)
	}
	if n.subtypes != nil {
		fmt.Fprintf(&b, " subtypes:%v", n.subtypes)
	}
	b.WriteString("}")
	return b.String()
}

// formatFields returns fields as text, sorted by id.
func formatFields[T any](m map[StrID]*T) string {
	var b strings.Builder
	b.WriteString("{")
	for _, name := range slices.Sorted(maps.Keys(m)) {
		fmt.Fprintf(&b, "%d:%+v ", name, *m[name])
	}
	b.WriteString("}")
	return b.String()
}

// expectNodeKinds fails the test unless the node types have the expected
// kinds, in order.
func expectNodeKinds(t *testing.T, nodeTypes []nodeInfoJSON, expected []StrID) {
	t.Helper()
	actual := make([]StrID, 0, len(nodeTypes))
	for _, n := range nodeTypes {
		actual = append(actual, n.kind)
	}
	if !slices.Equal(actual, expected) {
		t.Errorf("expected the kinds %v, got: %v", expected, actual)
	}
}

// expectFields fails the test unless the fields of a variable are the
// expected ones.
func expectFields(t *testing.T, actual, expected map[StrID]*FieldInfo) {
	t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("expected the fields %s, got: %s", formatFields(expected), formatFields(actual))
	}
}

// nodeTypesField returns a field.
//
// nodeTypesField is field in node_types.rs.
func nodeTypesField(p *RulePool, name string, content RuleID) RuleID {
	return p.Field(p.Intern(name), content)
}

// nodeTypesAlias returns an alias.
//
// nodeTypesAlias is alias in node_types.rs.
func nodeTypesAlias(p *RulePool, content RuleID, value string, isNamed bool) RuleID {
	return p.Alias(content, p.Intern(value), isNamed)
}

// nodeTypesExternal returns a new node of the external token with an index.
//
// nodeTypesExternal is external in node_types.rs.
func nodeTypesExternal(p *RulePool, index int) RuleID {
	return p.PushNode(Rule{Kind: RuleSym, Sym: ExternalSymbol(index)})
}

// nodeTypesVariable is a variable for nodeTypesSyntaxGrammar: its name, its
// kind and the steps of each of its productions.
type nodeTypesVariable struct {
	name  string
	kind  VariableType
	prods [][]ProductionStep
}

// nodeTypesSyntaxGrammar returns a syntax grammar with variables and
// supertypes.
//
// nodeTypesSyntaxGrammar is build_syntax_grammar in node_types.rs.
func nodeTypesSyntaxGrammar(interner *StrPool, variables []nodeTypesVariable, supertypeSymbols []Symbol) *SyntaxGrammar {
	g := &SyntaxGrammar{SupertypeSymbols: supertypeSymbols}
	for _, v := range variables {
		prodStart := uint32(len(g.Productions))
		for _, prodSteps := range v.prods {
			stepsStart := uint32(len(g.Steps))
			g.Steps = append(g.Steps, prodSteps...)
			g.Productions = append(g.Productions, Production{
				StepsStart: stepsStart,
				StepsLen:   uint32(len(g.Steps)) - stepsStart,
			})
		}
		g.VarProds = append(g.VarProds, [2]uint32{prodStart, uint32(len(g.Productions))})
		g.Variables = append(g.Variables, SyntaxVariable{Name: interner.Intern(v.name), Kind: v.kind})
	}
	return g
}

// nodeTypesLexicalGrammar returns a lexical grammar of ten named tokens.
//
// nodeTypesLexicalGrammar is build_lexical_grammar in node_types.rs.
func nodeTypesLexicalGrammar(interner *StrPool) *LexicalGrammar {
	g := &LexicalGrammar{}
	for i := range 10 {
		g.Variables = append(g.Variables, LexicalVariable{
			Name: interner.Intern(fmt.Sprintf("token_%d", i)),
			Kind: VariableNamed,
		})
	}
	return g
}

// nodeTypesStep returns a step of a symbol with a field, or with no field
// when field is zero.
//
// nodeTypesStep is step in node_types.rs.
func nodeTypesStep(symbol Symbol, field StrID) ProductionStep {
	return PackProductionStep(symbol, Precedence{}, AssociativityNone, Alias{}, false, field, NoReservedWords)
}
