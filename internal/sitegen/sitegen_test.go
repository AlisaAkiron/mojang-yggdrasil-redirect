package sitegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestBuild(t *testing.T) {
	src, out := t.TempDir(), t.TempDir()
	writeFiles(t, src, map[string]string{
		"config.json": `{"title": "Site", "locales": [
			{"dir": "", "lang": "en", "label": "English"},
			{"dir": "zh-cn", "lang": "zh-CN", "label": "中文"}]}`,
		"layout.html":            `<html lang="{{.Lang}}"><title>{{.Title}}</title>{{range .Alternates}}<a href="{{.Href}}">{{.Label}}</a>{{end}}<home>{{.Home}}</home>{{.Content}}</html>`,
		"content/index.md":       "---\ntitle: Hello\n---\n\n## Intro {#intro}\n\n| a | b |\n| - | - |\n| 1 | 2 |\n",
		"content/guide.md":       "No frontmatter.\n",
		"content/zh-cn/index.md": "---\ntitle: \"你好\"\n---\n\n正文\n",
		"static/style.css":       "body{}",
	})
	writeFiles(t, out, map[string]string{".gitkeep": "", "stale.html": "old"})

	if err := Build(src, out); err != nil {
		t.Fatal(err)
	}

	en := readFile(t, filepath.Join(out, "index.html"))
	for _, want := range []string{
		`<html lang="en">`, `<title>Hello</title>`, `<a href="/zh-cn/">中文</a>`, `<home>/</home>`,
		`<h2 id="intro">Intro</h2>`, `<div class="table-wrap"><table>`, `</table></div>`,
	} {
		if !strings.Contains(en, want) {
			t.Errorf("index.html missing %q:\n%s", want, en)
		}
	}
	zh := readFile(t, filepath.Join(out, "zh-cn", "index.html"))
	for _, want := range []string{`<html lang="zh-CN">`, `<title>你好</title>`, `<a href="/">English</a>`, `<home>/zh-cn/</home>`} {
		if !strings.Contains(zh, want) {
			t.Errorf("zh-cn/index.html missing %q:\n%s", want, zh)
		}
	}
	guide := readFile(t, filepath.Join(out, "guide", "index.html"))
	if !strings.Contains(guide, "<title>Site</title>") || strings.Contains(guide, "<a href=") {
		t.Errorf("guide/index.html should fall back to the site title and have no alternates:\n%s", guide)
	}

	if got := readFile(t, filepath.Join(out, "style.css")); got != "body{}" {
		t.Errorf("style.css = %q", got)
	}
	if _, err := os.Stat(filepath.Join(out, "stale.html")); !os.IsNotExist(err) {
		t.Errorf("stale output not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, ".gitkeep")); err != nil {
		t.Errorf(".gitkeep removed: %v", err)
	}
}

func TestExpandCallouts(t *testing.T) {
	src := "intro\n\n:::tip\nhello\n:::\n\n:::caution[小心 <b>]\n```sh\n:::\n```\n:::\n"
	got, err := renderMarkdown([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<div class=\"callout tip\"><strong>Tip</strong>\n<p>hello</p>\n</div>",
		`<div class="callout caution"><strong>小心 &lt;b&gt;</strong>`,
		// ::: inside a code fence is content, not a closing marker.
		"<pre><code class=\"language-sh\">:::\n</code></pre>\n</div>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}

	for _, bad := range []string{":::tip\nnever closed\n", "stray\n:::\n", ":::tip\n:::note\n:::\n:::\n"} {
		if _, err := expandCallouts([]byte(bad)); err == nil {
			t.Errorf("expandCallouts(%q) = nil error", bad)
		}
	}
}

func TestLocaleURL(t *testing.T) {
	for _, tc := range []struct{ dir, name, want string }{
		{"", "index", "/"},
		{"zh-cn", "index", "/zh-cn/"},
		{"", "guide", "/guide/"},
		{"", "guide/index", "/guide/"},
		{"", "reindex", "/reindex/"},
		{"zh-cn", "a/b", "/zh-cn/a/b/"},
	} {
		if got := localeURL(tc.dir, tc.name); got != tc.want {
			t.Errorf("localeURL(%q, %q) = %q, want %q", tc.dir, tc.name, got, tc.want)
		}
	}
}
