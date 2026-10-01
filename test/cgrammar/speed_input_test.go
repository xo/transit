package cgrammar

import (
	"fmt"
	"strings"
)

// This file writes the inputs of the speed benchmarks of D37. Each input is
// one statement of about size bytes: a JSON array, or one function of C,
// JavaScript, Python or Rust. The size of the input is the size of the
// statement that target 3 names. Each input holds the identifier item_<n>
// many times, and a key inserts an x into one of them (speedKeyOffset), so
// the text stays free of errors before and after the key.

// speedJSON returns a JSON array of objects with strings, numbers, booleans,
// null and nested arrays.
func speedJSON(size int) string {
	var b strings.Builder
	b.WriteString("[\n")
	for i := 0; b.Len() < size-64; i++ {
		if i > 0 {
			b.WriteString(",\n")
		}
		fmt.Fprintf(&b, `  {"id": %d, "item_%d": "value %d", "price": %d.%02d, "tags": ["a", "bé", "c"], "active": %t, "parent": null}`,
			i, i, i, i*3, i%100, i%2 == 0)
	}
	b.WriteString("\n]\n")
	return b.String()
}

// speedC returns a function of C with declarations, if and else, for loops,
// calls, a switch and casts.
func speedC(size int) string {
	var b strings.Builder
	b.WriteString("int process_items(struct item *items, int count) {\n  int total = 0;\n")
	for i := 0; b.Len() < size-32; i++ {
		switch i % 5 {
		case 0:
			fmt.Fprintf(&b, "  int item_%d = items[%d].value * %d + total;\n", i, i, i%9+1)
		case 1:
			fmt.Fprintf(&b, "  if (item_%d > %d && (items[%d].flags & 0x%x) != 0) {\n    total += item_%d;\n  } else {\n    total -= %d;\n  }\n", i-1, i, i, i, i-1, i)
		case 2:
			fmt.Fprintf(&b, "  printf(\"item %%d: %%s\\n\", item_%d, names[%d]);\n", i-2, i)
		case 3:
			fmt.Fprintf(&b, "  for (int j = 0; j < %d; j++) {\n    buffer[j] = (char)(item_%d >> 2);\n  }\n", i, i-3)
		case 4:
			fmt.Fprintf(&b, "  switch (item_%d) {\n  case %d:\n    total++;\n    break;\n  default:\n    total--;\n  }\n", i-4, i)
		}
	}
	b.WriteString("  return total;\n}\n")
	return b.String()
}

// speedJavaScript returns a function of JavaScript with declarations, if
// and else, template strings, for loops, object literals, arrow functions,
// comments and statements with no semicolon, which the external scanner
// reads.
func speedJavaScript(size int) string {
	var b strings.Builder
	b.WriteString("function processItems(items, options) {\n  let total = 0;\n")
	for i := 0; b.Len() < size-32; i++ {
		switch i % 6 {
		case 0:
			fmt.Fprintf(&b, "  const item_%d = items[%d].value * %d + total;\n", i, i, i%9+1)
		case 1:
			fmt.Fprintf(&b, "  if (item_%d > %d && options.flag) {\n    total += item_%d;\n  } else {\n    total -= %d;\n  }\n", i-1, i, i-1, i)
		case 2:
			fmt.Fprintf(&b, "  console.log(`item ${item_%d}: ${items[%d].name}`);\n", i-2, i)
		case 3:
			fmt.Fprintf(&b, "  for (let j = 0; j < %d; j++) {\n    buffer.push({ id: j, name: \"item\", value: item_%d >> 2 });\n  }\n", i, i-3)
		case 4:
			fmt.Fprintf(&b, "  const fn_%d = (a, b) => a + b * item_%d;\n  // the function %d\n", i, i-4, i)
		case 5:
			fmt.Fprintf(&b, "  total = total + fn_%d(item_%d, %d)\n", i-1, i-5, i)
		}
	}
	b.WriteString("  return total;\n}\n")
	return b.String()
}

// speedPython returns a function of Python with assignments, if and else,
// format strings, for loops, dictionaries, comments and strings in three
// quotes. The external scanner reads the indents and the strings.
func speedPython(size int) string {
	var b strings.Builder
	b.WriteString("def process_items(items, options):\n    total = 0\n")
	for i := 0; b.Len() < size-32; i++ {
		switch i % 5 {
		case 0:
			fmt.Fprintf(&b, "    item_%d = items[%d].value * %d + total\n", i, i, i%9+1)
		case 1:
			fmt.Fprintf(&b, "    if item_%d > %d and options.flag:\n        total += item_%d\n    else:\n        total -= %d\n", i-1, i, i-1, i)
		case 2:
			fmt.Fprintf(&b, "    print(f\"item {item_%d}: {items[%d].name}\")\n", i-2, i)
		case 3:
			fmt.Fprintf(&b, "    for j in range(%d):\n        buffer.append({\"id\": j, \"name\": 'item', \"value\": item_%d >> 2})\n", i, i-3)
		case 4:
			fmt.Fprintf(&b, "    # the text %d\n    text_%d = \"\"\"a text\n    of two lines\"\"\"\n", i, i)
		}
	}
	b.WriteString("    return total\n")
	return b.String()
}

// speedRust returns a function of Rust with let statements, if and else,
// macros, for loops, struct expressions, raw strings, block comments and
// match expressions. The external scanner reads the raw strings, the
// strings and the block comments.
func speedRust(size int) string {
	var b strings.Builder
	b.WriteString("fn process_items(items: &[Item], options: &Options) -> i64 {\n    let mut total: i64 = 0;\n")
	for i := 0; b.Len() < size-32; i++ {
		switch i % 6 {
		case 0:
			fmt.Fprintf(&b, "    let item_%d = items[%d].value * %d + total;\n", i, i, i%9+1)
		case 1:
			fmt.Fprintf(&b, "    if item_%d > %d && options.flag {\n        total += item_%d;\n    } else {\n        total -= %d;\n    }\n", i-1, i, i-1, i)
		case 2:
			fmt.Fprintf(&b, "    println!(\"item {}: {}\", item_%d, items[%d].name);\n", i-2, i)
		case 3:
			fmt.Fprintf(&b, "    for j in 0..%d {\n        buffer.push(Entry { id: j, name: r#\"item\"#, value: item_%d >> 2 });\n    }\n", i, i-3)
		case 4:
			fmt.Fprintf(&b, "    let text_%d = /* the text %d */ \"a text\";\n", i, i)
		case 5:
			fmt.Fprintf(&b, "    let m_%d = match item_%d {\n        0 => 1,\n        n if n > %d => n,\n        _ => 2,\n    };\n", i, i-5, i)
		}
	}
	b.WriteString("    total\n}\n")
	return b.String()
}

// speedKeyOffset returns the offset of the key in src: inside the first
// identifier item_<n> after the middle of src, after its second byte.
func speedKeyOffset(src []byte) int {
	i := strings.Index(string(src[len(src)/2:]), "item_")
	if i < 0 {
		panic("the input holds no item_ after its middle")
	}
	return len(src)/2 + i + 2
}
