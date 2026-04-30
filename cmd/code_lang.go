package cmd

import (
	"fmt"
	"os"
	"strings"
)

// notionCodeLanguages lists every language value accepted by the Notion API
// for a code block. Values not in this set must be normalized to an alias or
// fall back to "plain text", otherwise the API rejects the whole request.
//
// Source: Notion API reference, code block type (May 2024).
var notionCodeLanguages = map[string]struct{}{
	"abap": {}, "agda": {}, "arduino": {}, "ascii art": {}, "assembly": {},
	"bash": {}, "basic": {}, "bnf": {}, "c": {}, "c#": {}, "c++": {}, "clojure": {},
	"coffeescript": {}, "coq": {}, "css": {}, "dart": {}, "dhall": {}, "diff": {},
	"docker": {}, "ebnf": {}, "elixir": {}, "elm": {}, "erlang": {}, "f#": {},
	"flow": {}, "fortran": {}, "gherkin": {}, "glsl": {}, "go": {}, "graphql": {},
	"groovy": {}, "haskell": {}, "hcl": {}, "html": {}, "idris": {}, "java": {},
	"java/c/c++/c#": {}, "javascript": {}, "json": {}, "julia": {}, "kotlin": {},
	"latex": {}, "less": {}, "lisp": {}, "livescript": {}, "llvm ir": {}, "lua": {},
	"makefile": {}, "markdown": {}, "markup": {}, "matlab": {}, "mathematica": {},
	"mermaid": {}, "nix": {}, "notion formula": {}, "objective-c": {}, "ocaml": {},
	"pascal": {}, "perl": {}, "php": {}, "plain text": {}, "powershell": {},
	"prolog": {}, "protobuf": {}, "purescript": {}, "python": {}, "r": {},
	"racket": {}, "reason": {}, "ruby": {}, "rust": {}, "sass": {}, "scala": {},
	"scheme": {}, "scss": {}, "shell": {}, "smalltalk": {}, "solidity": {},
	"sql": {}, "swift": {}, "toml": {}, "typescript": {}, "vb.net": {}, "verilog": {},
	"vhdl": {}, "visual basic": {}, "webassembly": {}, "xml": {}, "yaml": {},
}

// codeLangAliases maps common markdown / editor fence labels to the exact
// enum Notion accepts. Values already matching the enum short-circuit and
// never hit this table.
var codeLangAliases = map[string]string{
	// JS / TS family
	"ts":         "typescript",
	"tsx":        "typescript",
	"js":         "javascript",
	"jsx":        "javascript",
	"mjs":        "javascript",
	"cjs":        "javascript",
	"node":       "javascript",
	"nodejs":     "javascript",
	"coffee":     "coffeescript",
	// Python
	"py":     "python",
	"python3": "python",
	// Shells — note: bash is already in the enum.
	"sh":   "shell",
	"zsh":  "shell",
	"ksh":  "shell",
	"fish": "shell",
	// Systems
	"rs":     "rust",
	"golang": "go",
	"cpp":    "c++",
	"cc":     "c++",
	"cxx":    "c++",
	"h":      "c",
	"hpp":    "c++",
	"cs":     "c#",
	"csharp": "c#",
	"fs":     "f#",
	"fsharp": "f#",
	"vb":     "vb.net",
	// Config / data
	"yml":        "yaml",
	"jsonc":      "json",
	"json5":      "json",
	"hcl2":       "hcl",
	"tf":         "hcl",
	"terraform":  "hcl",
	"proto":      "protobuf",
	"dockerfile": "docker",
	"conf":       "plain text",
	"ini":        "plain text",
	// Web
	"htm":    "html",
	"svg":    "html",
	"vue":    "html",
	"scss":   "scss",
	"stylus": "css",
	// Docs / markup
	"md":       "markdown",
	"markdown": "markdown",
	"mdx":      "markdown",
	"rst":      "plain text",
	"tex":      "latex",
	// Other common aliases
	"rb":       "ruby",
	"kt":       "kotlin",
	"kts":      "kotlin",
	"objc":     "objective-c",
	"objective-c++": "objective-c",
	"m":        "objective-c",
	"mm":       "objective-c",
	"wasm":     "webassembly",
	"wat":      "webassembly",
	"ps":       "powershell",
	"ps1":      "powershell",
	"psm1":     "powershell",
	"bat":      "shell",
	"cmd":      "shell",
	"pl":       "perl",
	"ex":       "elixir",
	"exs":      "elixir",
	"erl":      "erlang",
	"hs":       "haskell",
	"cl":       "lisp",
	"el":       "lisp",
	"clj":      "clojure",
	"cljs":     "clojure",
	"ml":       "ocaml",
	"gql":      "graphql",
	"text":     "plain text",
	"txt":      "plain text",
	"output":   "plain text",
	"log":      "plain text",
	"console":  "shell",
	"tty":      "shell",
}

// normalizeCodeLanguage converts a raw markdown fence language label into
// a value that the Notion API's code.language enum accepts.
//
// Empty input resolves to "plain text". Values already in the enum are
// returned lowercased and unchanged. Known aliases are mapped. Unknown
// values fall back to "plain text" and emit a one-line stderr warning so
// long docs don't hard-fail on a single unrecognized fence tag.
func normalizeCodeLanguage(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return "plain text"
	}
	// Normalize dashes/dots some editors emit (`.py`, `c-plus-plus`)
	s = strings.TrimPrefix(s, ".")

	if _, ok := notionCodeLanguages[s]; ok {
		return s
	}
	if mapped, ok := codeLangAliases[s]; ok {
		return mapped
	}
	fmt.Fprintf(os.Stderr, "note: unknown code language %q, falling back to plain text\n", raw)
	return "plain text"
}
