# miniskin

**This is a build-time tool for assembling templates that will be embedded into a Go binary. It performs no input sanitization, no escaping of user-supplied data, and no protection against code injection. It is designed to run during development as part of `go generate`, not at runtime. Do not expose it to untrusted input.**


Build-time template assembler for Go projects. Processes content files (HTML, CSS, JS) with percent-tag substitution, includes, skins (layouts), and generates Go embed code.

## Install

```
go get github.com/ot4go/miniskin
```

## Concepts

### Percent tags

Two types of substitution tags, resolved at generation time:

- `<%var%>` — replaced with HTML-escaped value
- `<%%var%%>` — replaced with literal (unescaped) value
- `<%%include:/path/to/file%%>` — replaced with file contents (resolved recursively)

These tags coexist safely with Go `{{ }}` template syntax, which passes through untouched.

### Front-matter

Files with `src` can have YAML-like front-matter delimited by `---`:

```
---
skin: default
title: Sign In
css: /assets/signin.css
---
<div class="login">
  <h1>{{.AppName}}</h1>
</div>
```

Front-matter variables are available as percent-tag values. The `skin` key is special — it triggers skin application.

### Skins

A skin is an HTML layout file in `_skins/` that uses percent tags:

```html
<!DOCTYPE html>
<html>
<head><title><%title%></title></head>
<body>
<%%content%%>
</body>
</html>
```

`<%%content%%>` is replaced with the processed body of the source file. Other front-matter variables (like `<%title%>`) are available with HTML escaping.

### Conditionals

Control blocks using single percent-tag syntax:

```html
<%if:user%>
  <p>Welcome, <%user%></p>
<%elseif:guest%>
  <p>Guest access</p>
<%else%>
  <p>Please sign in</p>
<%endif%>
```

- `<%if:var%>` — content is included if `var` is defined and non-empty
- `<%elseif:var%>` — checked only if all previous branches were false
- `<%else%>` — included if all previous branches were false
- `<%endif%>` — closes the block

Blocks can be nested. Undefined variables inside a false branch do not cause errors. Unmatched `<%endif%>`, `<%else%>`, or unclosed `<%if:...%>` blocks produce build-time errors.

### Includes

Fragment files included via `<%%include:/path%%>`. They:

- Are resolved relative to `contentPath`
- Can contain their own percent tags (resolved before insertion)
- Have no front-matter, no skin — raw fragments only
- Never written to disk — resolved in memory
- Cycle detection: if A includes B includes A, generation fails

### Items

Each item in a `miniskin.xml` resource list describes a content file:

```xml
<item type="html-template,nomux,parse" src="signin_src.html" file="signin.html" key="/login/signin" />
```

- `file` — output filename (what gets embedded)
- `src` — source filename (optional; if present, item is processed)
- `type` — comma-separated flags: `static`, `html-template`, `nomux`, `parse`, etc.
- `key` — logical key for asset lookup
- `url` / `alt-url-abs` — URL routing attributes

If `src` is absent, `file` is embedded as-is (no processing).

## XML format

All configuration uses `*.miniskin.xml` files with a `<miniskin>` root element.

### Root (in contentPath)

```xml
<miniskin>
  <globals>
    <var name="appName" value="MyApp" />
  </globals>

  <bucket-list filename="generated_embed.go" module="content" import="myproject/content">
    <bucket src="app" dst="/modules/app/reqctx/generated_assets.go"
            module-name="reqctx" recurse-folder="all" />
  </bucket-list>
</miniskin>
```

- `globals` — key-value pairs available to all templates
- `bucket-list` — defines the embed file and content buckets
  - `filename` — output Go file for embed declarations
  - `module` — Go package name for the embed file
  - `import` — full import path for the embed package
  - `template` — (optional) custom Go template for embed file generation
- `bucket` — maps a source directory to a generated Go file
  - `src` — source directory under contentPath
  - `dst` — destination Go file path (relative to modulesPath)
  - `module-name` — Go package name for the generated file
  - `recurse-folder="all"` — walk subdirectories for `*.miniskin.xml`
  - `template` — (optional) custom Go template for this bucket

### Subdirectory

```xml
<miniskin>
  <resource-list urlbase="/assets">
    <item type="static" file="app.css" />
    <item type="static" src="combined_src.css" file="combined.css" />
  </resource-list>
</miniskin>
```

## Generated Go code

### Embed file (generated_embed.go)

```go
package content

import _ "embed"

//go:embed app/assets/app.css
var AppAssetsAppCss []byte
```

One `[]byte` variable per item. Direct pointer to binary segment — no copy, no decompression.

### Bucket file (default template)

The default template generates:

```go
// Asset type with Key, Data, Mime, Type fields
// allAssets slice with all items
// parsedTemplates map for items with "parse" flag

func Assets() []Asset              // all assets
func Get(key string) *Asset        // by key
func GetParsedTemplate(key) *template.Template  // pre-parsed templates
func StaticFiles() []Asset         // items with "static" flag
func Templates() []Asset           // items with "html-template" flag
func RegisterRoutes(fn func(url, mime string, data []byte))  // static, non-nomux
```

Items with the `parse` flag are pre-parsed as `*template.Template` at init time.

## Custom templates

Both the embed file and bucket files support custom Go `text/template` templates.

### Embed template

Available functions: `embedPath`, `embedVar`. Data: full `Result` struct.

### Bucket template

Available functions: `embedVar`, `mimeType`, `hasFlag`, `embedPkg`, `embedImport`. Data: `BucketList`, `Bucket`, `Items`.

If you use a custom embed template, you must also provide custom bucket templates (since variable names may differ).

## Usage

```go
package main

import (
    "log"
    "github.com/ot4go/miniskin"
)

func main() {
    ms := miniskin.New("/path/to/content", "/path/to/modules")

    result, err := ms.Run()
    if err != nil {
        log.Fatalf("miniskin: %v", err)
    }

    if err := ms.GenerateAll(result); err != nil {
        log.Fatalf("miniskin generate: %v", err)
    }
}
```

## File structure example

```
content/
  content.miniskin.xml          # root: globals + bucket-list
  _skins/
    default.html                # skin layout
  app/
    _shared/
      header.html               # include fragment
    assets/
      assets.miniskin.xml       # resource-list for static files
      app.css
      app.js
    login_dialog/
      login.miniskin.xml        # resource-list with src items
      signin_src.html           # source with front-matter + skin
      signin.html               # generated output (gitignored)
```

## License

MIT
