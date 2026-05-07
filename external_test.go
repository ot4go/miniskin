package miniskin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// scaffoldExternalProject builds a self-contained miniskin project with an
// external block and (optionally) a miniskin-origin.xml. Returns the
// contentPath. The origin <local> points at originDir (which the caller
// has already populated with srcRel files). When originXML is empty no
// miniskin-origin.xml is written — useful to test the "missing origin" path.
func scaffoldExternalProject(t *testing.T, originXML, externalXML string) string {
	t.Helper()
	root := t.TempDir()

	// Bucket dir + bucket XML referencing it
	bucketDir := filepath.Join(root, "app")
	if err := os.MkdirAll(bucketDir, 0755); err != nil {
		t.Fatalf("mkdir bucket: %v", err)
	}
	rootXML := `<?xml version="1.0"?>
<miniskin>
  <bucket-list filename="generated_embed.go" module="content">
    <bucket src="/app" dst="/generated_embed_list.go" module-name="app" recurse-folder="all" />
  </bucket-list>
</miniskin>`
	if err := os.WriteFile(filepath.Join(root, "content.miniskin.xml"), []byte(rootXML), 0644); err != nil {
		t.Fatalf("write root xml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bucketDir, "app.miniskin.xml"), []byte(externalXML), 0644); err != nil {
		t.Fatalf("write app xml: %v", err)
	}
	if originXML != "" {
		if err := os.WriteFile(filepath.Join(root, "miniskin-origin.xml"), []byte(originXML), 0644); err != nil {
			t.Fatalf("write origin xml: %v", err)
		}
	}
	return root
}

func TestExternalLocalCopy(t *testing.T) {
	root := t.TempDir()

	// External source dir (the "origin")
	srcDir := filepath.Join(root, "vendor-src")
	if err := os.MkdirAll(filepath.Join(srcDir, "release"), 0755); err != nil {
		t.Fatal(err)
	}
	srcContent := []byte("// vendor payload\n")
	srcFile := filepath.Join(srcDir, "release", "lib.js")
	if err := os.WriteFile(srcFile, srcContent, 0644); err != nil {
		t.Fatal(err)
	}

	originXML := `<?xml version="1.0"?>
<miniskin>
  <origin name="vendor"><local>` + srcDir + `</local></origin>
</miniskin>`
	externalXML := `<?xml version="1.0"?>
<miniskin>
  <external>
    <external-item origin="vendor" src="./release/lib.js" dstfile="./src/lib.js" />
  </external>
</miniskin>`

	// Reuse the scaffold but override its TempDir so srcDir lives alongside.
	// Easier: build the project structure inline.
	contentDir := filepath.Join(root, "content")
	bucketDir := filepath.Join(contentDir, "app")
	if err := os.MkdirAll(bucketDir, 0755); err != nil {
		t.Fatal(err)
	}
	rootXML := `<?xml version="1.0"?>
<miniskin>
  <bucket-list filename="generated_embed.go" module="content">
    <bucket src="/app" dst="/generated_embed_list.go" module-name="app" recurse-folder="all" />
  </bucket-list>
</miniskin>`
	mustWrite(t, filepath.Join(contentDir, "content.miniskin.xml"), rootXML)
	mustWrite(t, filepath.Join(bucketDir, "app.miniskin.xml"), externalXML)
	mustWrite(t, filepath.Join(contentDir, "miniskin-origin.xml"), originXML)

	ms := newSilent(contentDir, contentDir)
	_, bl, err := ms.init()
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	origins, err := ms.loadOrigins()
	if err != nil {
		t.Fatalf("loadOrigins: %v", err)
	}
	if err := ms.processExternals(bl, origins); err != nil {
		t.Fatalf("processExternals: %v", err)
	}

	dstFile := filepath.Join(bucketDir, "src", "lib.js")
	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != string(srcContent) {
		t.Errorf("dst content mismatch: got %q want %q", got, srcContent)
	}

	// Second run should be a no-op (mtime + size match).
	srcInfoBefore, _ := os.Stat(srcFile)
	dstInfoBefore, _ := os.Stat(dstFile)
	if !dstInfoBefore.ModTime().Equal(srcInfoBefore.ModTime()) {
		t.Errorf("expected mtime to be propagated: src=%v dst=%v", srcInfoBefore.ModTime(), dstInfoBefore.ModTime())
	}
	if err := ms.processExternals(bl, origins); err != nil {
		t.Fatalf("second processExternals: %v", err)
	}
	dstInfoAfter, _ := os.Stat(dstFile)
	if !dstInfoAfter.ModTime().Equal(dstInfoBefore.ModTime()) {
		t.Errorf("dst was rewritten on idempotent run: before=%v after=%v", dstInfoBefore.ModTime(), dstInfoAfter.ModTime())
	}

	// Touching the source forces a refresh.
	newer := srcInfoBefore.ModTime().Add(2 * time.Second)
	if err := os.Chtimes(srcFile, newer, newer); err != nil {
		t.Fatal(err)
	}
	if err := ms.processExternals(bl, origins); err != nil {
		t.Fatalf("third processExternals: %v", err)
	}
	dstInfoFinal, _ := os.Stat(dstFile)
	if !dstInfoFinal.ModTime().Equal(newer) {
		t.Errorf("dst mtime did not refresh: got %v want %v", dstInfoFinal.ModTime(), newer)
	}
}

func TestExternalMissingOriginFile(t *testing.T) {
	externalXML := `<?xml version="1.0"?>
<miniskin>
  <external>
    <external-item origin="ghost" src="./x.js" dstfile="./y.js" />
  </external>
</miniskin>`
	root := scaffoldExternalProject(t, "", externalXML)

	ms := newSilent(root, root)
	_, bl, err := ms.init()
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	origins, err := ms.loadOrigins()
	if err != nil {
		t.Fatalf("loadOrigins: %v", err)
	}
	err = ms.processExternals(bl, origins)
	if err == nil {
		t.Fatal("expected error for missing origin, got nil")
	}
	if !strings.Contains(err.Error(), `origin "ghost" not found`) {
		t.Errorf("error did not mention missing origin name: %v", err)
	}
}

func TestExternalMissingOriginInRegistry(t *testing.T) {
	originXML := `<?xml version="1.0"?>
<miniskin>
  <origin name="other"><local>C:\nope</local></origin>
</miniskin>`
	externalXML := `<?xml version="1.0"?>
<miniskin>
  <external>
    <external-item origin="ghost" src="./x.js" dstfile="./y.js" />
  </external>
</miniskin>`
	root := scaffoldExternalProject(t, originXML, externalXML)

	ms := newSilent(root, root)
	_, bl, _ := ms.init()
	origins, err := ms.loadOrigins()
	if err != nil {
		t.Fatalf("loadOrigins: %v", err)
	}
	err = ms.processExternals(bl, origins)
	if err == nil || !strings.Contains(err.Error(), `"ghost"`) {
		t.Errorf("expected error about missing 'ghost' origin, got: %v", err)
	}
}

func TestExternalOriginWithoutLocalRejected(t *testing.T) {
	originXML := `<?xml version="1.0"?>
<miniskin>
  <origin name="vendor"></origin>
</miniskin>`
	root := scaffoldExternalProject(t, originXML, `<?xml version="1.0"?><miniskin></miniskin>`)

	ms := newSilent(root, root)
	_, err := ms.loadOrigins()
	if err == nil || !strings.Contains(err.Error(), "only local sources supported") {
		t.Errorf("expected rejection of origin without <local>, got: %v", err)
	}
}

func TestExternalSourceMissing(t *testing.T) {
	root := t.TempDir()
	bucketDir := filepath.Join(root, "app")
	if err := os.MkdirAll(bucketDir, 0755); err != nil {
		t.Fatal(err)
	}
	originDir := filepath.Join(root, "vendor")
	if err := os.MkdirAll(originDir, 0755); err != nil {
		t.Fatal(err)
	}

	mustWrite(t, filepath.Join(root, "content.miniskin.xml"), `<?xml version="1.0"?>
<miniskin>
  <bucket-list filename="generated_embed.go" module="content">
    <bucket src="/app" dst="/generated_embed_list.go" module-name="app" recurse-folder="all" />
  </bucket-list>
</miniskin>`)
	mustWrite(t, filepath.Join(bucketDir, "app.miniskin.xml"), `<?xml version="1.0"?>
<miniskin>
  <external>
    <external-item origin="vendor" src="./missing.js" dstfile="./out.js" />
  </external>
</miniskin>`)
	mustWrite(t, filepath.Join(root, "miniskin-origin.xml"), `<?xml version="1.0"?>
<miniskin>
  <origin name="vendor"><local>`+originDir+`</local></origin>
</miniskin>`)

	ms := newSilent(root, root)
	_, bl, _ := ms.init()
	origins, _ := ms.loadOrigins()
	err := ms.processExternals(bl, origins)
	if err == nil || !strings.Contains(err.Error(), "source not found") {
		t.Errorf("expected source-not-found error, got: %v", err)
	}
	// Error should include the absolute resolved source path.
	wantSrc := filepath.Join(originDir, "missing.js")
	if !strings.Contains(err.Error(), wantSrc) {
		t.Errorf("error missing resolved src path %q: %v", wantSrc, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
