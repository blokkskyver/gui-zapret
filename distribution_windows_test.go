package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func makeDistribution(t *testing.T, root string) {
	t.Helper()
	for _, d := range []string{"bin", "lists", "utils"} {
		if e := os.MkdirAll(filepath.Join(root, d), 0700); e != nil {
			t.Fatal(e)
		}
	}
	for _, f := range []string{"service.bat", "bin/winws.exe"} {
		if e := os.WriteFile(filepath.Join(root, f), []byte("fixture"), 0600); e != nil {
			t.Fatal(e)
		}
	}
}
func TestDiscoverPortableDistribution(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "downloaded-app")
	docs := filepath.Join(dir, "Documents", "zapret")
	saved := filepath.Join(dir, "saved")
	os.MkdirAll(exe, 0700)
	if got := discoverDistributionRoot("missing", exe, docs); got != docs {
		t.Fatal(got)
	}
	makeDistribution(t, docs)
	if got := discoverDistributionRoot("missing", exe, docs); got != docs {
		t.Fatal(got)
	}
	adjacent := filepath.Join(exe, "zapret")
	makeDistribution(t, adjacent)
	if got := discoverDistributionRoot("missing", exe, docs); got != adjacent {
		t.Fatal(got)
	}
	makeDistribution(t, saved)
	if got := discoverDistributionRoot(saved, exe, docs); got != saved {
		t.Fatal(got)
	}
}
func TestEmbeddedFrontendWithoutSourceDirectory(t *testing.T) {
	cwd, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chdir(t.TempDir()); e != nil {
		t.Fatal(e)
	}
	defer os.Chdir(cwd)
	index, e := assets.ReadFile("frontend/index.html")
	if e != nil {
		t.Fatal(e)
	}
	refs := regexp.MustCompile(`(?:src|href)="([^"]+)"`).FindAllStringSubmatch(string(index), -1)
	if len(refs) < 6 {
		t.Fatal("expected bundled scripts, style and icon")
	}
	for _, ref := range refs {
		if strings.Contains(ref[1], "://") {
			continue
		}
		b, e := assets.ReadFile("frontend/" + ref[1])
		if e != nil || len(b) == 0 {
			t.Fatalf("missing embedded asset %s: %v", ref[1], e)
		}
	}
}
