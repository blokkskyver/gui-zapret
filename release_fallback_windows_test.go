package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestRelease403FallbackAndCache(t *testing.T) {
	old := githubClient
	defer func() { githubClient = old }()
	calls := 0
	page := `<section data-release-anchor="release-2.0"><a href="/Flowseal/zapret-discord-youtube/releases/tag/2.0">2.0</a><span class="Label">Pre-release</span></section><section data-release-anchor="release-1.0"><a href="/Flowseal/zapret-discord-youtube/releases/tag/1.0">1.0</a><span class="Label">Latest</span></section>`
	githubClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		code, body := 200, page
		if r.URL.Host == "api.github.com" {
			code, body = 403, `{"message":"rate limit exceeded"}`
		}
		return &http.Response{StatusCode: code, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	app := newApp()
	app.ctx = context.Background()
	list, e := app.Releases()
	if e != nil {
		t.Fatal(e)
	}
	if len(list) != 2 || !list[0].Prerelease || list[1].Prerelease {
		t.Fatal(list)
	}
	if _, e = app.Releases(); e != nil {
		t.Fatal(e)
	}
	if calls != 2 {
		t.Fatalf("cache missed: %d calls", calls)
	}
	assets, e := parseReleaseAssets(strings.NewReader(`<a href="/Flowseal/zapret-discord-youtube/releases/download/1.0/zapret.zip">zip</a><a href="https://evil.example/archive.zip">bad</a><a href="/Flowseal/zapret-discord-youtube/releases/download/9.0/other.zip">wrong tag</a>`), "1.0")
	if e != nil || len(assets) != 1 || assets[0].URL != "https://github.com/Flowseal/zapret-discord-youtube/releases/download/1.0/zapret.zip" {
		t.Fatal(assets, e)
	}
}
func TestLiveReleasePageFixture(t *testing.T) {
	if os.Getenv("FLOWSEAL_PAGE_FIXTURE") == "" {
		t.Skip("optional captured GitHub page")
	}
	page, e := os.Open(".cache/fixtures/releases.html")
	if e != nil {
		t.Fatal(e)
	}
	defer page.Close()
	list, e := parseReleasePage(page)
	if e != nil || len(list) == 0 {
		t.Fatal(e)
	}
	assetPage, e := os.Open(".cache/fixtures/assets.html")
	if e != nil {
		t.Fatal(e)
	}
	defer assetPage.Close()
	assets, e := parseReleaseAssets(assetPage, list[0].Tag)
	if e != nil || len(assets) == 0 {
		t.Fatal(e)
	}
	t.Log("parsed public releases and ZIP links", len(list))
}
