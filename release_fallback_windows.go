package main

import (
	"context"
	"fmt"
	"golang.org/x/net/html"
	"io"
	"net/url"
	"path"
	"strings"
)

const flowsealWeb = "https://github.com/Flowseal/zapret-discord-youtube"

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func walk(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}
func nodeText(n *html.Node) string {
	var b strings.Builder
	walk(n, func(v *html.Node) {
		if v.Type == html.TextNode {
			b.WriteString(v.Data)
		}
	})
	return strings.TrimSpace(b.String())
}
func parseReleasePage(r io.Reader) ([]Release, error) {
	doc, e := html.Parse(r)
	if e != nil {
		return nil, e
	}
	out := []Release{}
	seen := map[string]bool{}
	walk(doc, func(n *html.Node) {
		if n.Data != "section" || !strings.HasPrefix(attr(n, "data-release-anchor"), "release-") {
			return
		}
		rel := Release{}
		walk(n, func(v *html.Node) {
			if v.Data == "a" && rel.Tag == "" {
				href := attr(v, "href")
				prefix := "/Flowseal/zapret-discord-youtube/releases/tag/"
				if strings.HasPrefix(href, prefix) {
					rel.Tag, _ = url.PathUnescape(strings.TrimPrefix(href, prefix))
				}
			}
			if v.Data == "span" && strings.Contains(attr(v, "class"), "Label") && strings.EqualFold(nodeText(v), "Pre-release") {
				rel.Prerelease = true
			}
		})
		if rel.Tag != "" && !seen[rel.Tag] {
			seen[rel.Tag] = true
			out = append(out, rel)
		}
	})
	if len(out) == 0 {
		return nil, fmt.Errorf("на публичной странице GitHub не найдены релизы")
	}
	return out, nil
}
func parseReleaseAssets(r io.Reader, tag string) ([]ReleaseAsset, error) {
	doc, e := html.Parse(r)
	if e != nil {
		return nil, e
	}
	out := []ReleaseAsset{}
	seen := map[string]bool{}
	prefix := "/Flowseal/zapret-discord-youtube/releases/download/" + url.PathEscape(tag) + "/"
	walk(doc, func(n *html.Node) {
		if n.Data != "a" {
			return
		}
		href := attr(n, "href")
		if strings.HasPrefix(href, prefix) && strings.HasSuffix(strings.ToLower(href), ".zip") && !seen[href] {
			seen[href] = true
			out = append(out, ReleaseAsset{Name: path.Base(href), URL: "https://github.com" + href})
		}
	})
	if len(out) == 0 {
		return nil, fmt.Errorf("ZIP-архив релиза %s не найден на GitHub", tag)
	}
	return out, nil
}
func webReleases(ctx context.Context) ([]Release, error) {
	r, e := githubGet(ctx, flowsealWeb+"/releases")
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	return parseReleasePage(io.LimitReader(r.Body, 8<<20))
}
func webReleaseAssets(ctx context.Context, tag string) ([]ReleaseAsset, error) {
	r, e := githubGet(ctx, flowsealWeb+"/releases/expanded_assets/"+url.PathEscape(tag))
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	return parseReleaseAssets(io.LimitReader(r.Body, 4<<20), tag)
}
