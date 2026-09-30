//! The Rust oracle of D71. It reads the languages, the name of the root
//! language and a text as JSON on stdin. It highlights the text with the
//! highlighter of upstream, and it prints each layer that the highlighter
//! builds as JSON on stdout, in the order of D72.

use std::{
    cell::RefCell,
    io::{self, Read, Write},
    process,
};

use serde::{Deserialize, Serialize};

/// The highlighter of upstream, which build.rs patches to call record_layer.
#[allow(dead_code, unused_imports, unused_variables, clippy::all)]
mod highlight {
    include!(concat!(env!("OUT_DIR"), "/highlight.rs"));
}

use highlight::{HighlightConfiguration, Highlighter};

/// A language of the input.
#[derive(Deserialize)]
struct LanguageInput {
    name: String,
    library: String,
    symbol: String,
    injections: String,
}

/// The input on stdin.
#[derive(Deserialize)]
struct Input {
    languages: Vec<LanguageInput>,
    root: String,
    source: String,
}

/// A range of a layer.
#[derive(Serialize)]
struct RangeOutput {
    start_byte: u64,
    end_byte: u64,
    start_point: [u64; 2],
    end_point: [u64; 2],
}

/// A layer that HighlightIterLayer::new pushed.
#[derive(Serialize)]
struct LayerOutput {
    name: String,
    depth: u64,
    ranges: Vec<RangeOutput>,
    tree: String,
}

thread_local! {
    /// The layers that record_layer recorded, in the order of the pushes.
    static LAYERS: RefCell<Vec<LayerOutput>> = const { RefCell::new(Vec::new()) };
}

/// Records a layer. The patched HighlightIterLayer::new calls it before it
/// pushes the layer.
fn record_layer(name: &str, depth: usize, ranges: &[tree_sitter::Range], tree: &tree_sitter::Tree) {
    let ranges = ranges
        .iter()
        .map(|r| RangeOutput {
            start_byte: r.start_byte as u64,
            end_byte: r.end_byte as u64,
            start_point: [r.start_point.row as u64, r.start_point.column as u64],
            end_point: [r.end_point.row as u64, r.end_point.column as u64],
        })
        .collect();
    let layer = LayerOutput {
        name: name.to_string(),
        depth: depth as u64,
        ranges,
        tree: tree.root_node().to_sexp(),
    };
    LAYERS.with(|l| l.borrow_mut().push(layer));
}

/// Loads the language that the function symbol of the shared library path
/// returns, as the loader of upstream does.
fn load_language(path: &str, symbol: &str) -> Result<tree_sitter::Language, String> {
    // SAFETY: the library is a grammar that the test module built, and the
    // function has the type of tree_sitter_<name> of a generated parser.
    unsafe {
        let lib = libloading::Library::new(path).map_err(|e| format!("loading {path}: {e}"))?;
        let func = lib
            .get::<libloading::Symbol<unsafe extern "C" fn() -> tree_sitter::Language>>(
                symbol.as_bytes(),
            )
            .map_err(|e| format!("finding {symbol} in {path}: {e}"))?;
        let language = func();
        // The language points into the library, so the library stays loaded
        // until the program ends.
        std::mem::forget(lib);
        Ok(language)
    }
}

/// Runs the oracle on the input text, and returns the output text.
fn run(input: &str) -> Result<String, String> {
    let input: Input =
        serde_json::from_str(input).map_err(|e| format!("reading the input: {e}"))?;
    let mut configs = Vec::with_capacity(input.languages.len());
    for l in &input.languages {
        let language = load_language(&l.library, &l.symbol)?;
        let config = HighlightConfiguration::new(language, l.name.as_str(), "", &l.injections, "")
            .map_err(|e| format!("making the configuration of {}: {e}", l.name))?;
        configs.push(config);
    }
    let root = configs
        .iter()
        .find(|c| c.language_name == input.root)
        .ok_or_else(|| format!("finding the root language {}", input.root))?;
    let source = input.source.as_bytes();
    let mut highlighter = Highlighter::new();
    let events = highlighter
        .highlight(root, source, None, None, |name| {
            configs.iter().find(|c| c.language_name == name)
        })
        .map_err(|e| format!("highlighting: {e}"))?;
    for event in events {
        event.map_err(|e| format!("highlighting: {e}"))?;
    }
    let mut layers = LAYERS.with(RefCell::take);
    // The order of D72: the start of the first range, then the depth. The
    // sort is stable, so equal layers keep the order of the pushes.
    layers.sort_by_key(|l| (l.ranges.first().map_or(0, |r| r.start_byte), l.depth));
    serde_json::to_string(&layers).map_err(|e| format!("writing the output: {e}"))
}

fn main() {
    let mut input = String::new();
    if let Err(e) = io::stdin().read_to_string(&mut input) {
        eprintln!("injectoracle: reading stdin: {e}");
        process::exit(1);
    }
    match run(&input) {
        Ok(out) => {
            if let Err(e) = writeln!(io::stdout(), "{out}") {
                eprintln!("injectoracle: writing stdout: {e}");
                process::exit(1);
            }
        }
        Err(e) => {
            eprintln!("injectoracle: {e}");
            process::exit(1);
        }
    }
}
