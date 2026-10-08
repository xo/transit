package main

// styleSheets holds the style sheets of the diagrams by name. They take the
// colors and the widths of the light and the dark style sheets of
// lukaslueg/railroad.
var styleSheets = map[string]string{
	"light": baseCSS + `svg.railroad {
  background-color: hsl(30, 20%, 95%);
  background-image: linear-gradient(to right, rgba(30, 30, 30, .05) 1px, transparent 1px),
    linear-gradient(to bottom, rgba(30, 30, 30, .05) 1px, transparent 1px);
}
svg.railroad rect.railroad_canvas { fill: hsl(30, 20%, 95%); }
svg.railroad path, svg.railroad rect { stroke: black; }
svg.railroad circle.marker { stroke: black; }
svg.railroad text { fill: black; }
svg.railroad text.comment { fill: hsl(0, 0%, 25%); }
svg.railroad text.field { fill: hsl(230, 25%, 40%); }
svg.railroad .terminal rect, svg.railroad .keyword rect, svg.railroad .nonterminal rect { fill: hsl(70, 70%, 90%); }
svg.railroad .pattern rect { fill: hsl(200, 45%, 90%); }
svg.railroad .external rect { fill: hsl(15, 70%, 88%); }
svg.railroad .group rect { stroke: grey; fill: rgb(90, 90, 150); fill-opacity: .08; }
svg.railroad a:hover rect { fill: hsl(70, 90%, 75%); }
body { background: hsl(30, 20%, 98%); color: black; }
a { color: hsl(230, 50%, 35%); }
`,
	"dark": baseCSS + `svg.railroad {
  background-color: hsl(230, 10%, 20%);
  background-image: linear-gradient(to right, rgba(150, 150, 150, .05) 1px, transparent 1px),
    linear-gradient(to bottom, rgba(150, 150, 150, .05) 1px, transparent 1px);
}
svg.railroad rect.railroad_canvas { fill: hsl(230, 10%, 20%); }
svg.railroad path { stroke: hsl(200, 10%, 60%); }
svg.railroad rect { stroke: hsl(200, 10%, 50%); }
svg.railroad circle.marker { stroke: hsl(200, 10%, 60%); }
svg.railroad text { fill: hsl(230, 30%, 80%); }
svg.railroad text.comment { fill: hsl(230, 15%, 65%); }
svg.railroad text.field { fill: hsl(200, 30%, 65%); }
svg.railroad .terminal rect, svg.railroad .keyword rect, svg.railroad .nonterminal rect { fill: hsl(230, 20%, 20%); }
svg.railroad .pattern rect { fill: hsl(200, 25%, 24%); }
svg.railroad .external rect { fill: hsl(15, 25%, 26%); }
svg.railroad .group rect { stroke: grey; fill: rgb(150, 150, 220); fill-opacity: .06; }
svg.railroad a:hover rect { fill: hsl(230, 25%, 32%); }
body { background: hsl(230, 10%, 15%); color: hsl(230, 30%, 85%); }
a { color: hsl(200, 60%, 70%); }
`,
}

// baseCSS holds the rules that do not set a color.
const baseCSS = `svg.railroad { background-size: 15px 15px; }
svg.railroad rect.railroad_canvas { stroke-width: 0; }
svg.railroad path { stroke-width: 3px; fill: none; stroke-linecap: round; stroke-linejoin: round; }
svg.railroad text { font: 14px "DejaVu Sans Mono", "Noto Sans Mono", Menlo, Consolas, monospace; text-anchor: middle; white-space: pre; }
svg.railroad .nonterminal text { font-weight: bold; }
svg.railroad text.comment { font-style: italic; }
svg.railroad text.field { font-size: 12px; font-style: italic; text-anchor: start; }
svg.railroad rect { stroke-width: 3px; }
svg.railroad circle.marker { stroke-width: 3px; fill: none; }
svg.railroad .group rect { stroke-width: 1px; stroke-dasharray: 5px; }
`

// pageCSS holds the rules of the HTML page around the diagrams.
const pageCSS = `body { font-family: sans-serif; margin: 16px; }
nav { columns: 16em; font-family: monospace; margin-bottom: 24px; }
nav a { display: block; }
section { margin: 0 0 28px; }
section h2 { font: bold 16px monospace; margin: 0 0 6px; }
section h2 a { color: inherit; text-decoration: none; }
section svg { display: block; max-width: 100%; height: auto; }
`
