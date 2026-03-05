package miniskin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// GenerateEmbed writes the generated_embed.go file with //go:embed directives.
// If bucket-list has a template attribute, uses that custom template.
// Otherwise uses the built-in default template.
func (ms *Miniskin) GenerateEmbed(result *Result) error {
	outPath := filepath.Join(ms.contentPath, result.BucketList.Filename)

	var tmplSrc string
	if result.BucketList.Template != "" {
		data, err := os.ReadFile(filepath.Join(ms.contentPath, result.BucketList.Template))
		if err != nil {
			return fmt.Errorf("reading embed template %s: %w", result.BucketList.Template, err)
		}
		tmplSrc = string(data)
	} else {
		tmplSrc = defaultEmbedTmpl
	}

	funcMap := template.FuncMap{
		"embedPath": func(item Item) string {
			return ms.relativeEmbedPath(item)
		},
		"embedVar": func(item Item) string {
			return embedVarName(ms.relativeEmbedPath(item))
		},
	}

	tmpl, err := template.New("embed").Funcs(funcMap).Parse(tmplSrc)
	if err != nil {
		return fmt.Errorf("parsing embed template: %w", err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", outPath, err)
	}
	defer f.Close()

	return tmpl.Execute(f, result)
}

// GenerateBucketFile writes the generated Go file for a bucket.
// If the bucket has a template attribute, uses that file.
// Otherwise uses the built-in default template.
func (ms *Miniskin) GenerateBucketFile(result *Result, br BucketResult) error {
	var tmplSrc string
	if br.Bucket.Template != "" {
		data, err := os.ReadFile(filepath.Join(ms.contentPath, br.Bucket.Template))
		if err != nil {
			return fmt.Errorf("reading template %s: %w", br.Bucket.Template, err)
		}
		tmplSrc = string(data)
	} else {
		tmplSrc = defaultBucketTmpl
	}

	funcMap := template.FuncMap{
		"embedVar": func(item Item) string {
			return embedVarName(ms.relativeEmbedPath(item))
		},
		"mimeType": func(item Item) string {
			return guessMime(item.File)
		},
		"hasFlag": func(item Item, flag string) bool {
			return item.HasFlag(flag)
		},
		"embedPkg": func() string {
			return result.BucketList.Module
		},
		"embedImport": func() string {
			return result.BucketList.Import
		},
	}

	tmpl, err := template.New("bucket").Funcs(funcMap).Parse(tmplSrc)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	data := struct {
		BucketList BucketList
		Bucket     Bucket
		Items      []Item
	}{
		BucketList: result.BucketList,
		Bucket:     br.Bucket,
		Items:      br.Items,
	}

	dstPath := filepath.Join(ms.modulesPath, filepath.FromSlash(br.Bucket.Dst))
	f, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dstPath, err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("executing template for %s: %w", dstPath, err)
	}

	return nil
}

// GenerateAll generates the embed file and all bucket files.
func (ms *Miniskin) GenerateAll(result *Result) error {
	if err := ms.GenerateEmbed(result); err != nil {
		return err
	}
	for _, br := range result.Buckets {
		if err := ms.GenerateBucketFile(result, br); err != nil {
			return err
		}
	}
	return nil
}

// --- helpers

func (ms *Miniskin) relativeEmbedPath(item Item) string {
	full := filepath.Join(item.Dir, item.File)
	rel, _ := filepath.Rel(ms.contentPath, full)
	return filepath.ToSlash(rel)
}

func embedVarName(relPath string) string {
	dir := filepath.ToSlash(filepath.Dir(relPath))
	base := filepath.Base(relPath)

	var parts []string
	for _, p := range strings.Split(dir, "/") {
		if p != "" && p != "." {
			parts = append(parts, sanitizePart(p))
		}
	}

	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	parts = append(parts, sanitizePart(name))
	parts = append(parts, sanitizeExt(ext))

	return strings.Join(parts, "")
}

func sanitizePart(s string) string {
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	if len(s) > 0 {
		s = strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}

func sanitizeExt(ext string) string {
	ext = strings.TrimPrefix(ext, ".")
	ext = strings.ReplaceAll(ext, "-", "_")
	if len(ext) > 0 {
		ext = strings.ToUpper(ext[:1]) + ext[1:]
	}
	return ext
}

func guessMime(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".ttf":
		return "application/octet-stream"
	case ".ico":
		return "image/x-icon"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}
