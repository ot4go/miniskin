## Percent-tag syntaxes (4 forms)

| Syntax | Behavior |
|---|---|
| `<%var%>` | value, escaped per `<escape>` rules |
| `<%%var%%>` | value, never escaped |
| `<!--%var%-->` | same as `<%>`, hidden in browser |
| `<!--%%var%%-->` | same as `<%%>`, hidden in browser |

Single-percent tags (`<%`, `<!--%`) apply the default escape configured via `<escape>` in XML. Double-percent tags (`<%%`, `<!--%%`) never escape. Default escape is none unless configured.

## Directives

| Directive | Description |
|---|---|
| `if:var` / `if-not:var` | Conditional (check if var is defined and non-empty) |
| `elseif:var` / `elseif-not:var` | Chained conditional |
| `else` | Fallback branch |
| `endif` / `end-if` / `end` | Close conditional block |
| `mockup-export:path [append\|overwrite] [ltrim\|rtrim\|trim]` | Extract content to file (mockup mode only). `ltrim`: dedent, `rtrim`: strip trailing whitespace, `trim`: both |
| `end-mockup-export` / `end` | Close mockup-export block |
| `mockup-import:path` | Insert file contents (mockup mode only) |
| `end-mockup-import` / `end` | Close mockup-import block |
| `include:path` | Include file (double tags only, resolved recursively). Absolute (`/path`) relative to bucket src; relative to current file otherwise |
| `note:text` | Discarded silently |
| `echo:text` | Emit text (uses default escape) |

## Escape types

Explicit escape prefix overrides default:
`<%url:var%>`, `<%%js:var%%>`, `<!--%sql:var%-->`, etc.

Prefixes: `html`, `xml`, `url`, `js`, `css`, `json`, `sql`, `sqlt` (sql + LIKE `_%` escaping)

Echo with escape prefix: `<%js:echo:it's "ok"%>` → `it\'s \"ok\"`. Plain `<%echo:text%>` uses configured default escape.

Default escape is none. Configurable per file extension: `<escape ext="*.html" as="html" />` in any XML block. Use `<escape ext="*" as="html" />` at `<miniskin>` level to escape everything as HTML.
Item-level override: `<item escape="sql" .../>`. Cascades like skin-dir and mux-include.
