// Package sitegen renders the Markdown docs in a site source directory into
// a static site:
//
//	<src>/config.json          site title, GitHub link, locales
//	<src>/layout.html          html/template for every page
//	<src>/content/**/*.md      pages; <locale dir>/<name>.md per language
//	<src>/static/**            copied to the output as-is
package sitegen

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Config is <src>/config.json.
type Config struct {
	Title  string `json:"title"`
	GitHub string `json:"github"`
	// Locales lists the site languages; the first is the default. A locale's
	// pages live under content/<Dir>/ ("" for the default at content/).
	Locales []Locale `json:"locales"`
}

type Locale struct {
	Dir   string `json:"dir"`
	Lang  string `json:"lang"`
	Label string `json:"label"`
}

// Link points at the same page in another locale.
type Link struct {
	Href  string
	Lang  string
	Label string
}

// Page is the data the layout template renders.
type Page struct {
	SiteTitle   string
	GitHub      string
	Title       string
	Description string
	Lang        string
	Home        string
	Alternates  []Link
	Content     template.HTML
}

// Build renders src into out. Everything in out except dotfiles (such as
// .gitkeep) is replaced.
func Build(src, out string) error {
	var cfg Config
	raw, err := os.ReadFile(filepath.Join(src, "config.json"))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("config.json: %w", err)
	}
	if len(cfg.Locales) == 0 {
		return errors.New("config.json: no locales")
	}
	layout, err := template.ParseFiles(filepath.Join(src, "layout.html"))
	if err != nil {
		return err
	}

	pages, err := collectPages(filepath.Join(src, "content"), cfg.Locales)
	if err != nil {
		return err
	}
	if err := cleanDir(out); err != nil {
		return err
	}
	if err := copyDir(filepath.Join(src, "static"), out); err != nil {
		return err
	}

	for _, p := range pages {
		source, err := os.ReadFile(p.file)
		if err != nil {
			return err
		}
		meta, body := splitFrontmatter(source)
		content, err := renderMarkdown(body)
		if err != nil {
			return fmt.Errorf("%s: %w", p.file, err)
		}
		page := Page{
			SiteTitle:   cfg.Title,
			GitHub:      cfg.GitHub,
			Title:       meta["title"],
			Description: meta["description"],
			Lang:        p.locale.Lang,
			Home:        localeURL(p.locale.Dir, "index"),
			Content:     template.HTML(content),
		}
		if page.Title == "" {
			page.Title = cfg.Title
		}
		for _, other := range pages {
			if other.name == p.name && other.locale != p.locale {
				page.Alternates = append(page.Alternates, Link{
					Href:  localeURL(other.locale.Dir, other.name),
					Lang:  other.locale.Lang,
					Label: other.locale.Label,
				})
			}
		}

		var buf bytes.Buffer
		if err := layout.Execute(&buf, page); err != nil {
			return fmt.Errorf("%s: %w", p.file, err)
		}
		dst := filepath.Join(out, filepath.FromSlash(outputPath(p.locale.Dir, p.name)))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	return nil
}

type sourcePage struct {
	file   string
	locale *Locale
	// name is the page path within its locale, without .md ("index",
	// "guide/setup").
	name string
}

func collectPages(contentDir string, locales []Locale) ([]sourcePage, error) {
	var pages []sourcePage
	err := filepath.WalkDir(contentDir, func(file string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(file) != ".md" {
			return err
		}
		rel, err := filepath.Rel(contentDir, file)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(strings.TrimSuffix(rel, ".md"))
		locale := matchLocale(rel, locales)
		name := strings.TrimPrefix(rel, locale.Dir+"/")
		if locale.Dir == "" {
			name = rel
		}
		pages = append(pages, sourcePage{file: file, locale: locale, name: name})
		return nil
	})
	if len(pages) == 0 && err == nil {
		err = fmt.Errorf("no Markdown pages in %s", contentDir)
	}
	return pages, err
}

// matchLocale picks the locale whose directory is the longest prefix of rel,
// falling back to the default (first) locale.
func matchLocale(rel string, locales []Locale) *Locale {
	best := &locales[0]
	for i := range locales {
		l := &locales[i]
		if l.Dir != "" && strings.HasPrefix(rel, l.Dir+"/") && len(l.Dir) > len(best.Dir) {
			best = l
		}
	}
	return best
}

// localeURL is the public URL of a page: index pages map to their directory,
// others to a directory of their own ("guide" -> /guide/).
func localeURL(dir, name string) string {
	if name == "index" {
		name = ""
	} else {
		name = strings.TrimSuffix(name, "/index")
	}
	u := path.Join("/", dir, name)
	if u != "/" {
		u += "/"
	}
	return u
}

func outputPath(dir, name string) string {
	return strings.TrimPrefix(path.Join(localeURL(dir, name), "index.html"), "/")
}

// splitFrontmatter separates a leading "---" block of "key: value" lines
// from the Markdown body.
func splitFrontmatter(src []byte) (map[string]string, []byte) {
	meta := map[string]string{}
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return meta, []byte(text)
	}
	block, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return meta, []byte(text)
	}
	for line := range strings.SplitSeq(block, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		meta[strings.TrimSpace(key)] = value
	}
	return meta, []byte(body)
}

// cleanDir empties dir (creating it if needed), keeping dotfiles.
func cleanDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(file string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) && file == src {
			return fs.SkipAll
		}
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, file)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
