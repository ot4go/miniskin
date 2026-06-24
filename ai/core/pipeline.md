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
| `@minify` | `0` (default), `1` | Minify output via tdewolff/minify, selected by output extension (html/css/js/json/svg/xml). Other types pass through unchanged. |
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
  <external>
    <external-item origin="closure-ui" src="./release/closure_ui.js" dstfile="./src/app_source.js" />
  </external>
</miniskin>
```

## External / Origin

`<external>` blocks copy files into the project at build time from sources declared in a per-developer registry. Used to keep a sibling project (e.g. a UI library) in its own repo while consuming its built artefact here, without checking the artefact into the consuming repo's history.

### Registry: `miniskin-origin.xml`

Lives at `contentPath` root, **not** committed (each developer points origins at their own local clones). Optional — only required when `<external>` blocks exist.

```xml
<miniskin>
  <origin name="closure-ui">
    <local>C:\HD\F\_sams\closure-ui</local>
  </origin>
</miniskin>
```

`<origin>` attributes:

| Attribute | Description |
|---|---|
| `name` | identifier referenced from `<external-item origin="…">` |

`<origin>` children:

| Element | Description |
|---|---|
| `<local>` | absolute path to a sibling repo / build output directory on the developer's machine |

Only `<local>` is supported — no fetch from network sources. The MVP exists to keep cross-platform complexity out (TLS, proxies, Windows path quirks, auth); each developer is expected to clone and build the sibling project themselves.

### `<external-item>`

Lives inside `<external>` in any subdirectory `*.miniskin.xml`. Resolved at pipeline step 0.

| Attribute | Description |
|---|---|
| `origin` | name of an entry in `miniskin-origin.xml` |
| `src`    | path inside the origin's `<local>` root |
| `dstfile`| destination relative to the directory of the declaring XML (same convention as `<item src>`) |

Copy is **mtime+size-aware**: dst is left untouched when it matches the source (size and mtime equal); otherwise it is overwritten and the source mtime is propagated. Hard errors with absolute paths and the declaring XML when:

- the origin name is not in the registry
- the origin has no `<local>`
- the source file is missing

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
- `type` — comma-separated flags: `static`, `html-template`, `response`, `nomux`, `parse`
- `key` — logical key for asset lookup
- `url` / `alt-url-abs` — URL routing attributes
- `escape` — override default escape type for this item (`html`, `js`, `url`, `sql`, etc.)
- `template-function-map` — Go expression returning `template.FuncMap`; injected via `.Funcs(expr)` before `.Parse()` for `parse` items (cascades from `<bucket>` → `<resource-list>` → `<item>`)

## Response items (`type="response"`)

A `response` item serves a **canned HTTP response** instead of file content: a
redirect (3xx + `Location`), a bare status (404, 410, …), or an error page with a
static body. The item points to a raw `.http` file — a status line, optional
headers, a blank line, and an optional body:

```
301 Moved Permanently
Location: https://www.example.com/products

```

```
404 Not Found
Content-Type: text/html; charset=utf-8
Cache-Control: no-store

<h1>Not found</h1>
```

Declared like any other item; the route comes from `key` (use it — without `key`
the route would be the `.http` filename):

```xml
<item type="response" file="old-page.http" key="/old-page/" />
```

Mechanics (mux template only):

- The `.http` is embedded exactly like a `static` asset — same `//go:embed`, same
  on-disk validation. No separate build step, no Go core code.
- Generated code calls `serveResponse(mux, route, embeddedBytes)`. The response is
  parsed **once, at registration**: first line → status code (the reason phrase is
  ignored, net/http derives it from the code); header lines until the blank line →
  response headers (any header, repeats preserved via `Add`); the rest → body.
- Bytes are fixed at build time (like `static`). For per-environment values, use a
  build-time percent tag with `src` (e.g. `Location: https://<%%domain%%>/x`).
  Anything dynamic/per-request belongs in your own `mux.HandleFunc`, not here.
- Headers you declare are sent verbatim; net/http still adds framing headers
  (`Date`, `Content-Length`) and sniffs `Content-Type` only when you omit it.

## Built-in templates

- `miniskin::default` — generic `Asset` type with `Get()`, `RegisterRoutes()`, `GetParsedTemplate()`
- `miniskin::mux` — `RegisterRoutes(mux *http.ServeMux, tmplHandlers)` with exact-path matching; `serveResponse` for `response` items

Custom templates via file path: `template="my_template.tmpl"`
