# miniskin (CLI)

[![Go Reference](https://pkg.go.dev/badge/github.com/ot4go/miniskin/cmd/miniskin.svg)](https://pkg.go.dev/github.com/ot4go/miniskin/cmd/miniskin)

Command-line interface for the [miniskin](https://pkg.go.dev/github.com/ot4go/miniskin) build-time template assembler.

miniskin started as a library helper for `//go:generate`. The CLI was added later for interactive operations that don't really fit a `go:generate` step — inspect dependencies, transform mockups into negative templates, combine and split XML catalogs, regenerate AI agent docs — and as a no-code shortcut for the `go:generate` integration itself.

## Install

```
go install github.com/ot4go/miniskin/cmd/miniskin@latest
```

## Commands

| Command | Description |
|---|---|
| `run` | Mockup update + build embed assets + generate Go code (full pipeline) |
| `generate` | Build embed assets + generate Go code (no mockup pass) |
| `generate-claude-skill` | Generate `.claude/skills/miniskin/SKILL.md` |
| `generate-agent-docs` | Generate `AGENTS.md` (Cursor, Aider, Windsurf, …) |
| `mockup update` | Export mockup pieces and refresh `mockup-import` blocks |
| `mockup clean` | Empty the inline content of `mockup-import` blocks |
| `mockup negative` | Transform a mockup file into a negative template |
| `deps` | Print the dependency map and processing order |
| `combine <dir>` | Combine subdirectory `*.miniskin.xml` files into one |
| `split <file>` | Split nested resource-lists into separate XML files |

## Flags

Common (any command):

| Flag | Default | Description |
|---|---|---|
| `-content` | `.` | Path to the content directory |
| `-modules` | `.` | Path to the modules directory |
| `-v` | | Verbose output (dependency analysis, processing order) |
| `-vv` | | Debug output (all internal details) |
| `-silent` | | Suppress all output |

`mockup negative`:

| Flag | Required | Description |
|---|---|---|
| `-src` | yes | Source mockup file |
| `-dst` | yes | Destination negative template file |

`generate-claude-skill`:

| Flag | Default | Description |
|---|---|---|
| `-dst` | `.claude/skills/miniskin/SKILL.md` | Destination path |
| `-force` | | Overwrite an existing destination file |

`generate-agent-docs`:

| Flag | Default | Description |
|---|---|---|
| `-dst` | `AGENTS.md` | Destination path |
| `-force` | | Overwrite an existing destination file |

## Interactive use

Day-to-day commands run from the terminal. The examples below assume you run miniskin from the project root and your assets live in a `content/` subdirectory:

```
miniskin run -content content
miniskin run -content content -v
miniskin deps -content content
miniskin mockup negative -src mockup_login.html -dst login_negative.html
miniskin generate-agent-docs -dst .cursor/rules/miniskin.mdc -force
miniskin combine content/app
miniskin split content/app/app.miniskin.xml
```

## Use with `go generate`

Two options, both produce the same output. Pick whichever fits your project:

### Option A — direct CLI invocation (simpler)

No driver file. Drop the directive in any Go source of your project and you're done:

```go
//go:generate miniskin run -content content
package main
```

Trade-off: every collaborator needs the binary installed (`go install github.com/ot4go/miniskin/cmd/miniskin@latest`).

### Option B — library driver (customizable, no install)

Write a tiny `main` in your repo that calls the library, and reference it from `go:generate`. The library becomes a regular module dependency pinned in `go.mod`, so nobody needs to `go install` anything.

```go
// gen.go
package main

import (
	"fmt"
	"os"

	"github.com/ot4go/miniskin"
)

func main() {
	if err := miniskin.MiniskinRun("content", "."); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
```

```go
//go:generate go run ./gen.go
package main
```

Use this option when you need to customize the pipeline (extra steps before/after, wrapping `MiniskinNew` directly, environment-driven configuration, etc.).

In both cases, `go generate ./...` then rebuilds the embedded assets as part of the standard Go workflow.

## See also

- [Library documentation](https://pkg.go.dev/github.com/ot4go/miniskin) — the `miniskin` package API (`MiniskinRun`, `MiniskinNew`, `DepMap`, …)
- [Project README](https://github.com/ot4go/miniskin#readme) — concepts, XML format, percent-tag syntax, mockup-driven development

## License

MIT
