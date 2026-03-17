## Path resolution

All paths inside a bucket resolve relative to the bucket's `src` directory (`contentPath + bucket.Src`):

- **Absolute paths** (starting with `/`): relative to bucket `src`
  - `include:/utils/helper.js` → `bucketSrc/utils/helper.js`
  - `mockup-export:/login/page.html` → `bucketSrc/login/page.html`
  - `mockup-import:/shared/header.html` → `bucketSrc/shared/header.html`
- **Relative paths** (no leading `/`): relative to the current source file's directory
  - `include:helper.js` → same directory as the file containing the include
- **`dst`** in `<bucket>`: relative to `project-root` (not bucket src)
  - `project-root` is set on `<bucket-list>` and is relative to `contentPath`
  - Example: `project-root=".."` with `dst="/modules/app/gen.go"` → `contentPath/../modules/app/gen.go`

## Validation

- During build embed, all output files are validated to exist on disk before code generation
- Missing files produce an error with the absolute path

## Key behaviors

- `mockup-export` inside `if:mockup` is silently skipped in normal mode (no error)
- `mockup-import` inside `mockup-export` works (imported content becomes part of export)
- Export block dependencies resolved at block level: if export A imports a path produced by export B (same or different file), B is processed first
- `mockup-export` inside `mockup-import` is ignored (raw text, not parsed)
- In mockup mode: variables, includes, echo, note pass through literally (not resolved)
- Conditionals in mockup mode check existence only (defined = true)
- `mockup=1` is auto-injected in mockup mode
- `end` is universal closer (works for if, mockup-export, mockup-import blocks)
- Specific closers: `end-if` (if only), `end-mockup-export` (export only), `end-mockup-import` (import only)
- `TransformNegative` replaces export...end blocks with import...end-mockup-import blocks
- Resource lists can be chained (multiple at the same level) and nested (with `src` for relative path resolution)
- Skin directory cascades: `<miniskin>` → `<bucket>` → `<resource-list>` → nested `<resource-list>` (default: `_skin`)
- Mux-include/mux-exclude cascades: `<miniskin>` → `<bucket-list>` → `<bucket>` → `<resource-list>` → nested `<resource-list>` (default: `mux-include="*"`, `mux-exclude=""`)
- Escape rules cascade through the same hierarchy, including nested resource-lists
- Items not matching `mux-include` or matching `mux-exclude` get `nomux` added automatically
- Explicit `nomux` in item type always takes precedence
- Save-mode cascades: `<mockup-list>` → `<item>` → tag-level mode
- Variable merge order: globals → mockup-list vars → item vars → front-matter vars
- First write to a file in a session always truncates; subsequent writes respect mode
- `refreshImports` is idempotent: single tags promoted to blocks, existing blocks get content replaced
