package mcp

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// mdConverter renders GitHub-Flavored Markdown to HTML. WithUnsafe keeps any
// raw HTML the author embedded (KB notes are trusted internal documentation),
// so callers can freely mix Markdown and HTML.
var mdConverter = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

// markdownToHTML converts a Markdown KB note/article body into HTML suitable for
// the ITPortal "article" field. Blank input is returned unchanged; on conversion
// error the original input is returned so content is never silently lost.
func markdownToHTML(md string) string {
	if strings.TrimSpace(md) == "" {
		return md
	}
	var buf bytes.Buffer
	if err := mdConverter.Convert([]byte(md), &buf); err != nil {
		return md
	}
	return buf.String()
}

// resolveKBArticleField normalises the KB note body in an update_entity fields
// map. An "article_markdown" pseudo-field is converted to HTML and stored under
// the real API field "article" (and removed). A plain "article" value is left
// untouched (treated as HTML). article_markdown takes precedence over article.
func resolveKBArticleField(fields map[string]interface{}) {
	raw, ok := fields["article_markdown"]
	if !ok {
		return
	}
	delete(fields, "article_markdown")
	md, _ := raw.(string)
	fields["article"] = markdownToHTML(md)
}
