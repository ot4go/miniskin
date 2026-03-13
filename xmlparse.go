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
	Log          string           `xml:"log,attr"`
	MuxInclude   string           `xml:"mux-include,attr"`
	MuxExclude   string           `xml:"mux-exclude,attr"`
	Globals      []xmlVar         `xml:"globals>var"`
	Escapes      []xmlEscape      `xml:"escape"`
	BucketList   *xmlBucketList   `xml:"bucket-list"`
	ResourceList *xmlResourceList `xml:"resource-list"`
	MockupList   *xmlMockupList   `xml:"mockup-list"`
}

type xmlMockupList struct {
	SkinDir  string          `xml:"skin-dir,attr"`
	SaveMode string          `xml:"save-mode,attr"`
	Vars     []xmlVar        `xml:"var"`
	Items    []xmlMockupItem `xml:"item"`
}

type xmlMockupItem struct {
	Src      string   `xml:"src,attr"`
	Negative string   `xml:"negative,attr"`
	SaveMode string   `xml:"save-mode,attr"`
	Vars     []xmlVar `xml:"var"`
}

type xmlVar struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type xmlEscape struct {
	Ext string `xml:"ext,attr"`
	As  string `xml:"as,attr"`
}

type xmlBucketList struct {
	Filename   string      `xml:"filename,attr"`
	Module     string      `xml:"module,attr"`
	Import     string      `xml:"import,attr"`
	Template   string      `xml:"template,attr"`
	MuxInclude string      `xml:"mux-include,attr"`
	MuxExclude string      `xml:"mux-exclude,attr"`
	Escapes    []xmlEscape `xml:"escape"`
	Buckets    []xmlBucket `xml:"bucket"`
}

type xmlBucket struct {
	Src           string      `xml:"src,attr"`
	Dst           string      `xml:"dst,attr"`
	ModuleName    string      `xml:"module-name,attr"`
	RecurseFolder string      `xml:"recurse-folder,attr"`
	Template      string      `xml:"template,attr"`
	SkinDir       string      `xml:"skin-dir,attr"`
	MuxInclude    string      `xml:"mux-include,attr"`
	MuxExclude    string      `xml:"mux-exclude,attr"`
	Escapes       []xmlEscape `xml:"escape"`
}

type xmlResourceList struct {
	URLBase    string      `xml:"urlbase,attr"`
	SkinDir    string      `xml:"skin-dir,attr"`
	MuxInclude string      `xml:"mux-include,attr"`
	MuxExclude string      `xml:"mux-exclude,attr"`
	Escapes    []xmlEscape `xml:"escape"`
	Items      []xmlItem   `xml:"item"`
}

type xmlItem struct {
	Type   string `xml:"type,attr"`
	File   string `xml:"file,attr"`
	Src    string `xml:"src,attr"`
	URL    string `xml:"url,attr"`
	AltURL string `xml:"alt-url-abs,attr"`
	Key    string `xml:"key,attr"`
	Escape string `xml:"escape,attr"`
}

// --- Parsed types

// BucketList holds the embed generation config from the root miniskin.xml.
type BucketList struct {
	Filename string
	Module   string
	Import   string
	Template string // custom template file for embed generation
	Buckets  []Bucket
}

// Bucket represents a content bucket.
type Bucket struct {
	Src        string
	Dst        string
	ModuleName string
	Template   string // custom template file for bucket generation
	recurse     bool
	skinDir     string
	muxInclude  string      // resolved cascaded mux-include pattern (default: "*")
	muxExclude  string      // resolved cascaded mux-exclude pattern (default: "")
	escapeRules []xmlEscape // cascaded escape rules
}

// Item represents a resource item from a miniskin.xml.
type Item struct {
	Type      string
	File      string
	Src       string // source file; if set, item needs processing
	URL       string
	AltURL    string
	Key       string
	Index     int    // position in the global embed list
	EmbedPath string // relative path for go:embed, computed during processing
	urlBase     string
	skinDir     string
	dir         string
	escapeRules []xmlEscape // cascaded escape rules
	escape      string      // item-level escape override
}

// NeedsProcessing returns true if the item has a src attribute.
func (it *Item) NeedsProcessing() bool {
	return it.Src != ""
}

func (it *Item) filePath() string {
	return filepath.Join(it.dir, it.File)
}

func (it *Item) srcPath() string {
	return filepath.Join(it.dir, it.Src)
}

// RouteURL returns the URL for serving this item.
// Uses Key if set, otherwise URLBase + "/" + File.
func (it *Item) RouteURL() string {
	if it.Key != "" {
		return it.Key
	}
	if it.urlBase != "" {
		return it.urlBase + "/" + it.File
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

// --- Mux cascade helpers

// cascadeMux returns child if non-empty, otherwise parent.
func cascadeMux(parent, child string) string {
	if child != "" {
		return child
	}
	return parent
}

// matchesMuxPattern checks if filename matches a comma-separated list of glob patterns.
func matchesMuxPattern(filename, patterns string) bool {
	if patterns == "" {
		return false
	}
	if patterns == "*" {
		return true
	}
	for _, p := range strings.Split(patterns, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if matched, _ := filepath.Match(p, filename); matched {
			return true
		}
	}
	return false
}

// isExcludedByMux returns true if the file should get the nomux flag
// based on resolved mux-include and mux-exclude patterns.
func isExcludedByMux(filename, muxInclude, muxExclude string) bool {
	if !matchesMuxPattern(filename, muxInclude) {
		return true
	}
	if muxExclude != "" && matchesMuxPattern(filename, muxExclude) {
		return true
	}
	return false
}

// hasTypeFlag checks if a comma-separated type string contains a specific flag.
func hasTypeFlag(typ, flag string) bool {
	for _, f := range strings.Split(typ, ",") {
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

func parseBucketList(xbl *xmlBucketList, defaultSkinDir string, parentMuxInclude, parentMuxExclude string, parentEscapeRules []xmlEscape) BucketList {
	// Cascade mux: parent (miniskin) → bucket-list
	blInclude := cascadeMux(parentMuxInclude, xbl.MuxInclude)
	blExclude := cascadeMux(parentMuxExclude, xbl.MuxExclude)
	blEscapes := cascadeEscapeRules(parentEscapeRules, xbl.Escapes)

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
			Src:         b.Src,
			Dst:         b.Dst,
			ModuleName:  b.ModuleName,
			Template:    b.Template,
			recurse:     b.RecurseFolder == "all",
			skinDir:     skinDir,
			muxInclude:  cascadeMux(blInclude, b.MuxInclude),
			muxExclude:  cascadeMux(blExclude, b.MuxExclude),
			escapeRules: cascadeEscapeRules(blEscapes, b.Escapes),
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

func parseResourceList(xrl *xmlResourceList, dir string, defaultSkinDir string, parentMuxInclude, parentMuxExclude string, parentEscapeRules []xmlEscape) []Item {
	skinDir := xrl.SkinDir
	if skinDir == "" {
		skinDir = defaultSkinDir
	}
	// Cascade mux: parent (bucket) → resource-list
	rlInclude := cascadeMux(parentMuxInclude, xrl.MuxInclude)
	rlExclude := cascadeMux(parentMuxExclude, xrl.MuxExclude)
	rlEscapes := cascadeEscapeRules(parentEscapeRules, xrl.Escapes)

	items := make([]Item, len(xrl.Items))
	for i, xi := range xrl.Items {
		itemType := xi.Type
		// Apply nomux automatically if item is excluded by mux patterns
		if !hasTypeFlag(itemType, "nomux") && isExcludedByMux(xi.File, rlInclude, rlExclude) {
			if itemType == "" {
				itemType = "nomux"
			} else {
				itemType += ",nomux"
			}
		}
		items[i] = Item{
			Type:        itemType,
			File:        xi.File,
			Src:         xi.Src,
			URL:         xi.URL,
			AltURL:      xi.AltURL,
			Key:         xi.Key,
			urlBase:     xrl.URLBase,
			skinDir:     skinDir,
			dir:         dir,
			escapeRules: rlEscapes,
			escape:      xi.Escape,
		}
	}
	return items
}

// walkBucket walks a bucket's source directory, finds all *.miniskin.xml files,
// parses them, and calls fn for each parsed result.
func (ms *Miniskin) walkBucket(bucket Bucket, fn func(parsed *xmlMiniskin, dir string) error) error {
	srcDir := filepath.Join(ms.contentPath, bucket.Src)

	visitFile := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".miniskin.xml") {
			parsed, err := parseMiniskinXML(path)
			if err != nil {
				return err
			}
			return fn(parsed, filepath.Dir(path))
		}
		return nil
	}

	if bucket.recurse {
		return filepath.Walk(srcDir, visitFile)
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("reading %s: %w", srcDir, err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".miniskin.xml") {
			info, err := e.Info()
			if err != nil {
				return err
			}
			path := filepath.Join(srcDir, e.Name())
			if err := visitFile(path, info, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// collectItems walks the bucket's source directory and returns all resource-list items.
// Mux-include/mux-exclude patterns from the bucket are cascaded to each resource-list.
func (ms *Miniskin) collectItems(bucket Bucket) ([]Item, error) {
	var result []Item
	err := ms.walkBucket(bucket, func(parsed *xmlMiniskin, dir string) error {
		if parsed.ResourceList != nil {
			items := parseResourceList(parsed.ResourceList, dir, bucket.skinDir, bucket.muxInclude, bucket.muxExclude, bucket.escapeRules)
			result = append(result, items...)
		}
		return nil
	})
	return result, err
}
