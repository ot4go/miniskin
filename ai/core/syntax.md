## Percent-tag syntaxes (4 equivalent forms)

| Syntax | Use case |
|---|---|
| `<%var%>` | Template variables |
| `<%%var%%>` | Variables, includes, mockup directives |
| `<!--%var%-->` | Browser-invisible (mockup HTML) |
| `<!--%%var%%-->` | Browser-invisible (mockup HTML) |

All four syntaxes behave the same. Default escape is none. Configure per file extension with `<escape>` in XML.

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
| `echo:text` | Emit text (uses default escape) |

## Escape types

Explicit escape prefix overrides default:
`<%url:var%>`, `<%%js:var%%>`, `<!--%sql:var%-->`, etc.

Prefixes: `html`, `xml`, `url`, `js`, `css`, `json`, `sql`, `sqlt` (sql + LIKE `_%` escaping)

Echo with escape prefix: `<%js:echo:it's "ok"%>` → `it\'s \"ok\"`. Plain `<%echo:text%>` uses configured default escape.

Default escape is none. Configurable per file extension: `<escape ext="*.html" as="html" />` in any XML block. Use `<escape ext="*" as="html" />` at `<miniskin>` level to escape everything as HTML.
Item-level override: `<item escape="sql" .../>`. Cascades like skin-dir and mux-include.
