package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"gui-zapret/internal/flowseal"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Release struct {
	Tag        string         `json:"tag_name"`
	Prerelease bool           `json:"prerelease"`
	Draft      bool           `json:"draft"`
	Assets     []ReleaseAsset `json:"assets"`
}
type ReleaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

var githubClient = &http.Client{Timeout: 5 * time.Minute}

func githubGet(ctx context.Context, url string) (*http.Response, error) {
	req, e := http.NewRequestWithContext(ctx, "GET", url, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "gui-zapret")
	r, e := githubClient.Do(req)
	if e != nil {
		return nil, e
	}
	if r.StatusCode != 200 {
		r.Body.Close()
		return nil, fmt.Errorf("GitHub: HTTP %d", r.StatusCode)
	}
	return r, nil
}
func (a *App) Releases() ([]Release, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	r, e := githubGet(ctx, "https://api.github.com/repos/Flowseal/zapret-discord-youtube/releases?per_page=50")
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	var releases []Release
	if e = json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&releases); e != nil {
		return nil, e
	}
	out := []Release{}
	for _, rel := range releases {
		if !rel.Draft {
			for _, asset := range rel.Assets {
				if strings.HasSuffix(strings.ToLower(asset.Name), ".zip") {
					out = append(out, rel)
					break
				}
			}
		}
	}
	return out, nil
}
func (a *App) DownloadRelease(tag string) (string, error) {
	// Resolve the asset from GitHub again rather than trusting a URL from the frontend.
	releases, e := a.Releases()
	if e != nil {
		return "", e
	}
	var asset ReleaseAsset
	for _, r := range releases {
		if r.Tag == tag {
			for _, v := range r.Assets {
				if strings.HasSuffix(strings.ToLower(v.Name), ".zip") && strings.HasPrefix(v.URL, "https://github.com/Flowseal/zapret-discord-youtube/releases/download/") {
					asset = v
					break
				}
			}
		}
	}
	if asset.URL == "" {
		return "", fmt.Errorf("ZIP релиза не найден")
	}
	if asset.Size > 200<<20 {
		return "", fmt.Errorf("слишком большой архив")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	base := filepath.Join(filepath.Dir(a.config), "releases")
	if e = os.MkdirAll(base, 0700); e != nil {
		return "", e
	}
	stage, e := os.MkdirTemp(base, "download-")
	if e != nil {
		return "", e
	}
	defer os.RemoveAll(stage)
	r, e := githubGet(a.ctx, asset.URL)
	if e != nil {
		return "", e
	}
	archive := filepath.Join(stage, "release.zip")
	f, e := os.Create(archive)
	if e != nil {
		r.Body.Close()
		return "", e
	}
	n, e := io.Copy(f, io.LimitReader(r.Body, (200<<20)+1))
	r.Body.Close()
	ce := f.Close()
	if e != nil {
		return "", e
	}
	if ce != nil {
		return "", ce
	}
	if n > 200<<20 {
		return "", fmt.Errorf("слишком большой архив")
	}
	unpack := filepath.Join(stage, "unpacked")
	if e = extractRelease(archive, unpack); e != nil {
		return "", e
	}
	root := unpack
	if flowseal.ValidateRoot(root) != nil {
		entries, _ := os.ReadDir(unpack)
		found := false
		for _, entry := range entries {
			candidate := filepath.Join(unpack, entry.Name())
			if entry.IsDir() && flowseal.ValidateRoot(candidate) == nil {
				root = candidate
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("архив не содержит готовую сборку Flowseal")
		}
	}
	// Keep user lists and flags when switching distributions.
	for _, rel := range []string{"utils/game_filter.enabled", "utils/check_updates.enabled", "lists/list-general-user.txt", "lists/list-exclude-user.txt", "lists/ipset-exclude-user.txt"} {
		b, err := os.ReadFile(filepath.Join(a.root, rel))
		if err == nil {
			if e = os.WriteFile(filepath.Join(root, rel), b, 0600); e != nil {
				return "", e
			}
		} else if !os.IsNotExist(err) {
			return "", err
		} else if rel == "utils/check_updates.enabled" {
			if err = os.Remove(filepath.Join(root, rel)); err != nil && !os.IsNotExist(err) {
				return "", err
			}
		}
	}
	safe := regexp.MustCompile(`[^a-zA-Z0-9._-]`).ReplaceAllString(tag, "_")
	target, e := os.MkdirTemp(base, safe+"-")
	if e != nil {
		return "", e
	}
	if e = os.Remove(target); e != nil {
		return "", e
	}
	if e = os.Rename(root, target); e != nil {
		return "", e
	}
	b, _ := json.Marshal(struct{ Root string }{target})
	if e = os.WriteFile(a.config, b, 0600); e != nil {
		return "", e
	}
	a.root = target
	a.record("Загружен релиз " + tag + ". Примените стратегию для переключения службы.")
	return target, nil
}
func extractRelease(archive, dest string) error {
	z, e := zip.OpenReader(archive)
	if e != nil {
		return e
	}
	defer z.Close()
	var total uint64
	for _, f := range z.File {
		total += f.UncompressedSize64
		if total > 600<<20 {
			return fmt.Errorf("распакованный архив слишком большой")
		}
		name := strings.ReplaceAll(f.Name, `\`, "/")
		clean := filepath.Clean(filepath.FromSlash(name))
		if strings.HasPrefix(name, "/") || clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || strings.Contains(clean, ":") || f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("недопустимый путь архива: %s", f.Name)
		}
		target := filepath.Join(dest, clean)
		if f.FileInfo().IsDir() {
			if e = os.MkdirAll(target, 0700); e != nil {
				return e
			}
			continue
		}
		if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			return e
		}
		src, e := f.Open()
		if e != nil {
			return e
		}
		out, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			src.Close()
			return e
		}
		_, e = io.Copy(out, io.LimitReader(src, int64(f.UncompressedSize64)+1))
		src.Close()
		ce := out.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
	}
	return nil
}
