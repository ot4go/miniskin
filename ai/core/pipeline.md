## Front-matter

YAML-like block delimited by `---` at the top of source files:

```
---
skin: default
title: Sign In
@minify:1
@eol:lf
---
<div>content</div>
```

`skin` triggers layout application. Other keys become template variables.
Lines starting with `@` are directives (processed by miniskin, not passed as variables).

### Directives

| Directive | Values | Description |
|---|---|---|
| `@minify` | `0` (default), `1` | Level 1: trim lines and remove empty lines |
| `@eol` | `lf`, `crlf`, `cr` | Convert line endings (default: no conversion) |

## XML structure

**Root** (`contentPath/*.miniskin.xml`):

```xml
<miniskin skin-dir="_skin" log="miniskin.log">
  <globals>
    <var name="appName" value="MyApp" />
  </globals>
  <escape ext="*.html,*.html.tmpl" as="html" />
  <escape ext="*.js,*.js.tmpl" as="js" />
  <bucket-list filename="generated_embed.go" module="content" import="pkg/content" project-root="..">
    <bucket src="app" dst="/modules/app/generated_assets.go"
            module-name="app" template="miniskin::mux" />
  </bucket-list>
</miniskin>
```

### `<bucket-list>` attributes

| Attribute | Description |
|---|---|
| `filename`     | embed file path written by `Codegen.GenerateEmbed` (e.g. `generated_embed.go`) |
| `module`       | Go package name for the generated embed file |
| `import`       | Go import path for the generated embed file |
| `template`     | custom embed template (`miniskin::default`, `miniskin::mux`, or a file) |
| `project-root` | path resolved from `contentPath` for resolving bucket `dst`s |
| `mux-include` / `mux-exclude` | cascading mux glob patterns |
| `omit`         | comma/space-separated codegen outputs to skip — `embed` (skip the embed file) and/or `module` (skip per-bucket module files). When both are listed, `filename` and `module` may be omitted entirely. Useful when miniskin is used to assemble assets for non-Go projects. |

**Subdirectory** (`subdir/*.miniskin.xml`):

```xml
<miniskin>
  <resource-list urlbase="/assets" skin-dir="skins">
    <item type="static" file="app.css" />
    <item type="html-template,parse" src="page_src.html" file="page.html" key="/page" />
  </resource-list>
  <mockup-list save-mode="append" line-mode="off">
    <var name="policybanner" value="1" />
    <item src="mockup_login.html">
      <var name="title" value="Login" />
    </item>
  </mockup-list>
</miniskin>
```

Resource lists can be **chained** (multiple at the same level) and **nested** (with `src` for relative path resolution):

```xml
<miniskin>
  <resource-list urlbase="/assets">
    <item type="static" file="app.css" />
    <resource-list src="login" urlbase="/login">
      <item src="signin_src.html" file="signin.html" />
    </resource-list>
  </resource-list>
</miniskin>
```

Nested resource lists inherit `skin-dir`, `mux-include`, `mux-exclude`, `template-function-map`, and `<escape>` rules from their parent.

## Item attributes

- `file` — output filename (what gets embedded)
- `src` — source filename (if present, processed through template engine)
- `type` — comma-separated flags: `static`, `html-template`, `nomux`, `parse`
- `key` — logical key for asset lookup
- `url` / `alt-url-abs` — URL routing attributes
- `escape` — override default escape type for this item (`html`, `js`, `url`, `sql`, etc.)
- `template-function-map` — Go expression returning `template.FuncMap`; injected via `.Funcs(expr)` before `.Parse()` for `parse` items (cascades from `<bucket>` → `<resource-list>` → `<item>`)

## Built-in templates

- `miniskin::default` — generic `Asset` type with `Get()`, `RegisterRoutes()`, `GetParsedTemplate()`
- `miniskin::mux` — `RegisterRoutes(mux *http.ServeMux, tmplHandlers)` with exact-path matching

Custom templates via file path: `template="my_template.tmpl"`
