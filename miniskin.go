package miniskin

import (
	"fmt"
	"os"
	"path/filepath"
)

// Miniskin is a build-time template assembler.
// It resolves percent tags and applies skins to produce
// final files ready for embedding.
type Miniskin struct {
	contentPath string
	modulesPath string
	globals     map[string]string
}

// New creates a Miniskin instance.
// contentPath: root directory where source files live.
// modulesPath: root directory where Go modules live.
func New(contentPath, modulesPath string) *Miniskin {
	return &Miniskin{
		contentPath: contentPath,
		modulesPath: modulesPath,
	}
}

// --- Result types

// BucketResult holds a parsed bucket and its collected items.
type BucketResult struct {
	Bucket Bucket
	Items  []Item
}

// Result holds everything miniskin parsed and processed.
type Result struct {
	BucketList BucketList
	Buckets    []BucketResult
}

// --- Run

// Run parses the root miniskin.xml, walks all buckets, processes items
// that have src, and returns the parsed structure.
func (ms *Miniskin) Run() (*Result, error) {
	rootFiles, err := filepath.Glob(filepath.Join(ms.contentPath, "*.miniskin.xml"))
	if err != nil {
		return nil, err
	}
	if len(rootFiles) == 0 {
		return nil, fmt.Errorf("no *.miniskin.xml found in %s", ms.contentPath)
	}
	if len(rootFiles) > 1 {
		return nil, fmt.Errorf("multiple *.miniskin.xml in root: %v", rootFiles)
	}

	root, err := parseMiniskinXML(rootFiles[0])
	if err != nil {
		return nil, err
	}
	if root.BucketList == nil {
		return nil, fmt.Errorf("root %s has no bucket-list", rootFiles[0])
	}

	ms.globals = parseGlobals(root.Globals)

	bl := parseBucketList(root.BucketList)
	result := &Result{BucketList: bl}

	idx := 0
	for _, bucket := range bl.Buckets {
		items, err := ms.collectItems(bucket)
		if err != nil {
			return nil, err
		}

		for i := range items {
			items[i].Index = idx
			idx++
			if items[i].NeedsProcessing() {
				if err := ms.processItem(&items[i]); err != nil {
					return nil, fmt.Errorf("processing %s: %w", items[i].File, err)
				}
			}
		}

		result.Buckets = append(result.Buckets, BucketResult{
			Bucket: bucket,
			Items:  items,
		})
	}

	return result, nil
}

// processItem reads from src, parses front-matter, resolves percent tags,
// applies skin if declared, and writes the result to file.
func (ms *Miniskin) processItem(item *Item) error {
	srcPath := item.SrcPath()

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", srcPath, err)
	}

	content := string(data)

	// Parse front-matter: extracts variables and skin declaration
	fmVars, body, err := parseFrontMatter(content)
	if err != nil {
		return fmt.Errorf("front-matter in %s: %w", item.Src, err)
	}

	// Extract skin from front-matter if present
	skinName := ""
	if fmVars != nil {
		if s, ok := fmVars["skin"]; ok {
			skinName = s
			delete(fmVars, "skin")
		}
	}

	// Resolve percent tags in the body
	vars := ms.mergeVars(fmVars)
	chain := []string{srcPath}
	body, err = ms.resolvePercent(body, vars, chain)
	if err != nil {
		return err
	}

	// Apply skin if declared
	if skinName != "" {
		body, err = ms.applySkin(skinName, body, fmVars)
		if err != nil {
			return err
		}
	}

	// Write to the output file
	outPath := item.FilePath()
	if err := os.WriteFile(outPath, []byte(body), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}

	return nil
}

// mergeVars creates a variable map with globals as base, overridden by local vars.
func (ms *Miniskin) mergeVars(local map[string]string) map[string]string {
	vars := make(map[string]string, len(ms.globals)+len(local))
	for k, v := range ms.globals {
		vars[k] = v
	}
	for k, v := range local {
		vars[k] = v
	}
	return vars
}
