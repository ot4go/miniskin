## Percent-tag syntaxes (4 equivalent forms)

| Syntax | Escaping | Use case |
|---|---|---|
| `<%var%>` | HTML-escaped | Template variables |
| `<%%var%%>` | Literal | Variables, includes, mockup directives |
| `<!--%var%-->` | HTML-escaped | Browser-invisible (mockup HTML) |
| `<!--%%var%%-->` | Literal | Browser-invisible (mockup HTML) |

Comment forms (`<!--%...%-->`, `<!--%%...%%-->`) are HTML comments — invisible in the browser, ideal for MDD mockup files.

## Directives

| Directive | Description |
|---|---|
| `if:var` / `if-not:var` | Conditional (check if var is defined and non-empty) |
| `elseif:var` / `elseif-not:var` | Chained conditional |
| `else` | Fallback branch |
| `endif` / `end-if` / `end` | Close conditional block |
| `mockup-export:path [append\|overwrite]` | Extract content to file (mockup mode only) |
| `end-mockup-export` / `end` | Close mockup-export block |
| `mockup-import:path` | Insert file contents (mockup mode only) |
| `end-mockup-import` / `end` | Close mockup-import block |
| `include:path` | Include file (double tags only, resolved recursively) |
| `note:text` | Discarded silently |
| `echo:text` | Emit text (escaped in single, literal in double) |

## Escape types

Explicit escape prefix overrides default (single=html, double=none):
`<%url:var%>`, `<%%js:var%%>`, `<!--%sql:var%-->`, etc.

Prefixes: `html`, `xml`, `url`, `js`, `css`, `json`, `sql`, `sqlt` (sql + LIKE `_%` escaping)

Echo with escape prefix: `<%js:echo:it's "ok"%>` → `it\'s \"ok\"`. Plain `<%echo:text%>` uses configured default escape.

Default escape configurable per file extension: `<escape ext="*.js" as="js" />` in any XML block.
Item-level override: `<item escape="sql" .../>`. Cascades like skin-dir and mux-include.
