## miniskin

Build-time template assembler for Go projects. Processes `*.miniskin.xml` catalogs,
resolves percent tags, applies skins (layouts), and generates Go source code with `//go:embed` directives. Supports Mockup-Driven Development (MDD).

## Pipeline

`Run()` executes four steps in order:

1. **Analyze dependencies** — build export/import graph, error on circular references
2. **Export mockups** — `mockup-export` directives write content to files on disk
3. **Update imports** — refresh inline content in `mockup-import` blocks
4. **Build embed** — process resource items, resolve variables, apply skins

Code generation (`GenerateAll`) runs after `Run()` to produce `//go:embed` Go files.
