package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListsEditor(t *testing.T) {
	a := newApp()
	a.root = t.TempDir()
	dir := filepath.Join(a.root, "lists")
	if e := os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	name := "list-general-user.txt"
	original := "# comment\r\nexample.com\r\n"
	path := filepath.Join(dir, name)
	os.WriteFile(path, []byte(original), 0600)
	os.WriteFile(filepath.Join(dir, "ignored.txt.bak"), []byte("backup"), 0600)
	lists, e := a.Lists()
	if e != nil || len(lists) != 1 || lists[0].Description == "" {
		t.Fatalf("catalog: %v %v", lists, e)
	}
	if _, e = a.ReadList("../outside.txt"); e == nil {
		t.Fatal("path escape accepted")
	}
	doc, e := a.ReadList(name)
	if e != nil || doc.Content != original {
		t.Fatal("read mismatch", e)
	}
	if e = a.SaveList("old-root", name, original, "example.org\n"); e == nil {
		t.Fatal("root change accepted")
	}
	if e = a.SaveList(a.root, name, "stale", "example.org\n"); e == nil {
		t.Fatal("concurrent edit accepted")
	}
	if e = a.SaveList(a.root, name, original, "\x00"); e == nil {
		t.Fatal("binary content accepted")
	}
	updated := "# comment\nexample.org\n"
	if e = a.SaveList(doc.Root, name, original, updated); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(path)
	if string(b) != updated {
		t.Fatal("save mismatch")
	}
	backups, _ := filepath.Glob(path + ".*.bak")
	if len(backups) != 1 {
		t.Fatal("backup missing")
	}
	b, _ = os.ReadFile(backups[0])
	if string(b) != original {
		t.Fatal("backup mismatch")
	}
}
