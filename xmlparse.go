package miniskin

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// --- XML structures

type xmlMiniskin struct {
	XMLName      xml.Name         `xml:"miniskin"`
	SkinDir      string           `xml:"skin-dir,attr"`
	Globals      []xmlVar         `xml:"globals>var"`
	BucketList   *xmlBucketList   `xml:"bucket-list"`
	ResourceList *xmlResourceList `xml:"resource-list"`
}

type xmlVar struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type xmlBucketList struct {
	Filename string      `xml:"filename,attr"`
	Module   string      `xml:"module,attr"`
	Import   string      `xml:"import,attr"`
	Template string      `xml:"template,attr"`
	Buckets  []xmlBucket `xml:"bucket"`
}

type xmlBucket struct {
	Src           string `xml:"src,attr"`
	Dst           string `xml:"dst,attr"`
	ModuleName    string `xml:"module-name,attr"`
	RecurseFolder string `xml:"recurse-folder,attr"`
	Template      string `xml:"template,attr"`
	SkinDir       string `xml:"skin-dir,attr"`
}

type xmlResourceList struct {
	URLBase  string    `xml:"urlbase,attr"`
	SkinDir  string    `xml:"skin-dir,attr"`
	Items    []xmlItem `xml:"item"`
}

type xmlItem struct {
	Type   string `xml:"type,attr"`
	File   string `xml:"file,attr"`
	Src    string `xml:"src,attr"`
	URL    string `xml:"url,attr"`
	AltURL string `xml:"alt-url-abs,attr"`
	Key    string `xml:"key,attr"`
}

// --- Parsed types

// BucketList holds the embed generation config from the root miniskin.xml.
type BucketList struct {
	Filename string
	Module   string
	Import   string
	Template string
	Buckets  []Bucket
}

// Bucket represents a content bucket.
type Bucket struct {
	Src        string
	Dst        string
	ModuleName string
	Recurse    bool
	Template   string
	SkinDir    string
}

// Item represents a resource item from a miniskin.xml.
type Item struct {
	Type    string
	File    string
	Src     string // source file; if set, item needs processing
	URL      string
	AltURL   string
	Key      string
	URLBase  string
	SkinDir  string
	Dir      string
	Index   int // position in the global embed list
}

// NeedsProcessing returns true if the item has a src attribute.
func (it *Item) NeedsProcessing() bool {
	return it.Src != ""
}

// FilePath returns the file path for embedding (always the output file).
func (it *Item) FilePath() string {
	return filepath.Join(it.Dir, it.File)
}

// SrcPath returns the source file path for processing.
func (it *Item) SrcPath() string {
	return filepath.Join(it.Dir, it.Src)
}

// RouteURL returns the URL for serving this item.
// Uses Key if set, otherwise URLBase + "/" + File.
func (it *Item) RouteURL() string {
	if it.Key != "" {
		return it.Key
	}
	if it.URLBase != "" {
		return it.URLBase + "/" + it.File
	}
	return "/" + it.File
}

// HasFlag returns true if the item's Type contains the given flag.
func (it *Item) HasFlag(flag string) bool {
	for _, f := range strings.Split(it.Type, ",") {
		if strings.TrimSpace(f) == flag {
			return true
		}
	}
	return false
}

// --- Parsing

func parseMiniskinXML(path string) (*xmlMiniskin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var ms xmlMiniskin
	if err := xml.Unmarshal(data, &ms); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &ms, nil
}

func parseBucketList(xbl *xmlBucketList, defaultSkinDir string) BucketList {
	bl := BucketList{
		Filename: xbl.Filename,
		Module:   xbl.Module,
		Import:   xbl.Import,
		Template: xbl.Template,
		Buckets:  make([]Bucket, len(xbl.Buckets)),
	}
	for i, b := range xbl.Buckets {
		skinDir := b.SkinDir
		if skinDir == "" {
			skinDir = defaultSkinDir
		}
		bl.Buckets[i] = Bucket{
			Src:        b.Src,
			Dst:        b.Dst,
			ModuleName: b.ModuleName,
			Recurse:    b.RecurseFolder == "all",
			Template:   b.Template,
			SkinDir:   skinDir,
		}
	}
	return bl
}

func parseGlobals(vars []xmlVar) map[string]string {
	m := make(map[string]string, len(vars))
	for _, v := range vars {
		m[v.Name] = v.Value
	}
	return m
}

func parseResourceList(xrl *xmlResourceList, dir string, defaultSkinDir string) []Item {
	skinDir := xrl.SkinDir
	if skinDir == "" {
		skinDir = defaultSkinDir
	}
	items := make([]Item, len(xrl.Items))
	for i, xi := range xrl.Items {
		items[i] = Item{
			Type:     xi.Type,
			File:     xi.File,
			Src:      xi.Src,
			URL:      xi.URL,
			AltURL:   xi.AltURL,
			Key:      xi.Key,
			URLBase:  xrl.URLBase,
			SkinDir: skinDir,
			Dir:      dir,
		}
	}
	return items
}

// collectItems walks the bucket's source directory, finds all *.miniskin.xml
// files, parses them, and returns all items.
func (ms *Miniskin) collectItems(bucket Bucket) ([]Item, error) {
	srcDir := filepath.Join(ms.contentPath, bucket.Src)
	var result []Item

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".miniskin.xml") {
			parsed, err := parseMiniskinXML(path)
			if err != nil {
				return err
			}
			if parsed.ResourceList != nil {
				items := parseResourceList(parsed.ResourceList, filepath.Dir(path), bucket.SkinDir)
				result = append(result, items...)
			}
		}
		return nil
	}

	if bucket.Recurse {
		if err := filepath.Walk(srcDir, walkFn); err != nil {
			return nil, err
		}
	} else {
		entries, err := os.ReadDir(srcDir)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", srcDir, err)
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".miniskin.xml") {
				info, err := e.Info()
				if err != nil {
					return nil, err
				}
				path := filepath.Join(srcDir, e.Name())
				if err := walkFn(path, info, nil); err != nil {
					return nil, err
				}
			}
		}
	}

	return result, nil
}
