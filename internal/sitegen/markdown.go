package sitegen

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

var md = goldmark.New(
	goldmark.WithExtensions(extension.Table, extension.Strikethrough),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(), // `## Title {#id}` overrides the generated id
		parser.WithAttribute(),
	),
	// Content is our own docs and callouts expand to raw HTML.
	goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
)

func renderMarkdown(src []byte) (string, error) {
	expanded, err := expandCallouts(src)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := md.Convert(expanded, &buf); err != nil {
		return "", err
	}
	// Wide tables scroll inside a wrapper instead of widening the page.
	out := strings.ReplaceAll(buf.String(), "<table>", `<div class="table-wrap"><table>`)
	out = strings.ReplaceAll(out, "</table>", "</table></div>")
	return out, nil
}

var (
	calloutOpen = regexp.MustCompile(`^:::([a-z]+)(?:\[(.*)\])?\s*$`)
	fenceOpen   = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
)

// expandCallouts turns container blocks into callout divs:
//
//	:::tip[Optional title]
//	Markdown content
//	:::
//
// The title defaults to the capitalized kind. Lines inside fenced code blocks
// are left alone. Callouts do not nest.
func expandCallouts(src []byte) ([]byte, error) {
	var out bytes.Buffer
	var fence string
	open := 0 // line number of the open callout, 0 if none
	for i, line := range strings.Split(string(src), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case fence != "":
			if strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]) == "" {
				fence = ""
			}
		case fenceOpen.MatchString(line):
			fence = fenceOpen.FindStringSubmatch(line)[1]
		case trimmed == ":::":
			if open == 0 {
				return nil, fmt.Errorf("line %d: ::: without an open callout", i+1)
			}
			open = 0
			out.WriteString("\n</div>\n\n")
			continue
		case calloutOpen.MatchString(trimmed):
			if open != 0 {
				return nil, fmt.Errorf("line %d: callout opened inside the callout from line %d", i+1, open)
			}
			m := calloutOpen.FindStringSubmatch(trimmed)
			kind, title := m[1], m[2]
			if title == "" {
				title = strings.ToUpper(kind[:1]) + kind[1:]
			}
			open = i + 1
			fmt.Fprintf(&out, "<div class=\"callout %s\"><strong>%s</strong>\n\n", kind, html.EscapeString(title))
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	if open != 0 {
		return nil, fmt.Errorf("line %d: callout is never closed", open)
	}
	return out.Bytes(), nil
}
