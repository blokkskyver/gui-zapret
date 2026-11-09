package main

import (
	"archive/zip"
	"context"
	"gui-zapret/internal/flowseal"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiveRelease(t *testing.T) {
	if os.Getenv("FLOWSEAL_RELEASE_TEST") == "" {
		t.Skip("optional GitHub download check")
	}
	app := newApp()
	app.ctx = context.Background()
	releases, e := app.Releases()
	if e != nil {
		t.Fatal(e)
	}
	if len(releases) == 0 {
		t.Fatal("no ZIP releases")
	}
	var asset ReleaseAsset
	for _, a := range releases[0].Assets {
		if strings.HasSuffix(a.Name, ".zip") {
			asset = a
			break
		}
	}
	r, e := githubGet(app.ctx, asset.URL)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Body.Close()
	dir := t.TempDir()
	archive := filepath.Join(dir, "release.zip")
	f, e := os.Create(archive)
	if e != nil {
		t.Fatal(e)
	}
	_, e = io.Copy(f, io.LimitReader(r.Body, 200<<20))
	f.Close()
	if e != nil {
		t.Fatal(e)
	}
	dest := filepath.Join(dir, "unpacked")
	if e = extractRelease(archive, dest); e != nil {
		t.Fatal(e)
	}
	if flowseal.ValidateRoot(dest) != nil {
		entries, _ := os.ReadDir(dest)
		found := false
		for _, v := range entries {
			if flowseal.ValidateRoot(filepath.Join(dest, v.Name())) == nil {
				found = true
			}
		}
		if !found {
			t.Fatal("not a distribution")
		}
	}
	t.Log("Downloaded and validated release", releases[0].Tag)
}

func TestExtractRelease(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", `C:\escape`, `..\escape`, "lists/test.txt"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.zip")
			f, e := os.Create(path)
			if e != nil {
				t.Fatal(e)
			}
			z := zip.NewWriter(f)
			w, e := z.Create(name)
			if e != nil {
				t.Fatal(e)
			}
			w.Write([]byte("data"))
			z.Close()
			f.Close()
			e = extractRelease(path, filepath.Join(dir, "out"))
			if name == "lists/test.txt" {
				if e != nil {
					t.Fatal(e)
				}
				b, e := os.ReadFile(filepath.Join(dir, "out", "lists", "test.txt"))
				if e != nil || string(b) != "data" {
					t.Fatal("bad extraction")
				}
			} else if e == nil {
				t.Fatal("accepted unsafe archive")
			}
		})
	}
}
func TestSelectedTestScript(t *testing.T) {
	src := "$rootDir = Split-Path $PSScriptRoot\r\n$batFiles = Get-ChildItem -Path $targetDir\r\n$mode = Read-ModeSelection\r\n$testType = Read-TestType"
	out, e := selectedTestScript(src, `C:\O'Brien\zapret`, "general (ALT).bat")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out, "O''Brien") || strings.Contains(out, "$batFiles = Get-ChildItem") || !strings.Contains(out, "$mode = 'all'") || !strings.Contains(out, "$testType = Read-TestType") {
		t.Fatal(out)
	}
	if _, e = selectedTestScript("unsupported", "root", "name"); e == nil {
		t.Fatal("unsupported script accepted")
	}
}
func TestStrategyEditor(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "utils"), 0700)
	name := "general.bat"
	original := `start "zapret" /min "%BIN%winws.exe" --wf-tcp=443`
	os.WriteFile(filepath.Join(root, name), []byte(original), 0600)
	app := newApp()
	app.root = root
	if e := app.SaveStrategy(name, "stale", original); e == nil {
		t.Fatal("concurrent edit accepted")
	}
	if e := app.SaveStrategy(name, original, "invalid"); e == nil {
		t.Fatal("invalid strategy accepted")
	}
	updated := original + " --new"
	if e := app.SaveStrategy(name, original, updated); e != nil {
		t.Fatal(e)
	}
	backups, _ := filepath.Glob(filepath.Join(root, "*.bak"))
	if len(backups) != 1 {
		t.Fatal("missing backup")
	}
	b, _ := os.ReadFile(backups[0])
	if string(b) != original {
		t.Fatal("backup differs")
	}
}
