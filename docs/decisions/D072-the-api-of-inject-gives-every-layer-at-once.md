# D72. The API of inject gives every layer at once

Status: Decided.

Ken decided on 2026-09-30, answering question 62, the Go API of the package
`inject` (D27, D71). Upstream has no public API for injections.
`HighlightIterLayer` of `crates/highlight` finds the layers inside the
iterator of highlight events. transit ports that part without the
highlighting, so its Go API is new. Gemini and DeepSeek reviewed the first
proposal, and this decision takes their points that hold against upstream.

```go
func NewConfig(language *transit.Language, name, injectionQuery string) (*Config, error)
func (c *Config) Name() string
func (c *Config) Language() *transit.Language

type Layer struct {
	Name   string // the language name that the query gave, such as "js"
	Config *Config
	Tree   *transit.Tree
	Ranges []transit.Range
	Depth  int // 0 for the root layer
}

func (c *Config) Layers(ctx context.Context, p *transit.Parser, src []byte,
	lookup func(name string) (*Config, bool)) ([]Layer, error)
```

`Layers` parses every layer at once, with no old tree. These rules come with
it:

1. The caller passes the parser, so that rline can reuse one parser and its
   logger. `Layers` clears the included ranges of the parser before it
   returns. The language of the last layer stays set.
2. `NewConfig` takes the injection query only. Upstream joins it with the
   locals and highlight queries, and they change nothing about the layers.
3. The layers come sorted by the start of their first range, then by depth.
   Two layers that are equal on both keep the order in which the port finds
   them. The root layer comes first. Upstream never makes a list of layers,
   so it has no order to match. The Rust oracle of D71 prints its layers in
   the same order.
4. The injection matches use the exported `Query` and `QueryCursor`, which
   evaluate the predicates, such as `#eq?` and `#match?`.
5. When `lookup` finds no configuration for a name, the injection is skipped
   with no error. A layer with empty ranges is not built. When the parser
   rejects the included ranges of a layer, the layer is skipped with no
   error, as upstream does.
6. When the language of a layer cannot be set, or when the context ends,
   `Layers` returns the error, wrapped, and no layers.
7. Upstream has no limit on depth, so a grammar that injects itself over the
   same range recurses with no end. The port keeps this fault (hard rule 6).
8. `injection.parent` names the root language at every depth, as upstream
   does (hard rule 6).
9. A `Config` does not change after `NewConfig`, so goroutines can share it.
   A call to `Layers` uses its own query cursor.

A `Document` that keeps the layers of the last parse, and parses each layer
again with its old tree after an edit, is not part of this decision. It adds
to upstream, so it needs a decision of its own (D28). It can be added on top
of this API with no break, if a measurement against the targets of D37 shows
that `Layers` is too slow. A lazy `iter.Seq2` of layers was also proposed,
and Ken did not choose it. Upstream is lazy only to interleave parsing with
highlight events.
