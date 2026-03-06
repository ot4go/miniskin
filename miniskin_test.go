package miniskin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testdataPath() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "testdata")
}

func cleanup(items []Item) {
	for _, it := range items {
		if it.NeedsProcessing() {
			os.Remove(it.FilePath())
		}
	}
}

// ---

func TestParseRootMiniskinXML(t *testing.T) {
	ms := New(testdataPath(), testdataPath())
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	if result.BucketList.Filename != "generated_embed.go" {
		t.Errorf("expected filename generated_embed.go, got %s", result.BucketList.Filename)
	}
	if len(result.BucketList.Buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(result.BucketList.Buckets))
	}
	if result.BucketList.Buckets[0].Src != "app" {
		t.Errorf("expected bucket src app, got %s", result.BucketList.Buckets[0].Src)
	}
}

// ---

func TestCollectAllItems(t *testing.T) {
	ms := New(testdataPath(), testdataPath())
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	items := result.Buckets[0].Items
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
}

// ---

func TestSkinFromFrontMatter(t *testing.T) {
	ms := New(testdataPath(), testdataPath())
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	var signin *Item
	for i := range result.Buckets[0].Items {
		if result.Buckets[0].Items[i].File == "signin.html" {
			signin = &result.Buckets[0].Items[i]
			break
		}
	}
	if signin == nil {
		t.Fatal("signin.html not found")
	}

	data, err := os.ReadFile(signin.FilePath())
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "<!DOCTYPE html>") {
		t.Error("missing DOCTYPE from skin")
	}
	if !strings.Contains(content, "<title>TestApp - Sign In</title>") {
		t.Errorf("missing title with global + front-matter, got:\n%s", content)
	}
	if !strings.Contains(content, `<div class="login">`) {
		t.Error("missing body content")
	}
	if !strings.Contains(content, "{{.Username}}") {
		t.Error("Go template syntax not preserved")
	}
}

// ---

func TestPercentInclude(t *testing.T) {
	ms := New(testdataPath(), testdataPath())
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	var plain *Item
	for i := range result.Buckets[0].Items {
		if result.Buckets[0].Items[i].File == "plain.css" {
			plain = &result.Buckets[0].Items[i]
			break
		}
	}
	if plain == nil {
		t.Fatal("plain.css not found")
	}

	data, err := os.ReadFile(plain.FilePath())
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "* { margin: 0; padding: 0; }") {
		t.Error("missing included reset.css content")
	}
	if !strings.Contains(content, ".plain { color: red; }") {
		t.Error("missing original content")
	}
}

// ---

func TestStaticNotProcessed(t *testing.T) {
	ms := New(testdataPath(), testdataPath())
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	defer cleanup(result.Buckets[0].Items)

	var appCss *Item
	for i := range result.Buckets[0].Items {
		if result.Buckets[0].Items[i].File == "app.css" {
			appCss = &result.Buckets[0].Items[i]
			break
		}
	}
	if appCss == nil {
		t.Fatal("app.css not found")
	}
	if appCss.NeedsProcessing() {
		t.Error("app.css should not need processing")
	}
}

// ---

func TestCycleDetection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "cycle.html"), []byte(`<%%include:/cycle.html%%>`), 0644)

	ms := New(dir, dir)
	_, err := ms.resolvePercent(`<%%include:/cycle.html%%>`, nil, nil)
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected cycle error, got: %v", err)
	}
}

// ---

func TestNestedIncludes(t *testing.T) {
	dir := t.TempDir()

	// a.html includes b.html, b.html includes c.html
	os.WriteFile(filepath.Join(dir, "a.html"), []byte(`[A-start]<%%include:/b.html%%>[A-end]`), 0644)
	os.WriteFile(filepath.Join(dir, "b.html"), []byte(`[B-start]<%%include:/c.html%%>[B-end]`), 0644)
	os.WriteFile(filepath.Join(dir, "c.html"), []byte(`[C]`), 0644)

	ms := New(dir, dir)
	result, err := ms.resolvePercent(`<%%include:/a.html%%>`, nil, nil)
	if err != nil {
		t.Fatalf("nested includes failed: %v", err)
	}
	expected := "[A-start][B-start][C][B-end][A-end]"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// ---

func TestIndirectCycle(t *testing.T) {
	dir := t.TempDir()

	// a includes b, b includes c, c includes a -> cycle
	os.WriteFile(filepath.Join(dir, "a.html"), []byte(`<%%include:/b.html%%>`), 0644)
	os.WriteFile(filepath.Join(dir, "b.html"), []byte(`<%%include:/c.html%%>`), 0644)
	os.WriteFile(filepath.Join(dir, "c.html"), []byte(`<%%include:/a.html%%>`), 0644)

	ms := New(dir, dir)
	_, err := ms.resolvePercent(`<%%include:/a.html%%>`, nil, nil)
	if err == nil {
		t.Fatal("expected cycle detection error for indirect cycle")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected cycle error, got: %v", err)
	}
}

// ---

func TestUnclosedSingleTag(t *testing.T) {
	ms := New(t.TempDir(), t.TempDir())
	_, err := ms.resolvePercent(`before <%oops after`, nil, nil)
	if err == nil {
		t.Fatal("expected error for unclosed single tag")
	}
}

// ---

func TestUnclosedDoubleTag(t *testing.T) {
	ms := New(t.TempDir(), t.TempDir())
	_, err := ms.resolvePercent(`before <%%oops after`, nil, nil)
	if err == nil {
		t.Fatal("expected error for unclosed double tag")
	}
}

// ---

func TestUndefinedVariable(t *testing.T) {
	ms := New(t.TempDir(), t.TempDir())
	_, err := ms.resolvePercent(`<%noexist%>`, nil, nil)
	if err == nil {
		t.Fatal("expected error for undefined variable")
	}
}

// ---

func TestIncludeFileNotFound(t *testing.T) {
	dir := t.TempDir()
	ms := New(dir, dir)
	_, err := ms.resolvePercent(`<%%include:/nonexistent.html%%>`, nil, nil)
	if err == nil {
		t.Fatal("expected error for missing include file")
	}
}

// ---

func TestIfTrue(t *testing.T) {
	ms := New(t.TempDir(), t.TempDir())
	vars := map[string]string{"x": "yes"}
	result, err := ms.resolvePercent(`before<%if:x%>SHOW<%endif%>after`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "beforeSHOWafter" {
		t.Errorf("expected %q, got %q", "beforeSHOWafter", result)
	}
}

// ---

func TestIfFalse(t *testing.T) {
	ms := New(t.TempDir(), t.TempDir())
	result, err := ms.resolvePercent(`before<%if:x%>HIDE<%endif%>after`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "beforeafter" {
		t.Errorf("expected %q, got %q", "beforeafter", result)
	}
}

// ---

func TestIfEmptyIsFalse(t *testing.T) {
	ms := New(t.TempDir(), t.TempDir())
	vars := map[string]string{"x": ""}
	result, err := ms.resolvePercent(`<%if:x%>HIDE<%endif%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

// ---

func TestIfElse(t *testing.T) {
	ms := New(t.TempDir(), t.TempDir())
	result, err := ms.resolvePercent(`<%if:x%>YES<%else%>NO<%endif%>`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "NO" {
		t.Errorf("expected %q, got %q", "NO", result)
	}
}

// ---

func TestIfElseTrue(t *testing.T) {
	ms := New(t.TempDir(), t.TempDir())
	vars := map[string]string{"x": "1"}
	result, err := ms.resolvePercent(`<%if:x%>YES<%else%>NO<%endif%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "YES" {
		t.Errorf("expected %q, got %q", "YES", result)
	}
}

// ---

func TestElseif(t *testing.T) {
	ms := New(t.TempDir(), t.TempDir())
	vars := map[string]string{"b": "1"}
	result, err := ms.resolvePercent(`<%if:a%>A<%elseif:b%>B<%else%>C<%endif%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "B" {
		t.Errorf("expected %q, got %q", "B", result)
	}
}

// ---

func TestElseifFirstTrue(t *testing.T) {
	ms := New(t.TempDir(), t.TempDir())
	vars := map[string]string{"a": "1", "b": "1"}
	result, err := ms.resolvePercent(`<%if:a%>A<%elseif:b%>B<%else%>C<%endif%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "A" {
		t.Errorf("expected %q, got %q", "A", result)
	}
}

// ---

func TestElseifFallthrough(t *testing.T) {
	ms := New(t.TempDir(), t.TempDir())
	result, err := ms.resolvePercent(`<%if:a%>A<%elseif:b%>B<%else%>C<%endif%>`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "C" {
		t.Errorf("expected %q, got %q", "C", result)
	}
}

// ---

func TestNestedIf(t *testing.T) {
	ms := New(t.TempDir(), t.TempDir())
	vars := map[string]string{"a": "1"}
	result, err := ms.resolvePercent(`<%if:a%><%if:b%>INNER<%else%>ALT<%endif%><%endif%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ALT" {
		t.Errorf("expected %q, got %q", "ALT", result)
	}
}

// ---

func TestNestedIfParentFalse(t *testing.T) {
	ms := New(t.TempDir(), t.TempDir())
	vars := map[string]string{"b": "1"}
	result, err := ms.resolvePercent(`<%if:a%><%if:b%>INNER<%endif%><%else%>OUT<%endif%>`, vars, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "OUT" {
		t.Errorf("expected %q, got %q", "OUT", result)
	}
}

// ---

func TestUndefinedVarInSkippedBlock(t *testing.T) {
	ms := New(t.TempDir(), t.TempDir())
	// <%undef%> inside a false block should NOT error
	result, err := ms.resolvePercent(`<%if:x%><%undef%><%endif%>ok`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected %q, got %q", "ok", result)
	}
}

// ---

func TestUnclosedIf(t *testing.T) {
	ms := New(t.TempDir(), t.TempDir())
	vars := map[string]string{"x": "1"}
	_, err := ms.resolvePercent(`<%if:x%>stuff`, vars, nil)
	if err == nil {
		t.Fatal("expected error for unclosed if block")
	}
}

// ---

func TestEndifWithoutIf(t *testing.T) {
	ms := New(t.TempDir(), t.TempDir())
	_, err := ms.resolvePercent(`<%endif%>`, nil, nil)
	if err == nil {
		t.Fatal("expected error for endif without if")
	}
}

// ---

func TestElseWithoutIf(t *testing.T) {
	ms := New(t.TempDir(), t.TempDir())
	_, err := ms.resolvePercent(`<%else%>`, nil, nil)
	if err == nil {
		t.Fatal("expected error for else without if")
	}
}

// ---

func TestSkinDirDefault(t *testing.T) {
	dir := t.TempDir()

	// _skin/basic.html (default dir)
	os.MkdirAll(filepath.Join(dir, "_skin"), 0755)
	os.WriteFile(filepath.Join(dir, "_skin", "basic.html"), []byte(`[DEFAULT]<%%content%%>[/DEFAULT]`), 0644)

	// bucket "pages" with a src item that uses skin: basic
	os.MkdirAll(filepath.Join(dir, "pages"), 0755)
	os.WriteFile(filepath.Join(dir, "pages", "pages.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/pages">
		<item type="html-template" src="page_src.html" file="page.html" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "pages", "page_src.html"), []byte("---\nskin: basic\n---\nHello"), 0644)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="pages" dst="/gen.go" module-name="pages" />
	</bucket-list>
</miniskin>`), 0644)

	ms := New(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "pages", "page.html"))
	if string(data) != "[DEFAULT]Hello[/DEFAULT]" {
		t.Errorf("expected default _skin, got %q", string(data))
	}
	cleanup(result.Buckets[0].Items)
}

// ---

func TestSkinDirOnMiniskin(t *testing.T) {
	dir := t.TempDir()

	// custom skin dir at root level
	os.MkdirAll(filepath.Join(dir, "layouts"), 0755)
	os.WriteFile(filepath.Join(dir, "layouts", "main.html"), []byte(`[LAYOUT]<%%content%%>[/LAYOUT]`), 0644)

	os.MkdirAll(filepath.Join(dir, "pages"), 0755)
	os.WriteFile(filepath.Join(dir, "pages", "pages.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/pages">
		<item type="html-template" src="p_src.html" file="p.html" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "pages", "p_src.html"), []byte("---\nskin: main\n---\nBody"), 0644)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin skin-dir="layouts">
	<bucket-list filename="embed.go" module="content">
		<bucket src="pages" dst="/gen.go" module-name="pages" />
	</bucket-list>
</miniskin>`), 0644)

	ms := New(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "pages", "p.html"))
	if string(data) != "[LAYOUT]Body[/LAYOUT]" {
		t.Errorf("expected layouts/main.html skin, got %q", string(data))
	}
	cleanup(result.Buckets[0].Items)
}

// ---

func TestSkinDirOnBucketOverridesRoot(t *testing.T) {
	dir := t.TempDir()

	// root-level skin dir (should be overridden)
	os.MkdirAll(filepath.Join(dir, "layouts"), 0755)
	os.WriteFile(filepath.Join(dir, "layouts", "wrap.html"), []byte(`[ROOT]<%%content%%>[/ROOT]`), 0644)

	// bucket-level skin dir
	os.MkdirAll(filepath.Join(dir, "app", "myskins"), 0755)
	os.WriteFile(filepath.Join(dir, "app", "myskins", "wrap.html"), []byte(`[BUCKET]<%%content%%>[/BUCKET]`), 0644)

	os.MkdirAll(filepath.Join(dir, "app"), 0755)
	os.WriteFile(filepath.Join(dir, "app", "app.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/app">
		<item type="html-template" src="v_src.html" file="v.html" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "app", "v_src.html"), []byte("---\nskin: wrap\n---\nContent"), 0644)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin skin-dir="layouts">
	<bucket-list filename="embed.go" module="content">
		<bucket src="app" dst="/gen.go" module-name="app" skin-dir="app/myskins" />
	</bucket-list>
</miniskin>`), 0644)

	ms := New(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "app", "v.html"))
	if string(data) != "[BUCKET]Content[/BUCKET]" {
		t.Errorf("expected bucket skin-dir override, got %q", string(data))
	}
	cleanup(result.Buckets[0].Items)
}

// ---

func TestSkinDirOnResourceListOverridesBucket(t *testing.T) {
	dir := t.TempDir()

	// bucket-level skin dir (should be overridden by resource-list)
	os.MkdirAll(filepath.Join(dir, "bskins"), 0755)
	os.WriteFile(filepath.Join(dir, "bskins", "base.html"), []byte(`[BUCKET]<%%content%%>[/BUCKET]`), 0644)

	// resource-list level skin dir
	os.MkdirAll(filepath.Join(dir, "rskins"), 0755)
	os.WriteFile(filepath.Join(dir, "rskins", "base.html"), []byte(`[RESLIST]<%%content%%>[/RESLIST]`), 0644)

	os.MkdirAll(filepath.Join(dir, "mod"), 0755)
	os.WriteFile(filepath.Join(dir, "mod", "mod.miniskin.xml"), []byte(`<miniskin>
	<resource-list urlbase="/mod" skin-dir="rskins">
		<item type="html-template" src="x_src.html" file="x.html" />
	</resource-list>
</miniskin>`), 0644)
	os.WriteFile(filepath.Join(dir, "mod", "x_src.html"), []byte("---\nskin: base\n---\nHi"), 0644)

	os.WriteFile(filepath.Join(dir, "root.miniskin.xml"), []byte(`<miniskin>
	<bucket-list filename="embed.go" module="content">
		<bucket src="mod" dst="/gen.go" module-name="mod" skin-dir="bskins" />
	</bucket-list>
</miniskin>`), 0644)

	ms := New(dir, dir)
	result, err := ms.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "mod", "x.html"))
	if string(data) != "[RESLIST]Hi[/RESLIST]" {
		t.Errorf("expected resource-list skin-dir override, got %q", string(data))
	}
	cleanup(result.Buckets[0].Items)
}

// ---

func TestDiamondInclude(t *testing.T) {
	dir := t.TempDir()

	// a includes b and c, both include d -> no cycle, d appears twice
	os.WriteFile(filepath.Join(dir, "b.html"), []byte(`[B]<%%include:/d.html%%>`), 0644)
	os.WriteFile(filepath.Join(dir, "c.html"), []byte(`[C]<%%include:/d.html%%>`), 0644)
	os.WriteFile(filepath.Join(dir, "d.html"), []byte(`[D]`), 0644)

	ms := New(dir, dir)
	result, err := ms.resolvePercent(`<%%include:/b.html%%>|<%%include:/c.html%%>`, nil, nil)
	if err != nil {
		t.Fatalf("diamond include failed: %v", err)
	}
	expected := "[B][D]|[C][D]"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
