package main

import (
	"golang.org/x/sys/windows"
	"gui-zapret/internal/flowseal"
	"os"
	"path/filepath"
	"sort"
)

func defaultDistributionRoot() string {
	documents, e := windows.KnownFolderPath(windows.FOLDERID_Documents, 0)
	if e != nil {
		home, _ := os.UserHomeDir()
		documents = filepath.Join(home, "Documents")
	}
	return filepath.Join(documents, "zapret")
}
func discoverDistributionRoot(saved, exeDir, defaultRoot string) string {
	for _, root := range []string{saved, exeDir, filepath.Join(exeDir, "zapret"), filepath.Join(exeDir, "Flowseal"), defaultRoot} {
		if root != "" && flowseal.ValidateRoot(root) == nil {
			return root
		}
	}
	entries, _ := os.ReadDir(filepath.Join(defaultRoot, "releases"))
	sort.Slice(entries, func(i, j int) bool {
		a, ea := entries[i].Info()
		b, eb := entries[j].Info()
		if ea != nil || eb != nil {
			return entries[i].Name() > entries[j].Name()
		}
		return a.ModTime().After(b.ModTime())
	})
	for _, entry := range entries {
		root := filepath.Join(defaultRoot, "releases", entry.Name())
		if entry.IsDir() && flowseal.ValidateRoot(root) == nil {
			return root
		}
	}
	return defaultRoot
}
