## Go API

```go
// One-liner: full pipeline + code generation
miniskin.MiniskinRun(contentPath, modulesPath)
miniskin.MiniskinRun(contentPath, modulesPath, miniskin.VerbosityVerbose)

// Step by step
ms := miniskin.MiniskinNew(contentPath, modulesPath)
ms.SetVerbosity(miniskin.VerbosityVerbose)

// Dependency analysis
dm, _ := ms.AnalyzeDeps()
order, _ := dm.ProcessingOrder()
if dm.HasCycles() { /* error */ }

// Full pipeline
result, _ := ms.Run()

// Code generation
cg := miniskin.CodegenNew(contentPath, modulesPath)
cg.GenerateAll(result)

// Mockup update only (export + refresh imports)
miniskin.MiniskinMockupUpdate(contentPath, modulesPath)

// Generate only (build + codegen, no mockups)
miniskin.MiniskinGenerate(contentPath, modulesPath)

// Single-file negative transform
neg := miniskin.TransformNegative(content)
```

## CLI

```
miniskin run                                       # full pipeline
miniskin run -v                                    # verbose
miniskin generate                                  # build + codegen only
miniskin generate-claude-skill                     # generate Claude Code SKILL.md
miniskin mockup update                             # export + refresh imports
miniskin mockup negative -src m.html -dst n.html   # single file negative
miniskin deps                                      # show dependency map
```
