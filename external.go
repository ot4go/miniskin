package miniskin

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Origin describes a resolved external source declared in miniskin-origin.xml.
// MVP supports only Local (filesystem path); github-release and others come later.
type Origin struct {
	Name  string
	Local string
}

// originFilename is the fixed name of the per-developer origin registry,
// expected at the contentPath root and typically gitignored.
const originFilename = "miniskin-origin.xml"

// loadOrigins reads contentPath/miniskin-origin.xml if present and returns
// the origins keyed by name. Returns (nil, nil) when the file does not exist —
// projects without externals don't need an origin file.
func (ms *Miniskin) loadOrigins() (map[string]Origin, error) {
	path := filepath.Join(ms.contentPath, originFilename)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat %s: %w", absPath(path), err)
	}
	parsed, err := parseMiniskinXML(path)
	if err != nil {
		return nil, err
	}
	origins := make(map[string]Origin, len(parsed.Origins))
	for _, o := range parsed.Origins {
		if o.Name == "" {
			return nil, fmt.Errorf("origin with empty name in %s", absPath(path))
		}
		if o.Local == "" {
			return nil, fmt.Errorf("origin %q in %s has no <local> (only local sources supported)", o.Name, absPath(path))
		}
		origins[o.Name] = Origin{Name: o.Name, Local: o.Local}
	}
	return origins, nil
}

// processExternals walks all buckets, finds <external> blocks in any
// .miniskin.xml, resolves each <external-item> through the origins map,
// and copies the source file into place. Errors hard on missing origin
// or missing source file, with absolute paths and the declaring XML.
func (ms *Miniskin) processExternals(bl BucketList, origins map[string]Origin) error {
	count := 0
	for _, bucket := range bl.Buckets {
		err := ms.walkBucket(bucket, func(parsed *xmlMiniskin, dir string, xmlFile string) error {
			if parsed.External == nil {
				return nil
			}
			for _, ext := range parsed.External.Items {
				if ext.Origin == "" {
					return fmt.Errorf("external-item missing origin attribute in %s", xmlFile)
				}
				origin, ok := origins[ext.Origin]
				if !ok {
					return fmt.Errorf("external-item origin %q not found in %s\n\t(declared in %s)", ext.Origin, absPath(filepath.Join(ms.contentPath, originFilename)), xmlFile)
				}
				srcPath := filepath.Join(origin.Local, ext.Src)
				dstPath := filepath.Join(dir, ext.Dstfile)
				changed, err := copyIfChanged(srcPath, dstPath)
				if err != nil {
					return fmt.Errorf("external %s → %s: %w\n\t(declared in %s)", absPath(srcPath), absPath(dstPath), err, xmlFile)
				}
				if changed {
					ms.logf("  external: %s %s → %s", ext.Origin, ext.Src, ext.Dstfile)
				} else {
					ms.logVerbose("  external: %s %s → %s (unchanged)", ext.Origin, ext.Src, ext.Dstfile)
				}
				count++
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if count > 0 {
		ms.logVerbose("externals processed: %d", count)
	}
	return nil
}

// copyIfChanged copies src to dst if dst is missing or differs in size/mtime
// from src. Returns true if a copy occurred. Creates dst's parent directory.
func copyIfChanged(src, dst string) (bool, error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return false, fmt.Errorf("source not found: %w", err)
	}
	if srcInfo.IsDir() {
		return false, fmt.Errorf("source is a directory: %s", absPath(src))
	}
	if dstInfo, err := os.Stat(dst); err == nil {
		if dstInfo.Size() == srcInfo.Size() && dstInfo.ModTime().Equal(srcInfo.ModTime()) {
			return false, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return false, fmt.Errorf("mkdir %s: %w", absPath(filepath.Dir(dst)), err)
	}
	in, err := os.Open(src)
	if err != nil {
		return false, err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return false, err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return false, err
	}
	if err := out.Close(); err != nil {
		return false, err
	}
	if err := os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		return false, err
	}
	return true, nil
}
