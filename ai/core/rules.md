## Path resolution

`/` and `\` are treated as equivalent separators in all paths. The rule is uniform:

- **`/` or `\` prefix** → relative to `bucketSrc` (`contentPath + bucket.Src`)
  - `include:/utils/helper.js` → `bucketSrc/utils/helper.js`
  - `mockup-export:/login/page.html` → `bucketSrc/login/page.html`
  - `mockup-import:/shared/header.html` → `bucketSrc/shared/header.html`
- **No prefix** → relative to the current source file's directory
  - `include:helper.js` → same directory as the file containing the include
- **`bucket.Src`** → relative to `contentPath` (both `/app` and `app` resolve the same)
- **`skin-dir`** → relative to `bucketSrc` (fallback: `contentPath` when no bucket context)
- **`dst`** in `<bucket>`: relative to `project-root` (not bucket src)
  - `project-root` is set on `<bucket-list>` and is relative to `contentPath`
  - Example: `project-root=".."` with `dst="/modules/app/gen.go"` → `contentPath/../modules/app/gen.go`

## Validation

- During build embed, all output files are validated to exist on disk before code generation
- Missing files produce an error with item name, absolute path, and XML origin:
  ```
  item "app.css" not found at: /abs/path/content/app/app.css
      (declared in /abs/path/content/app/app.miniskin.xml line 7)
  ```

## Key behaviors

- `mockup-export` inside `if:mockup` is silently skipped in normal mode (no error)
- `mockup-import` inside `mockup-export` works (imported content becomes part of export)
- Export block dependencies resolved at block level: if export A imports a path produced by export B (same or different file), B is processed first
- `mockup-export` inside `mockup-import` is ignored (raw text, not parsed)
- In mockup mode: variables, includes, echo, note pass through literally (not resolved)
- Conditionals in mockup mode check existence only (defined = true)
- **Inside `mockup-export` blocks**: all conditionals (`if:`, `else`, `endif`, etc.) pass through literally to the export buffer — nothing is excluded. FSM block stack stays balanced for correct nesting.
- `mockup=1` is auto-injected in mockup mode
- `end` is universal closer (works for if, mockup-export, mockup-import blocks)
- Specific closers: `end-if` (if only), `end-mockup-export` (export only), `end-mockup-import` (import only)
- `TransformNegative` replaces export...end blocks with import...end-mockup-import blocks
- Resource lists can be chained (multiple at the same level) and nested (with `src` for relative path resolution)
- Skin directory cascades: `<miniskin>` → `<bucket>` → `<resource-list>` → nested `<resource-list>` (default: `_skin`)
- Mux-include/mux-exclude cascades: `<miniskin>` → `<bucket-list>` → `<bucket>` → `<resource-list>` → nested `<resource-list>` (default: `mux-include="*"`, `mux-exclude=""`)
- Template-function-map cascades: `<bucket>` → `<resource-list>` → `<item>`. Injects `template.FuncMap` into parsed templates via `.Funcs(expr)` before `.Parse()`
- Escape rules cascade through the same hierarchy, including nested resource-lists
- Items not matching `mux-include` or matching `mux-exclude` get `nomux` added automatically
- Explicit `nomux` in item type always takes precedence
- Save-mode cascades: `<mockup-list>` → `<item>` → tag-level mode
- Variable merge order: globals → mockup-list vars → item vars → front-matter vars
- First write to a file in a session always truncates; subsequent writes respect mode
- `refreshImports` is idempotent: single tags promoted to blocks, existing blocks get content replaced
- **`line-mode`** (default: on): when `mockup-import`, `mockup-export`, or `include` tags appear inside a line with surrounding content (e.g. `/* <%%include:file.js%%> */`), the entire line is consumed — content before the tag is truncated, content after is discarded. Disable with `<mockup-list line-mode="off">`
- **JS-comment wrapper** (`/*<% … %>*/` and `/*<%% … %%>*/`): two extra tag syntaxes (5 and 6) recognised by the FSM. The `/*` and `*/` are part of the delimiter and consumed with the tag — they do **not** appear in the output. Apertura and closure are independent: a tag opened with `/*<%` may close with `%>` (the `*/` is not consumed) and vice versa. Useful for embedding tags inside `.js` / `.css` so the file remains valid when read raw (e.g. during mockup development). Apertura matches only `/*<%` — `/*<!` is **not** treated as an HTML-comment wrapper.
