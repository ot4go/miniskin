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
