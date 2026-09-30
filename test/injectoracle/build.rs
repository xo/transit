//! Copies highlight.rs of the checkout of upstream to OUT_DIR and patches it,
//! so that main.rs can include it as a module and record the layers, which
//! are private. The checkout is not edited (hard rule 4 of AGENTS.md).

use std::{env, fs, path::PathBuf, process};

/// The text before which the patch records a layer. It is in
/// HighlightIterLayer::new.
const PUSH: &str = "result.push(HighlightIterLayer {";

/// The text that the patch inserts before PUSH. The variables config, depth,
/// ranges and tree are those of HighlightIterLayer::new.
const RECORD: &str = "crate::record_layer(&config.language_name, depth, &ranges, &tree);\n";

/// Stops the build with a message.
fn fail(msg: &str) -> ! {
    eprintln!("injectoracle: {msg}");
    process::exit(1);
}

/// Removes the line that is equal to line. It stops the build when the text
/// does not hold that line exactly once.
fn remove_line(text: &str, line: &str) -> String {
    let count = text.lines().filter(|l| l.trim_end() == line).count();
    if count != 1 {
        fail(&format!(
            "highlight.rs holds the line {line:?} {count} times, and the patch needs it once. Upstream changed, so change build.rs."
        ));
    }
    text.lines()
        .filter(|l| l.trim_end() != line)
        .map(|l| format!("{l}\n"))
        .collect()
}

fn main() {
    let manifest = PathBuf::from(env::var("CARGO_MANIFEST_DIR").unwrap());
    let src = manifest.join("../../tree-sitter/crates/highlight/src/highlight.rs");
    println!("cargo::rerun-if-changed={}", src.display());
    let text = fs::read_to_string(&src)
        .unwrap_or_else(|e| fail(&format!("reading {}: {e}", src.display())));

    let text = remove_line(
        &text,
        r#"#![cfg_attr(not(any(test, doctest)), doc = include_str!("../README.md"))]"#,
    );
    let text = remove_line(&text, "pub mod c_lib;");
    let text = remove_line(&text, "pub use c_lib as c;");

    let count = text.matches(PUSH).count();
    if count != 1 {
        fail(&format!(
            "highlight.rs holds {PUSH:?} {count} times, and the patch needs it once. Upstream changed, so change build.rs."
        ));
    }
    let text = text.replacen(PUSH, &format!("{RECORD}{PUSH}"), 1);

    let out = PathBuf::from(env::var("OUT_DIR").unwrap()).join("highlight.rs");
    fs::write(&out, text).unwrap_or_else(|e| fail(&format!("writing {}: {e}", out.display())));
}
